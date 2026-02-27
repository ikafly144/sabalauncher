package fyne

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func createHeader() fyne.CanvasObject {
	return container.NewHBox(
		widget.NewLabel("SabaLauncher"),
		layout.NewSpacer(),
		widget.NewButton("Settings", func() {}),
	)
}
