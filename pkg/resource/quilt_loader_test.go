package resource

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestQuiltLoader_Install(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "quilt-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldDataDir := DataDir
	DataDir = tempDir
	defer func() { DataDir = oldDataDir }()

	loader := NewQuiltLoader("1.20.1", "0.23.1")
	profile := &Profile{
		Path: filepath.Join(tempDir, "profiles", "test"),
	}

	ctx := context.Background()
	err = loader.Install(ctx, profile)
	if err != nil {
		t.Fatalf("Quilt install failed: %v", err)
	}

	metaPath := filepath.Join(tempDir, "versions", "1.20.1-quilt-0.23.1", "quilt-meta.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Errorf("quilt-meta.json was not created")
	}
}

func TestQuiltLoader_GenerateLaunchConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "quilt-test-config")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldDataDir := DataDir
	DataDir = tempDir
	defer func() { DataDir = oldDataDir }()

	versionDir := filepath.Join(tempDir, "versions", "1.20.1-quilt-0.23.1")
	os.MkdirAll(versionDir, 0755)

	meta := QuiltLauncherMeta{}
	meta.MainClass = "org.quiltmc.loader.impl.launch.knot.KnotClient"

	metaFile, _ := os.Create(filepath.Join(versionDir, "quilt-meta.json"))
	json.NewEncoder(metaFile).Encode(meta)
	metaFile.Close()

	loader := NewQuiltLoader("1.20.1", "0.23.1")
	profile := &Profile{
		Path: filepath.Join(tempDir, "profiles", "test"),
	}

	if loader.GameVersion != "1.20.1" {
		t.Errorf("Expected game version 1.20.1, got %s", loader.GameVersion)
	}
	if profile.Path == "" {
		t.Errorf("Profile path is empty")
	}
}
