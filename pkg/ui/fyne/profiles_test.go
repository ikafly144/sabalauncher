package fyne

import (
	"fmt"
	"testing"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/core"
	"github.com/stretchr/testify/mock"
)

type mockProfileManager struct {
	mock.Mock
}

func (m *mockProfileManager) GetProfiles() ([]core.Profile, error) {
	args := m.Called()
	return args.Get(0).([]core.Profile), args.Error(1)
}

func (m *mockProfileManager) AddProfile(url string) error {
	args := m.Called(url)
	return args.Error(0)
}

func (m *mockProfileManager) DeleteProfile(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *mockProfileManager) RefreshProfiles() error {
	args := m.Called()
	return args.Error(0)
}

func TestShowProfileView(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	m := new(mockProfileManager)
	m.On("GetProfiles").Return([]core.Profile{
		{Name: "test", DisplayName: "Test Profile"},
	}, nil)
	
	ui := &FyneUI{
		app:      a,
		window:   w,
		profiles: m,
	}
	
	ui.showProfileView()
	
	if ui.window.Content() == nil {
		t.Fatal("Window content is nil")
	}
}

func TestShowProfileView_Error(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	m := new(mockProfileManager)
	m.On("GetProfiles").Return([]core.Profile{}, fmt.Errorf("error"))
	
	ui := &FyneUI{
		app:      a,
		window:   w,
		profiles: m,
	}
	
	ui.showProfileView()
}

func TestShowAddProfileDialog(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	ui := &FyneUI{
		app:    a,
		window: w,
	}
	
	ui.showAddProfileDialog()
}

func TestShowProfileView_Buttons(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")
	
	m := new(mockProfileManager)
	m.On("GetProfiles").Return([]core.Profile{}, nil)
	
	ui := &FyneUI{
		app:      a,
		window:   w,
		profiles: m,
		auth:     new(mockAuthenticator),
	}
	ui.auth.(*mockAuthenticator).On("GetStatus").Return(core.AuthStatusLoggedOut)
	
	ui.showProfileView()
	
	// Manual button calls
	container := ui.window.Content().(*fyne.Container)
	for _, obj := range container.Objects {
		if btn, ok := obj.(*widget.Button); ok {
			btn.OnTapped()
		}
	}
}
