package core

import (
	"context"
	"io"

	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/v2/pkg/msa"
	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
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

// InstanceManager defines the interface for managing game instances.
type InstanceManager interface {
	// GetInstances returns the list of all available instances.
	GetInstances() ([]*resource.Instance, error)
	// DeleteInstance removes an instance by its name or UID.
	DeleteInstance(instanceID uuid.UUID) error
	// RefreshInstances updates all instances from local storage.
	RefreshInstances() error
	// GetInstance returns a specific instance.
	GetInstance(id uuid.UUID) (*resource.Instance, error)
	// ImportInstance imports a modpack from an .sbpack file.
	ImportInstance(ctx context.Context, packPath string) error
	// AddRemoteInstance registers a remote modpack repository.
	AddRemoteInstance(ctx context.Context, manifestURL string) error
	// UpdateInstance updates an instance using an .sbpatch file.
	UpdateInstance(ctx context.Context, instanceID uuid.UUID, patchPath string) error
	// CheckUpdate checks if a remote update is available for the instance.
	CheckUpdate(ctx context.Context, instanceID uuid.UUID) (bool, error)
	// RepairInstance verifies and repairs instance files.
	RepairInstance(ctx context.Context, instanceID uuid.UUID) error
	// SubscribeProgress returns a channel that receives progress updates.
	SubscribeProgress() <-chan ProgressEvent
}

// LaunchOptions contains options for game launch.
type LaunchOptions struct {
	QuickPlayMultiplayer  string
	QuickPlaySingleplayer string
}

// GameRunner defines the interface for launching and managing the game process.
type GameRunner interface {
	// Launch starts the game with the specified instance.
	Launch(instanceID uuid.UUID, options *LaunchOptions) error
	// Stop terminates the running game process.
	Stop() error
	// IsRunning returns true if the game is currently active.
	IsRunning() bool
	// SubscribeProgress returns a channel that receives progress updates.
	SubscribeProgress() <-chan ProgressEvent
	// GetLogReader returns an io.ReadCloser to read game logs.
	GetLogReader() (io.ReadCloser, error)
}

// DiscordManager defines the interface for managing Discord Rich Presence.
type DiscordManager interface {
	SetActivity(instanceID uuid.UUID) error
	ClearActivity() error
}
