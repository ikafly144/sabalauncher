package fyne

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/browser"
	"github.com/ikafly144/sabalauncher/pkg/i18n"
)

func (ui *FyneUI) showImportModpackDialog() {
	// Attempt to get HWND. On Windows, Fyne uses GLFW.
	// We pass 0 and let the browser package handle it if needed.
	path, err := browser.SelectFile(0, "SBPack files (*.sbpack)|*.sbpack")
	if err != nil {
		dialog.ShowError(err, ui.window)
		return
	}
	if path == "" {
		return // Canceled
	}

	// Show progress or immediate import
	progress := dialog.NewCustom(i18n.T("importing_progress"), i18n.T("cancel"), widget.NewProgressBarInfinite(), ui.window)
	progress.Show()

	go func() {
		err := ui.instances.ImportInstance(path)
		progress.Hide()
		if err != nil {
			dialog.ShowError(err, ui.window)
		} else {
			ui.showMainView()
		}
	}()
}

func (ui *FyneUI) showRegisterRemoteModpackDialog() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("https://repository.example/repo/manifest.json")

	items := []*widget.FormItem{
		widget.NewFormItem(i18n.T("manifest_url_label"), entry),
	}

	d := dialog.NewForm(i18n.T("register_remote_title"), i18n.T("register_btn"), i18n.T("cancel"), items, func(ok bool) {
		if ok {
			progress := dialog.NewCustom(i18n.T("registering_progress"), i18n.T("cancel"), widget.NewProgressBarInfinite(), ui.window)
			progress.Show()
			go func() {
				err := ui.instances.AddRemoteInstance(entry.Text)
				progress.Hide()
				if err != nil {
					dialog.ShowError(err, ui.window)
				} else {
					ui.showMainView()
				}
			}()
		}
	}, ui.window)

	d.Resize(fyne.NewSize(500, 200))
	d.Show()
}

func (ui *FyneUI) showUpdateInstanceDialog(instanceName string) {
	path, err := browser.SelectFile(0, "Update files (*.sbpatch, *.sbpack)|*.sbpatch;*.sbpack")
	if err != nil {
		dialog.ShowError(err, ui.window)
		return
	}
	if path == "" {
		return // Canceled
	}

	ui.startUpdate(instanceName, path)
}

func (ui *FyneUI) startUpdate(instanceName string, path string) {
	progress := dialog.NewCustom(i18n.T("updating_progress"), i18n.T("cancel"), widget.NewProgressBarInfinite(), ui.window)
	progress.Show()

	go func() {
		err := ui.instances.UpdateInstance(instanceName, path)
		progress.Hide()
		if err != nil {
			dialog.ShowError(err, ui.window)
		} else {
			ui.showMainView()
		}
	}()
}
