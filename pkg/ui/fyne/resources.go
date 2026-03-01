package fyne

import (
	"fyne.io/fyne/v2"
	"github.com/ikafly144/sabalauncher/v2/assets"
)

var resourceDefaultIcon = &fyne.StaticResource{
	StaticName:    "launcher_icon.png",
	StaticContent: assets.LauncherIconPng,
}

var resourceLauncherIcon = &fyne.StaticResource{
	StaticName:    "launcher_icon.ico",
	StaticContent: assets.LauncherIconIco,
}
