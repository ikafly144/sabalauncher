package core

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ikafly144/sabalauncher/pkg/resource"
)

type gameRunner struct {
	auth     Authenticator
	profiles ProfileManager
	dataPath string

	progressChan chan ProgressEvent
	logsChan     chan LogEntry

	running bool
	mu      sync.RWMutex
}

func NewGameRunner(auth Authenticator, profiles ProfileManager, dataDir string) GameRunner {
	return &gameRunner{
		auth:         auth,
		profiles:     profiles,
		dataPath:     dataDir,
		progressChan: make(chan ProgressEvent, 100),
		logsChan:     make(chan LogEntry, 1000),
	}
}

func (r *gameRunner) Launch(profileName string) error {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return fmt.Errorf("game is already running")
	}
	r.running = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()

	fullProfile, err := r.profiles.GetFullProfile(profileName)
	if err != nil {
		return err
	}

	account, err := r.auth.GetMinecraftAccount()
	if err != nil {
		return err
	}

	// 1. Start Setup
	fullProfile.Manifest.StartSetup(r.dataPath, fullProfile.Path)

	// 2. Monitor Progress
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for !fullProfile.Manifest.IsDone() {
		select {
		case <-ticker.C:
			r.progressChan <- ProgressEvent{
				TaskName:   fullProfile.Manifest.CurrentStatus(),
				Percentage: fullProfile.Manifest.TotalProgress() * 100.0,
				Status:     fmt.Sprintf("%.1f%%", fullProfile.Manifest.CurrentProgress()*100.0),
				IsFinished: false,
			}
		}
	}

	if err := fullProfile.Manifest.Error(); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	r.progressChan <- ProgressEvent{
		TaskName:   "Starting Game...",
		Percentage: 100.0,
		Status:     "Done",
		IsFinished: true,
	}

	stdout := &logWriter{source: "Game", ch: r.logsChan}
	stderr := &logWriter{source: "Game", ch: r.logsChan}

	// 3. Boot Game using ModLoader and LaunchConfig
	loader, err := resource.GetModLoader(fullProfile)
	if err != nil {
		return fmt.Errorf("failed to get mod loader: %w", err)
	}

	config, err := loader.GenerateLaunchConfig(fullProfile)
	if err != nil {
		return fmt.Errorf("failed to generate launch config: %w", err)
	}

	manifest := fullProfile.Manifest.GetClientManifest()
	if manifest == nil {
		return fmt.Errorf("client manifest is missing")
	}

	javaPath, err := resource.GetJavaExecutablePath(manifest.JavaVersion.Component, "C:\\")
	if err != nil {
		return fmt.Errorf("failed to get java executable path: %w", err)
	}

	mcAccount, err := account.GetMinecraftAccount()
	if err != nil {
		return fmt.Errorf("failed to get minecraft account: %w", err)
	}

	if err := resource.BootGameFromConfig(javaPath, config, manifest, fullProfile, mcAccount, stdout, stderr); err != nil {
		return fmt.Errorf("boot failed: %w", err)
	}

	return nil
}

func (r *gameRunner) Stop() error {
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
