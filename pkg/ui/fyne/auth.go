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
	"github.com/ikafly144/sabalauncher/pkg/browser"
	"github.com/ikafly144/sabalauncher/pkg/core"
	"github.com/ikafly144/sabalauncher/pkg/i18n"
	"github.com/ikafly144/sabalauncher/pkg/msa"
	"github.com/skip2/go-qrcode"
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
	browserLoginBtn := widget.NewButton(i18n.T("login_browser"), func() {
		go ui.startLogin(msa.LoginMethodBrowser)
	})
	browserLoginBtn.Importance = widget.HighImportance

	deviceLoginBtn := widget.NewButton(i18n.T("login_device"), func() {
		go ui.startLogin(msa.LoginMethodDeviceCode)
	})

	content := container.NewVBox(
		widget.NewLabelWithStyle(i18n.T("welcome_title"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(i18n.T("login_prompt"), fyne.TextAlignCenter, fyne.TextStyle{}),
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
		qrCode, err := qrcode.Encode(url, qrcode.Medium, 256)
		var qrImage fyne.CanvasObject
		if err == nil {
			img := canvas.NewImageFromResource(fyne.NewStaticResource("qr.png", qrCode))
			img.SetMinSize(fyne.NewSize(200, 200))
			img.FillMode = canvas.ImageFillContain
			qrImage = img
		} else {
			qrImage = widget.NewLabel("Failed to generate QR code")
		}

		content = container.NewVBox(
			widget.NewLabelWithStyle(i18n.T("logging_in"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			container.NewCenter(qrImage),
			widget.NewLabel(i18n.T("device_code_step1", url)),
			widget.NewLabel(i18n.T("device_code_step2", code)),
			widget.NewButton(i18n.T("open_browser_btn"), func() {
				_ = browser.Open(url)
			}),
			layout.NewSpacer(),
			widget.NewProgressBarInfinite(),
		)
	} else {
		content = container.NewVBox(
			widget.NewLabelWithStyle(i18n.T("logging_in"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewLabel(i18n.T("browser_login_prompt")),
			widget.NewButton(i18n.T("open_browser_btn"), func() {
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
	dashboardBtn := widget.NewButton(i18n.T("go_to_dashboard"), func() {
		ui.showMainView()
	})
	dashboardBtn.Importance = widget.HighImportance

	logoutBtn := widget.NewButton(i18n.T("logout"), func() {
		_ = ui.auth.Logout()
		ui.showAuthView()
	})

	content := container.NewVBox(
		widget.NewLabelWithStyle(i18n.T("logged_in_as"), fyne.TextAlignCenter, fyne.TextStyle{}),
		widget.NewLabelWithStyle(ui.auth.GetUserDisplay(), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		dashboardBtn,
		logoutBtn,
	)
	rect := canvas.NewRectangle(color.Transparent)
	rect.SetMinSize(fyne.NewSize(300, 0))
	return container.NewStack(rect, content)
}

func (ui *FyneUI) createErrorView() fyne.CanvasObject {
	retryBrowserBtn := widget.NewButton(i18n.T("retry_browser"), func() {
		go ui.startLogin(msa.LoginMethodBrowser)
	})
	retryBrowserBtn.Importance = widget.HighImportance

	retryDeviceBtn := widget.NewButton(i18n.T("retry_device"), func() {
		go ui.startLogin(msa.LoginMethodDeviceCode)
	})

	err := ui.auth.GetLastError()
	errMsg := i18n.T("default_login_error")
	if err != nil {
		errMsg = err.Error()
	}

	errLabel := widget.NewLabelWithStyle(errMsg, fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
	errLabel.Wrapping = fyne.TextWrapWord

	content := container.NewVBox(
		widget.NewLabelWithStyle(i18n.T("auth_error_title"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
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
