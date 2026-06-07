package main

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
)

func TestRepoCommands(t *testing.T) {
	tempDir := t.TempDir()
	oldWd, _ := os.Getwd()
	_ = os.Chdir(tempDir)
	defer func() { _ = os.Chdir(oldWd) }()

	// Test Init
	runRepoInit([]string{"TestRepo"})

	if _, err := os.Stat("manifest.json"); os.IsNotExist(err) {
		t.Fatalf("manifest.json was not created")
	}

	manifestBytes, _ := os.ReadFile("manifest.json")
	var repo resource.SBRepository
	_ = json.Unmarshal(manifestBytes, &repo)

	if repo.Name != "TestRepo" {
		t.Errorf("expected name 'TestRepo', got '%s'", repo.Name)
	}
	if len(repo.Patches) != 0 {
		t.Errorf("expected 0 patches, got %d", len(repo.Patches))
	}

	// Test Add
	testFilePath := "test_file.sbpack"
	_ = os.WriteFile(testFilePath, []byte("dummy data"), 0644)

	runRepoAdd([]string{"1.0.0", "sbpack", testFilePath, "http://example.com/v1.sbpack", "local_v1.sbpack"})

	manifestBytes, _ = os.ReadFile("manifest.json")
	_ = json.Unmarshal(manifestBytes, &repo)

	if len(repo.Patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(repo.Patches))
	}

	patch := repo.Patches[0]
	if patch.ID != "1.0.0" || patch.Type != "sbpack" || patch.RemotePath != "http://example.com/v1.sbpack" || patch.LocalPath != "local_v1.sbpack" {
		t.Errorf("patch data incorrect: %+v", patch)
	}
	if patch.Hash["sha256"] == "" {
		t.Errorf("expected sha256 hash to be calculated")
	}
}
