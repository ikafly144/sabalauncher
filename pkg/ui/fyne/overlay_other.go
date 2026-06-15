//go:build !windows

package fyne

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func (ui *FyneUI) showOverlayNotification(title, message string, duration time.Duration) {
	fyne.Do(func() {
		// Simple fallback for non-Windows platforms using a regular window or toast
		w := ui.app.NewWindow(title)
		w.SetContent(widget.NewLabel(message))
		w.Resize(fyne.NewSize(300, 100))
		w.Show()

		go func() {
			time.Sleep(duration)
			fyne.Do(func() {
				w.Close()
			})
		}()
	})
}
