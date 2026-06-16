package fyne

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
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
	isUpdatePromptShown     bool
}

func NewFyneUI(a fyne.App, auth core.Authenticator, instances core.InstanceManager, runner core.GameRunner, discord core.DiscordManager, config *core.LauncherConfig, version string) *FyneUI {
	a.SetIcon(resourceLauncherIcon)
	w := a.NewWindow(i18n.T("app_title") + " " + version)
	w.Resize(fyne.NewSize(800, 600))
	w.SetFixedSize(false)

	ui := &FyneUI{
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

	w.SetCloseIntercept(func() {
		w.Hide()
	})

	return ui
}

func (ui *FyneUI) Run() {
	ui.CheckForUpdates(ui.version)
	ui.StartBackgroundUpdateCheck()
	ui.setupSystray()

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

func (ui *FyneUI) setupSystray() {
	if desk, ok := ui.app.(desktop.App); ok {
		m := fyne.NewMenu(i18n.T("app_title"),
			fyne.NewMenuItem(i18n.T("systray_show"), func() {
				ui.window.Show()
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem(i18n.T("systray_quit"), func() {
				ui.app.Quit()
			}),
		)
		desk.SetSystemTrayMenu(m)
	}
}
