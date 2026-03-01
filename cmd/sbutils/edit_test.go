package main

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
)

func sha1Hex(data []byte) string {
	h := sha1.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func sha256Hex(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func writeSBIndex(t *testing.T, path string, index resource.SBIndex) {
	t.Helper()
	b, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test index: %v", err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("failed to write test index: %v", err)
	}
}

func readSBIndex(t *testing.T, path string) resource.SBIndex {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read test index: %v", err)
	}
	var index resource.SBIndex
	if err := json.Unmarshal(b, &index); err != nil {
		t.Fatalf("failed to unmarshal test index: %v", err)
	}
	return index
}

func TestExecuteEdit_UpdatesFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sb.index.json")
	writeSBIndex(t, path, resource.SBIndex{
		FormatVersion: 1,
		Name:          "Old Name",
		Version:       "1.0.0",
		Dependencies: map[string]string{
			"minecraft":     "1.20.1",
			"fabric-loader": "0.15.0",
		},
		Files: []resource.SBFile{
			{Path: "mods/sample.jar", Hashes: map[string]string{"sha256": "abc"}, FileSize: 123},
		},
	})

	var out bytes.Buffer
	err := executeEdit([]string{
		"-indexfile", path,
		"-name", "New Name",
		"-version", "2.0.0",
		"-require", "minecraft=1.21.0",
		"-require", "quilt-loader@0.26.0",
		"-droprequire", "fabric-loader",
	}, &out)
	if err != nil {
		t.Fatalf("executeEdit failed: %v", err)
	}

	updated := readSBIndex(t, path)
	if updated.Name != "New Name" {
		t.Fatalf("expected Name=New Name, got %s", updated.Name)
	}
	if updated.Version != "2.0.0" {
		t.Fatalf("expected Version=2.0.0, got %s", updated.Version)
	}
	if updated.Dependencies["minecraft"] != "1.21.0" {
		t.Fatalf("expected minecraft=1.21.0, got %s", updated.Dependencies["minecraft"])
	}
	if updated.Dependencies["quilt-loader"] != "0.26.0" {
		t.Fatalf("expected quilt-loader=0.26.0, got %s", updated.Dependencies["quilt-loader"])
	}
	if _, exists := updated.Dependencies["fabric-loader"]; exists {
		t.Fatalf("fabric-loader should be removed")
	}
	if len(updated.Files) != 1 || updated.Files[0].Path != "mods/sample.jar" {
		t.Fatalf("files should be preserved, got %+v", updated.Files)
	}
	if !strings.Contains(out.String(), "Successfully updated") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestExecuteEdit_PrintDoesNotWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sb.index.json")
	writeSBIndex(t, path, resource.SBIndex{
		FormatVersion: 1,
		Name:          "Pack",
		Version:       "1.0.0",
		Dependencies: map[string]string{
			"minecraft": "1.20.1",
		},
	})

	var out bytes.Buffer
	err := executeEdit([]string{"-indexfile", path, "-version", "3.0.0", "-print"}, &out)
	if err != nil {
		t.Fatalf("executeEdit failed: %v", err)
	}

	current := readSBIndex(t, path)
	if current.Version != "1.0.0" {
		t.Fatalf("expected file to remain unchanged, got version %s", current.Version)
	}
	if !strings.Contains(out.String(), `"version": "3.0.0"`) {
		t.Fatalf("print output should include updated version, got %s", out.String())
	}
}

func TestExecuteEdit_InvalidRequire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sb.index.json")
	writeSBIndex(t, path, resource.SBIndex{
		FormatVersion: 1,
		Name:          "Pack",
		Version:       "1.0.0",
		Dependencies:  map[string]string{},
	})

	err := executeEdit([]string{"-indexfile", path, "-require", "invalid-format"}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("expected error for invalid require format")
	}
	if !strings.Contains(err.Error(), "id=version or id@version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteEdit_FileUpsertAndDrop(t *testing.T) {
	content := []byte("updated-mod-content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mod.jar" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(content)
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "sb.index.json")
	writeSBIndex(t, path, resource.SBIndex{
		FormatVersion: 1,
		Name:          "Pack",
		Version:       "1.0.0",
		Dependencies:  map[string]string{},
		Files: []resource.SBFile{
			{
				Path:      "mods\\sample.jar",
				Hashes:    map[string]string{"sha256": "old"},
				Downloads: []string{"https://example.com/old.jar"},
				FileSize:  1,
				Env: &resource.SBEnvironment{
					Client: "required",
					Server: "optional",
				},
			},
			{
				Path:      "mods/remove.jar",
				Hashes:    map[string]string{"sha256": "remove"},
				Downloads: []string{"https://example.com/remove.jar"},
				FileSize:  2,
			},
		},
	})

	var out bytes.Buffer
	err := executeEdit([]string{
		"-indexfile", path,
		"-file", "mods/sample.jar", server.URL + "/mod.jar",
		"-file", "mods/new.jar", server.URL + "/mod.jar",
		"-dropfile", "mods/remove.jar",
	}, &out)
	if err != nil {
		t.Fatalf("executeEdit failed: %v", err)
	}

	updated := readSBIndex(t, path)
	if len(updated.Files) != 2 {
		t.Fatalf("expected 2 files after upsert/drop, got %d", len(updated.Files))
	}

	var updatedSample *resource.SBFile
	var addedNew *resource.SBFile
	for i := range updated.Files {
		switch updated.Files[i].Path {
		case "mods/sample.jar":
			updatedSample = &updated.Files[i]
		case "mods/new.jar":
			addedNew = &updated.Files[i]
		}
	}
	if updatedSample == nil || addedNew == nil {
		t.Fatalf("expected sample and new files, got %+v", updated.Files)
	}

	if updatedSample.Hashes["sha1"] != sha1Hex(content) || updatedSample.Hashes["sha256"] != sha256Hex(content) {
		t.Fatalf("sample hashes not updated correctly: %+v", updatedSample.Hashes)
	}
	if updatedSample.FileSize != int64(len(content)) {
		t.Fatalf("sample fileSize mismatch: %d", updatedSample.FileSize)
	}
	if len(updatedSample.Downloads) != 1 || updatedSample.Downloads[0] != server.URL+"/mod.jar" {
		t.Fatalf("sample downloads mismatch: %+v", updatedSample.Downloads)
	}
	if updatedSample.Env == nil || updatedSample.Env.Client != "required" || updatedSample.Env.Server != "optional" {
		t.Fatalf("sample env should be preserved, got %+v", updatedSample.Env)
	}

	if addedNew.Hashes["sha1"] != sha1Hex(content) || addedNew.Hashes["sha256"] != sha256Hex(content) {
		t.Fatalf("new file hashes mismatch: %+v", addedNew.Hashes)
	}
	if addedNew.Env != nil {
		t.Fatalf("new file env should be nil, got %+v", addedNew.Env)
	}
}

func TestExecuteEdit_FileRequiresPathAndURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sb.index.json")
	writeSBIndex(t, path, resource.SBIndex{
		FormatVersion: 1,
		Name:          "Pack",
		Version:       "1.0.0",
		Dependencies:  map[string]string{},
	})

	err := executeEdit([]string{"-indexfile", path, "-file", "mods/sample.jar"}, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("expected error for incomplete -file args")
	}
	if !strings.Contains(err.Error(), "-file requires <path> <url>") {
		t.Fatalf("unexpected error: %v", err)
	}
}
