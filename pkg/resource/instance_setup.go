package resource

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// SetupInstance prepares an Instance by orchestrating downloads and installations
// based on its specified versions (vanilla, forge, fabric, etc.) and mods.
func SetupInstance(dataPath string, inst *Instance) *SetupState {
	state := NewState(fmt.Sprintf("%s のセットアップ", inst.Name), "instance_setup")

	go func() {
		_ = os.MkdirAll(dataPath, 0755)
		_ = os.MkdirAll(inst.Path, 0755)

		// 1. Identify Vanilla Version
		var vanillaVersion string
		for _, v := range inst.Versions {
			if v.ID == "minecraft" {
				vanillaVersion = v.Version
				break
			}
		}

		if vanillaVersion == "" {
			slog.Error("Vanilla minecraft version is missing in instance", "instance", inst.Name)
			state.Fail(fmt.Errorf("vanilla minecraft version is missing"))
			return
		}

		ver, err := GetVersion(vanillaVersion)
		if err != nil {
			slog.Error("Failed to get vanilla version info", "error", err)
			state.Fail(err)
			return
		}

		m, err := GetClientManifest(ver)
		if err != nil {
			slog.Error("Failed to get client manifest", "error", err)
			state.Fail(err)
			return
		}

		// 2. Add Vanilla Setup Steps
		state.AddStep(&JavaSetupStep{manifest: m})
		state.AddStep(&ClientDownloadStep{manifest: m})
		state.AddStep(&AssetsDownloadStep{manifest: m})
		state.AddStep(&LibraryDownloadStep{manifest: m})

		// 3. Add Mod Loader Setup Steps
		loader, err := GetModLoader(inst)
		if err != nil && err.Error() != "no mod loader found" {
			slog.Error("Failed to initialize mod loader", "error", err)
			state.Fail(err)
			return
		}

		if loader != nil {
			state.AddStep(&InstanceLoaderSetupStep{
				loader: loader,
				inst:   inst,
			})
		}

		// 4. Mod Installation (TODO: Download Mod instances)
		if len(inst.Mods) > 0 {
			state.AddStep(&InstanceModsSetupStep{
				inst: inst,
			})
		}

		if err := state.Do(&SetupContext{
			dataPath:    dataPath,
			profilePath: inst.Path,
		}); err != nil {
			slog.Error("Failed to run setup state", "error", err)
			state.Fail(err)
		}
	}()

	return state
}

type InstanceLoaderSetupStep struct {
	loader ModLoader
	inst   *Instance
}

func (s *InstanceLoaderSetupStep) FriendlyName() string {
	return "Mod Loaderのインストール"
}

func (s *InstanceLoaderSetupStep) Name() string {
	return "loader_setup"
}

func (s *InstanceLoaderSetupStep) Progress() float32 {
	return 0.0
}

func (s *InstanceLoaderSetupStep) Do(ctx *SetupContext) error {
	return s.loader.Install(context.Background(), s.inst)
}

type InstanceModsSetupStep struct {
	inst *Instance
}

func (s *InstanceModsSetupStep) FriendlyName() string {
	return "Modのダウンロード"
}

func (s *InstanceModsSetupStep) Name() string {
	return "mods_setup"
}

func (s *InstanceModsSetupStep) Progress() float32 {
	// Dummy progress for now
	return 0.0
}

func (s *InstanceModsSetupStep) Do(ctx *SetupContext) error {
	// TODO: Implement Mod downloads from Source
	return nil
}

// Helper method to resolve ClientManifest from an instance
func GetClientManifestForInstance(inst *Instance) (*ClientManifest, error) {
	var vanillaVersion string
	for _, v := range inst.Versions {
		if v.ID == "minecraft" {
			vanillaVersion = v.Version
			break
		}
	}
	if vanillaVersion == "" {
		return nil, fmt.Errorf("vanilla minecraft version missing")
	}
	ver, err := GetVersion(vanillaVersion)
	if err != nil {
		return nil, err
	}
	return GetClientManifest(ver)
}
