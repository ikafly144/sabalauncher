package fyne

import (
	"context"
	"fmt"
	"testing"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/core"
	"github.com/ikafly144/sabalauncher/pkg/msa"
	"github.com/stretchr/testify/mock"
)

type mockAuthenticator struct {
	mock.Mock
}

func (m *mockAuthenticator) TrySilentLogin(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockAuthenticator) Login(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockAuthenticator) Logout() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockAuthenticator) GetStatus() core.AuthStatus {
	args := m.Called()
	return args.Get(0).(core.AuthStatus)
}

func (m *mockAuthenticator) GetUserDisplay() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockAuthenticator) DeviceCode() (string, string) {
	args := m.Called()
	return args.String(0), args.String(1)
}

func (m *mockAuthenticator) WaitLogin(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockAuthenticator) GetMinecraftProfile() (*msa.MinecraftProfile, error) {
	args := m.Called()
	return args.Get(0).(*msa.MinecraftProfile), args.Error(1)
}

func (m *mockAuthenticator) GetMinecraftAccount() (*msa.MinecraftAccount, error) {
	args := m.Called()
	return args.Get(0).(*msa.MinecraftAccount), args.Error(1)
}

type mockDiscordManager struct {
	mock.Mock
}

func (m *mockDiscordManager) SetActivity(profileName string) error {
	args := m.Called(profileName)
	return args.Error(0)
}

func (m *mockDiscordManager) ClearActivity() error {
	args := m.Called()
	return args.Error(0)
}

func TestShowAuthView_LoggedOut_Login(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	m := new(mockAuthenticator)
	m.On("GetStatus").Return(core.AuthStatusLoggedOut)
	m.On("Login", mock.Anything).Return(nil)
	m.On("WaitLogin", mock.Anything).Return(nil)
	
	mp := new(mockProfileManager)
	mp.On("GetProfiles").Return([]core.Profile{}, nil)

	ui := &FyneUI{
		app:      a,
		window:   w,
		auth:     m,
		profiles: mp,
		discord:  new(mockDiscordManager),
	}
	
	view := ui.createLoggedOutView()
	container := view.(*fyne.Container)
	for _, obj := range container.Objects {
		if btn, ok := obj.(*widget.Button); ok {
			btn.OnTapped()
		}
	}
}

func TestShowAuthView_LoggedIn_Buttons(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	m := new(mockAuthenticator)
	m.On("GetStatus").Return(core.AuthStatusLoggedIn)
	m.On("GetUserDisplay").Return("TestUser")
	m.On("Logout").Return(nil)
	
	ui := &FyneUI{
		app:      a,
		window:   w,
		auth:     m,
		profiles: new(mockProfileManager),
		discord:  new(mockDiscordManager),
	}
	ui.profiles.(*mockProfileManager).On("GetProfiles").Return([]core.Profile{}, nil)
	
	view := ui.createLoggedInView()
	container := view.(*fyne.Container)
	for _, obj := range container.Objects {
		if btn, ok := obj.(*widget.Button); ok {
			btn.OnTapped()
		}
	}
}

func TestShowAuthView_Error_Retry(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	m := new(mockAuthenticator)
	m.On("GetStatus").Return(core.AuthStatusError)
	m.On("Login", mock.Anything).Return(nil)
	m.On("WaitLogin", mock.Anything).Return(nil)
	
	mp := new(mockProfileManager)
	mp.On("GetProfiles").Return([]core.Profile{}, nil)

	ui := &FyneUI{
		app:      a,
		window:   w,
		auth:     m,
		profiles: mp,
		discord:  new(mockDiscordManager),
	}
	
	view := ui.createErrorView()
	container := view.(*fyne.Container)
	for _, obj := range container.Objects {
		if btn, ok := obj.(*widget.Button); ok {
			btn.OnTapped()
		}
	}
}

func TestShowAuthView_LoggedIn_Logout(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	m := new(mockAuthenticator)
	m.On("GetStatus").Return(core.AuthStatusLoggedIn).Once()
	m.On("GetUserDisplay").Return("TestUser")
	m.On("Logout").Return(nil)
	m.On("GetStatus").Return(core.AuthStatusLoggedOut).Once()
	
	ui := &FyneUI{
		app:    a,
		window: w,
		auth:   m,
		discord: new(mockDiscordManager),
	}
	
	ui.showAuthView()
	
	// Manual logout call to verify logic
	_ = ui.auth.Logout()
	ui.showAuthView()
	
	m.AssertExpectations(t)
}

func TestShowAuthView_Error(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	m := new(mockAuthenticator)
	m.On("GetStatus").Return(core.AuthStatusError)
	
	ui := &FyneUI{
		app:    a,
		window: w,
		auth:   m,
		discord: new(mockDiscordManager),
	}
	
	ui.showAuthView()
	
	if ui.window.Content() == nil {
		t.Fatal("Window content is nil")
	}
}

func TestStartLogin(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	m := new(mockAuthenticator)
	m.On("Login", mock.Anything).Return(nil)
	m.On("GetStatus").Return(core.AuthStatusLoggingIn).Once()
	m.On("DeviceCode").Return("http://example.com", "CODE")
	m.On("WaitLogin", mock.Anything).Return(nil)
	
	mp := new(mockProfileManager)
	mp.On("GetProfiles").Return([]core.Profile{}, nil)

	ui := &FyneUI{
		app:      a,
		window:   w,
		auth:     m,
		profiles: mp,
		discord:  new(mockDiscordManager),
	}
	
	ui.startLogin()
	
	m.AssertExpectations(t)
}

func TestStartLogin_Fail(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	m := new(mockAuthenticator)
	m.On("Login", mock.Anything).Return(fmt.Errorf("fail"))
	m.On("GetStatus").Return(core.AuthStatusError)
	
	ui := &FyneUI{
		app:    a,
		window: w,
		auth:   m,
		discord: new(mockDiscordManager),
	}
	
	ui.startLogin()
	
	m.AssertExpectations(t)
}

func TestStartLogin_WaitFail(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	m := new(mockAuthenticator)
	m.On("Login", mock.Anything).Return(nil)
	m.On("GetStatus").Return(core.AuthStatusLoggingIn).Once()
	m.On("DeviceCode").Return("http://example.com", "CODE")
	m.On("WaitLogin", mock.Anything).Return(fmt.Errorf("fail"))
	m.On("GetStatus").Return(core.AuthStatusError).Once()
	
	ui := &FyneUI{
		app:    a,
		window: w,
		auth:   m,
		discord: new(mockDiscordManager),
	}
	
	ui.startLogin()
	
	m.AssertExpectations(t)
}
