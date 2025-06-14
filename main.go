package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/bugph0bia/go-logging"
	"github.com/google/go-github/v72/github"
	"github.com/ikafly144/sabalauncher/pages"
	"github.com/ikafly144/sabalauncher/pages/account"
	"github.com/ikafly144/sabalauncher/pages/launcher"
	"github.com/ikafly144/sabalauncher/pages/licenses"
	"github.com/ikafly144/sabalauncher/pkg/browser"
	"github.com/ikafly144/sabalauncher/pkg/msa"
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

const devVersion = "0.0.0-dev"

var installerRegex = regexp.MustCompile(`SabaLauncher-(\d+\.\d+\.\d+)(-(\d+))?\.msi`)

var (
	appName        = "SabaLauncher"
	version        = devVersion
	currentVersion = func() *semver.Version {
		v, err := semver.NewVersion(version)
		if err != nil {
			panic(fmt.Sprintf("failed to parse current version %s: %v", version, err))
		}
		return v
	}()
	commit = "unknown"
	date   = "unknown"
	branch = "unknown"
)

func buildInfo() string {
	if version == devVersion {
		return "unknown"
	}
	return fmt.Sprintf("%s-%s-%s", branch, commit, date)
}

func init() {
	logging.MaxSizeMB = 8
	logging.WithStdout = true
	slog.SetDefault(logging.NewLogger(filepath.Join(resource.DataDir, "log", "latest.log")))
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

	router := pages.NewRouter(appName, fmt.Sprintf("%s+%s", version, buildInfo()))
	cache, err := msa.NewCacheAccessor(filepath.Join(resource.DataDir, "msa_cache.json"))
	if err != nil {
		slog.Error("failed to create cache accessor", "err", err)
		return err
	}
	router.Cache = cache
	router.Register(1, launcher.New(&router))
	router.Register(2, account.New(&router))
	router.Register(3, licenses.New(&router))

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
	if version == devVersion {
		return nil
	}

	client := github.NewClient(nil)

	release, _, err := client.Repositories.GetLatestRelease(context.Background(), "ikafly144", "sabalauncher")
	if err != nil {
		slog.Error("failed to get latest release", "err", err)
		return nil
	}
	latestVersion, err := semver.NewVersion(release.GetTagName())
	if err != nil {
		slog.Error("failed to parse latest version", "err", err)
		return nil
	}
	if latestVersion.GreaterThan(currentVersion) {
		assets, _, err := client.Repositories.ListReleaseAssets(context.Background(), "ikafly144", "sabalauncher", release.GetID(), nil)
		if err != nil {
			slog.Error("failed to list release assets", "err", err)
			return nil
		}
		var assetID int64
		for _, asset := range assets {
			if asset.GetName() == "SabaLauncher.msi" || installerRegex.MatchString(asset.GetName()) {
				assetID = asset.GetID()
				break
			}
		}
		if assetID == 0 {
			slog.Error("failed to find installer asset in release", "release", release.GetTagName())
			return nil
		} else {
			if err := installNewVersion(client, release.GetTagName(), assetID); err == nil {
				slog.Info("install new version", "version", release.GetTagName())
				os.Exit(0)
			} else {
				slog.Error("failed to install new version", "err", err)
			}
		}
		_ = browser.Open(release.GetURL())
		for {
			switch e := w.Event().(type) {
			case app.DestroyEvent:
				return fmt.Errorf("new version available %s: %w", release.GetTagName(), e.Err)
			case app.FrameEvent:
				gtx := app.NewContext(ops, e)
				layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx C) D {
						return material.H5(th, fmt.Sprintf("新しいバージョンがあります: %s", release.GetTagName())).Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return material.Body1(th, "新しいバージョンの自動インストール中にエラーが発生しました。以下のリンクから手動でダウンロードしてください。").Layout(gtx)
					}),
					layout.Rigid(func(gtx C) D {
						return material.Body1(th, release.GetURL()).Layout(gtx)
					}),
				)
				e.Frame(gtx.Ops)
			}
		}
	}

	return nil
}

func installNewVersion(client *github.Client, versionTag string, assetID int64) error {
	installer, _, err := client.Repositories.DownloadReleaseAsset(context.Background(), "ikafly144", "sabalauncher", assetID, http.DefaultClient)
	if err != nil {
		slog.Error("failed to download installer", "err", err)
		return err
	}
	defer installer.Close()
	dir := os.TempDir()
	file, err := os.CreateTemp(dir, fmt.Sprintf("SabaLauncher-%s-*.msi", versionTag))
	if err != nil {
		slog.Error("failed to create installer", "err", err)
		return err
	}
	cl := sync.OnceFunc(func() {
		file.Close()
	})
	defer cl()
	if _, err := io.Copy(file, installer); err != nil {
		slog.Error("failed to copy installer", "err", err)
		return err
	}
	cl()
	cmd := exec.Command("msiexec", "/passive", "/i", file.Name())
	cmd.Stdout = slog.NewLogLogger(slog.Default().Handler(), slog.LevelInfo).Writer()
	cmd.Stderr = slog.NewLogLogger(slog.Default().Handler(), slog.LevelInfo).Writer()
	cmd.Dir = dir
	cmd.SysProcAttr = runcmd.GetSysProcAttr()
	if err := cmd.Start(); err != nil {
		slog.Error("failed to start installer", "err", err)
		return err
	}
	return nil
}
