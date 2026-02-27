package fyne

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/core"
)

type FyneUI struct {
	app    fyne.App
	window fyne.Window
	
	auth    core.Authenticator
	profiles core.ProfileManager
	runner  core.GameRunner
}

func NewFyneUI(auth core.Authenticator, profiles core.ProfileManager, runner core.GameRunner) *FyneUI {
	a := app.New()
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
	ui.window.SetContent(container.NewVBox(
		widget.NewLabel("SabaLauncher - Fyne Migration"),
		widget.NewButton("Quit", func() {
			ui.app.Quit()
		}),
	))
	
	ui.window.ShowAndRun()
}
