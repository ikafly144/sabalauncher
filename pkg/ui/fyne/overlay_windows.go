//go:build windows

package fyne

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/gonutz/w32/v2"
)

func (ui *FyneUI) showOverlayNotification(title, message string, duration time.Duration) {
	fyne.Do(func() {
		w := ui.app.Driver().(desktop.Driver).CreateSplashWindow()
		w.SetFixedSize(true)
		w.SetPadded(false)

		titleLabel := widget.NewLabelWithStyle(title, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
		msgLabel := widget.NewLabel(message)
		msgLabel.Wrapping = fyne.TextWrapWord

		// Background: pure Black will be keyed out to be fully transparent
		bg := canvas.NewRectangle(color.Black)

		// Visible notification area (dark gray)
		notificationBg := canvas.NewRectangle(color.NRGBA{R: 30, G: 30, B: 30, A: 10})

		content := container.NewStack(
			bg,
			container.NewStack(
				notificationBg,
				container.NewPadded(container.NewVBox(
					titleLabel,
					msgLabel,
				)),
			),
		)

		w.SetContent(content)
		w.Resize(fyne.NewSize(300, 100))

		// Show first, then apply native styles to ensure they stick
		w.Show()

		if native, ok := w.(driver.NativeWindow); ok {
			native.RunNative(func(ctx any) {
				if winCtx, ok := ctx.(driver.WindowsWindowContext); ok {
					hwnd := w32.HWND(winCtx.HWND)

					// 1. Borderless Style (WS_POPUP)
					w32.SetWindowLongPtr(hwnd, w32.GWL_STYLE, w32.WS_POPUP|w32.WS_VISIBLE)

					// 2. Extended Style (Topmost, Layered, Click-through, No taskbar)
					w32.SetWindowLongPtr(hwnd, w32.GWL_EXSTYLE, w32.WS_EX_TOPMOST|w32.WS_EX_LAYERED|w32.WS_EX_TRANSPARENT|w32.WS_EX_TOOLWINDOW)

					// 3. Color Keying: Make pure Black (0,0,0) completely transparent
					w32.SetLayeredWindowAttributes(hwnd, 0, 0, w32.LWA_COLORKEY)

					// 4. Position and Force Topmost
					screenWidth := w32.GetSystemMetrics(w32.SM_CXSCREEN)
					w32.SetWindowPos(hwnd, w32.HWND_TOPMOST, int(screenWidth)-320, 50, 300, 100, w32.SWP_SHOWWINDOW)
				}
			})
		}

		go func() {
			time.Sleep(duration)
			fyne.Do(func() {
				w.Close()
			})
		}()
	})
}
