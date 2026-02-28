package fyne

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/resource"
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
	instances, err := ui.instances.GetInstances()
	if err != nil {
		return widget.NewLabel("Error: " + err.Error())
	}

	if ui.selectedInstanceName == "" && len(instances) > 0 {
		ui.selectedInstanceName = instances[0].Name
	}

	// Instance selection (left side) with Icons
	instanceList := widget.NewList(
		func() int { return len(instances) },
		func() fyne.CanvasObject {
			icon := canvas.NewImageFromImage(nil)
			icon.SetMinSize(fyne.NewSize(32, 32))
			icon.FillMode = canvas.ImageFillContain
			return container.NewHBox(icon, widget.NewLabel("Template Label"))
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(instances) {
				return
			}
			p := instances[id]
			box := obj.(*fyne.Container)
			// Wait: Instance no longer has IconImage by default. Let's leave a blank or default icon logic here
			// icon := box.Objects[0].(*canvas.Image)
			label := box.Objects[1].(*widget.Label)

			// if p.IconImage != nil {
			// 	icon.Image = p.IconImage
			// } else {
			// 	icon.Image = nil
			// }
			// icon.Refresh()

			label.SetText(p.Name)
			if p.Name == ui.selectedInstanceName {
				label.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				label.TextStyle = fyne.TextStyle{}
			}
		},
	)
	instanceList.OnSelected = func(id widget.ListItemID) {
		ui.selectedInstanceName = instances[id].Name
		ui.showMainView()
	}

	// Sidebar with Import Profile button
	importBtn := widget.NewButton("Import Modpack", func() {
		ui.showImportModpackDialog()
	})
	sidebar := container.NewBorder(nil, container.NewPadded(importBtn), nil, nil, instanceList)

	// Right side: Detail View
	var currentInstance *resource.Instance
	found := false
	for _, p := range instances {
		if p.Name == ui.selectedInstanceName {
			currentInstance = p
			found = true
			break
		}
	}

	var detailArea fyne.CanvasObject
	if found {
		detailTitle := widget.NewLabelWithStyle(currentInstance.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		
		versionStr := "Unknown Version"
		for _, v := range currentInstance.Versions {
			if v.ID == "minecraft" {
				versionStr = v.Version
				break
			}
		}
		
		detailVersion := widget.NewLabel("Version: " + versionStr)

		// Action Buttons for current instance
		playBtn := widget.NewButton("PLAY", func() {
			ui.showLaunchOverlay()
			go func() {
				_ = ui.discord.SetActivity(ui.selectedInstanceName)
				if err := ui.runner.Launch(ui.selectedInstanceName); err != nil {
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

		updateBtn := widget.NewButton("Update", func() {
			ui.showUpdateInstanceDialog(currentInstance.Name)
		})

		deleteBtn := widget.NewButton("Delete Instance", func() {
			dialog.ShowConfirm("Delete Instance", "Are you sure you want to delete "+currentInstance.Name+"?", func(ok bool) {
				if ok {
					if err := ui.instances.DeleteInstance(currentInstance.Name); err != nil {
						dialog.ShowError(err, ui.window)
					} else {
						ui.selectedInstanceName = "" // Reset selection
						ui.showMainView()
					}
				}
			}, ui.window)
		})
		deleteBtn.Importance = widget.DangerImportance

		actions := container.NewHBox(playBtn, updateBtn, deleteBtn)

		detailContainer := container.NewVBox(
			detailTitle,
			detailVersion,
			widget.NewSeparator(),
			layout.NewSpacer(),
			container.NewPadded(actions),
		)
		detailArea = container.NewPadded(detailContainer)
	} else {
		detailArea = container.NewCenter(widget.NewLabel("Select an instance to see details"))
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
