package fyne

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/i18n"
	"github.com/ikafly144/sabalauncher/pkg/resource"
)

func (ui *FyneUI) showMainView() {
	tabs := container.NewAppTabs(
		container.NewTabItem(i18n.T("tab_launcher"), ui.makeDashboardView()),
		container.NewTabItem(i18n.T("tab_settings"), ui.makeSettingsView()),
	)
	ui.window.SetContent(tabs)
}

func (ui *FyneUI) showDashboardView() {
	ui.showMainView()
}

func (ui *FyneUI) makeDashboardView() fyne.CanvasObject {
	instances, err := ui.instances.GetInstances()
	if err != nil {
		return widget.NewLabel(i18n.T("error_prefix", err.Error()))
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
			label := box.Objects[1].(*widget.Label)

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
	importBtn := widget.NewButton(i18n.T("import_modpack"), func() {
		ui.showImportModpackDialog()
	})
	registerRemoteBtn := widget.NewButton(i18n.T("register_remote"), func() {
		ui.showRegisterRemoteModpackDialog()
	})
	sidebar := container.NewBorder(nil, container.NewVBox(container.NewPadded(importBtn), container.NewPadded(registerRemoteBtn)), nil, nil, instanceList)

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
		
		versionStr := i18n.T("unknown_version")
		for _, v := range currentInstance.Versions {
			if v.ID == "minecraft" {
				versionStr = v.Version
				break
			}
		}
		
		detailVersion := widget.NewLabel(i18n.T("version_label", versionStr))

		// Action Buttons for current instance
		isRemote := currentInstance.Upstream != nil && currentInstance.Upstream.ManifestURL != ""

		playBtn := widget.NewButton(i18n.T("play_btn"), func() {
			ui.showLaunchOverlay()
			go func() {
				if isRemote {
					// Force update before launch
					if err := ui.instances.UpdateInstance(currentInstance.Name, ""); err != nil {
						fyne.Do(func() {
							dialog.ShowError(fmt.Errorf("failed to update before play: %w", err), ui.window)
							ui.showMainView()
						})
						return
					}
				}

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

		updateBtn := widget.NewButton(i18n.T("update_btn"), func() {
			ui.showUpdateInstanceDialog(currentInstance.Name)
		})

		deleteBtn := widget.NewButton(i18n.T("delete_instance_btn"), func() {
			dialog.ShowConfirm(i18n.T("delete_instance_confirm_title"), i18n.T("delete_instance_confirm_body", currentInstance.Name), func(ok bool) {
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

		var actions fyne.CanvasObject
		if isRemote {
			actions = container.NewHBox(playBtn, deleteBtn)
		} else {
			actions = container.NewHBox(playBtn, updateBtn, deleteBtn)
		}

		detailContainer := container.NewVBox(
			detailTitle,
			detailVersion,
			widget.NewSeparator(),
			layout.NewSpacer(),
			container.NewPadded(actions),
		)
		detailArea = container.NewPadded(detailContainer)
	} else {
		detailArea = container.NewCenter(widget.NewLabel(i18n.T("select_instance_prompt")))
	}

	// Main Layout (Responsive Split)
	mainSplit := container.NewHSplit(sidebar, detailArea)
	mainSplit.Offset = 0.3

	return mainSplit
}

func (ui *FyneUI) makeSettingsView() fyne.CanvasObject {
	// Account section
	profile, err := ui.auth.GetMinecraftProfile()
	var accountInfo fyne.CanvasObject
	if err == nil {
		uri, _ := storage.ParseURI(fmt.Sprintf("https://mc-heads.net/avatar/%s/64", profile.UUID))
		avatar := canvas.NewImageFromURI(uri)
		avatar.SetMinSize(fyne.NewSize(64, 64))
		avatar.FillMode = canvas.ImageFillContain

		usernameLabel := widget.NewLabel(i18n.T("username_label", profile.Username))
		uuidLabel := widget.NewLabel(i18n.T("uuid_label", profile.UUID.String()))
		uuidLabel.TextStyle = fyne.TextStyle{Monospace: true}

		accountInfo = container.NewHBox(
			avatar,
			container.NewVBox(usernameLabel, uuidLabel),
		)
	} else {
		accountInfo = widget.NewLabel("Failed to load account info: " + err.Error())
	}

	logoutBtn := widget.NewButton(i18n.T("logout"), func() {
		_ = ui.auth.Logout()
		ui.showAuthView()
	})
	logoutBtn.Importance = widget.DangerImportance

	return container.NewVBox(
		widget.NewLabelWithStyle(i18n.T("settings_title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabelWithStyle(i18n.T("account_section_title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewPadded(accountInfo),
		layout.NewSpacer(),
		container.NewPadded(logoutBtn),
	)
}

func (ui *FyneUI) showLaunchOverlay() {
	progress := widget.NewProgressBar()
	status := widget.NewLabel(i18n.T("preparing"))
	stopBtn := widget.NewButton(i18n.T("stop_btn"), func() {
		ui.runner.Stop()
	})
	stopBtn.Importance = widget.DangerImportance

	topInfo := container.NewVBox(status, progress)

	logEntry := widget.NewMultiLineEntry()
	logEntry.Disable()

	go func() {
		pChan := ui.runner.SubscribeProgress()
		lChan := ui.runner.SubscribeLogs()

		for {
			select {
			case p, ok := <-pChan:
				if !ok {
					return
				}
				fyne.Do(func() {
					status.SetText(fmt.Sprintf("%s (%s)", p.TaskName, p.Status))
					progress.SetValue(p.Percentage / 100.0)
				})
			case l, ok := <-lChan:
				if !ok {
					return
				}
				fyne.Do(func() {
					logEntry.Append(fmt.Sprintf("[%s] %s\n", l.Source, l.Message))
				})
			}
		}
	}()

	content := container.NewBorder(
		container.NewVBox(createHeader(), container.NewPadded(topInfo)),
		container.NewPadded(stopBtn),
		nil,
		nil,
		container.NewPadded(container.NewScroll(logEntry)),
	)

	ui.window.SetContent(content)
}
