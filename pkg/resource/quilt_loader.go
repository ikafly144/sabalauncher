package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// QuiltLoader implements the ModLoader interface for the Quilt mod loader.
type QuiltLoader struct {
	GameVersion   string
	LoaderVersion string
}

// NewQuiltLoader creates a new QuiltLoader instance.
func NewQuiltLoader(gameVersion, loaderVersion string) *QuiltLoader {
	return &QuiltLoader{
		GameVersion:   gameVersion,
		LoaderVersion: loaderVersion,
	}
}

type QuiltLauncherMeta struct {
	ID        string             `json:"id"`
	MainClass string             `json:"mainClass"`
	Libraries []QuiltLibraryInfo `json:"libraries"`
}

type QuiltLibraryInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Install handles the downloading of Quilt loader and its dependencies.
func (q *QuiltLoader) Install(ctx context.Context, inst *Instance) error {
	slog.Info("Installing Quilt", "gameVersion", q.GameVersion, "loaderVersion", q.LoaderVersion)

	url := fmt.Sprintf("https://meta.quiltmc.org/v3/versions/loader/%s/%s/profile/json", q.GameVersion, q.LoaderVersion)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch quilt meta: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("quilt meta returned status: %s", resp.Status)
	}

	var meta QuiltLauncherMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return fmt.Errorf("failed to decode quilt meta: %w", err)
	}

	dataPath := DataDir
	var worker DownloadWorker

	// Add libraries to download worker
	for _, lib := range meta.Libraries {
		libPath := mavenToPath(lib.Name, "/")
		fullPath := filepath.Join(dataPath, "libraries", filepath.FromSlash(libPath))

		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			libURL := lib.URL
			if !strings.HasSuffix(libURL, "/") {
				libURL += "/"
			}
			downloadURL := libURL + libPath

			worker.addTask(func() error {
				return downloadFile(downloadURL, fullPath)
			})
		}
	}

	if worker.Remain() > 0 {
		if err := worker.Run(); err != nil {
			return fmt.Errorf("failed to download quilt libraries: %w", err)
		}
	}

	// Save the meta for launch config generation later
	metaPath := filepath.Join(dataPath, "versions", q.GameVersion+"-quilt-"+q.LoaderVersion, "quilt-meta.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0755); err != nil {
		return err
	}

	metaFile, err := os.Create(metaPath)
	if err != nil {
		return err
	}
	defer metaFile.Close()

	return json.NewEncoder(metaFile).Encode(meta)
}

// GenerateLaunchConfig produces the configuration required to launch the game with Quilt.
func (q *QuiltLoader) GenerateLaunchConfig(inst *Instance) (*LaunchConfig, error) {
	dataPath := DataDir
	metaPath := filepath.Join(dataPath, "versions", q.GameVersion+"-quilt-"+q.LoaderVersion, "quilt-meta.json")

	file, err := os.Open(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open quilt meta: %w", err)
	}
	defer file.Close()

	var meta QuiltLauncherMeta
	if err := json.NewDecoder(file).Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode quilt meta: %w", err)
	}

	// 1. Get Vanilla Launch Config as base
	vanillaLoader := NewVanillaLoader(q.GameVersion)
	config, err := vanillaLoader.GenerateLaunchConfig(inst)
	if err != nil {
		return nil, err
	}

	// 2. Add Quilt libraries to Classpath
	for _, lib := range meta.Libraries {
		libPath := filepath.Join(dataPath, "libraries", filepath.FromSlash(mavenToPath(lib.Name, "/")))
		config.Classpath = append(config.Classpath, libPath)
	}

	// 3. Update Main Class
	config.MainClass = meta.MainClass

	// 4. Update Arguments if needed
	config.GameArguments = append(config.GameArguments, "--version", q.GameVersion)

	return config, nil
}
