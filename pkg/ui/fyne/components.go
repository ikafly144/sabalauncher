package fyne

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func createHeader() fyne.CanvasObject {
	icon := canvas.NewImageFromResource(resourceLauncherIcon)
	icon.SetMinSize(fyne.NewSize(24, 24))
	icon.FillMode = canvas.ImageFillContain

	return container.NewHBox(
		icon,
		widget.NewLabel("SabaLauncher"),
		layout.NewSpacer(),
	)
}
