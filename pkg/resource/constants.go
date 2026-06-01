package resource

import (
	"runtime/debug"
)

var (
	LauncherName    = "SabaLauncher"
	LauncherVersion = "v0.0.0-dev"
)

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				// Optional: handle VCS info
			}
		}
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			LauncherVersion = info.Main.Version
		}
	}
}

const (
	// URLs
	FabricMetaURL            = "https://meta.fabricmc.net/v2/versions/loader"
	FabricMavenURL           = "https://maven.fabricmc.net/"
	ForgeMavenURL            = "https://maven.minecraftforge.net/net/minecraftforge/forge"
	JavaRuntimeMetaURL       = "https://launchermeta.mojang.com/v1/products/java-runtime/2ec0cc96c44e5a76b9c8b7c39df7210883d12871/all.json"
	CurseForgeBaseURL        = "https://api.curseforge.com"
	ModrinthBaseURL          = "https://api.modrinth.com/v2"
	NeoForgeMavenURL         = "https://maven.neoforged.net/releases/net/neoforged/neoforge"
	QuiltMetaURL             = "https://meta.quiltmc.org/v3/versions/loader"
	MojangVersionManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"
	MojangAssetResourceURL   = "https://resources.download.minecraft.net/"
	ProjectGithubURL         = "https://github.com/ikafly144/sabalauncher"
	CurseForgeWebURL         = "https://www.curseforge.com/projects"
	ModrinthWebURL           = "https://modrinth.com/project"

	// Defaults
	DefaultResolutionWidth  = "1280"
	DefaultResolutionHeight = "720"
)
