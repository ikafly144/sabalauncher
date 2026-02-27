package fyne

import (
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (ui *FyneUI) showProfileView() {
	profiles, err := ui.profiles.GetProfiles()
	if err != nil {
		dialog.ShowError(err, ui.window)
		return
	}
	
	list := container.NewVBox()
	for _, p := range profiles {
		profile := p // Capture for closure
		item := container.NewHBox(
			widget.NewLabel(profile.DisplayName),
			widget.NewButton("Delete", func() {
				dialog.ShowConfirm("Delete Profile", "Are you sure you want to delete "+profile.DisplayName+"?", func(ok bool) {
					if ok {
						if err := ui.profiles.DeleteProfile(profile.Source); err != nil {
							dialog.ShowError(err, ui.window)
						} else {
							ui.showProfileView()
						}
					}
				}, ui.window)
			}),
		)
		list.Add(item)
	}
	
	addBtn := widget.NewButton("Add Profile", func() {
		ui.showAddProfileDialog()
	})
	
	backBtn := widget.NewButton("Back to Auth", func() {
		ui.showAuthView()
	})
	
	ui.window.SetContent(container.NewVBox(
		createHeader(),
		widget.NewLabel("Profiles"),
		list,
		addBtn,
		backBtn,
	))
}

func (ui *FyneUI) showAddProfileDialog() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("https://example.com/profiles.json")
	
	items := []*widget.FormItem{
		widget.NewFormItem("Profile URL", entry),
	}
	
	dialog.ShowForm("Add New Profile", "Add", "Cancel", items, func(ok bool) {
		if ok {
			if err := ui.profiles.AddProfile(entry.Text); err != nil {
				dialog.ShowError(err, ui.window)
			} else {
				ui.showProfileView()
			}
		}
	}, ui.window)
}
