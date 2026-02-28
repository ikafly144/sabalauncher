package resource

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNeoForgeLoader_Install(t *testing.T) {
	// Setup temporary data directory
	tempDir, err := os.MkdirTemp("", "neoforge-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldDataDir := DataDir
	DataDir = tempDir
	defer func() { DataDir = oldDataDir }()

	// We won't run the actual installer in unit tests as it requires java and internet
	// and takes a long time. We will mock the manifest to test GenerateLaunchConfig later.
	loader := NewNeoForgeLoader("1.20.1", "47.1.0")

	// Just test if we can create the loader
	if loader.VanillaVersion != "1.20.1" || loader.NeoForgeVersion != "47.1.0" {
		t.Errorf("Loader not initialized correctly")
	}
}

func TestNeoForgeLoader_GenerateLaunchConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "neoforge-test-config")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldDataDir := DataDir
	DataDir = tempDir
	defer func() { DataDir = oldDataDir }()

	neoforgeDir := "1.20.1-neoforge-47.1.0"
	versionDir := filepath.Join(tempDir, "versions", neoforgeDir)
	os.MkdirAll(versionDir, 0755)

	// Create mock manifest
	manifest := ClientManifest{
		ID:        neoforgeDir,
		MainClass: "net.minecraft.boot.Knot",
		Libraries: []Library{
			{
				Downloads: LibraryDownloads{
					Artifact: LibraryArtifact{
						Path: "net/neoforged/neoforge/47.1.0/neoforge-47.1.0.jar",
					},
				},
			},
		},
	}
	manifestBytes, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(versionDir, neoforgeDir+".json"), manifestBytes, 0644)

	loader := NewNeoForgeLoader("1.20.1", "47.1.0")
	inst := &Instance{
		Path: filepath.Join(tempDir, "profiles", "test"),
	}

	config, err := loader.GenerateLaunchConfig(inst)
	if err != nil {
		t.Fatalf("Failed to generate launch config: %v", err)
	}

	if config.MainClass != manifest.MainClass {
		t.Errorf("Expected main class %s, got %s", manifest.MainClass, config.MainClass)
	}
}
