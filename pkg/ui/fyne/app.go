package fyne

import (
	"fyne.io/fyne/v2"
	"github.com/ikafly144/sabalauncher/pkg/core"
)

type FyneUI struct {
	app    fyne.App
	window fyne.Window
	
	auth    core.Authenticator
	profiles core.ProfileManager
	runner  core.GameRunner
}

func NewFyneUI(a fyne.App, auth core.Authenticator, profiles core.ProfileManager, runner core.GameRunner) *FyneUI {
	w := a.NewWindow("SabaLauncher")
	
	return &FyneUI{
		app:      a,
		window:   w,
		auth:     auth,
		profiles: profiles,
		runner:   runner,
	}
}

func (ui *FyneUI) Run() {
	ui.showAuthView()
	ui.window.ShowAndRun()
}
