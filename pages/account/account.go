package account

import (
	"fmt"
	"image/color"
	"io"
	"log/slog"
	"strings"

	"github.com/ikafly144/sabalauncher/applayout"
	"github.com/ikafly144/sabalauncher/icon"
	"github.com/ikafly144/sabalauncher/pages"
	"github.com/ikafly144/sabalauncher/pkg/browser"
	"github.com/ikafly144/sabalauncher/pkg/msa"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gioui.org/x/component"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

type Page struct {
	loginBtn widget.Clickable

	copyClick, openClick widget.Clickable

	success  *bool
	session  msa.Session
	loginErr error

	deviceCode component.TextField

	accountName *string

	widget.List
	*pages.Router
}

var _ pages.Page = (*Page)(nil)

func New(router *pages.Router) *Page {
	p := &Page{
		Router: router,
	}
	go refreshAccount(p)
	return p
}

func refreshAccount(p *Page) {
	session, err := msa.NewSession(p.Cache)
	if err != nil {
		slog.Error("Failed to create session", "error", err)
		return
	}
	a, err := msa.NewMinecraftAccount(session)
	if err != nil {
		slog.Error("Failed to create Minecraft account", "error", err)
		return
	}
	ma, err := a.GetMinecraftAccount()
	if err != nil {
		slog.Error("Failed to get Minecraft account", "error", err)
		return
	}
	profile, err := ma.GetMinecraftProfile()
	if err != nil {
		slog.Error("Failed to get Minecraft profile", "error", err)
		return
	}
	p.accountName = &profile.Username
}

func (p *Page) Actions() []component.AppBarAction {
	return []component.AppBarAction{}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "アカウント",
		Icon: icon.AccountIcon,
	}
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	p.List.Axis = layout.Vertical
	return material.List(th, &p.List).Layout(gtx, 1, func(gtx C, _ int) D {
		return layout.Flex{
			Axis: layout.Vertical,
		}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top:    unit.Dp(16),
					Bottom: unit.Dp(16),
					Left:   unit.Dp(16),
					Right:  unit.Dp(16),
				}.Layout(gtx, func(gtx C) D {
					if p.success != nil && !*p.success {
						if p.loginErr == nil {
							return material.Label(th, 24, "アカウント: ログインに失敗しました").Layout(gtx)
						}
						return material.Label(th, 24, fmt.Sprintf("アカウント: ログインに失敗しました (%s)", p.loginErr.Error())).Layout(gtx)
					}
					if p.accountName == nil {
						return material.Label(th, 24, "アカウント: ログインしていません").Layout(gtx)
					}
					return material.Label(th, 24, fmt.Sprintf("アカウント: ログイン済み (%s)", *p.accountName)).Layout(gtx)
				})
			}),
			layout.Rigid(func(gtx C) D {
				if p.loginBtn.Clicked(gtx) && p.session == nil {
					p.startLogin()
				}
				gtx.Execute(op.InvalidateCmd{})
				if p.accountName != nil {
					return applayout.DefaultInset.Layout(gtx, material.Button(th, &p.loginBtn, "別のアカウントでログインする").Layout)
				}
				if p.session == nil {
					return applayout.DefaultInset.Layout(gtx, material.Button(th, &p.loginBtn, "ログインする").Layout)
				}
				return applayout.DefaultInset.Layout(gtx, material.Button(th, &p.loginBtn, "ログイン中...").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return applayout.DefaultInset.Layout(gtx, material.Body1(th, "Microsoftアカウントでログインするには、以下の手順に従ってください:").Layout)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{
					Top:    unit.Dp(16),
					Bottom: unit.Dp(16),
					Left:   unit.Dp(16),
					Right:  unit.Dp(16),
				}.Layout(gtx, func(gtx C) D {
					return material.Label(th, 16, "1. 上のボタンをクリックして、Microsoftアカウントでログインを開始します。\n2. 表示されたデバイスコードをコピーします。\n3. 下の水色のボタンをクリックしてログインページにアクセスし、デバイスコードを入力します。\n4. ログインが成功すると、アカウント情報が表示されます。").Layout(gtx)
				})
			}),

			// show the device code if available
			layout.Rigid(func(gtx C) D {
				return layout.Flex{
					Axis:      layout.Vertical,
					Alignment: layout.Start,
				}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return layout.Flex{
							Axis: layout.Horizontal,
						}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.UniformInset(unit.Dp(16)).Layout(gtx, material.Label(th, 16, "Device Code:").Layout)
							}),
							layout.Rigid(func(gtx C) D {
								gtx.Constraints.Max.X = gtx.Dp(unit.Dp(16 * 9))
								if p.session != nil && p.session.DeviceCode() != nil && p.deviceCode.Text() == "" {
									p.deviceCode.SetText(p.session.DeviceCode().Result.UserCode)
								} else if p.session == nil || p.session.DeviceCode() == nil {
									p.deviceCode.SetText("")
								}
								p.deviceCode.SingleLine = true
								p.deviceCode.ReadOnly = true
								return p.deviceCode.Layout(gtx, th, "")
							}),
							layout.Rigid(func(gtx C) D {
								if p.success == nil {
									if p.session == nil || p.session.DeviceCode() == nil {
										return layout.Inset{
											Top:    unit.Dp(16),
											Bottom: unit.Dp(16),
											Left:   unit.Dp(16),
											Right:  unit.Dp(16),
										}.Layout(gtx, func(gtx C) D {
											return layout.Spacer{Width: unit.Dp(24), Height: unit.Dp(24)}.Layout(gtx)
										})
									}
									return layout.Inset{
										Top:    unit.Dp(16),
										Bottom: unit.Dp(16),
										Left:   unit.Dp(16),
										Right:  unit.Dp(16),
									}.Layout(gtx, func(gtx C) D {
										gtx.Constraints.Max.X = gtx.Dp(unit.Dp(24))
										gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(24))
										return material.Loader(th).Layout(gtx)
									})
								}
								if *p.success {
									return layout.Inset{
										Top:    unit.Dp(16),
										Bottom: unit.Dp(16),
										Left:   unit.Dp(16),
										Right:  unit.Dp(16),
									}.Layout(gtx, func(gtx C) D {
										gtx.Constraints.Max.X = gtx.Dp(unit.Dp(24))
										gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(24))
										return icon.SuccessIcon.Layout(gtx, color.NRGBA{0x22, 0xda, 0x5e, 0xff})
									})
								} else {
									return layout.Inset{
										Top:    unit.Dp(16),
										Bottom: unit.Dp(16),
										Left:   unit.Dp(16),
										Right:  unit.Dp(16),
									}.Layout(gtx, func(gtx C) D {
										gtx.Constraints.Max.X = gtx.Dp(unit.Dp(24))
										gtx.Constraints.Max.Y = gtx.Dp(unit.Dp(24))
										return icon.FailureIcon.Layout(gtx, color.NRGBA{0xef, 0x53, 0x50, 0xff})
									})
								}
							}),
							layout.Rigid(func(gtx C) D {
								return layout.Inset{
									Top:    unit.Dp(4),
									Bottom: unit.Dp(4),
									Left:   unit.Dp(16),
									Right:  unit.Dp(16),
								}.Layout(gtx, func(gtx C) D {
									if p.copyClick.Clicked(gtx) {
										if p.session != nil && p.session.DeviceCode() != nil {
											gtx.Execute(clipboard.WriteCmd{
												Data: io.NopCloser(strings.NewReader(p.session.DeviceCode().Result.UserCode)),
											})
										}
									}
									return component.SimpleIconButton(
										color.NRGBA{0xff, 0xff, 0xff, 0xff},
										color.NRGBA{0x00, 0x00, 0x00, 0xff},
										&p.copyClick,
										icon.CopyIcon,
									).Layout(gtx)
								})
							}),
							layout.Rigid(func(gtx C) D {
								return layout.Inset{
									Top:    unit.Dp(4),
									Bottom: unit.Dp(4),
									Left:   unit.Dp(16),
									Right:  unit.Dp(16),
								}.Layout(gtx, func(gtx C) D {
									if p.openClick.Clicked(gtx) {
										if p.session != nil && p.session.DeviceCode() != nil {
											_ = browser.Open(p.session.DeviceCode().Result.VerificationURL)
										}
									}
									return component.SimpleIconButton(
										color.NRGBA{0x79, 0xc0, 0xff, 0xff},
										color.NRGBA{0xff, 0xff, 0xff, 0xff},
										&p.openClick,
										icon.OpenIcon,
									).Layout(gtx)
								})
							}),
						)
					}),
				)
			}),
		)
	})
}
