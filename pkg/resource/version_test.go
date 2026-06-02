package resource

import (
	"testing"

	"github.com/ikafly144/sabalauncher/v2/pkg/buildinfo"
)

func TestLauncherVersion(t *testing.T) {
	t.Logf("LauncherName: %s", buildinfo.LauncherName)
	t.Logf("LauncherVersion: %s", buildinfo.LauncherVersion)

	if buildinfo.LauncherName == "" {
		t.Error("LauncherName should not be empty")
	}
	if buildinfo.LauncherVersion == "" {
		t.Error("LauncherVersion should not be empty")
	}
}
