package fyne

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/v2/pkg/core"
	"github.com/ikafly144/sabalauncher/v2/pkg/i18n"
	"github.com/ikafly144/sabalauncher/v2/pkg/osinfo"
	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
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

func (ui *FyneUI) getInstanceIcon(inst *resource.Instance) fyne.Resource {
	if inst.Properties.Icon != "" {
		iconPath := filepath.Join(inst.Path, inst.Properties.Icon)
		if _, err := os.Stat(iconPath); err == nil {
			if res, err := fyne.LoadResourceFromPath(iconPath); err == nil {
				return res
			}
		}
	}
	return resourceDefaultIcon
}

func (ui *FyneUI) makeDashboardView() fyne.CanvasObject {
	instances, err := ui.instances.GetInstances()
	if err != nil {
		return widget.NewLabel(i18n.T("error_prefix", err.Error()))
	}

	if ui.selectedInstanceUID == uuid.Nil && len(instances) > 0 {
		ui.selectedInstanceUID = instances[0].UID
		ui.checkForInstanceUpdate(ui.selectedInstanceUID)
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
			icon := box.Objects[0].(*canvas.Image)
			label := box.Objects[1].(*widget.Label)

			icon.Resource = ui.getInstanceIcon(p)
			icon.Refresh()

			label.SetText(p.Name)
			if p.UID == ui.selectedInstanceUID {
				label.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				label.TextStyle = fyne.TextStyle{}
			}
		},
	)
	instanceList.OnSelected = func(id widget.ListItemID) {
		ui.selectedInstanceUID = instances[id].UID
		ui.checkForInstanceUpdate(ui.selectedInstanceUID)
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
		if p.UID == ui.selectedInstanceUID {
			currentInstance = p
			found = true
			break
		}
	}

	var detailArea fyne.CanvasObject
	if found {
		detailTitle := widget.NewLabelWithStyle(currentInstance.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

		// Icon
		largeIcon := canvas.NewImageFromResource(ui.getInstanceIcon(currentInstance))
		largeIcon.SetMinSize(fyne.NewSize(64, 64))
		largeIcon.FillMode = canvas.ImageFillContain

		versionStr := i18n.T("unknown_version")
		if currentInstance.Upstream != nil && currentInstance.Upstream.Version != "" {
			patchVer := currentInstance.Upstream.Version
			versionStr = patchVer
		}
		var sb strings.Builder
		sb.WriteString("(")
		for i, v := range currentInstance.Versions {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%s %s", v.ID, v.Version)
		}
		sb.WriteString(")")
		if len(currentInstance.Versions) > 0 {
			versionStr = fmt.Sprintf("%s %s", versionStr, sb.String())
		}

		detailVersion := widget.NewLabel(i18n.T("version_label", versionStr))
		detailVersion.Wrapping = fyne.TextWrapBreak

		playTimeStr := formatPlayTime(currentInstance.PlayTimeSeconds)
		detailPlayTime := widget.NewLabel(i18n.T("playtime_label", playTimeStr))

		// Description
		var description fyne.CanvasObject
		if currentInstance.Properties.Description != "" {
			desc := widget.NewRichTextFromMarkdown(currentInstance.Properties.Description)
			desc.Wrapping = fyne.TextWrapWord
			description = desc
		} else {
			description = layout.NewSpacer()
		}

		// Action Buttons for current instance
		isRemote := currentInstance.Upstream != nil && currentInstance.Upstream.ManifestURL != ""
		updateAvailable := ui.instanceUpdateAvailable[currentInstance.UID]

		launchFunc := func(opts *core.LaunchOptions) {
			if opts == nil {
				opts = &core.LaunchOptions{}
			}

			intendedMemory := ui.config.MaxMemory
			if currentInstance.Properties.Memory > 0 && uint64(currentInstance.Properties.Memory) > intendedMemory {
				intendedMemory = uint64(currentInstance.Properties.Memory)
			}

			totalMemory := osinfo.GetTotalPhysicalMemory()
			limitMB := uint64(float64(totalMemory) * 0.8 / (1024 * 1024))

			doLaunch := func() {
				ctx, closeOverlay := ui.showLaunchOverlay()
				go func() {
					if isRemote {
						// Force update before launch
						if err := ui.instances.UpdateInstance(ctx, currentInstance.UID, ""); err != nil {
							fyne.Do(func() {
								closeOverlay()
								if !errors.Is(err, context.Canceled) {
									dialog.ShowError(fmt.Errorf("failed to update before play: %w", err), ui.window)
								}
								ui.showMainView()
							})
							return
						}
					}

					_ = ui.discord.SetActivity(currentInstance.UID)
					if err := ui.runner.Launch(currentInstance.UID, opts); err != nil {
						fyne.Do(func() {
							dialog.ShowError(err, ui.window)
						})
					}
					_ = ui.discord.ClearActivity()
					fyne.Do(func() {
						closeOverlay()
						ui.showMainView()
					})
				}()
			}

			if totalMemory > 0 && intendedMemory > limitMB {
				opts.MemoryMB = limitMB
				d := dialog.NewInformation(i18n.T("memory_limit_title"), i18n.T("memory_limit_body", intendedMemory, limitMB), ui.window)
				d.SetOnClosed(doLaunch)
				d.Show()
			} else {
				doLaunch()
			}
		}

		var playBtn fyne.CanvasObject
		if isRemote && updateAvailable {
			btn := widget.NewButton(i18n.T("update_btn"), func() {
				ui.startUpdate(currentInstance.UID, "")
			})
			btn.Importance = widget.HighImportance
			playBtn = btn
		} else if currentInstance.Properties.QuickLaunch.MultiPlayer != "" || currentInstance.Properties.QuickLaunch.SinglePlayer != "" {
			options := []string{i18n.T("normal_play")}
			if currentInstance.Properties.QuickLaunch.MultiPlayer != "" {
				options = append(options, i18n.T("quick_launch_multiplayer_label"))
			}
			if currentInstance.Properties.QuickLaunch.SinglePlayer != "" {
				options = append(options, i18n.T("quick_launch_singleplayer_label"))
			}

			sel := widget.NewSelect(options, nil)
			sel.SetSelected(options[0])
			sel.Alignment = fyne.TextAlignTrailing

			btn := widget.NewButton(i18n.T("play_btn"), func() {
				var opts *core.LaunchOptions
				if sel.Selected == i18n.T("quick_launch_multiplayer_label") {
					opts = &core.LaunchOptions{QuickPlayMultiplayer: currentInstance.Properties.QuickLaunch.MultiPlayer}
				} else if sel.Selected == i18n.T("quick_launch_singleplayer_label") {
					opts = &core.LaunchOptions{QuickPlaySingleplayer: currentInstance.Properties.QuickLaunch.SinglePlayer}
				}
				launchFunc(opts)
			})
			btn.Importance = widget.HighImportance
			playBtn = container.NewBorder(nil, nil, nil, sel, btn)
		} else {
			btn := widget.NewButton(i18n.T("play_btn"), func() {
				launchFunc(nil)
			})
			btn.Importance = widget.HighImportance
			playBtn = btn
		}

		updateBtn := widget.NewButton(i18n.T("update_btn"), func() {
			ui.showUpdateInstanceDialog(currentInstance.UID)
		})

		repairBtn := widget.NewButton(i18n.T("repair_btn"), func() {
			ui.showRepairInstanceDialog(currentInstance.UID)
		})

		deleteBtn := widget.NewButton(i18n.T("delete_instance_btn"), func() {
			dialog.ShowConfirm(i18n.T("delete_instance_confirm_title"), i18n.T("delete_instance_confirm_body", currentInstance.Name), func(ok bool) {
				if ok {
					if err := ui.instances.DeleteInstance(currentInstance.UID); err != nil {
						dialog.ShowError(err, ui.window)
					} else {
						ui.selectedInstanceUID = uuid.Nil // Reset selection
						ui.showMainView()
					}
				}
			}, ui.window)
		})
		deleteBtn.Importance = widget.DangerImportance

		// Create Actions button with popup menu
		menu := fyne.NewMenu("",
			fyne.NewMenuItem(i18n.T("repair_btn"), repairBtn.OnTapped),
			fyne.NewMenuItem(i18n.T("delete_instance_btn"), deleteBtn.OnTapped),
		)
		if !isRemote {
			menu.Items = append([]*fyne.MenuItem{fyne.NewMenuItem(i18n.T("update_btn"), updateBtn.OnTapped)}, menu.Items...)
		}

		actionsBtn := widget.NewButtonWithIcon("", theme.MenuIcon(), nil)
		actionsBtn.OnTapped = func() {
			position := fyne.CurrentApp().Driver().AbsolutePositionForObject(actionsBtn)
			widget.ShowPopUpMenuAtPosition(menu, ui.window.Canvas(), position)
		}

		var actions fyne.CanvasObject = container.NewBorder(nil, nil, nil, actionsBtn, playBtn)

		detailContainer := container.NewVBox(
			container.NewHBox(largeIcon, detailTitle),
			detailVersion,
			detailPlayTime,
			widget.NewSeparator(),
			description,
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

	// Launcher section
	memoryEntry := widget.NewEntry()
	memoryEntry.SetText(strconv.FormatUint(ui.config.MaxMemory, 10))
	memoryEntry.OnChanged = func(s string) {
		val, err := strconv.ParseUint(s, 10, 64)
		if err == nil {
			ui.config.MaxMemory = val
			_ = ui.config.Save(resource.DataDir)
		}
	}

	launcherSettings := container.NewVBox(
		widget.NewLabel(i18n.T("max_memory_label")),
		memoryEntry,
	)

	return container.NewVBox(
		widget.NewLabelWithStyle(i18n.T("settings_title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		widget.NewLabelWithStyle(i18n.T("account_section_title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewPadded(accountInfo),
		widget.NewSeparator(),
		widget.NewLabelWithStyle(i18n.T("launcher_section_title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewPadded(launcherSettings),
		layout.NewSpacer(),
		container.NewPadded(logoutBtn),
	)
}

func (ui *FyneUI) showLaunchOverlay() (context.Context, func()) {
	multiProg := NewMultiProgress(i18n.T("preparing"))

	ctx, cancel := context.WithCancel(context.Background())

	stopBtn := widget.NewButton(i18n.T("stop_btn"), func() {
		cancel()
		if err := ui.runner.Stop(); err != nil {
			slog.Error("failed to stop game runner", "err", err)
		}
	})
	stopBtn.Importance = widget.DangerImportance

	logWrapper := container.NewStack(widget.NewLabel("Waiting for logs..."))

	go func() {
		pChan := ui.runner.SubscribeProgress()

		logReaderReady := false
		for {
			select {
			case <-ctx.Done():
				return
			case p, ok := <-pChan:
				if !ok {
					continue
				}
				fyne.Do(func() {
					multiProg.Update(p)
				})

				// Once game starts (Setup is finished), open the log reader
				if p.IsFinished && !logReaderReady {
					lr, err := ui.runner.GetLogReader()
					if err == nil {
						mv := NewMmapLogView(lr)
						if mv != nil {
							fyne.Do(func() {
								logWrapper.Objects[0] = mv
								logWrapper.Refresh()
							})
							logReaderReady = true
						}
					}
				}
			}
		}
	}()

	content := container.NewBorder(
		container.NewVBox(createHeader(), container.NewPadded(multiProg)),
		container.NewPadded(stopBtn),
		nil,
		nil,
		logWrapper,
	)

	ui.window.SetContent(content)

	return ctx, func() {
		cancel()
	}
}

func (ui *FyneUI) checkForInstanceUpdate(uid uuid.UUID) {
	if ui.checkingUpdate[uid] {
		return
	}
	ui.checkingUpdate[uid] = true
	go func() {
		available, err := ui.instances.CheckUpdate(context.Background(), uid)
		fyne.Do(func() {
			ui.checkingUpdate[uid] = false
			if err == nil {
				ui.instanceUpdateAvailable[uid] = available
				if uid == ui.selectedInstanceUID {
					ui.showMainView()
				}
			}
		})
	}()
}

func formatPlayTime(seconds int64) string {
	if seconds == 0 {
		return "0m"
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
