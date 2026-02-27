package fyne

import (
	"context"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/core"
)

func (ui *FyneUI) showAuthView() {
	status := ui.auth.GetStatus()
	
	var content fyne.CanvasObject
	
	switch status {
	case core.AuthStatusLoggedOut:
		content = ui.createLoggedOutView()
	case core.AuthStatusLoggingIn:
		content = ui.createLoggingInView()
	case core.AuthStatusLoggedIn:
		content = ui.createLoggedInView()
	case core.AuthStatusError:
		content = ui.createErrorView()
	}
	
	ui.window.SetContent(container.NewVBox(
		createHeader(),
		content,
	))
}

func (ui *FyneUI) createLoggedOutView() fyne.CanvasObject {
	loginBtn := widget.NewButton("Login with Microsoft", func() {
		go ui.startLogin()
	})
	
	return container.NewVBox(
		widget.NewLabel("Not logged in"),
		loginBtn,
	)
}

func (ui *FyneUI) createLoggingInView() fyne.CanvasObject {
	url, code := ui.auth.DeviceCode()
	return container.NewVBox(
		widget.NewLabel("Logging in..."),
		widget.NewLabel("Go to: "+url),
		widget.NewLabel("Enter code: "+code),
		widget.NewProgressBarInfinite(),
	)
}

func (ui *FyneUI) createLoggedInView() fyne.CanvasObject {
	profilesBtn := widget.NewButton("Go to Profiles", func() {
		ui.showProfileView()
	})
	
	logoutBtn := widget.NewButton("Logout", func() {
		_ = ui.auth.Logout()
		ui.showAuthView()
	})
	
	return container.NewVBox(
		widget.NewLabel("Logged in as: "+ui.auth.GetUserDisplay()),
		profilesBtn,
		logoutBtn,
	)
}

func (ui *FyneUI) createErrorView() fyne.CanvasObject {
	retryBtn := widget.NewButton("Retry", func() {
		go ui.startLogin()
	})
	
	return container.NewVBox(
		widget.NewLabel("Authentication Error"),
		retryBtn,
	)
}

func (ui *FyneUI) startLogin() {
	ctx := context.Background()
	if err := ui.auth.Login(ctx); err != nil {
		ui.showAuthView()
		return
	}
	
	ui.showAuthView() // Update to LoggingIn state
	
	if err := ui.auth.WaitLogin(ctx); err != nil {
		ui.showAuthView()
		return
	}
	
	ui.showAuthView() // Update to LoggedIn state
}
