package feature

import (
	"image/color"

	"github.com/ikafly144/sabalauncher/window"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

func NewMainWindow() *MainWindow {
	w := &MainWindow{
		list: &widget.List{
			List: layout.List{
				Axis: layout.Vertical,
			},
		},
	}
	w.Window = window.NewWindow("main", w)
	return w
}

var _ window.IFRender = (*MainWindow)(nil)

type MainWindow struct {
	Window *window.Window[*MainWindow]

	loginScreen *LoginScreen

	list *widget.List
}

var (
	loginButton = new(widget.Clickable)
)

var (
	login *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.ActionAccountCircle)
		if err != nil {
			panic(err)
		}
		return icon
	}()
)

// Render implements window.IFRender.
func (m *MainWindow) Render(ctx *window.RenderContext) layout.Dimensions {
	return layout.Flex{
		Axis:      layout.Vertical,
		Spacing:   10,
		Alignment: layout.Start,
	}.Layout(ctx.Gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{
				Top:    10,
				Bottom: 10,
				Left:   10,
				Right:  10,
			}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				for loginButton.Clicked(gtx) {
					m.openLoginScreen()
				}
				return material.ButtonLayout(ctx.Theme, loginButton).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					th := ctx.Theme.WithPalette(material.Palette{
						Fg: color.NRGBA{0xff, 0xff, 0xff, 0xff},
					})
					return layout.Inset{
						Top:    8,
						Right:  5,
						Left:   5,
						Bottom: 8,
					}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{
							Axis:      layout.Horizontal,
							Spacing:   layout.SpaceAround,
							Alignment: layout.Start,
						}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{
									Top: 4,
								}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return login.Layout(gtx, color.NRGBA{0x79, 0xc0, 0xff, 0xff})
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return material.Label(&th, gtx.Metric.PxToSp(24), "Microsoft Login").Layout(gtx)
							}),
						)
					})
				})
			})
		}),
	)
}

func (m *MainWindow) openLoginScreen() {
	if m.loginScreen == nil {
		m.loginScreen = NewLoginScreen()
		m.loginScreen.w.HookAsync()
	}
	m.loginScreen.w.HandleClose(func() {
		m.loginScreen = nil
	})
	go m.loginScreen.startLogin()
}
