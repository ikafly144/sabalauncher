package fyne

import (
	"fyne.io/fyne/v2"
	"github.com/ikafly144/sabalauncher/pkg/core"
)

type FyneUI struct {
	app    fyne.App
	window fyne.Window

	auth      core.Authenticator
	instances core.InstanceManager
	runner    core.GameRunner
	discord   core.DiscordManager

	version              string
	selectedInstanceName string
}

func NewFyneUI(a fyne.App, auth core.Authenticator, instances core.InstanceManager, runner core.GameRunner, discord core.DiscordManager, version string) *FyneUI {
	w := a.NewWindow("SabaLauncher")
	w.Resize(fyne.NewSize(800, 600))
	w.SetFixedSize(false)

	return &FyneUI{
		app:       a,
		window:    w,
		auth:      auth,
		instances: instances,
		runner:    runner,
		discord:   discord,
		version:   version,
	}
}

func (ui *FyneUI) Run() {
	ui.CheckForUpdates(ui.version)
	if ui.auth.GetStatus() == core.AuthStatusLoggedIn {
		ui.showMainView()
	} else {
		ui.showAuthView()
	}
	ui.window.ShowAndRun()
}
