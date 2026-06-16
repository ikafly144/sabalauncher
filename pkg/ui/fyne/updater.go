package fyne

import (
	"log/slog"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/v2/pkg/core"
	"github.com/ikafly144/sabalauncher/v2/pkg/i18n"
)

func (ui *FyneUI) CheckForUpdates(currentVersion string) {
	if ui.isUpdatePromptShown {
		return
	}

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
			if !ui.isUpdatePromptShown {
				ui.showUpdatePrompt(update)
			}
		})
	}()
}

func (ui *FyneUI) StartBackgroundUpdateCheck() {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for range ticker.C {
			ui.CheckForUpdates(ui.version)
		}
	}()
}

func (ui *FyneUI) showUpdatePrompt(update *core.UpdateInfo) {
	ui.isUpdatePromptShown = true
	title := i18n.T("update_available_title")
	header := widget.NewLabel(i18n.T("update_available_header", update.Version))
	header.TextStyle = fyne.TextStyle{Bold: true}

	changelog := widget.NewRichTextFromMarkdown(update.ReleaseNotes)
	changelog.Wrapping = fyne.TextWrapWord

	scroll := container.NewScroll(changelog)
	scroll.SetMinSize(fyne.NewSize(400, 300))

	content := container.NewBorder(header, nil, nil, nil, scroll)

	d := dialog.NewCustomConfirm(title, i18n.T("yes"), i18n.T("no"), content, func(ok bool) {
		ui.isUpdatePromptShown = false
		if ok {
			ui.startUpdateDownload(update.DownloadURL)
		}
	}, ui.window)

	d.Show()
}

func (ui *FyneUI) startUpdateDownload(url string) {
	prog := widget.NewProgressBar()
	progress := dialog.NewCustom(i18n.T("downloading_update"), i18n.T("cancel"), prog, ui.window)
	progress.Show()

	go func() {
		err := core.DownloadAndRunInstaller(url, func(percentage float64) {
			fyne.Do(func() {
				prog.SetValue(percentage / 100.0)
			})
		})
		if err != nil {
			fyne.Do(func() {
				progress.Hide()
				dialog.ShowError(err, ui.window)
			})
		}
		// If success, the app exits in DownloadAndRunInstaller
	}()
}
