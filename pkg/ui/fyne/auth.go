package fyne

import (
	"context"
	"image/color"
	"log/slog"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/ikafly144/sabalauncher/pkg/core"
	"github.com/ikafly144/sabalauncher/pkg/msa"
	"github.com/ikafly144/sabalauncher/pkg/browser"
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

	// Responsive Layout: Header at top, content centered
	ui.window.SetContent(container.NewBorder(
		createHeader(),
		nil, nil, nil,
		container.NewCenter(container.NewPadded(content)),
	))
}

func (ui *FyneUI) createLoggedOutView() fyne.CanvasObject {
	browserLoginBtn := widget.NewButton("Login with Microsoft (Browser)", func() {
		go ui.startLogin(msa.LoginMethodBrowser)
	})
	browserLoginBtn.Importance = widget.HighImportance

	deviceLoginBtn := widget.NewButton("Login with Microsoft (Device Code)", func() {
		go ui.startLogin(msa.LoginMethodDeviceCode)
	})

	content := container.NewVBox(
		widget.NewLabelWithStyle("Welcome to SabaLauncher", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle("Please login to continue", fyne.TextAlignCenter, fyne.TextStyle{}),
		browserLoginBtn,
		deviceLoginBtn,
	)

	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(fyne.NewSize(300, 0))
	return container.NewStack(rect, content)
}

func (ui *FyneUI) createLoggingInView() fyne.CanvasObject {
	url, code := ui.auth.DeviceCode()
	
	var content *fyne.Container
	if url != "" && code != "" {
		content = container.NewVBox(
			widget.NewLabelWithStyle("Logging in...", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewLabel("1. Open: "+url),
			widget.NewLabel("2. Enter code: "+code),
			widget.NewButton("Open Login Page in Browser", func() {
				_ = browser.Open(url)
			}),
			layout.NewSpacer(),
			widget.NewProgressBarInfinite(),
		)
	} else {
		content = container.NewVBox(
			widget.NewLabelWithStyle("Logging in...", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewLabel("Please complete the login in your browser."),
			widget.NewButton("Open Login Page in Browser", func() {
				loginURL := ui.auth.LoginURL()
				if loginURL != "" {
					_ = browser.Open(loginURL)
				}
			}),
			layout.NewSpacer(),
			widget.NewProgressBarInfinite(),
		)
	}

	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(fyne.NewSize(400, 0))
	return container.NewStack(rect, content)
}

func (ui *FyneUI) createLoggedInView() fyne.CanvasObject {
	dashboardBtn := widget.NewButton("Go to Dashboard", func() {
		ui.showMainView()
	})
	dashboardBtn.Importance = widget.HighImportance

	logoutBtn := widget.NewButton("Logout", func() {
		_ = ui.auth.Logout()
		ui.showAuthView()
	})

	content := container.NewVBox(
		widget.NewLabelWithStyle("Logged in as:", fyne.TextAlignCenter, fyne.TextStyle{}),
		widget.NewLabelWithStyle(ui.auth.GetUserDisplay(), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		dashboardBtn,
		logoutBtn,
	)
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(fyne.NewSize(300, 0))
	return container.NewStack(rect, content)
}

func (ui *FyneUI) createErrorView() fyne.CanvasObject {
	retryBrowserBtn := widget.NewButton("Retry Login (Browser)", func() {
		go ui.startLogin(msa.LoginMethodBrowser)
	})
	retryBrowserBtn.Importance = widget.HighImportance

	retryDeviceBtn := widget.NewButton("Retry Login (Device Code)", func() {
		go ui.startLogin(msa.LoginMethodDeviceCode)
	})

	err := ui.auth.GetLastError()
	errMsg := "Something went wrong during login."
	if err != nil {
		errMsg = err.Error()
	}

	errLabel := widget.NewLabelWithStyle(errMsg, fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	errLabel.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(
		widget.NewLabelWithStyle("Authentication Error", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		errLabel,
		retryBrowserBtn,
		retryDeviceBtn,
	)
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(fyne.NewSize(400, 0))
	return container.NewStack(rect, content)
}

func (ui *FyneUI) startLogin(method msa.LoginMethod) {
	ctx := context.Background()
	if err := ui.auth.Login(ctx, method); err != nil {
		slog.Error("Failed to start login", "error", err, "method", method)
		fyne.Do(func() {
			ui.showAuthView()
		})
		return
	}

	fyne.Do(func() {
		ui.showAuthView() // Update to LoggingIn state
	})

	if err := ui.auth.WaitLogin(ctx); err != nil {
		slog.Error("Failed to complete login", "error", err, "method", method)
		fyne.Do(func() {
			ui.showAuthView()
		})
		return
	}

	fyne.Do(func() {
		ui.showMainView() // Update to Main View on success
	})
}
