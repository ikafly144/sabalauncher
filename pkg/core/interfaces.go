package core

import (
	"context"
	"github.com/ikafly144/sabalauncher/pkg/msa"
	"github.com/ikafly144/sabalauncher/pkg/resource"
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
	// TrySilentLogin attempts to login using cached credentials.
	TrySilentLogin(ctx context.Context) error
	// Login starts the interactive login process.
	Login(ctx context.Context, method msa.LoginMethod) error // Logout clears the current session.
	Logout() error
	// GetStatus returns the current authentication status.
	GetStatus() AuthStatus
	// GetUserDisplay returns the name of the logged-in user.
	GetUserDisplay() string
	// GetMinecraftProfile returns the Minecraft profile of the logged-in user.
	GetMinecraftProfile() (*msa.MinecraftProfile, error)
	// GetMinecraftAccount returns the raw Minecraft account object.
	GetMinecraftAccount() (*msa.MinecraftAccount, error)
	// DeviceCode returns the device code information for the user to login.
	// This should only be called when status is AuthStatusLoggingIn.
	DeviceCode() (url, code string)
	// LoginURL returns the URL to open in a browser for the current login process.
	LoginURL() string
	// WaitLogin blocks until the authentication process is complete or the context is cancelled.
	WaitLogin(ctx context.Context) error
	// GetLastError returns the last error that occurred during authentication.
	GetLastError() error
}

// Profile represents a game configuration.
type Profile struct {
	Name        string
	DisplayName string
	Description string
	VersionName string
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
	// GetFullProfile returns the underlying resource.Profile.
	// This is used for launch logic.
	GetFullProfile(name string) (*resource.Profile, error)
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

// DiscordManager defines the interface for managing Discord Rich Presence.
type DiscordManager interface {
	SetActivity(profileName string) error
	ClearActivity() error
}
