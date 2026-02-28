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
		container.NewTabItem("Account", ui.makeAccountView()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// Border layout ensures tabs fill the remaining space below header
	content := container.NewBorder(createHeader(), nil, nil, nil, tabs)
	ui.window.SetContent(content)
}

func (ui *FyneUI) showDashboardView() {
	ui.showMainView()
}

func (ui *FyneUI) makeDashboardView() fyne.CanvasObject {
	profiles, err := ui.profiles.GetProfiles()
	if err != nil {
		return widget.NewLabel("Error: " + err.Error())
	}

	if ui.selectedProfileName == "" && len(profiles) > 0 {
		ui.selectedProfileName = profiles[0].Name
	}

	// Profile selection (left side) with Icons
	profileList := widget.NewList(
		func() int { return len(profiles) },
		func() fyne.CanvasObject {
			icon := canvas.NewImageFromImage(nil)
			icon.SetMinSize(fyne.NewSize(32, 32))
			icon.FillMode = canvas.ImageFillContain
			return container.NewHBox(icon, widget.NewLabel("Template Label"))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(profiles) {
				return
			}
			p := profiles[id]
			box := obj.(*fyne.Container)
			icon := box.Objects[0].(*canvas.Image)
			label := box.Objects[1].(*widget.Label)

			if p.IconImage != nil {
				icon.Image = p.IconImage
			} else {
				icon.Image = nil
			}
			icon.Refresh()

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
		ui.showMainView()
	}

	// Sidebar with Add Profile button
	addBtn := widget.NewButton("Add Profile", func() {
		ui.showAddProfileDialog()
	})
	sidebar := container.NewBorder(nil, container.NewPadded(addBtn), nil, nil, profileList)

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

	var detailArea fyne.CanvasObject
	if found {
		detailTitle := widget.NewLabelWithStyle(currentProfile.DisplayName, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		detailDesc := widget.NewLabel(currentProfile.Description)
		detailDesc.Wrapping = fyne.TextWrapWord
		detailVersion := widget.NewLabel("Version: " + currentProfile.VersionName)

		icon := canvas.NewImageFromImage(currentProfile.IconImage)
		icon.SetMinSize(fyne.NewSize(64, 64))
		icon.FillMode = canvas.ImageFillContain

		// Action Buttons for current profile
		playBtn := widget.NewButton("PLAY", func() {
			ui.showLaunchOverlay()
			go func() {
				_ = ui.discord.SetActivity(ui.selectedProfileName)
				if err := ui.runner.Launch(ui.selectedProfileName); err != nil {
					fyne.Do(func() {
						dialog.ShowError(err, ui.window)
					})
				}
				_ = ui.discord.ClearActivity()
				fyne.Do(func() {
					ui.showMainView()
				})
			}()
		})
		playBtn.Importance = widget.HighImportance

		deleteBtn := widget.NewButton("Delete Profile", func() {
			dialog.ShowConfirm("Delete Profile", "Are you sure you want to delete "+currentProfile.DisplayName+"?", func(ok bool) {
				if ok {
					if err := ui.profiles.DeleteProfile(currentProfile.Source); err != nil {
						dialog.ShowError(err, ui.window)
					} else {
						ui.selectedProfileName = "" // Reset selection
						ui.showMainView()
					}
				}
			}, ui.window)
		})
		deleteBtn.Importance = widget.DangerImportance

		actions := container.NewHBox(playBtn, deleteBtn)

		detailContainer := container.NewVBox(
			container.NewHBox(icon, detailTitle),
			detailVersion,
			widget.NewSeparator(),
			detailDesc,
			layout.NewSpacer(),
			container.NewPadded(actions),
		)
		detailArea = container.NewPadded(detailContainer)
	} else {
		detailArea = container.NewCenter(widget.NewLabel("Select a profile to see details"))
	}

	// Main Layout (Responsive Split)
	mainSplit := container.NewHSplit(sidebar, detailArea)
	mainSplit.Offset = 0.3

	return mainSplit
}

func (ui *FyneUI) makeAccountView() fyne.CanvasObject {
	logoutBtn := widget.NewButton("Logout", func() {
		_ = ui.auth.Logout()
		ui.showAuthView()
	})
	logoutBtn.Importance = widget.DangerImportance

	content := container.NewVBox(
		widget.NewLabelWithStyle("Account Information", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Logged in as: "+ui.auth.GetUserDisplay()),
		layout.NewSpacer(),
		logoutBtn,
	)
	return container.NewPadded(content)
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
			fyne.Do(func() {
				taskLabel.SetText(event.TaskName)
				statusLabel.SetText(event.Status)
				progressBar.SetValue(event.Percentage / 100.0)
			})
		}
	}()

	go func() {
		for entry := range logsChan {
			fyne.Do(func() {
				logLine := "[" + string(entry.Level) + "] " + entry.Message + "\n"
				logEntry.SetText(logEntry.Text + logLine)
			})
		}
	}()

	// Responsive launch overlay layout
	topInfo := container.NewVBox(taskLabel, progressBar, statusLabel)

	// NewBorder makes the logEntry (center) fill the window between top info and stop button
	content := container.NewBorder(
		container.NewVBox(createHeader(), container.NewPadded(topInfo)),
		container.NewPadded(stopBtn),
		nil,
		nil,
		container.NewPadded(container.NewScroll(logEntry)),
	)

	ui.window.SetContent(content)
}
