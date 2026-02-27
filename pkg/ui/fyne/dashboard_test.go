package fyne

import (
	"testing"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/ikafly144/sabalauncher/pkg/core"
	"github.com/stretchr/testify/mock"
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
	
	mp := new(mockProfileManager)
	mp.On("GetProfiles").Return([]core.Profile{
		{Name: "test", DisplayName: "Test Profile"},
	}, nil)
	
	mr := new(mockGameRunner)
	
	ui := &FyneUI{
		app:      a,
		window:   w,
		profiles: mp,
		runner:   mr,
	}
	
	// This should fail to compile or run as showDashboardView doesn't exist
	ui.showDashboardView()
	
	if ui.window.Content() == nil {
		t.Fatal("Window content is nil")
	}
	_ = fyne.NewPos(0, 0) // Use fyne
}
