package buildinfo

import "runtime/debug"

var (
	ProjectGithubURL = "https://github.com/ikafly144/sabalauncher"
)

var (
	LauncherName    = "SabaLauncher"
	LauncherVersion = "v0.0.0-dev"
)

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			LauncherVersion = info.Main.Version
		}
	}
}
