package icon

import (
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var (
	AccountIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.ActionAccountCircle)
		if err != nil {
			panic(err)
		}
		return icon
	}()
	LauncherIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.NavigationApps)
		if err != nil {
			panic(err)
		}
		return icon
	}()
	MenuIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.NavigationMenu)
		if err != nil {
			panic(err)
		}
		return icon
	}()
)

var (
	SuccessIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.ActionCheckCircle)
		if err != nil {
			panic(err)
		}
		return icon
	}()
	FailureIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.AlertError)
		if err != nil {
			panic(err)
		}
		return icon
	}()
	OpenIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.ActionOpenInNew)
		if err != nil {
			panic(err)
		}
		return icon
	}()
	CopyIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.ContentContentCopy)
		if err != nil {
			panic(err)
		}
		return icon
	}()
	MenuButtonIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.NavigationMoreHoriz)
		if err != nil {
			panic(err)
		}
		return icon
	}()
	DeleteIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.ActionDeleteForever)
		if err != nil {
			panic(err)
		}
		return icon
	}()
	UpdateIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.FileCloudDownload)
		if err != nil {
			panic(err)
		}
		return icon
	}()
)
