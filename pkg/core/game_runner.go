package core

import (
	"fmt"
	"io"
	"sync"
)

type gameRunner struct {
	auth        Authenticator
	profiles    ProfileManager
	dataPath    string
	
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

	profiles, err := r.profiles.GetProfiles()
	if err != nil {
		return err
	}

	var targetProfile *Profile
	for _, p := range profiles {
		if p.Name == profileName {
			targetProfile = &p
			break
		}
	}

	if targetProfile == nil {
		return fmt.Errorf("profile not found: %s", profileName)
	}

	// We need to convert core.Profile back to resource.Profile or similar
	// This shows that our core.Profile needs to hold enough info for launching
	// or we need a way to get the original resource.Profile.
	
	return fmt.Errorf("launch implementation in progress")
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
