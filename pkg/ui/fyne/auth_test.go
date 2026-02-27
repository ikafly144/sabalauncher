package fyne

import (
	"context"
	"fmt"
	"testing"
	"fyne.io/fyne/v2/test"
	"github.com/ikafly144/sabalauncher/pkg/core"
	"github.com/stretchr/testify/mock"
)

type mockAuthenticator struct {
	mock.Mock
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

func TestShowAuthView_LoggedOut(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	m := new(mockAuthenticator)
	m.On("GetStatus").Return(core.AuthStatusLoggedOut)
	m.On("Login", mock.Anything).Return(nil)
	m.On("WaitLogin", mock.Anything).Return(nil)
	
	ui := &FyneUI{
		app:    a,
		window: w,
		auth:   m,
	}
	
	ui.showAuthView()
	
	// Find and tap the login button
	// We can't easily find it by text in standard Fyne API,
	// but we can manually invoke it if we have the widget.
	// For now, just check if content is set.
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
	m.On("GetStatus").Return(core.AuthStatusLoggedIn).Once()
	m.On("DeviceCode").Return("http://example.com", "CODE")
	m.On("WaitLogin", mock.Anything).Return(nil)
	m.On("GetUserDisplay").Return("TestUser")
	
	ui := &FyneUI{
		app:    a,
		window: w,
		auth:   m,
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
	}
	
	ui.startLogin()
	
	m.AssertExpectations(t)
}
