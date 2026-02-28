package fyne

import (
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/browser"
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
	progress := dialog.NewCustom("Importing...", "Cancel", widget.NewProgressBarInfinite(), ui.window)
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

func (ui *FyneUI) showUpdateInstanceDialog(instanceName string) {
	path, err := browser.SelectFile(0, "SBPatch files (*.sbpatch)|*.sbpatch")
	if err != nil {
		dialog.ShowError(err, ui.window)
		return
	}
	if path == "" {
		return // Canceled
	}

	progress := dialog.NewCustom("Updating...", "Cancel", widget.NewProgressBarInfinite(), ui.window)
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
