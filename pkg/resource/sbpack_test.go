package resource_test

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
	"github.com/kr/binarydist"
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
		switch r.URL.Path {
		case "/mod1.jar":
			_, _ = w.Write(mod1Content)
		case "/mod2.jar":
			_, _ = w.Write(mod2Content)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	packPath := filepath.Join(tempDir, "test.sbpack")
	patchPath := filepath.Join(tempDir, "test.sbpatch")
	destDir := filepath.Join(tempDir, "instance")

	v1ID, _ := uuid.NewV7()
	v2ID, _ := uuid.NewV7()

	// 2. Create .sbpack
	index := resource.SBIndex{
		FormatVersion: resource.SBPackFormatVersion,
		Name:          "Test Pack",
		ID:            v1ID,
		Dependencies: map[string]string{
			"minecraft": "1.20.1",
		},
		Files: []resource.SBFile{
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
	inst, err := resource.ImportSBPack(packPath, destDir, testUID)
	if err != nil {
		t.Fatalf("ImportSBPack failed: %v", err)
	}

	if inst.UID != testUID {
		t.Errorf("expected UID %v, got %v", testUID, inst.UID)
	}

	if inst.Name != "Test Pack" {
		t.Errorf("expected instance name 'Test Pack', got %s", inst.Name)
	}
	if inst.Upstream.Version != v1ID.String() {
		t.Errorf("expected version '%s', got %s", v1ID.String(), inst.Upstream.Version)
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
	patchIndex := resource.SBIndex{
		FormatVersion: resource.SBPackFormatVersion,
		Name:          "Test Pack",
		ID:            v2ID,
		Dependencies: map[string]string{
			"minecraft": "1.20.2",
		},
		Files: []resource.SBFile{
			{
				Path:      "mods/mod2.jar",
				Hashes:    map[string]string{"sha256": calculateSHA256(mod2Content)},
				Downloads: []string{server.URL + "/mod2.jar"},
				FileSize:  int64(len(mod2Content)),
			},
		},
	}

	patch := resource.SBPatch{
		FormatVersion: resource.SBPatchFormatVersion,
		BaseID:        v1ID,
		Index:         patchIndex,
		RemovedFiles:  []string{"mods/mod1.jar", "config/m1.txt"},
	}
	patchBytes, _ := json.Marshal(patch)

	createMockZip(t, patchPath, map[string][]byte{
		"sb.patch.json":           patchBytes,
		"overrides/config/m2.txt": []byte("config2"),
	})

	// 5. Test Apply
	if err := resource.ApplySBPatch(inst, patchPath); err != nil {
		t.Fatalf("ApplySBPatch failed: %v", err)
	}

	if inst.Upstream.Version != v2ID.String() {
		t.Errorf("expected version '%s', got %s", v2ID.String(), inst.Upstream.Version)
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
	content, err := os.ReadFile(filepath.Join(destDir, "mods/mod2.jar"))
	if err != nil || string(content) != "mod2_content" {
		t.Errorf("mod2.jar missing or content mismatch: %v", err)
	}
	content, err = os.ReadFile(filepath.Join(destDir, "config/m2.txt"))
	if err != nil || string(content) != "config2" {
		t.Errorf("config/m2.txt missing or content mismatch: %v", err)
	}
}

func TestSBPatchBinaryPatch(t *testing.T) {
	tempDir := t.TempDir()
	destDir := filepath.Join(tempDir, "instance")
	_ = os.MkdirAll(destDir, 0755)

	v1ID, _ := uuid.NewV7()
	v2ID, _ := uuid.NewV7()

	// 1. Create base instance
	v1Content := []byte("original content for binary patching test")
	v1Data := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x02, 0x03}
	v1Index := resource.SBIndex{
		FormatVersion: resource.SBPackFormatVersion,
		Name:          "Binary Patch Test",
		ID:            v1ID,
	}
	v1IndexBytes, _ := json.Marshal(v1Index)
	_ = os.WriteFile(filepath.Join(destDir, "sb.index.json"), v1IndexBytes, 0644)
	_ = os.MkdirAll(filepath.Join(destDir, "config"), 0755)
	_ = os.WriteFile(filepath.Join(destDir, "config/test.txt"), v1Content, 0644)
	_ = os.WriteFile(filepath.Join(destDir, "data.bin"), v1Data, 0644)

	inst := &resource.Instance{
		Path: destDir,
		Upstream: &resource.Upstream{
			Version: v1ID.String(),
		},
	}

	// 2. Create binary patch
	v2Content := []byte("modified content for binary patching test! Extra data here.")
	v2Data := []byte{0xDE, 0xAD, 0xBE, 0xEF, 0x09, 0x02, 0x03} // Changed one byte

	// Helper to generate bsdiff patch in memory
	genPatch := func(old, new []byte) []byte {
		var buf bytes.Buffer
		err := binarydist.Diff(bytes.NewReader(old), bytes.NewReader(new), &buf)
		if err != nil {
			t.Fatalf("failed to generate binary diff: %v", err)
		}
		return buf.Bytes()
	}

	patchTxt := genPatch(v1Content, v2Content)
	patchBin := genPatch(v1Data, v2Data)

	patch := resource.SBPatch{
		FormatVersion: resource.SBPatchFormatVersion,
		BaseID:        v1ID,
		Index: resource.SBIndex{
			FormatVersion: resource.SBPackFormatVersion,
			ID:            v2ID,
		},
	}
	patchBytes, _ := json.Marshal(patch)

	patchPath := filepath.Join(tempDir, "test.sbpatch")
	createMockZip(t, patchPath, map[string][]byte{
		"sb.patch.json":           patchBytes,
		"patches/config/test.txt": patchTxt,
		"patches/data.bin":        patchBin,
		"overrides/new.txt":       []byte("freshly added"),
	})

	// 3. Apply patch
	if err := resource.ApplySBPatch(inst, patchPath); err != nil {
		t.Fatalf("ApplySBPatch failed: %v", err)
	}

	// 4. Verify results
	if got, _ := os.ReadFile(filepath.Join(destDir, "config/test.txt")); !bytes.Equal(got, v2Content) {
		t.Errorf("text file patch mismatch\nexp: %q\ngot: %q", v2Content, got)
	}
	if got, _ := os.ReadFile(filepath.Join(destDir, "data.bin")); !bytes.Equal(got, v2Data) {
		t.Errorf("binary file patch mismatch\nexp: %v\ngot: %v", v2Data, got)
	}
	if got, _ := os.ReadFile(filepath.Join(destDir, "new.txt")); string(got) != "freshly added" {
		t.Errorf("new file content mismatch: %q", got)
	}
	if inst.Upstream.Version != v2ID.String() {
		t.Errorf("expected version %s, got %s", v2ID.String(), inst.Upstream.Version)
	}
}

func TestImportRemoteSBPack(t *testing.T) {
	mod1Content := []byte("mod1_content")
	mod2Content := []byte("mod2_content")

	v1ID, _ := uuid.NewV7()
	v2ID, _ := uuid.NewV7()

	// Pre-calculate ZIP contents so we can pre-calculate hashes for the manifest
	v1Index := resource.SBIndex{
		FormatVersion: resource.SBPackFormatVersion,
		Name:          "Remote Pack",
		ID:            v1ID,
		Dependencies:  map[string]string{"minecraft": "1.20.1"},
		Files: []resource.SBFile{
			{
				Path:      "mods/mod1.jar",
				Hashes:    map[string]string{"sha256": calculateSHA256(mod1Content)},
				Downloads: []string{"WILL_BE_REPLACED_V1"},
				FileSize:  int64(len(mod1Content)),
			},
		},
	}

	v11Patch := resource.SBPatch{
		FormatVersion: resource.SBPatchFormatVersion,
		BaseID:        v1ID,
		Index: resource.SBIndex{
			FormatVersion: resource.SBPackFormatVersion,
			Name:          "Remote Pack",
			ID:            v2ID,
			Dependencies:  map[string]string{"minecraft": "1.20.1"},
			Files: []resource.SBFile{
				{
					Path:      "mods/mod2.jar",
					Hashes:    map[string]string{"sha256": calculateSHA256(mod2Content)},
					Downloads: []string{"WILL_BE_REPLACED_V11"},
					FileSize:  int64(len(mod2Content)),
				},
			},
		},
		RemovedFiles: []string{"mods/mod1.jar"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repo/v1.sbpack":
			tempZip := filepath.Join(t.TempDir(), "v1.sbpack")
			idx := v1Index
			idx.Files[0].Downloads = []string{fmt.Sprintf("http://%s/mod1.jar", r.Host)}
			idxB, _ := json.Marshal(idx)
			createMockZip(t, tempZip, map[string][]byte{"sb.index.json": idxB})
			packB, _ := os.ReadFile(tempZip)
			_, _ = w.Write(packB)
		case "/repo/v1.1.sbpatch":
			tempZip := filepath.Join(t.TempDir(), "v1.1.sbpatch")
			patch := v11Patch
			patch.Index.Files[0].Downloads = []string{fmt.Sprintf("http://%s/mod2.jar", r.Host)}
			idxB, _ := json.Marshal(patch)
			createMockZip(t, tempZip, map[string][]byte{"sb.patch.json": idxB})
			patchB, _ := os.ReadFile(tempZip)
			_, _ = w.Write(patchB)
		case "/mod1.jar":
			_, _ = w.Write(mod1Content)
		case "/mod2.jar":
			_, _ = w.Write(mod2Content)
		case "/repo/manifest.json":
			// Calculate real hashes for the ZIPs we would serve
			v1Temp := filepath.Join(t.TempDir(), "v1_hash.sbpack")
			v1Idx := v1Index
			v1Idx.Files[0].Downloads = []string{fmt.Sprintf("http://%s/mod1.jar", r.Host)}
			v1IdxB, _ := json.Marshal(v1Idx)
			createMockZip(t, v1Temp, map[string][]byte{"sb.index.json": v1IdxB})
			v1Hash, _ := os.ReadFile(v1Temp)

			v11Temp := filepath.Join(t.TempDir(), "v11_hash.sbpatch")
			v11P := v11Patch
			v11P.Index.Files[0].Downloads = []string{fmt.Sprintf("http://%s/mod2.jar", r.Host)}
			v11IdxB, _ := json.Marshal(v11P)
			createMockZip(t, v11Temp, map[string][]byte{"sb.patch.json": v11IdxB})
			v11Hash, _ := os.ReadFile(v11Temp)

			repo := resource.SBRepository{
				Name:        "Remote Pack",
				LatestPatch: v2ID.String(),
				Patches: []resource.SBRepoPatch{
					{
						ID:         v1ID.String(),
						Type:       "sbpack",
						Hash:       map[string]string{"sha256": calculateSHA256(v1Hash)},
						RemotePath: fmt.Sprintf("http://%s/repo/v1.sbpack", r.Host),
					},
					{
						ID:         v2ID.String(),
						Type:       "sbpatch",
						Hash:       map[string]string{"sha256": calculateSHA256(v11Hash)},
						RemotePath: fmt.Sprintf("http://%s/repo/v1.1.sbpatch", r.Host),
					},
				},
			}
			_ = json.NewEncoder(w).Encode(repo)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Update DataDir for caching
	oldDataDir := resource.DataDir
	resource.DataDir = t.TempDir()
	defer func() { resource.DataDir = oldDataDir }()

	destDir := filepath.Join(t.TempDir(), "remote-instance")
	uid := uuid.New()

	inst, err := resource.ImportRemoteSBPack(server.URL+"/repo/manifest.json", destDir, uid)
	if err != nil {
		t.Fatalf("ImportRemoteSBPack failed: %v", err)
	}

	if inst.Upstream.Version != v2ID.String() {
		t.Errorf("expected version %s, got %s", v2ID.String(), inst.Upstream.Version)
	}

	if _, err := os.Stat(filepath.Join(destDir, "mods/mod2.jar")); err != nil {
		t.Errorf("mod2.jar missing: %v", err)
	}
}
