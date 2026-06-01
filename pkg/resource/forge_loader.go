package resource

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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
func (f *ForgeLoader) Install(ctx context.Context, inst *Instance) error {
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
func (f *ForgeLoader) GenerateLaunchConfig(inst *Instance) (*LaunchConfig, error) {
	dataPath := DataDir
	forgeDir := f.VanillaVersion + "-forge-" + f.ForgeVersion

	// 1. Load Forge Manifest recursively (handles inheritance from vanilla)
	manifest, err := GetClientManifestRecursive(dataPath, forgeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load forge manifest: %w", err)
	}

	// 2. Generate Classpath
	var classpath []string
	classpath = append(classpath, filepath.Join(dataPath, "versions", manifest.ID, manifest.ID+".jar"))
	for _, library := range manifest.Libraries {
		if library.Downloads.Classifiers != nil {
			for _, classifier := range library.Downloads.Classifiers {
				classpath = append(classpath, filepath.Join(dataPath, "libraries", classifier.Path))
			}
		}
		if library.Downloads.Artifact.Path != "" {
			classpath = append(classpath, filepath.Join(dataPath, "libraries", library.Downloads.Artifact.Path))
		}
	}

	var jvmArgs []string
	memory := uint64(2048) // Default memory
	jvmArgs = append(jvmArgs, "-Xmx"+fmt.Sprintf("%d", memory)+"M")
	jvmArgs = append(jvmArgs, defaultJvmArgs...)

	for _, arg := range manifest.Arguments.Jvm {
		if arg == nil {
			continue
		}
		switch arg := arg.(type) {
		case JvmArgumentString:
			jvmArgs = append(jvmArgs, arg.String())
		case JvmArgumentRule:
			if !slices.ContainsFunc(arg.Rules, func(rule JvmArgumentRuleType) bool {
				return rule.Action.Allowed() != rule.OS.Matched()
			}) {
				continue
			}
			jvmArgs = append(jvmArgs, arg.Value...)
		}
	}

	var gameArgs []string
	for _, arg := range manifest.Arguments.Game {
		if arg == nil {
			continue
		}
		switch arg := arg.(type) {
		case GameArgumentString:
			gameArgs = append(gameArgs, arg.String())
		case GameArgumentRule:
			for _, a := range arg.Value {
				gameArgs = append(gameArgs, a)
			}
		}
	}

	config := &LaunchConfig{
		MainClass:     manifest.MainClass,
		JVMArguments:  jvmArgs,
		GameArguments: gameArgs,
		Classpath:     classpath,
	}
	return config, nil
}
