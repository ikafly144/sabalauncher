package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ForgeLoader implements the ModLoader interface for the Forge mod loader.
type ForgeLoader struct {
	VanillaVersion string
	ForgeVersion   string
}

// NewForgeLoader creates a new ForgeLoader instance.
func NewForgeLoader(vanillaVersion, forgeVersion string) *ForgeLoader {
	return &ForgeLoader{
		VanillaVersion: vanillaVersion,
		ForgeVersion:   forgeVersion,
	}
}

// Install handles the downloading and installation of Forge.
func (f *ForgeLoader) Install(ctx context.Context, profile *Profile) error {
	slog.Info("Installing Forge", "vanillaVersion", f.VanillaVersion, "forgeVersion", f.ForgeVersion)
	
	// This logic is currently spread across ForgeManifestLoader and SetupState.
	// For the refactor, we'll implement it here or call existing helpers.
	// We need dataPath here. We might need to add it to the Install signature or use a global.
	// DataDir is available in profile.go.
	
	dataPath := DataDir
	
	// 1. Get Vanilla Manifest
	ver, err := GetVersion(f.VanillaVersion)
	if err != nil {
		return fmt.Errorf("failed to get vanilla version: %w", err)
	}
	vanillaManifest, err := GetClientManifest(ver)
	if err != nil {
		return fmt.Errorf("failed to get vanilla manifest: %w", err)
	}
	_ = vanillaManifest // TODO: Use this to merge with forge manifest after installation

	// 2. Download Forge Installer if not present
	forgeDir := f.VanillaVersion + "-forge-" + f.ForgeVersion
	installerPath := filepath.Join(os.TempDir(), forgeDir+"-installer.jar")
	
	if _, err := os.Stat(filepath.Join(dataPath, "versions", forgeDir, forgeDir+".json")); os.IsNotExist(err) {
		worker, path, err := DownloadForge(f.VanillaVersion+"-"+f.ForgeVersion, forgeDir, dataPath)
		if err != nil {
			return fmt.Errorf("failed to download forge: %w", err)
		}
		if err := worker.Run(); err != nil {
			return fmt.Errorf("failed to run forge download worker: %w", err)
		}
		installerPath = path
		
		// 3. Install Forge
		if err := InstallForge(installerPath, dataPath); err != nil {
			return fmt.Errorf("failed to install forge: %w", err)
		}
		defer os.Remove(installerPath)
	}

	return nil
}

// GenerateLaunchConfig produces the configuration required to launch the game with Forge.
func (f *ForgeLoader) GenerateLaunchConfig(profile *Profile) (*LaunchConfig, error) {
	dataPath := DataDir
	forgeDir := f.VanillaVersion + "-forge-" + f.ForgeVersion
	
	// 1. Load Forge Manifest
	manifestPath := filepath.Join(dataPath, "versions", forgeDir, forgeDir+".json")
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open forge manifest: %w", err)
	}
	defer file.Close()
	
	var manifest ClientManifest
	if err := json.NewDecoder(file).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode forge manifest: %w", err)
	}

	// 2. Generate Classpath
	classpath := filepath.Join(dataPath, "versions", manifest.ID, manifest.ID+".jar")
	classpathSeparator := string(os.PathListSeparator)
	for _, library := range manifest.Libraries {
		if library.Downloads.Classifiers != nil {
			for _, classifier := range library.Downloads.Classifiers {
				classpath += classpathSeparator + filepath.Join(dataPath, "libraries", classifier.Path)
			}
		}
		classpath += classpathSeparator + filepath.Join(dataPath, "libraries", library.Downloads.Artifact.Path)
	}

	// 3. Resolve Arguments
	cmdMap := map[string]string{
		"natives_directory":   filepath.Join(dataPath, "bin", manifest.ID),
		"launcher_name":       "SabaLauncher",
		"launcher_version":    "1.0",
		"classpath":           classpath,
		"library_directory":   filepath.Join(dataPath, "libraries"),
		"classpath_separator": classpathSeparator,
	}

	var jvmArgs []string
	memory, err := profile.ActualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get actual memory: %w", err)
	}
	jvmArgs = append(jvmArgs, "-Xmx"+fmt.Sprintf("%d", memory)+"M")
	jvmArgs = append(jvmArgs, defaultJvmArgs...)

	for _, arg := range manifest.Arguments.Jvm {
		if arg == nil {
			continue
		}
		switch arg := arg.(type) {
		case JvmArgumentString:
			val := arg.String()
			for before, after := range cmdMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s}", before), after)
			}
			jvmArgs = append(jvmArgs, val)
		case JvmArgumentRule:
			if !slices.ContainsFunc(arg.Rules, func(rule JvmArgumentRuleType) bool {
				return rule.Action.Allowed() != rule.OS.Matched()
			}) {
				continue
			}
			for _, a := range arg.Value {
				for before, after := range cmdMap {
					a = strings.ReplaceAll(a, fmt.Sprintf("${%s}", before), after)
				}
				jvmArgs = append(jvmArgs, a)
			}
		}
	}

	// Forge specific game arguments are usually in manifest.Arguments.Game
	var gameArgs []string
	// Note: tokens and player info will be handled by GameRunner/BootGameFromConfig
	
	for _, arg := range manifest.Arguments.Game {
		if arg == nil {
			continue
		}
		switch arg := arg.(type) {
		case GameArgumentString:
			// We skip placeholders that are handled dynamically during boot
			val := arg.String()
			if !strings.Contains(val, "${") {
				gameArgs = append(gameArgs, val)
			} else {
				// For now, let's include them and let BootGameFromConfig handle them
				gameArgs = append(gameArgs, val)
			}
		case GameArgumentRule:
			// TODO: Handle rules if necessary
			for _, a := range arg.Value {
				gameArgs = append(gameArgs, a)
			}
		}
	}

	config := &LaunchConfig{
		MainClass:     manifest.MainClass,
		JVMArguments:  jvmArgs,
		GameArguments: gameArgs,
		Classpath:     []string{classpath},
	}
	
	return config, nil
}
