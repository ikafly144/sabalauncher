package main

import (
	"flag"
	"fmt"
	"launcher/pages"
	"launcher/pages/account"
	"launcher/pages/launcher"
	"launcher/pkg/resource"
	"log"
	"log/slog"
	"os"

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
	version = "1.0.0"
)

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

	router := pages.NewRouter(appName, version)
	cred, err := resource.LoadCredential()
	if err != nil {
		slog.Error("failed to load credential", "err", err)
	}
	router.MinecraftAccount = cred
	router.Register(1, launcher.New(&router))
	router.Register(2, account.New(&router))

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
