package main

//go:generate go run github.com/akavel/rsrc@latest -ico assets/launcher_icon.ico -o rsrc_windows_amd64.syso

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/Microsoft/go-winio"
	"fyne.io/fyne/v2/app"
	"github.com/Masterminds/semver/v3"
	"github.com/bugph0bia/go-logging"
	"github.com/ikafly144/sabalauncher/v2/pkg/core"
	"github.com/ikafly144/sabalauncher/v2/pkg/msa"
	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
	"github.com/ikafly144/sabalauncher/v2/pkg/rpc"
	"github.com/ikafly144/sabalauncher/v2/pkg/ui/fyne"
	"github.com/ikafly144/sabalauncher/v2/secret"
)

const (
	devVersion = "0.0.0-indev"
	pipeName   = `\\.\pipe\sabalauncher-ipc`
)

var (
	_              = appName
	_              = currentVersion
	_              = commit
	_              = date
	_              = branch
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
	logging.MaxSizeMB = 8
	logging.WithStdout = true
	logging.MaxBackups = 200
	slog.SetDefault(logging.NewLogger(filepath.Join(resource.DataDir, "log", "latest.log")))

	msa.ClientID = secret.GetSecret("MSA_CLIENT_ID")
	resource.CurseForgeAPIKey = secret.GetSecret("CURSEFORGE_API_KEY")

	if err := rpc.Login(); err != nil {
		slog.Error("failed to login to Discord RPC", "err", err)
	} else {
		slog.Info("Discord RPC logged in")
	}
}

func main() {
	flag.Parse()

	// Single instance check
	if checkExistingInstance() {
		os.Exit(0)
	}

	// Initialize Core Services
	auth, err := core.NewAuthenticator(filepath.Join(resource.DataDir, "msa_cache"))
	if err != nil {
		log.Fatalf("failed to initialize authenticator: %v", err)
	}

	instances, err := core.NewInstanceManager(resource.DataDir)
	if err != nil {
		log.Fatalf("failed to initialize instance manager: %v", err)
	}

	config, err := core.LoadConfig(resource.DataDir)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	runner := core.NewGameRunner(auth, instances, resource.DataDir, config)
	discord := core.NewDiscordManager(auth, instances)

	// Try to resume session
	_ = auth.TrySilentLogin(context.Background())

	// Initialize Fyne App
	a := app.NewWithID("net.sabafly.sabalauncher")
	ui := fyne.NewFyneUI(a, auth, instances, runner, discord, config, version)

	// Start IPC listener
	go startIPCListener(ui)

	// Run UI
	ui.Run()

	// Cleanup
	rpc.Logout()
}

func checkExistingInstance() bool {
	conn, err := winio.DialPipe(pipeName, nil)
	if err != nil {
		return false
	}
	defer conn.Close()

	slog.Info("Existing instance detected, sending show command")
	_, _ = conn.Write([]byte("show"))
	return true
}

func startIPCListener(ui *fyne.FyneUI) {
	l, err := winio.ListenPipe(pipeName, nil)
	if err != nil {
		slog.Error("failed to listen on pipe", "err", err)
		return
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			slog.Error("failed to accept pipe connection", "err", err)
			continue
		}

		go func(c net.Conn) {
			defer c.Close()
			buf := make([]byte, 1024)
			n, err := c.Read(buf)
			if err != nil && err != io.EOF {
				return
			}
			if string(buf[:n]) == "show" {
				ui.ShowWindow()
			}
		}(conn)
	}
}

