package core

import (
	"context"
	"image"
)

// AuthStatus represents the current state of authentication.
type AuthStatus int

const (
	AuthStatusLoggedOut AuthStatus = iota
	AuthStatusLoggingIn
	AuthStatusLoggedIn
	AuthStatusError
)

// Authenticator defines the interface for handling Minecraft/Microsoft authentication.
type Authenticator interface {
	// Login starts the interactive login process.
	Login(ctx context.Context) error
	// Logout clears the current session.
	Logout() error
	// GetStatus returns the current authentication status.
	GetStatus() AuthStatus
	// GetUserDisplay returns the name of the logged-in user.
	GetUserDisplay() string
	// DeviceCode returns the device code information for the user to login.
	// This should only be called when status is AuthStatusLoggingIn.
	DeviceCode() (url, code string)
	// WaitLogin blocks until the authentication process is complete or the context is cancelled.
	WaitLogin(ctx context.Context) error
}

// Profile represents a game configuration.
type Profile struct {
	Name        string
	DisplayName string
	Description string
	IconImage   image.Image
	IsActive    bool
	Source      string
}

// ProfileManager defines the interface for managing game profiles.
type ProfileManager interface {
	// GetProfiles returns the list of all available profiles.
	GetProfiles() ([]Profile, error)
	// AddProfile fetches and adds a new profile from the given URL.
	AddProfile(url string) error
	// DeleteProfile removes a profile by its name.
	DeleteProfile(name string) error
	// RefreshProfiles updates all profiles from their sources.
	RefreshProfiles() error
}

// GameRunner defines the interface for launching and managing the game process.
type GameRunner interface {
	// Launch starts the game with the specified profile.
	Launch(profileName string) error
	// Stop terminates the running game process.
	Stop() error
	// IsRunning returns true if the game is currently active.
	IsRunning() bool
	// SubscribeProgress returns a channel that receives progress updates.
	SubscribeProgress() <-chan ProgressEvent
	// SubscribeLogs returns a channel that receives log entries.
	SubscribeLogs() <-chan LogEntry
}
