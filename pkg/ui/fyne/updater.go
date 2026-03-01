package fyne

import (
	"log/slog"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/core"
	"github.com/ikafly144/sabalauncher/pkg/i18n"
)

func (ui *FyneUI) CheckForUpdates(currentVersion string) {
	go func() {
		update, err := core.CheckForUpdate(currentVersion)
		if err != nil {
			slog.Error("failed to check for updates", "err", err)
			return
		}

		if update == nil {
			slog.Info("no updates available")
			return
		}

		fyne.Do(func() {
			ui.showUpdatePrompt(update)
		})
	}()
}

func (ui *FyneUI) showUpdatePrompt(update *core.UpdateInfo) {
	msg := i18n.T("update_available_body", update.Version, update.ReleaseNotes)

	d := dialog.NewConfirm(i18n.T("update_available_title"), msg, func(ok bool) {
		if ok {
			ui.startUpdateDownload(update.DownloadURL)
		}
	}, ui.window)

	d.Show()
}

func (ui *FyneUI) startUpdateDownload(url string) {
	progress := dialog.NewCustom(i18n.T("downloading_update"), i18n.T("cancel"), widget.NewProgressBarInfinite(), ui.window)
	progress.Show()

	go func() {
		err := core.DownloadAndRunInstaller(url)
		if err != nil {
			fyne.Do(func() {
				progress.Hide()
				dialog.ShowError(err, ui.window)
			})
		}
		// If success, the app exits in DownloadAndRunInstaller
	}()
}
