package fyne

import (
	"context"
	"errors"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/v2/pkg/browser"
	"github.com/ikafly144/sabalauncher/v2/pkg/i18n"
)

func (ui *FyneUI) showImportModpackDialog() {
	// Attempt to get HWND. On Windows, Fyne uses GLFW.
	// We pass 0 and let the browser package handle it if needed.
	path, err := browser.SelectFile(0, "SBPack files (*.sbpack)|*.sbpack")
	if err != nil {
		dialog.ShowError(err, ui.window)
		return
	}
	if path == "" {
		return // Canceled
	}

	// Show progress or immediate import
	minWidth := canvas.NewRectangle(color.Transparent)
	minWidth.SetMinSize(fyne.NewSize(400, 0))
	multiProg := NewMultiProgress(i18n.T("importing_progress"))

	ctx, cancel := context.WithCancel(context.Background())

	progress := dialog.NewCustom(i18n.T("importing_progress"), i18n.T("cancel"), container.NewStack(minWidth, multiProg), ui.window)
	progress.SetOnClosed(func() {
		cancel()
	})
	progress.Show()

	go func() {
		pChan := ui.instances.SubscribeProgress()
		done := make(chan bool)
		go func() {
			for {
				select {
				case p := <-pChan:
					fyne.Do(func() {
						multiProg.Update(p)
					})
				case <-done:
					return
				}
			}
		}()

		err := ui.instances.ImportInstance(ctx, path)
		done <- true
		fyne.Do(progress.Hide)
		if err != nil {
			fyne.Do(func() {
				if !errors.Is(err, context.Canceled) {
					dialog.ShowError(err, ui.window)
				}
			})
		} else {
			fyne.Do(func() {
				ui.showMainView()
			})
		}
	}()
}

func (ui *FyneUI) showRegisterRemoteModpackDialog() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("https://repository.example/repo/manifest.json")

	items := []*widget.FormItem{
		widget.NewFormItem(i18n.T("manifest_url_label"), entry),
	}

	d := dialog.NewForm(i18n.T("register_remote_title"), i18n.T("register_btn"), i18n.T("cancel"), items, func(ok bool) {
		if ok {
			minWidth := canvas.NewRectangle(color.Transparent)
			minWidth.SetMinSize(fyne.NewSize(400, 0))

			multiProg := NewMultiProgress(i18n.T("registering_progress"))

			ctx, cancel := context.WithCancel(context.Background())

			d := dialog.NewCustom(i18n.T("register_remote_title"), i18n.T("cancel"), container.NewStack(minWidth, multiProg), ui.window)
			d.SetOnClosed(func() {
				cancel()
			})
			d.Show()

			go func() {
				pChan := ui.instances.SubscribeProgress()
				done := make(chan bool)
				go func() {
					for {
						select {
						case p := <-pChan:
							fyne.Do(func() {
								multiProg.Update(p)
							})
						case <-done:
							return
						}
					}
				}()

				err := ui.instances.AddRemoteInstance(ctx, entry.Text)
				done <- true
				fyne.Do(d.Hide)
				if err != nil {
					fyne.Do(func() {
						if !errors.Is(err, context.Canceled) {
							dialog.ShowError(err, ui.window)
						}
					})
				} else {
					fyne.Do(func() {
						ui.showMainView()
					})
				}
			}()
		}
	}, ui.window)

	d.Resize(fyne.NewSize(500, 200))
	d.Show()
}

func (ui *FyneUI) showUpdateInstanceDialog(instanceID uuid.UUID) {
	path, err := browser.SelectFile(0, "Update files (*.sbpatch, *.sbpack)|*.sbpatch;*.sbpack")
	if err != nil {
		dialog.ShowError(err, ui.window)
		return
	}
	if path == "" {
		return // Canceled
	}

	ui.startUpdate(instanceID, path)
}

func (ui *FyneUI) startUpdate(instanceID uuid.UUID, path string) {
	minWidth := canvas.NewRectangle(color.Transparent)
	minWidth.SetMinSize(fyne.NewSize(400, 0))
	multiProg := NewMultiProgress(i18n.T("updating_progress"))

	ctx, cancel := context.WithCancel(context.Background())

	progress := dialog.NewCustom(i18n.T("updating_progress"), i18n.T("cancel"), container.NewStack(minWidth, multiProg), ui.window)
	progress.SetOnClosed(func() {
		cancel()
	})
	progress.Show()

	go func() {
		pChan := ui.instances.SubscribeProgress()
		done := make(chan bool)
		go func() {
			for {
				select {
				case p := <-pChan:
					fyne.Do(func() {
						multiProg.Update(p)
					})
				case <-done:
					return
				}
			}
		}()

		err := ui.instances.UpdateInstance(ctx, instanceID, path)
		done <- true
		fyne.Do(progress.Hide)
		if err != nil {
			fyne.Do(func() {
				if !errors.Is(err, context.Canceled) {
					dialog.ShowError(err, ui.window)
				}
			})
		} else {
			fyne.Do(func() {
				ui.showMainView()
			})
		}
	}()
}
