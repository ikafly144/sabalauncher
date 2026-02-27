package core

import (
	"time"
)

// ProgressEvent represents a progress update from a background task (e.g., downloading).
type ProgressEvent struct {
	TaskName   string  // e.g., "Downloading Forge"
	Percentage float64 // 0.0 to 100.0
	Status     string  // e.g., "45 MB / 100 MB"
	Total      int
	Current    int
	IsFinished bool
}

// LogLevel represents the severity of a log entry.
type LogLevel string

const (
	LogLevelInfo    LogLevel = "INFO"
	LogLevelWarning LogLevel = "WARN"
	LogLevelError   LogLevel = "ERROR"
	LogLevelDebug   LogLevel = "DEBUG"
)

// LogEntry represents a single log message from the launcher or game.
type LogEntry struct {
	Timestamp time.Time
	Level     LogLevel
	Source    string // e.g., "Launcher", "Game", "Downloader"
	Message   string
}

// EventBus provides a way to subscribe to launcher events.
// This facilitates the Observer pattern mentioned in the spec.
type EventBus interface {
	SubscribeProgress() <-chan ProgressEvent
	SubscribeLogs() <-chan LogEntry
}
