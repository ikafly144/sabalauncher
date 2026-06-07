package fyne

import (
	"io"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/v2/pkg/core"
	"github.com/ikafly144/sabalauncher/v2/pkg/msa"
	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
	"github.com/stretchr/testify/mock"
)

type mockGameRunner struct {
	mock.Mock
}

func (m *mockGameRunner) Launch(instanceID uuid.UUID) error {
	args := m.Called(instanceID)
	return args.Error(0)
}

func (m *mockGameRunner) SubscribeProgress() <-chan core.ProgressEvent {
	args := m.Called()
	return args.Get(0).(<-chan core.ProgressEvent)
}

func (m *mockGameRunner) GetLogReader() (io.ReadCloser, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *mockGameRunner) IsRunning() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockGameRunner) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func TestShowDashboardView(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")

	mp := new(mockInstanceManager)
	mp.On("GetInstances").Return([]*resource.Instance{
		{Name: "Instance 1", Versions: []resource.InstanceVersion{{ID: "minecraft", Version: "1.20.1"}}},
	}, nil)
	mp.On("SubscribeProgress").Return(make(chan core.ProgressEvent))

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
		config:    core.DefaultConfig(),
	}

	ui.showDashboardView()

	if ui.window.Content() == nil {
		t.Fatal("Window content is nil")
	}
}

func TestShowLaunchOverlay(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")

	pChan := make(chan core.ProgressEvent, 1)
	mr := new(mockGameRunner)
	mr.On("SubscribeProgress").Return((<-chan core.ProgressEvent)(pChan))
	mr.On("GetLogReader").Return(nil, nil)
	mr.On("IsRunning").Return(true)
	mr.On("Stop").Return(nil)

	ui := &FyneUI{
		app:     a,
		window:  w,
		runner:  mr,
		discord: new(mockDiscordManager),
		config:  core.DefaultConfig(),
	}

	ui.showLaunchOverlay()

	// Trigger log reader by sending finished progress
	pChan <- core.ProgressEvent{IsFinished: true}

	if ui.window.Content() == nil {
		t.Fatal("Window content is nil")
	}
}
