package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/ikafly144/sabalauncher/pages"
	"github.com/ikafly144/sabalauncher/pages/account"
	"github.com/ikafly144/sabalauncher/pages/launcher"
	"github.com/ikafly144/sabalauncher/pkg/browser"
	"github.com/ikafly144/sabalauncher/pkg/resource"

	"gioui.org/app"
	"gioui.org/font/gofont"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget/material"
)

type (
	C = layout.Context
	D = layout.Dimensions
)

var (
	appName = "SabaLauncher"
	version = "devel"
	commit  = "unknown"
	branch  = "unknown"
)

const versionUrl = "https://raw.githubusercontent.com/ikafly144/sabalauncher/master/meta/version.json"

func main() {
	flag.Parse()
	go func() {
		w := new(app.Window)
		w.Option(
			app.MinSize(600, 400),
			app.Title(fmt.Sprintf("%s %s", appName, version)),
		)
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func loop(w *app.Window) error {
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	var ops op.Ops

	router := pages.NewRouter(appName, fmt.Sprintf("%s-%s-%s", version, commit, branch))
	cred, err := resource.LoadCredential()
	if err != nil {
		slog.Error("failed to load credential", "err", err)
	}
	router.MinecraftAccount = cred
	router.Register(1, launcher.New(&router))
	router.Register(2, account.New(&router))

	if err := checkVersion(w, th, &ops); err != nil {
		return err
	}

	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			router.Layout(gtx, th)
			e.Frame(gtx.Ops)
		}
	}
}

func checkVersion(w *app.Window, th *material.Theme, ops *op.Ops) error {
	resp, err := http.Get(versionUrl)
	if err != nil {
		slog.Error("failed to get version", "err", err)
		return nil
	}
	defer resp.Body.Close()
	versionOk := resp.StatusCode == http.StatusOK
	if versionOk {
		var versionInfo struct {
			Version string `json:"version"`
			Url     string `json:"url"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err != nil {
			slog.Error("failed to decode version", "err", err)
		}
		if versionInfo.Version != version && versionOk {
			slog.Info("new version available", "version", versionInfo.Version)
			if err := browser.Open(versionInfo.Url); err != nil {
				slog.Error("failed to open browser", "err", err)
			}
			for {
				switch e := w.Event().(type) {
				case app.DestroyEvent:
					return e.Err
				case app.FrameEvent:
					gtx := app.NewContext(ops, e)
					layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return material.H6(th, "New version available").Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							return material.H6(th, versionInfo.Version).Layout(gtx)
						}),
					)
					e.Frame(gtx.Ops)
				}
			}
		}
	} else {
		slog.Error("failed to get version", "status", resp.StatusCode)
	}
	return nil
}
