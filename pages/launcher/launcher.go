package launcher

import (
	"archive/zip"
	"fmt"
	"image"
	"image/color"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/ikafly144/sabalauncher/applayout"
	"github.com/ikafly144/sabalauncher/icon"
	"github.com/ikafly144/sabalauncher/pages"
	"github.com/ikafly144/sabalauncher/pkg/msa"
	"github.com/ikafly144/sabalauncher/pkg/resource"

	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
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
	Profiles []Profile

	modal           *component.ModalLayer
	modalDrag       gesture.Drag
	addProfileInput component.TextField
	addProfileBtn   widget.Clickable
	addProfileList  widget.List

	playModal     *component.ModalLayer
	playModalDrag gesture.Drag
	success       *bool
	booted        bool
	bootError     error
	saveLogButton widget.Clickable
	saveLog       atomic.Bool
	saveError     error
	confirmBtn    *widget.Clickable
	// playStatus        string
	// playDownloadTotal int
	// worker            *resource.DownloadWorker
	// playResult        *bool

	profileList widget.List
	downloadBtn widget.Clickable
	updateBtn   widget.Clickable

	loading sync.Mutex
	widget.List
	*pages.Router
}

type Profile struct {
	resource.Profile
	playBtn     widget.Clickable
	deleteBtn   widget.Clickable
	reloadBtn   widget.Clickable
	menuInit    bool
	menu        component.MenuState
	contextArea component.ContextArea
}

var _ pages.Page = (*Page)(nil)

func New(router *pages.Router) *Page {
	modal := component.NewModal()
	playModal := component.NewModal()

	p := &Page{
		Router:    router,
		modal:     modal,
		playModal: playModal,
	}
	go p.loadProfiles()
	return p
}

func (p *Page) Actions() []component.AppBarAction {
	return []component.AppBarAction{
		{
			OverflowAction: component.OverflowAction{
				Name: "プロファイルを更新",
				Tag:  "update",
			},
			Layout: func(gtx C, bg, fg color.NRGBA) D {
				for p.updateBtn.Clicked(gtx) {
					go p.loadProfiles()
				}
				return component.SimpleIconButton(bg, fg, &p.updateBtn, icon.UpdateIcon).Layout(gtx)
			},
		},
	}
}

func (p *Page) Overflow() []component.OverflowAction {
	return []component.OverflowAction{}
}

func (p *Page) NavItem() component.NavItem {
	return component.NavItem{
		Name: "プロファイル",
		Icon: icon.LauncherIcon,
	}
}

func (p *Page) Layout(gtx C, th *material.Theme) D {
	gtx.Execute(op.InvalidateCmd{})
	if p.saveLogButton.Clicked(gtx) && !p.saveLog.Load() {
		go func() {
			p.saveLog.Store(true)
			defer p.saveLog.Store(false)

			file, err := p.Router.Explorer.CreateFile("launcher_log.zip")
			if err != nil {
				slog.Error("Failed to create log file", "error", err)
				p.saveError = err
				return
			}
			defer file.Close()

			dir := os.DirFS(filepath.Join(resource.DataDir, "log"))
			zip.NewWriter(file)
			zw := zip.NewWriter(file)
			defer func() {
				if err := zw.Close(); err != nil {
					slog.Error("Failed to close zip writer", "error", err)
					p.saveError = err
				}
			}()
			if err := zw.AddFS(dir); err != nil {
				slog.Error("Failed to add files to zip", "error", err)
				p.saveError = err
				return
			}
			if err := zw.Flush(); err != nil {
				slog.Error("Failed to flush zip writer", "error", err)
				p.saveError = err
				return
			}
		}()
	}
	if p.confirmBtn != nil && (p.confirmBtn.Clicked(gtx) || !p.playModal.Visible()) {
		p.confirmBtn = nil
	}
	p.List.Axis = layout.Vertical
	layout.Flex{
		Alignment: layout.Middle,
		Axis:      layout.Vertical,
	}.Layout(gtx,
		layout.Rigid(func(gtx C) D {
			gtx.Constraints.Max.Y -= gtx.Dp(60)
			gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
			p.profileList.Axis = layout.Vertical
			dims := material.List(th, &p.profileList).Layout(gtx, len(p.Profiles), func(gtx C, index int) D {
				if !p.Profiles[index].menuInit {
					p.Profiles[index].menuInit = true
					p.Profiles[index].menu = component.MenuState{
						Options: []func(gtx C) D{
							component.MenuItem(th, &p.Profiles[index].deleteBtn, "プロファイルを削除").Layout,
							component.MenuItem(th, &p.Profiles[index].reloadBtn, "キャッシュを削除").Layout,
						},
					}
				}
				for p.Profiles[index].deleteBtn.Clicked(gtx) {
					go p.removeProfileSource(p.Profiles[index].Source)
				}
				for p.Profiles[index].reloadBtn.Clicked(gtx) {
					go func() {
						if err := p.Profiles[index].DeleteManifestCache(p.Profiles[index].Path); err != nil {
							slog.Error("Failed to delete manifest cache", "error", err)
						}
					}()
				}
				return layout.Background{}.Layout(gtx,
					func(gtx C) D {
						return layout.Inset{
							Top:    unit.Dp(4),
							Bottom: unit.Dp(4),
							Left:   unit.Dp(4),
							Right:  unit.Dp(12),
						}.Layout(gtx, component.Rect{
							Color: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xFF},
							Size:  gtx.Constraints.Min,
						}.Layout)
					},
					func(gtx C) D {
						return (layout.Stack{}.Layout(gtx,
							layout.Stacked(func(gtx C) D {
								return layout.Flex{
									Alignment: layout.Middle,
									Axis:      layout.Vertical,
								}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										gtx.Constraints.Min.X = gtx.Constraints.Max.X
										return applayout.DefaultInset.Layout(gtx, func(gtx C) D {
											return layout.Flex{
												Alignment: layout.Start,
												Axis:      layout.Horizontal,
											}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													gtx.Constraints = layout.Exact(image.Pt(gtx.Dp(128), gtx.Dp(128)))
													return p.Profiles[index].GetIcon().Layout(gtx)
												}),
												layout.Rigid(func(gtx C) D {
													return layout.Flex{
														Alignment: layout.Start,
														Axis:      layout.Vertical,
													}.Layout(gtx,
														layout.Rigid(func(gtx C) D {
															return layout.Inset{
																Top:    unit.Dp(16),
																Left:   unit.Dp(16),
																Right:  unit.Dp(16),
																Bottom: unit.Dp(8),
															}.Layout(gtx, material.Label(th, 16, p.Profiles[index].Display()).Layout)
														}),
														layout.Rigid(func(gtx C) D {
															return layout.UniformInset(unit.Dp(8)).Layout(gtx, material.Body2(th, p.Profiles[index].Description).Layout)
														}),
														layout.Rigid(func(gtx C) D {
															return layout.UniformInset(unit.Dp(8)).Layout(gtx, material.Body2(th, "バージョン: "+p.Profiles[index].Manifest.VersionName()).Layout)
														}),
													)
												}),
												layout.Rigid(func(gtx C) D {
													op.Offset(image.Pt(gtx.Constraints.Max.X-gtx.Dp(90), 0)).Add(gtx.Ops)
													return layout.Flex{
														Alignment: layout.Start,
														Axis:      layout.Vertical,
													}.Layout(gtx,
														layout.Rigid(func(gtx C) D {
															if p.Profiles[index].playBtn.Clicked(gtx) {
																p.success = nil
																p.booted = false
																p.bootError = nil
																p.playModal.Appear(gtx.Now)

																memoryOk, err := p.Profiles[index].CheckMemory()
																if err != nil {
																	slog.Error("Failed to check memory", "error", err)
																	p.bootError = err
																	p.success = new(bool)
																}
																if !memoryOk {
																	p.confirmBtn = new(widget.Clickable)
																}

																if err := p.Profiles[index].Fetch(); err != nil {
																	slog.Error("Failed to fetch profile", "error", err)
																}
																p.Profiles[index].Manifest.StartSetup(resource.DataDir, p.Profiles[index].Path)
																p.playModal.Widget = (func(gtx C, __th *material.Theme, anim *component.VisibilityAnimation) D {
																	for {
																		_, ok := p.playModalDrag.Update(gtx.Metric, gtx.Source, gesture.Horizontal)
																		if !ok {
																			break
																		}
																	}
																	__p := __th.Palette
																	if anim.Animating() || anim.State == component.Invisible {
																		revealed := anim.Revealed(gtx)
																		currentAlpha := uint8(float32(255) * revealed)
																		__p = material.Palette{
																			Bg:         component.WithAlpha(__th.Bg, currentAlpha),
																			Fg:         component.WithAlpha(__th.Fg, currentAlpha),
																			ContrastBg: component.WithAlpha(__th.ContrastBg, currentAlpha),
																			ContrastFg: component.WithAlpha(__th.ContrastFg, currentAlpha),
																		}
																	}
																	th := th.WithPalette(__p)
																	if p.Profiles[index].Manifest.IsDone() && p.success == nil && p.confirmBtn == nil {
																		if p.Profiles[index].Manifest.Error() != nil {
																			slog.Error("Failed to start game", "error", p.Profiles[index].Manifest.Error())
																			p.success = new(bool)
																		} else if !p.booted {
																			p.booted = true
																			go func() {
																				success := false
																				defer func() {
																					p.success = &success
																				}()
																				session, err := msa.NewSession(p.Cache)
																				if err != nil {
																					slog.Error("Failed to create session", "error", err)
																					p.bootError = err
																					return
																				}
																				account, err := msa.NewMinecraftAccount(session)
																				if err != nil {
																					slog.Error("Failed to create Minecraft account", "error", err)
																					p.bootError = err
																					return
																				}
																				if err := p.Profiles[index].Manifest.Boot(resource.DataDir, &p.Profiles[index].Profile, account); err != nil {
																					slog.Error("Failed to start game", "error", err)
																					p.bootError = err
																				}
																				success = true
																				p.booted = false
																			}()
																		}
																	}
																	gtx.Execute(op.InvalidateCmd{})
																	return layout.Inset{
																		Top:    unit.Dp(80),
																		Bottom: unit.Dp(80),
																		Left:   unit.Dp(gtx.Metric.PxToDp(gtx.Constraints.Max.X/2) - 200),
																		Right:  unit.Dp(gtx.Metric.PxToDp(gtx.Constraints.Max.X/2) - 200),
																	}.Layout(gtx, func(gtx C) D {
																		for p.success == nil {
																			_, ok := p.playModalDrag.Update(gtx.Metric, gtx.Source, gesture.Horizontal)
																			if !ok {
																				break
																			}
																		}
																		if p.success != nil || p.confirmBtn != nil {
																			pr := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max})
																			defer pr.Push(gtx.Ops).Pop()
																			event.Op(gtx.Ops, p.playModalDrag)
																		}
																		dims := layout.Background{}.Layout(gtx,
																			func(gtx C) D {
																				return component.Rect{
																					Color: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
																					Size:  gtx.Constraints.Max,
																				}.Layout(gtx)
																			},
																			func(gtx C) D {
																				return applayout.DefaultInset.Layout(gtx, func(gtx C) D {
																					return layout.Flex{
																						Alignment: layout.Middle,
																						Axis:      layout.Vertical,
																					}.Layout(gtx,
																						layout.Rigid(func(gtx C) D {
																							return applayout.DefaultInset.Layout(gtx, material.ProgressBar(&th, float32(p.Profiles[index].Manifest.TotalProgress())).Layout)
																						}),
																						layout.Rigid(func(gtx C) D {
																							return applayout.DefaultInset.Layout(gtx, material.ProgressBar(&th, float32(p.Profiles[index].Manifest.CurrentProgress())).Layout)
																						}),
																						layout.Rigid(func(gtx C) D {
																							if p.confirmBtn != nil {
																								return layout.Flex{
																									Axis: layout.Vertical,
																								}.Layout(gtx,
																									layout.Rigid(func(gtx C) D {
																										mem, err := p.Profiles[index].ActualMemory()
																										if err != nil {
																											return material.Caption(&th, fmt.Sprintf("メモリの取得に失敗しました: %v", err)).Layout(gtx)
																										}
																										return layout.UniformInset(unit.Dp(16)).Layout(gtx, material.Label(&th, 16, fmt.Sprintf("推奨割り当てメモリ%dMBに対して、\nシステムの搭載メモリに十分な余裕がないため、%dMBを割り当てて起動します。\n これにより、動作が不安定になる場合があります。", p.Profiles[index].RecommendedMemoryMB, mem)).Layout)
																									}),
																									layout.Rigid(func(gtx C) D {
																										return layout.UniformInset(unit.Dp(16)).Layout(gtx, material.Button(&th, p.confirmBtn, "確認しました").Layout)
																									}),
																								)
																							}
																							if p.bootError != nil {
																								return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx C) D {
																									return layout.Flex{
																										Axis: layout.Vertical,
																									}.Layout(gtx,
																										layout.Rigid(func(gtx C) D {
																											return material.Label(&th, 28, p.bootError.Error()).Layout(gtx)
																										}),
																										layout.Rigid(func(gtx C) D {
																											return material.Button(&th, &p.saveLogButton, "ログを保存").Layout(gtx)
																										}),
																									)
																								})
																							}
																							if p.booted && p.success == nil {
																								return layout.UniformInset(unit.Dp(16)).Layout(gtx, material.Label(&th, 28, "ゲームを起動中").Layout)
																							} else if p.success != nil && *p.success {
																								return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx C) D {
																									return layout.Flex{
																										Axis: layout.Vertical,
																									}.Layout(gtx,
																										layout.Rigid(func(gtx C) D {
																											return material.Label(&th, 28, "ゲームが終了しました").Layout(gtx)
																										}),
																										layout.Rigid(func(gtx C) D {
																											return material.Button(&th, &p.saveLogButton, "ログを保存").Layout(gtx)
																										}),
																									)
																								})
																							}
																							if p.Profiles[index].Manifest.Error() != nil {
																								return layout.UniformInset(unit.Dp(16)).Layout(gtx, material.Label(&th, 28, p.Profiles[index].Manifest.Error().Error()).Layout)
																							}
																							return layout.UniformInset(unit.Dp(16)).Layout(gtx, material.Label(&th, 28, p.Profiles[index].Manifest.CurrentStatus()).Layout)
																						}),
																					)
																				})
																			},
																		)
																		if p.success != nil || p.confirmBtn != nil {
																			defer pointer.PassOp{}.Push(gtx.Ops).Pop()
																			defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
																		} else {
																			p.playModalDrag.Add(gtx.Ops)
																		}
																		return dims
																	})
																})
															}
															return applayout.DefaultInset.Layout(gtx, material.Button(th, &p.Profiles[index].playBtn, "プレイ").Layout)
														}),
													)
												}),
											)
										})
									}),
								)
							}),
							layout.Expanded(func(gtx C) D {
								return p.Profiles[index].contextArea.Layout(gtx, func(gtx C) D {
									gtx.Constraints.Min = image.Point{}
									palette := th.Palette
									palette.Bg = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
									bg := th.WithPalette(palette)
									return component.Menu(&bg, &p.Profiles[index].menu).Layout(gtx)
								})
							}),
						))
					})
			})
			return dims
		}),
		layout.Rigid(func(gtx C) D {
			p.modal.Widget = (func(gtx C, __th *material.Theme, anim *component.VisibilityAnimation) D {
				__p := __th.Palette
				animating := anim.Animating() || anim.State == component.Invisible
				if animating {
					revealed := anim.Revealed(gtx)
					currentAlpha := uint8(float32(255) * revealed)
					if currentAlpha < 10 {
						currentAlpha = 0
					}
					slog.Info("revealed", "revealed", revealed, "currentAlpha", currentAlpha)
					__p = material.Palette{
						Bg:         component.WithAlpha(__th.Bg, currentAlpha),
						Fg:         component.WithAlpha(__th.Fg, currentAlpha),
						ContrastBg: component.WithAlpha(__th.ContrastBg, currentAlpha),
						ContrastFg: component.WithAlpha(__th.ContrastFg, currentAlpha),
					}
				}
				th := th.WithPalette(__p)
				return layout.Inset{
					Top:    unit.Dp(80),
					Bottom: unit.Dp(80),
					Left:   unit.Dp(gtx.Metric.PxToDp(gtx.Constraints.Max.X/2) - 200),
					Right:  unit.Dp(gtx.Metric.PxToDp(gtx.Constraints.Max.X/2) - 200),
				}.Layout(gtx, func(gtx C) D {
					for {
						_, ok := p.modalDrag.Update(gtx.Metric, gtx.Source, gesture.Horizontal)
						if !ok {
							break
						}
					}
					pr := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max})
					defer pr.Push(gtx.Ops).Pop()
					event.Op(gtx.Ops, p)
					dims := layout.Background{}.Layout(gtx,
						func(gtx C) D {
							return component.Rect{
								Color: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
								Size:  gtx.Constraints.Max,
							}.Layout(gtx)
						},
						func(gtx C) D {
							return applayout.DefaultInset.Layout(gtx, func(gtx C) D {
								p.addProfileList.Axis = layout.Vertical
								return layout.Flex{
									Alignment: layout.Middle,
									Axis:      layout.Vertical,
								}.Layout(gtx,
									layout.Rigid(func(gtx C) D {
										gtx.Constraints.Max.Y -= gtx.Dp(64)
										return material.List(&th, &p.addProfileList).Layout(gtx, 1, func(gtx C, _ int) D {
											return layout.Flex{
												Alignment: layout.Middle,
												Axis:      layout.Vertical,
											}.Layout(gtx,
												layout.Rigid(func(gtx C) D {
													return applayout.DefaultInset.Layout(gtx, material.Label(&th, 28, "プロファイルを追加").Layout)
												}),
												layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
												layout.Rigid(func(gtx C) D {
													if animating {
														return D{}
													}
													gtx.Constraints.Max = gtx.Constraints.Max.Sub(image.Pt(gtx.Dp(32), 0))
													p.addProfileInput.SingleLine = true
													p.addProfileInput.InputHint = key.HintURL
													p.addProfileInput.Submit = true
													return p.addProfileInput.Layout(gtx, &th, "追加するプロファイル")
												}),
											)
										})
									}),
									layout.Rigid(func(gtx C) D {
										for p.addProfileBtn.Clicked(gtx) {
											url, err := url.Parse(p.addProfileInput.Text())
											if err != nil || url.Scheme == "" {
												p.addProfileInput.SetError("URLが無効です")
												break
											}

											p.addProfileSource(*url)

											p.modal.Disappear(gtx.Now)
										}
										op.Offset(image.Pt(0, gtx.Constraints.Max.Y-gtx.Dp(64))).Add(gtx.Ops)
										return applayout.DefaultInset.Layout(gtx, material.Button(&th, &p.addProfileBtn, "追加").Layout)
									}),
								)
							})
						},
					)
					defer pointer.PassOp{}.Push(gtx.Ops).Pop()
					defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
					p.modalDrag.Add(gtx.Ops)
					return dims
				})
			})
			if p.downloadBtn.Clicked(gtx) {
				p.addProfileInput.Clear()
				p.modal.Appear(gtx.Now)
			}
			return applayout.DefaultInset.Layout(gtx, material.Button(th, &p.downloadBtn, "プロファイルを追加").Layout)
		}),
	)
	p.playModal.Layout(gtx, th)
	p.modal.Layout(gtx, th)
	return D{
		Size: gtx.Constraints.Max,
	}
}
