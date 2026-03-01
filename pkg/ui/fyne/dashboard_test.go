package fyne

import (
	"fyne.io/fyne/v2/test"
	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/pkg/core"
	"github.com/ikafly144/sabalauncher/pkg/msa"
	"github.com/ikafly144/sabalauncher/pkg/resource"
	"github.com/stretchr/testify/mock"
	"testing"
)

type mockGameRunner struct {
	mock.Mock
}

func (m *mockGameRunner) Launch(profileName string) error {
	args := m.Called(profileName)
	return args.Error(0)
}

func (m *mockGameRunner) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockGameRunner) IsRunning() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockGameRunner) SubscribeProgress() <-chan core.ProgressEvent {
	args := m.Called()
	return args.Get(0).(<-chan core.ProgressEvent)
}

func (m *mockGameRunner) SubscribeLogs() <-chan core.LogEntry {
	args := m.Called()
	return args.Get(0).(<-chan core.LogEntry)
}

func TestShowDashboardView(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")

	mp := new(mockInstanceManager)
	mp.On("GetInstances").Return([]*resource.Instance{
		{Name: "test"},
	}, nil)

	mr := new(mockGameRunner)
	ma := new(mockAuthenticator)
	ma.On("GetStatus").Return(core.AuthStatusLoggedIn)
	ma.On("GetUserDisplay").Return("TestUser")
	ma.On("GetMinecraftProfile").Return(&msa.MinecraftProfile{Username: "TestUser", UUID: uuid.New()}, nil).Maybe()

	ui := &FyneUI{
		app:       a,
		window:    w,
		instances: mp,
		runner:    mr,
		auth:      ma,
		discord:   new(mockDiscordManager),
	}

	ui.showDashboardView()

	if ui.window.Content() == nil {
		t.Fatal("Window content is nil")
	}
}

func TestShowLaunchOverlay(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")

	mr := new(mockGameRunner)
	mr.On("SubscribeProgress").Return(make(<-chan core.ProgressEvent))
	mr.On("SubscribeLogs").Return(make(<-chan core.LogEntry))
	mr.On("IsRunning").Return(true)
	mr.On("Stop").Return(nil)

	ui := &FyneUI{
		app:     a,
		window:  w,
		runner:  mr,
		discord: new(mockDiscordManager),
	}

	ui.showLaunchOverlay()

	if ui.window.Content() == nil {
		t.Fatal("Window content is nil")
	}
}
