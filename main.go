package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"path/filepath"

	"fyne.io/fyne/v2/app"
	"github.com/Masterminds/semver/v3"
	"github.com/bugph0bia/go-logging"
	"github.com/ikafly144/sabalauncher/pkg/core"
	"github.com/ikafly144/sabalauncher/pkg/msa"
	"github.com/ikafly144/sabalauncher/pkg/resource"
	"github.com/ikafly144/sabalauncher/pkg/ui/fyne"
	"github.com/ikafly144/sabalauncher/secret"
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

func init() {
	msa.ClientID = secret.GetSecret("MSA_CLIENT_ID")
	resource.CurseForgeAPIKey = secret.GetSecret("CURSEFORGE_API_KEY")
	resource.DiscordClientID = secret.GetSecret("DISCORD_CLIENT_ID")
}

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

	instances, err := core.NewInstanceManager(resource.DataDir)
	if err != nil {
		log.Fatalf("failed to initialize instance manager: %v", err)
	}

	runner := core.NewGameRunner(auth, instances, resource.DataDir)
	discord := core.NewDiscordManager(auth, instances)

	// Try to resume session
	_ = auth.TrySilentLogin(context.Background())

	// Initialize Fyne App
	a := app.NewWithID("net.sabafly.sabalauncher")
	ui := fyne.NewFyneUI(a, auth, instances, runner, discord, version)

	// Run UI
	ui.Run()

	// Cleanup
	resource.Logout()
}
