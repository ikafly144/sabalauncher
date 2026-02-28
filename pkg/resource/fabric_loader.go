package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// FabricLoader implements the ModLoader interface for the Fabric mod loader.
type FabricLoader struct {
	GameVersion   string
	LoaderVersion string
}

// NewFabricLoader creates a new FabricLoader instance.
func NewFabricLoader(gameVersion, loaderVersion string) *FabricLoader {
	return &FabricLoader{
		GameVersion:   gameVersion,
		LoaderVersion: loaderVersion,
	}
}

type FabricMetaResponse struct {
	Loader       FabricLoaderInfo   `json:"loader"`
	Intermediary FabricLibrary      `json:"intermediary"`
	LauncherMeta FabricLauncherMeta `json:"launcherMeta"`
}

type FabricLoaderInfo struct {
	Separator string `json:"separator"`
	Build     int    `json:"build"`
	Maven     string `json:"maven"`
	Version   string `json:"version"`
	Stable    bool   `json:"stable"`
}

type FabricLibrary struct {
	Maven   string `json:"maven"`
	Version string `json:"version"`
}

type FabricLauncherMeta struct {
	MainClass struct {
		Client string `json:"client"`
		Server string `json:"server"`
	} `json:"mainClass"`
	Libraries struct {
		Common []FabricLibraryInfo `json:"common"`
		Client []FabricLibraryInfo `json:"client"`
		Server []FabricLibraryInfo `json:"server"`
	} `json:"libraries"`
}

type FabricLibraryInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Install handles the downloading of Fabric loader and its dependencies.
func (f *FabricLoader) Install(ctx context.Context, inst *Instance) error {
	slog.Info("Installing Fabric", "gameVersion", f.GameVersion, "loaderVersion", f.LoaderVersion)

	url := fmt.Sprintf("https://meta.fabricmc.net/v2/versions/loader/%s/%s", f.GameVersion, f.LoaderVersion)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch fabric meta: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fabric meta returned status: %s", resp.Status)
	}

	var meta FabricMetaResponse
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return fmt.Errorf("failed to decode fabric meta: %w", err)
	}

	dataPath := DataDir
	var worker DownloadWorker

	// Add libraries to download worker
	libs := append(meta.LauncherMeta.Libraries.Common, meta.LauncherMeta.Libraries.Client...)
	// Add loader and intermediary
	libs = append(libs, FabricLibraryInfo{Name: meta.Loader.Maven, URL: "https://maven.fabricmc.net/"})
	libs = append(libs, FabricLibraryInfo{Name: meta.Intermediary.Maven, URL: "https://maven.fabricmc.net/"})

	for _, lib := range libs {
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
			return fmt.Errorf("failed to download fabric libraries: %w", err)
		}
	}

	// Save the meta for launch config generation later
	metaPath := filepath.Join(dataPath, "versions", f.GameVersion+"-fabric-"+f.LoaderVersion, "fabric-meta.json")
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

	// GenerateLaunchConfig produces the configuration required to launch the game with Fabric.
	func (f *FabricLoader) GenerateLaunchConfig(inst *Instance) (*LaunchConfig, error) {
		dataPath := DataDir
		metaPath := filepath.Join(dataPath, "versions", f.GameVersion+"-fabric-"+f.LoaderVersion, "fabric-meta.json")
	file, err := os.Open(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open fabric meta: %w", err)
	}
	defer file.Close()

	var meta FabricMetaResponse
	if err := json.NewDecoder(file).Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode fabric meta: %w", err)
	}

	// 1. Get Vanilla Launch Config as base
	vanillaLoader := NewVanillaLoader(f.GameVersion)
	config, err := vanillaLoader.GenerateLaunchConfig(inst)
	if err != nil {
		return nil, err
	}

	// 2. Add Fabric libraries to Classpath
	libs := append(meta.LauncherMeta.Libraries.Common, meta.LauncherMeta.Libraries.Client...)
	libs = append(libs, FabricLibraryInfo{Name: meta.Loader.Maven})
	libs = append(libs, FabricLibraryInfo{Name: meta.Intermediary.Maven})

	for _, lib := range libs {
		libPath := filepath.Join(dataPath, "libraries", filepath.FromSlash(mavenToPath(lib.Name, "/")))
		config.Classpath = append(config.Classpath, libPath)
	}
	// 3. Update Main Class
	config.MainClass = meta.LauncherMeta.MainClass.Client

	// 4. Update Arguments if needed
	// Fabric usually uses standard Minecraft arguments, but we might need to add loader info.
	config.GameArguments = append(config.GameArguments, "--version", f.GameVersion)

	return config, nil
}

func mavenToPath(mavenName string, separator string) string {
	parts := strings.Split(mavenName, ":")
	if len(parts) < 3 {
		return ""
	}
	group := strings.ReplaceAll(parts[0], ".", separator)
	artifact := parts[1]
	version := parts[2]

	filename := fmt.Sprintf("%s-%s.jar", artifact, version)
	return strings.Join([]string{group, artifact, version, filename}, separator)
}

func downloadFile(url, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download: %s", resp.Status)
	}

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
