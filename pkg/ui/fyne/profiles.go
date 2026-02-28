package fyne

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/browser"
)

func (ui *FyneUI) showAddProfileDialog() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("profile url or select local file")

	browseBtn := widget.NewButton("Browse", func() {
		// Attempt to get HWND. On Windows, Fyne uses GLFW.
		// For now, we pass 0 and let the browser package handle it if needed,
		// or we can try to find the window.
		path, err := browser.SelectFile(0, "JSON files (*.json)|*.json")
		if err != nil {
			dialog.ShowError(err, ui.window)
			return
		}
		if path != "" {
			entry.SetText(path)
		}
	})

	// Force a minimum width for the entry to make the dialog wider
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(fyne.NewSize(400, 0))
	inputContainer := container.NewStack(rect, entry)

	items := []*widget.FormItem{
		widget.NewFormItem("Profile URL/Path", container.NewBorder(nil, nil, nil, browseBtn, inputContainer)),
	}

	d := dialog.NewForm("Add New Profile", "Add", "Cancel", items, func(ok bool) {
		if ok {
			if err := ui.profiles.AddProfile(entry.Text); err != nil {
				dialog.ShowError(err, ui.window)
			} else {
				ui.showMainView()
			}
		}
	}, ui.window)

	d.Resize(fyne.NewSize(500, 200))
	d.Show()
}
