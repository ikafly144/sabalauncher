package fyne

import (
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (ui *FyneUI) showDashboardView() {
	profiles, err := ui.profiles.GetProfiles()
	if err != nil {
		dialog.ShowError(err, ui.window)
		return
	}

	if ui.selectedProfileName == "" && len(profiles) > 0 {
		ui.selectedProfileName = profiles[0].Name
	}

	profileSelect := widget.NewSelect(nil, func(selected string) {
		for _, p := range profiles {
			if p.DisplayName == selected {
				ui.selectedProfileName = p.Name
				break
			}
		}
	})

	var options []string
	for _, p := range profiles {
		options = append(options, p.DisplayName)
		if p.Name == ui.selectedProfileName {
			profileSelect.SetSelected(p.DisplayName)
		}
	}
	profileSelect.Options = options

	playBtn := widget.NewButton("PLAY", func() {
		if ui.selectedProfileName == "" {
			dialog.ShowInformation("No Profile", "Please select a profile first.", ui.window)
			return
		}
		if err := ui.runner.Launch(ui.selectedProfileName); err != nil {
			dialog.ShowError(err, ui.window)
		}
	})
	playBtn.Importance = widget.HighImportance

	manageProfilesBtn := widget.NewButton("Manage Profiles", func() {
		ui.showProfileView()
	})

	logoutBtn := widget.NewButton("Logout", func() {
		_ = ui.auth.Logout()
		ui.showAuthView()
	})

	ui.window.SetContent(container.NewVBox(
		createHeader(),
		widget.NewLabel("Main Dashboard"),
		container.NewHBox(widget.NewLabel("Selected Profile:"), profileSelect),
		playBtn,
		container.NewHBox(manageProfilesBtn, logoutBtn),
	))
}
