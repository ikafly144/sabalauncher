package core

import (
	"context"
	"fmt"
	"sync"

	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
	"github.com/ikafly144/sabalauncher/pkg/msa"
)

type msaAuthenticator struct {
	client    public.Client
	session   msa.Session
	status    AuthStatus
	user      string
	mcProfile *msa.MinecraftProfile
	mu        sync.RWMutex
}

func NewAuthenticator(cachePath string) (Authenticator, error) {
	cache, err := msa.NewCacheAccessor(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create cache accessor: %w", err)
	}
	
	// We need the client ID from the msa package's internal state or redefined here.
	// For now, we'll use a placeholder or try to extract it if possible.
	// The current msa package doesn't export NewSession with a custom client, 
	// so we might need to refactor msa package as well.
	
	sess, err := msa.NewSession(cache)
	if err != nil {
		return nil, fmt.Errorf("failed to create msa session: %w", err)
	}

	return &msaAuthenticator{
		session: sess,
		status:  AuthStatusLoggedOut,
	}, nil
}

func (a *msaAuthenticator) Login(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = AuthStatusLoggingIn

	if err := a.session.StartLogin(); err != nil {
		a.status = AuthStatusError
		return err
	}
	return nil
}

func (a *msaAuthenticator) WaitLogin(ctx context.Context) error {
	// This blocks until the device code flow is complete or fails.
	_, err := a.session.AuthResult()
	if err != nil {
		a.mu.Lock()
		a.status = AuthStatusError
		a.mu.Unlock()
		return err
	}

	// Once logged in to MS, we need to get the Minecraft account.
	account, err := msa.NewMinecraftAccount(a.session)
	if err != nil {
		a.mu.Lock()
		a.status = AuthStatusError
		a.mu.Unlock()
		return err
	}

	mcAuth, err := account.GetMinecraftAccount()
	if err != nil {
		a.mu.Lock()
		a.status = AuthStatusError
		a.mu.Unlock()
		return err
	}

	profile, err := mcAuth.GetMinecraftProfile()
	if err != nil {
		a.mu.Lock()
		a.status = AuthStatusError
		a.mu.Unlock()
		return err
	}

	a.mu.Lock()
	a.user = profile.Username
	a.mcProfile = profile
	a.status = AuthStatusLoggedIn
	a.mu.Unlock()

	return nil
}

func (a *msaAuthenticator) GetMinecraftProfile() (*msa.MinecraftProfile, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.mcProfile == nil {
		return nil, fmt.Errorf("no minecraft profile available")
	}
	return a.mcProfile, nil
}

func (a *msaAuthenticator) Logout() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	// MSA library handles token removal via client.RemoveAccount, 
	// but we'd need to expose that from the msa package.
	a.user = ""
	a.mcProfile = nil
	a.status = AuthStatusLoggedOut
	return nil
}

func (a *msaAuthenticator) GetStatus() AuthStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

func (a *msaAuthenticator) GetUserDisplay() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.user
}

func (a *msaAuthenticator) DeviceCode() (url, code string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.session == nil || a.session.DeviceCode() == nil {
		return "", ""
	}
	dc := a.session.DeviceCode()
	return dc.Result.VerificationURL, dc.Result.UserCode
}
