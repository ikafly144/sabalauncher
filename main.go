package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/ikafly144/sabalauncher/pages"
	"github.com/ikafly144/sabalauncher/pages/account"
	"github.com/ikafly144/sabalauncher/pages/launcher"
	"github.com/ikafly144/sabalauncher/pkg/browser"
	"github.com/ikafly144/sabalauncher/pkg/resource"
	"github.com/ikafly144/sabalauncher/pkg/runcmd"

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
	date    = "unknown"
	branch  = "unknown"
)

func buildInfo() string {
	if version == "devel" {
		return "unknown"
	}
	return fmt.Sprintf("%s-%s-%s", commit, date, branch)
}

const versionUrl = "https://raw.githubusercontent.com/ikafly144/sabalauncher/master/meta/version.json"
const installerUrl = "https://github.com/ikafly144/sabalauncher/releases/download/{version}/SabaLauncher.msi"

type versionInfo struct {
	Version string `json:"version"`
	Url     string `json:"url"`
}

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

	router := pages.NewRouter(appName, fmt.Sprintf("%s-%s", version, buildInfo()))
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
	if version == "devel" {
		return nil
	}
	resp, err := http.Get(versionUrl)
	if err != nil {
		slog.Error("failed to get version", "err", err)
		return nil
	}
	defer resp.Body.Close()
	versionOk := resp.StatusCode == http.StatusOK
	if versionOk {
		var versionInfo versionInfo
		if err := json.NewDecoder(resp.Body).Decode(&versionInfo); err != nil {
			slog.Error("failed to decode version", "err", err)
		}
		if versionInfo.Version != version && versionOk {
			slog.Info("new version available", "version", versionInfo.Version)
			err := installNewVersion(versionInfo)
			if err == nil {
				slog.Info("install new version", "version", versionInfo.Version)
				os.Exit(0)
			}
			slog.Error("failed to install new version", "err", err)
			_ = browser.Open(versionInfo.Url)
			for {
				switch e := w.Event().(type) {
				case app.DestroyEvent:
					return fmt.Errorf("new version available %s: %w", versionInfo.Version, e.Err)
				case app.FrameEvent:
					gtx := app.NewContext(ops, e)
					layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx C) D {
							return material.H5(th, fmt.Sprintf("New version available: %s", versionInfo.Version)).Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							return material.Body1(th, "An error occurred while automatically installing the new version. Please download it manually from the link below.").Layout(gtx)
						}),
						layout.Rigid(func(gtx C) D {
							return material.Body1(th, versionInfo.Url).Layout(gtx)
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

func installNewVersion(versionInfo versionInfo) error {
	resp, err := http.Get(strings.ReplaceAll(installerUrl, "{version}", versionInfo.Version))
	if err != nil {
		slog.Error("failed to get installer", "err", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Error("failed to get installer", "status", resp.StatusCode)
		return err
	}
	dir := os.TempDir()
	file, err := os.CreateTemp(dir, fmt.Sprintf("SabaLauncher-%s-*.msi", versionInfo.Version))
	if err != nil {
		slog.Error("failed to create installer", "err", err)
		return err
	}
	cl := sync.OnceFunc(func() {
		file.Close()
	})
	defer cl()
	if _, err := io.Copy(file, resp.Body); err != nil {
		slog.Error("failed to copy installer", "err", err)
		return err
	}
	cl()
	cmd := exec.Command("msiexec", "/passive", "/i", file.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	cmd.SysProcAttr = runcmd.GetSysProcAttr()
	if err := cmd.Start(); err != nil {
		slog.Error("failed to start installer", "err", err)
		return err
	}
	return nil
}
