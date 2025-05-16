package feature

import (
	"image/color"
	"io"
	"launcher/pkg/browser"
	"launcher/pkg/msa"
	"launcher/window"
	"log/slog"
	"strings"
	"sync"

	"gioui.org/app"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
	"github.com/AzureAD/microsoft-authentication-library-for-go/apps/public"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

func NewLoginScreen() *LoginScreen {
	loginScreen := &LoginScreen{}
	loginScreen.w = window.NewWindow("login", loginScreen)

	loginScreen.w.Option(
		app.Decorated(true),
		app.MinSize(240, 100),
		app.MaxSize(240, 100),
		app.Size(240, 100),
		app.Windowed.Option(),
	)

	return loginScreen
}

func (l *LoginScreen) startLogin() {
	session, err := msa.NewSession()
	if err != nil {
		return
	}
	l.session = session

	go func(l *LoginScreen) {
		success := false
		defer func() {
			l.w.InjectEvent() <- func() {
				l.loginResult(success)
			}
		}()
		if err := session.StartLogin(); err != nil {
			slog.Error("Login failed", "error", err)
			return
		}
		result, err := l.session.AuthResult()
		if err != nil {
			slog.Error("Failed to get auth result", "error", err)
			return
		}
		l.result = result
		success = true
	}(l)
}

func (l *LoginScreen) loginResult(success bool) {
	l.success = &success
	if success {
		slog.Info("Login successful")
	} else {
		slog.Error("Login failed")
	}
}

var _ window.IFRender = (*LoginScreen)(nil)

type LoginScreen struct {
	w    *window.Window[*LoginScreen]
	init func()

	success *bool
	session msa.Session
	result  *public.AuthResult

	deviceCode component.TextField

	copyClick widget.Clickable
	openClick widget.Clickable
}

func (l *LoginScreen) Init() {
	if l.init == nil {
		l.init = sync.OnceFunc(func() {
			l.deviceCode.SingleLine = true
			l.deviceCode.ReadOnly = true
		})
	}
	l.init()
}

var (
	copyIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.ContentContentCopy)
		if err != nil {
			panic(err)
		}
		return icon
	}()
	successIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.ActionDone)
		if err != nil {
			panic(err)
		}
		return icon
	}()
	failureIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.AlertError)
		if err != nil {
			panic(err)
		}
		return icon
	}()
	openIcon *widget.Icon = func() *widget.Icon {
		icon, err := widget.NewIcon(icons.ActionOpenInNew)
		if err != nil {
			panic(err)
		}
		return icon
	}()
)

// Render implements window.IFRender.
func (l *LoginScreen) Render(ctx *window.RenderContext) layout.Dimensions {

	l.Init()
	return layout.Flex{
		Axis:      layout.Vertical,
		Alignment: layout.Baseline,
		Spacing:   layout.SpaceAround,
	}.Layout(ctx.Gtx,
		// show the device code if available
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {

			if l.copyClick.Clicked(gtx) {
				if l.session != nil && l.session.DeviceCode() != nil {
					slog.Info("copy device code")
					gtx.Execute(clipboard.WriteCmd{
						Data: io.NopCloser(strings.NewReader(l.session.DeviceCode().Result.UserCode)),
					})
				}
			}

			if l.openClick.Clicked(gtx) {
				if l.session != nil && l.session.DeviceCode() != nil {
					slog.Info("open device code")
					_ = browser.Open(l.session.DeviceCode().Result.VerificationURL)
				}
			}

			return layout.Stack{
				Alignment: layout.S,
			}.Layout(gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					if l.session != nil && l.session.DeviceCode() != nil && l.deviceCode.Text() == "" {
						l.deviceCode.SetText(l.session.DeviceCode().Result.UserCode)
					} else if l.session == nil || l.session.DeviceCode() == nil {
						l.deviceCode.SetText("")
					}
					return l.deviceCode.Layout(ctx.Gtx, ctx.Theme, "")
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{
						Axis: layout.Horizontal,
					}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if l.success == nil {
								return layout.Inset{
									Top:    unit.Dp(16),
									Bottom: unit.Dp(16),
									Left:   unit.Dp(16),
									Right:  unit.Dp(16),
								}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									gtx.Constraints.Max.X = gtx.Dp(unit.Dp(24))
									gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(24))
									return material.Loader(ctx.Theme).Layout(gtx)
								})
							}
							if *l.success {
								return layout.Inset{
									Top:    unit.Dp(4),
									Bottom: unit.Dp(4),
									Left:   unit.Dp(16),
									Right:  unit.Dp(16),
								}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									gtx.Constraints.Max.X = gtx.Dp(unit.Dp(24))
									gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(24))
									return successIcon.Layout(gtx, color.NRGBA{0x22, 0xda, 0x5e, 0xff})
								})
							} else {
								return layout.Inset{
									Top:    unit.Dp(4),
									Bottom: unit.Dp(4),
									Left:   unit.Dp(16),
									Right:  unit.Dp(16),
								}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									gtx.Constraints.Max.X = gtx.Dp(unit.Dp(24))
									gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(24))
									return failureIcon.Layout(gtx, color.NRGBA{0xef, 0x53, 0x50, 0xff})
								})
							}
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{
								Top:    unit.Dp(4),
								Bottom: unit.Dp(4),
								Left:   unit.Dp(16),
								Right:  unit.Dp(16),
							}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return component.SimpleIconButton(
									color.NRGBA{0xff, 0xff, 0xff, 0xff},
									color.NRGBA{0x00, 0x00, 0x00, 0xff},
									&l.copyClick,
									copyIcon,
								).Layout(gtx)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{
								Top:    unit.Dp(4),
								Bottom: unit.Dp(4),
								Left:   unit.Dp(16),
								Right:  unit.Dp(16),
							}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return component.SimpleIconButton(
									color.NRGBA{0x79, 0xc0, 0xff, 0xff},
									color.NRGBA{0xff, 0xff, 0xff, 0xff},
									&l.openClick,
									openIcon,
								).Layout(gtx)
							})
						}),
					)
				}),
			)
		}),
	)
}
