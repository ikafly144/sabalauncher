package fyne

import (
	"fyne.io/fyne/v2"
	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/v2/pkg/core"
	"github.com/ikafly144/sabalauncher/v2/pkg/i18n"
)

type FyneUI struct {
	app    fyne.App
	window fyne.Window

	auth      core.Authenticator
	instances core.InstanceManager
	runner    core.GameRunner
	discord   core.DiscordManager
	config    *core.LauncherConfig

	version             string
	selectedInstanceUID uuid.UUID

	instanceUpdateAvailable map[uuid.UUID]bool
	checkingUpdate          map[uuid.UUID]bool
}

func NewFyneUI(a fyne.App, auth core.Authenticator, instances core.InstanceManager, runner core.GameRunner, discord core.DiscordManager, config *core.LauncherConfig, version string) *FyneUI {
	a.SetIcon(resourceLauncherIcon)
	w := a.NewWindow(i18n.T("app_title") + " " + version)
	w.Resize(fyne.NewSize(800, 600))
	w.SetFixedSize(false)

	return &FyneUI{
		app:       a,
		window:    w,
		auth:      auth,
		instances: instances,
		runner:    runner,
		discord:   discord,
		config:    config,
		version:   version,

		instanceUpdateAvailable: make(map[uuid.UUID]bool),
		checkingUpdate:          make(map[uuid.UUID]bool),
	}
}

func (ui *FyneUI) Run() {
	ui.CheckForUpdates(ui.version)

	// Start notification listener
	go func() {
		nChan := ui.runner.SubscribeNotifications()
		for n := range nChan {
			ui.showOverlayNotification(n.Title, n.Message, n.Duration)
		}
	}()

	if ui.auth.GetStatus() == core.AuthStatusLoggedIn {
		ui.showMainView()
	} else {
		ui.showAuthView()
	}
	ui.window.ShowAndRun()
}
