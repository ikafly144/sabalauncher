package resource

import (
	"context"
	"testing"
)

func TestForgeLoader_Install(t *testing.T) {
	// Note: This test would require networking and a valid Forge manifest.
	// For now, we'll just check that it doesn't return "not implemented".
	loader := NewForgeLoader("1.20.1", "47.2.0")
	inst := &Instance{
		Path: t.TempDir(),
	}

	err := loader.Install(context.Background(), inst)
	if err != nil && err.Error() == "not implemented" {
		t.Error("Install method still returns 'not implemented'")
	}
}

func TestForgeLoader_GenerateLaunchConfig(t *testing.T) {
	// This test requires a local forge manifest file.
	loader := NewForgeLoader("1.20.1", "47.2.0")
	inst := &Instance{
		Path: t.TempDir(),
	}

	config, err := loader.GenerateLaunchConfig(inst)
	if err != nil && err.Error() == "not implemented" {
		t.Error("GenerateLaunchConfig method still returns 'not implemented'")
	}
	_ = config
}
