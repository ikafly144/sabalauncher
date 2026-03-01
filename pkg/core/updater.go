package core

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
)

const RepoURL = "https://api.github.com/repos/ikafly144/sabalauncher/releases/latest"

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

	req, err := http.NewRequest(http.MethodGet, RepoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SabaLauncher-Updater")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}

	latestVersion, err := semver.NewVersion(release.TagName)
	if err != nil {
		return nil, fmt.Errorf("invalid release version %s: %w", release.TagName, err)
	}

	if !latestVersion.GreaterThan(currentVersion) {
		return nil, nil // No update needed
	}

	// Find the MSI asset
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == "SabaLauncher.msi" {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("SabaLauncher.msi not found in release assets")
	}

	return &UpdateInfo{
		Version:      release.TagName,
		DownloadURL:  downloadURL,
		ReleaseNotes: release.Body,
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
