package core

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v66/github"
)

const (
	RepoOwner = "ikafly144"
	RepoName  = "sabalauncher"
)

type UpdateInfo struct {
	Version      string
	DownloadURL  string
	ReleaseNotes string
}

func CheckForUpdate(currentVersionStr string) (*UpdateInfo, error) {
	// If version is "0.0.0-indev", don't update.
	if currentVersionStr == "" || currentVersionStr == "0.0.0-indev" || currentVersionStr == "unknown" {
		return nil, nil
	}

	currentVersion, err := semver.NewVersion(currentVersionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid current version %s: %w", currentVersionStr, err)
	}

	client := github.NewClient(nil)
	release, _, err := client.Repositories.GetLatestRelease(context.Background(), RepoOwner, RepoName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}

	if release.TagName == nil {
		return nil, fmt.Errorf("release tag name is nil")
	}

	latestVersion, err := semver.NewVersion(*release.TagName)
	if err != nil {
		return nil, fmt.Errorf("invalid release version %s: %w", *release.TagName, err)
	}

	if !latestVersion.GreaterThan(currentVersion) {
		return nil, nil // No update needed
	}

	// Find the MSI asset
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.GetName() == "SabaLauncher.msi" {
			downloadURL = asset.GetBrowserDownloadURL()
			break
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("SabaLauncher.msi not found in release assets")
	}

	return &UpdateInfo{
		Version:      *release.TagName,
		DownloadURL:  downloadURL,
		ReleaseNotes: release.GetBody(),
	}, nil
}

func DownloadAndRunInstaller(downloadURL string) error {
	slog.Info("Downloading installer", "url", downloadURL)
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download installer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code while downloading: %d", resp.StatusCode)
	}

	tempDir := os.TempDir()
	msiPath := filepath.Join(tempDir, "SabaLauncher_Update.msi")

	out, err := os.Create(msiPath)
	if err != nil {
		return fmt.Errorf("failed to create temp msi file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save installer: %w", err)
	}
	out.Close()

	slog.Info("Running installer", "path", msiPath)

	// Run msiexec /i <path> /passive
	// /passive: display only progress bar
	// /i: install
	cmd := exec.Command("msiexec", "/i", msiPath, "/passive")
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start installer: %w", err)
	}

	// Exit the application so the installer can replace the files
	slog.Info("Installer started, exiting application")
	os.Exit(0)

	return nil
}
