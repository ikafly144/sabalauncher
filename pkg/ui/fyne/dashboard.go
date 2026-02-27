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
		ui.showLaunchOverlay()
		go func() {
			_ = ui.discord.SetActivity(ui.selectedProfileName)
			if err := ui.runner.Launch(ui.selectedProfileName); err != nil {
				dialog.ShowError(err, ui.window)
				ui.showDashboardView()
			}
			_ = ui.discord.ClearActivity()
		}()
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

func (ui *FyneUI) showLaunchOverlay() {
	taskLabel := widget.NewLabel("Preparing...")
	statusLabel := widget.NewLabel("")
	progressBar := widget.NewProgressBar()
	
	logEntry := widget.NewMultiLineEntry()
	logEntry.Disable()
	logEntry.SetText("Waiting for logs...\n")

	stopBtn := widget.NewButton("STOP", func() {
		_ = ui.runner.Stop()
		ui.showDashboardView()
	})
	stopBtn.Importance = widget.DangerImportance

	progressChan := ui.runner.SubscribeProgress()
	logsChan := ui.runner.SubscribeLogs()

	go func() {
		for event := range progressChan {
			taskLabel.SetText(event.TaskName)
			statusLabel.SetText(event.Status)
			progressBar.SetValue(event.Percentage / 100.0)
			if event.IsFinished {
				// We don't necessarily want to go back to dashboard yet, 
				// as the game might be starting.
			}
		}
	}()

	go func() {
		for entry := range logsChan {
			logLine := "[" + string(entry.Level) + "] " + entry.Message + "\n"
			logEntry.SetText(logEntry.Text + logLine)
			logEntry.CursorColumn = 0
			logEntry.CursorRow = len(logEntry.Text) // Simple scroll to end
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
