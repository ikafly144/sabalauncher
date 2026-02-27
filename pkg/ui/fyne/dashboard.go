package fyne

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/core"
)

func (ui *FyneUI) showMainView() {
	tabs := container.NewAppTabs(
		container.NewTabItem("Launcher", ui.makeDashboardView()),
		container.NewTabItem("Profiles", ui.makeProfileView()),
		container.NewTabItem("Account", ui.makeAccountView()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// Wrap in a layout that includes the header
	content := container.NewBorder(createHeader(), nil, nil, nil, tabs)
	ui.window.SetContent(content)
}

func (ui *FyneUI) showDashboardView() {
	ui.showMainView() // Default to tabbed view
}

func (ui *FyneUI) makeDashboardView() fyne.CanvasObject {
	profiles, err := ui.profiles.GetProfiles()
	if err != nil {
		return widget.NewLabel("Error: " + err.Error())
	}

	if ui.selectedProfileName == "" && len(profiles) > 0 {
		ui.selectedProfileName = profiles[0].Name
	}

	// Profile selection (left side)
	profileList := widget.NewList(
		func() int { return len(profiles) },
		func() fyne.CanvasObject { return widget.NewLabel("Template Label") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(profiles) {
				return
			}
			p := profiles[id]
			label := obj.(*widget.Label)
			label.SetText(p.DisplayName)
			if p.Name == ui.selectedProfileName {
				label.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				label.TextStyle = fyne.TextStyle{}
			}
		},
	)
	profileList.OnSelected = func(id widget.ListItemID) {
		ui.selectedProfileName = profiles[id].Name
		ui.showMainView() // Refresh to update detail view
	}

	// Right side: Detail View
	var currentProfile core.Profile
	found := false
	for _, p := range profiles {
		if p.Name == ui.selectedProfileName {
			currentProfile = p
			found = true
			break
		}
	}

	var detailContainer fyne.CanvasObject
	if found {
		detailTitle := widget.NewLabelWithStyle(currentProfile.DisplayName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		detailDesc := widget.NewLabel(currentProfile.Description)
		detailDesc.Wrapping = fyne.TextWrapWord
		detailVersion := widget.NewLabel("Version: " + currentProfile.VersionName)
		
		detailContainer = container.NewVBox(
			detailTitle,
			detailVersion,
			widget.NewSeparator(),
			detailDesc,
		)
	} else {
		detailContainer = widget.NewLabel("Select a profile to see details")
	}

	// Banner
	banner := canvas.NewImageFromFile("assets/launcher_banner.jpg")
	banner.FillMode = canvas.ImageFillContain
	banner.SetMinSize(fyne.NewSize(400, 150))

	// Main Layout
	mainSplit := container.NewHSplit(profileList, container.NewScroll(detailContainer))
	mainSplit.Offset = 0.3

	// Bottom Bar: Play Button
	playBtn := widget.NewButton("PLAY", func() {
		if ui.selectedProfileName == "" {
			dialog.ShowInformation("No Profile", "Please select a profile first.", ui.window)
			return
		}
		ui.showLaunchOverlay()
		go func() {
			_ = ui.discord.SetActivity(ui.selectedProfileName)
			if err := ui.runner.Launch(ui.selectedProfileName); err != nil {
				dialog.ShowError(err, ui.window)
			}
			_ = ui.discord.ClearActivity()
			ui.showMainView()
		}()
	})
	playBtn.Importance = widget.HighImportance
	
	return container.NewBorder(banner, playBtn, nil, nil, mainSplit)
}

func (ui *FyneUI) makeAccountView() fyne.CanvasObject {
	logoutBtn := widget.NewButton("Logout", func() {
		_ = ui.auth.Logout()
		ui.showAuthView()
	})
	logoutBtn.Importance = widget.DangerImportance

	return container.NewVBox(
		widget.NewLabelWithStyle("Account Information", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Logged in as: "+ui.auth.GetUserDisplay()),
		layout.NewSpacer(),
		logoutBtn,
	)
}

func (ui *FyneUI) showLaunchOverlay() {
	taskLabel := widget.NewLabel("Preparing...")
	taskLabel.Alignment = fyne.TextAlignCenter
	
	statusLabel := widget.NewLabel("")
	statusLabel.Alignment = fyne.TextAlignCenter
	
	progressBar := widget.NewProgressBar()
	
	logEntry := widget.NewMultiLineEntry()
	logEntry.Disable()
	logEntry.SetText("Waiting for logs...\n")

	stopBtn := widget.NewButton("STOP", func() {
		_ = ui.runner.Stop()
		ui.showMainView()
	})
	stopBtn.Importance = widget.DangerImportance

	progressChan := ui.runner.SubscribeProgress()
	logsChan := ui.runner.SubscribeLogs()

	go func() {
		for event := range progressChan {
			taskLabel.SetText(event.TaskName)
			statusLabel.SetText(event.Status)
			progressBar.SetValue(event.Percentage / 100.0)
		}
	}()

	go func() {
		for entry := range logsChan {
			logLine := "[" + string(entry.Level) + "] " + entry.Message + "\n"
			logEntry.SetText(logEntry.Text + logLine)
		}
	}()

	ui.window.SetContent(container.NewBorder(
		container.NewVBox(createHeader(), taskLabel, progressBar, statusLabel),
		stopBtn,
		nil,
		nil,
		container.NewScroll(logEntry),
	))
}
