package resource

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func createMockZip(t *testing.T, path string, files map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create mock zip %s: %v", path, err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := io.Copy(f, bytes.NewReader(content)); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}
}

func calculateSHA256(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func TestSBPackImportAndUpdate(t *testing.T) {
	// 1. Setup mock server for downloads
	mod1Content := []byte("mod1_content")
	mod2Content := []byte("mod2_content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/mod1.jar" {
			w.Write(mod1Content)
		} else if r.URL.Path == "/mod2.jar" {
			w.Write(mod2Content)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	packPath := filepath.Join(tempDir, "test.sbpack")
	patchPath := filepath.Join(tempDir, "test.sbpatch")
	destDir := filepath.Join(tempDir, "instance")

	// 2. Create .sbpack
	index := SBIndex{
		FormatVersion: 1,
		Name:          "Test Pack",
		Version:       "1.0.0",
		Dependencies: map[string]string{
			"minecraft": "1.20.1",
		},
		Files: []SBFile{
			{
				Path:      "mods/mod1.jar",
				Hashes:    map[string]string{"sha256": calculateSHA256(mod1Content)},
				Downloads: []string{server.URL + "/mod1.jar"},
				FileSize:  int64(len(mod1Content)),
			},
		},
	}
	indexBytes, _ := json.Marshal(index)

	createMockZip(t, packPath, map[string][]byte{
		"sb.index.json":           indexBytes,
		"overrides/config/m1.txt": []byte("config1"),
	})

	// 3. Test Import
	testUID := uuid.New()
	inst, err := ImportSBPack(packPath, destDir, testUID)
	if err != nil {
		t.Fatalf("ImportSBPack failed: %v", err)
	}

	if inst.UID != testUID {
		t.Errorf("expected UID %v, got %v", testUID, inst.UID)
	}

	if inst.Name != "Test Pack" {
		t.Errorf("expected instance name 'Test Pack', got %s", inst.Name)
	}
	if inst.Upstream.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %s", inst.Upstream.Version)
	}
	if len(inst.Mods) != 1 {
		t.Errorf("expected 1 mod, got %d", len(inst.Mods))
	}

	// Verify files
	if content, err := os.ReadFile(filepath.Join(destDir, "mods/mod1.jar")); err != nil || string(content) != "mod1_content" {
		t.Errorf("mod1.jar missing or content mismatch: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(destDir, "config/m1.txt")); err != nil || string(content) != "config1" {
		t.Errorf("config/m1.txt missing or content mismatch: %v", err)
	}

	// 4. Create .sbpatch
	patchIndex := SBIndex{
		FormatVersion: 1,
		Name:          "Test Pack",
		Version:       "1.1.0",
		Dependencies: map[string]string{
			"minecraft": "1.20.2",
		},
		Files: []SBFile{
			{
				Path:      "mods/mod2.jar",
				Hashes:    map[string]string{"sha256": calculateSHA256(mod2Content)},
				Downloads: []string{server.URL + "/mod2.jar"},
				FileSize:  int64(len(mod2Content)),
			},
		},
	}

	patch := SBPatch{
		FormatVersion: 1,
		FromVersion:   "1.0.0",
		ToVersion:     "1.1.0",
		NewIndex:      patchIndex,
		RemovedFiles:  []string{"mods/mod1.jar", "config/m1.txt"},
	}
	patchBytes, _ := json.Marshal(patch)

	createMockZip(t, patchPath, map[string][]byte{
		"sb.patch.json":           patchBytes,
		"overrides/config/m2.txt": []byte("config2"),
	})

	// 5. Test Apply
	if err := ApplySBPatch(inst, patchPath); err != nil {
		t.Fatalf("ApplySBPatch failed: %v", err)
	}

	if inst.Upstream.Version != "1.1.0" {
		t.Errorf("expected version '1.1.0', got %s", inst.Upstream.Version)
	}
	if len(inst.Mods) != 1 || inst.Mods[0].Name != "mod2.jar" {
		t.Errorf("expected 1 mod (mod2.jar), got %v", inst.Mods)
	}
	if inst.Versions[0].Version != "1.20.2" {
		t.Errorf("expected minecraft 1.20.2, got %s", inst.Versions[0].Version)
	}

	// Verify files
	if _, err := os.Stat(filepath.Join(destDir, "mods/mod1.jar")); !os.IsNotExist(err) {
		t.Errorf("mod1.jar should have been removed")
	}
	if _, err := os.Stat(filepath.Join(destDir, "config/m1.txt")); !os.IsNotExist(err) {
		t.Errorf("config/m1.txt should have been removed")
	}
	if content, err := os.ReadFile(filepath.Join(destDir, "mods/mod2.jar")); err != nil || string(content) != "mod2_content" {
		t.Errorf("mod2.jar missing or content mismatch: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(destDir, "config/m2.txt")); err != nil || string(content) != "config2" {
		t.Errorf("config/m2.txt missing or content mismatch: %v", err)
	}
}
