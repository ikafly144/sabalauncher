package core

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ikafly144/sabalauncher/v2/pkg/i18n"
	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
)

type gameRunner struct {
	auth      Authenticator
	instances InstanceManager
	dataPath  string

	progressChan chan ProgressEvent
	logsChan     chan LogEntry

	running bool
	cancel  context.CancelFunc
	mu      sync.RWMutex
}

func NewGameRunner(auth Authenticator, instances InstanceManager, dataDir string) GameRunner {
	return &gameRunner{
		auth:         auth,
		instances:    instances,
		dataPath:     dataDir,
		progressChan: make(chan ProgressEvent, 100),
		logsChan:     make(chan LogEntry, 1000),
	}
}

func (r *gameRunner) Launch(instanceName string) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("game is already running")
	}
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

	inst, err := r.instances.GetInstance(instanceName)
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

	stdout := &logWriter{source: "Game", ch: r.logsChan}
	stderr := &logWriter{source: "Game", ch: r.logsChan}

	// 3. Boot Game using ModLoader and LaunchConfig
	loader, err := resource.GetModLoader(inst)
	if err != nil {
		return fmt.Errorf("failed to get mod loader: %w", err)
	}

	config, err := loader.GenerateLaunchConfig(inst)
	if err != nil {
		return fmt.Errorf("failed to generate launch config: %w", err)
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

	if err := resource.BootGameFromConfig(ctx, javaPath, config, manifest, inst, mcAccount, stdout, stderr); err != nil {
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

func (r *gameRunner) SubscribeLogs() <-chan LogEntry {
	return r.logsChan
}

type logWriter struct {
	source string
	ch     chan<- LogEntry
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.ch <- LogEntry{
		Source:  w.source,
		Message: string(p),
	}
	return len(p), nil
}

var _ io.Writer = (*logWriter)(nil)
