package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"path/filepath"

	"fyne.io/fyne/v2/app"
	"github.com/Masterminds/semver/v3"
	"github.com/bugph0bia/go-logging"
	"github.com/ikafly144/sabalauncher/pkg/core"
	"github.com/ikafly144/sabalauncher/pkg/resource"
	"github.com/ikafly144/sabalauncher/pkg/ui/fyne"
)

const devVersion = "0.0.0-indev"

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
	logging.MaxBackups = 200
	slog.SetDefault(logging.NewLogger(filepath.Join(resource.DataDir, "log", "latest.log")))
	if err := resource.Login(); err != nil {
		slog.Error("failed to login to Discord RPC", "err", err)
	} else {
		slog.Info("Discord RPC logged in")
	}
}

func main() {
	flag.Parse()

	// Initialize Core Services
	auth, err := core.NewAuthenticator(filepath.Join(resource.DataDir, "msa_cache.json"))
	if err != nil {
		log.Fatalf("failed to initialize authenticator: %v", err)
	}

	profiles, err := core.NewProfileManager(resource.DataDir)
	if err != nil {
		log.Fatalf("failed to initialize profile manager: %v", err)
	}

	runner := core.NewGameRunner(auth, profiles, resource.DataDir)

	// Initialize Fyne App
	a := app.NewWithID("net.sabafly.sabalauncher")
	ui := fyne.NewFyneUI(a, auth, profiles, runner)

	// Run UI
	ui.Run()

	// Cleanup
	resource.Logout()
}
