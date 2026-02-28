package fyne

import (
	"fyne.io/fyne/v2/test"
	"github.com/ikafly144/sabalauncher/pkg/core"
	"github.com/ikafly144/sabalauncher/pkg/resource"
	"github.com/stretchr/testify/mock"
	"testing"
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

func (m *mockProfileManager) GetFullProfile(name string) (*resource.Profile, error) {
	args := m.Called(name)
	return args.Get(0).(*resource.Profile), args.Error(1)
}

func TestShowAddProfileDialog(t *testing.T) {
	a := test.NewApp()
	w := a.NewWindow("Test")

	ui := &FyneUI{
		app:     a,
		window:  w,
		discord: new(mockDiscordManager),
	}

	ui.showAddProfileDialog()
}
