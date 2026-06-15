package core

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/v2/pkg/i18n"
	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
)

type gameRunner struct {
	auth      Authenticator
	instances InstanceManager
	dataPath  string
	config    *LauncherConfig

	progressChan     chan ProgressEvent
	notificationChan chan NotificationEvent
	logFile          *os.File

	running bool
	cancel  context.CancelFunc
	mu      sync.RWMutex
}

func NewGameRunner(auth Authenticator, instances InstanceManager, dataDir string, config *LauncherConfig) GameRunner {
	return &gameRunner{
		auth:             auth,
		instances:        instances,
		dataPath:         dataDir,
		config:           config,
		progressChan:     make(chan ProgressEvent, 100),
		notificationChan: make(chan NotificationEvent, 100),
	}
}

func (r *gameRunner) Launch(instanceID uuid.UUID, options *LaunchOptions) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("game is already running")
	}

	// Create temporary log file
	logPath := filepath.Join(os.TempDir(), fmt.Sprintf("saba-game-%s.log", time.Now().Format("20060102-150405")))
	f, err := os.Create(logPath)
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("failed to create log file: %w", err)
	}
	r.logFile = f

	ctx, cancel := context.WithCancel(context.Background())
	r.running = true
	r.cancel = cancel
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.running = false
		r.cancel = nil
		r.mu.Unlock()
	}()

	inst, err := r.instances.GetInstance(instanceID)
	if err != nil {
		return err
	}

	account, err := r.auth.GetMinecraftAccount()
	if err != nil {
		return err
	}

	// 1. Start Setup
	state := resource.SetupInstance(r.dataPath, inst)

	// 2. Monitor Progress
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for !state.IsDone() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			r.progressChan <- ProgressEvent{
				TaskName:   state.FriendlyName(),
				Percentage: float64(state.Progress()) * 100.0,
				Status:     fmt.Sprintf("%.1f%%", state.CurrentProgress()*100.0),
				IsFinished: false,
			}
		}
	}

	if err := state.Error(); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	r.progressChan <- ProgressEvent{
		TaskName:   i18n.T("starting_game"),
		Percentage: 100.0,
		Status:     "Done",
		IsFinished: true,
	}

	// 3. Boot Game using ModLoader and LaunchConfig
	loader, err := resource.GetModLoader(inst)
	if err != nil {
		return fmt.Errorf("failed to get mod loader: %w", err)
	}

	features := map[string]bool{
		"is_demo_user":          false,
		"has_custom_resolution": true,
	}

	if options != nil {
		if options.QuickPlayMultiplayer != "" {
			features["is_quick_play_multiplayer"] = true
		}
		if options.QuickPlaySingleplayer != "" {
			features["is_quick_play_singleplayer"] = true
		}
	}

	maxMemory := r.config.MaxMemory
	if options != nil && options.MemoryMB > 0 {
		maxMemory = options.MemoryMB
	} else if inst.Properties.Memory > 0 {
		if uint64(inst.Properties.Memory) > maxMemory {
			maxMemory = uint64(inst.Properties.Memory)
		}
	}

	config, err := loader.GenerateLaunchConfig(inst, features, maxMemory)
	if err != nil {
		return fmt.Errorf("failed to generate launch config: %w", err)
	}

	// Handle quick launch:
	// 1. Replace placeholders if they exist (modern versions)
	quickPlayHandled := false
	for i := range config.GameArguments {
		if options != nil && options.QuickPlayMultiplayer != "" {
			if strings.Contains(config.GameArguments[i], "${quickPlayMultiplayer}") {
				config.GameArguments[i] = strings.ReplaceAll(config.GameArguments[i], "${quickPlayMultiplayer}", options.QuickPlayMultiplayer)
				quickPlayHandled = true
			}
		}
		if options != nil && options.QuickPlaySingleplayer != "" {
			if strings.Contains(config.GameArguments[i], "${quickPlaySingleplayer}") {
				config.GameArguments[i] = strings.ReplaceAll(config.GameArguments[i], "${quickPlaySingleplayer}", options.QuickPlaySingleplayer)
				quickPlayHandled = true
			}
		}
	}

	// 2. If not handled by placeholders, append flags manually (fallback/legacy)
	if options != nil && !quickPlayHandled {
		if options.QuickPlayMultiplayer != "" {
			config.GameArguments = append(config.GameArguments, "--server", options.QuickPlayMultiplayer)
		} else if options.QuickPlaySingleplayer != "" {
			config.GameArguments = append(config.GameArguments, "--quickPlaySingleplayer", options.QuickPlaySingleplayer)
		}
	}

	manifest, err := resource.GetClientManifestForInstance(inst)
	if err != nil {
		return fmt.Errorf("failed to get client manifest: %w", err)
	}

	javaPath, err := resource.GetJavaExecutablePath(manifest.JavaVersion.Component, "C:\\")
	if err != nil {
		return fmt.Errorf("failed to get java executable path: %w", err)
	}

	mcAccount, err := account.GetMinecraftAccount()
	if err != nil {
		return fmt.Errorf("failed to get minecraft account: %w", err)
	}

	profile, err := mcAccount.GetMinecraftProfile()
	if err != nil {
		return fmt.Errorf("failed to get minecraft profile: %w", err)
	}

	start := time.Now()

	// Start playtime monitor
	monitorCtx, monitorCancel := context.WithCancel(ctx)
	defer monitorCancel()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		lastMilestoneHours := inst.PlayTimeSeconds / 3600
		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				currentTotalSeconds := inst.PlayTimeSeconds + int64(time.Since(start).Seconds())
				currentHours := currentTotalSeconds / 3600
				if currentHours > lastMilestoneHours {
					lastMilestoneHours = currentHours
					r.notificationChan <- NotificationEvent{
						Title:    i18n.T("playtime_milestone_title"),
						Message:  i18n.T("playtime_milestone_msg", currentHours),
						Duration: 5 * time.Second,
					}
				}
			}
		}
	}()

	err = resource.BootGameFromConfig(ctx, javaPath, config, manifest, inst, profile, mcAccount.AccessToken, r.logFile, r.logFile)
	duration := time.Since(start)

	// Update playtime
	inst.PlayTimeSeconds += int64(duration.Seconds())
	if saveErr := r.instances.SaveInstance(inst); saveErr != nil {
		slog.Error("Failed to save instance playtime", "error", saveErr)
	}

	if err != nil {
		return fmt.Errorf("boot failed: %w", err)
	}

	return nil
}

func (r *gameRunner) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.running || r.cancel == nil {
		return nil
	}
	r.cancel()
	if r.logFile != nil {
		r.logFile.Close()
	}
	return nil
}

func (r *gameRunner) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

func (r *gameRunner) SubscribeProgress() <-chan ProgressEvent {
	return r.progressChan
}

func (r *gameRunner) SubscribeNotifications() <-chan NotificationEvent {
	return r.notificationChan
}

func (r *gameRunner) GetLogReader() (io.ReadCloser, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.logFile == nil {
		return nil, fmt.Errorf("no log file available")
	}
	return os.Open(r.logFile.Name())
}
