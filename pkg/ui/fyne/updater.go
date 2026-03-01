package fyne

import (
	"fmt"
	"log/slog"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/core"
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
	msg := fmt.Sprintf("A new version (%s) is available.\nWould you like to update now?\n\nRelease Notes:\n%s",
		update.Version, update.ReleaseNotes)

	d := dialog.NewConfirm("Update Available", msg, func(ok bool) {
		if ok {
			ui.startUpdateDownload(update.DownloadURL)
		}
	}, ui.window)

	d.Show()
}

func (ui *FyneUI) startUpdateDownload(url string) {
	progress := dialog.NewCustom("Downloading Update", "Cancel", widget.NewProgressBarInfinite(), ui.window)
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
