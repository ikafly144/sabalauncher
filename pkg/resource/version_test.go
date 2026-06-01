package resource

import (
	"testing"
)

func TestLauncherVersion(t *testing.T) {
	t.Logf("LauncherName: %s", LauncherName)
	t.Logf("LauncherVersion: %s", LauncherVersion)
	
	if LauncherName == "" {
		t.Error("LauncherName should not be empty")
	}
	if LauncherVersion == "" {
		t.Error("LauncherVersion should not be empty")
	}
}
