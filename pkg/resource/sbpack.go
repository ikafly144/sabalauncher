package resource

import (
	"archive/zip"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type SBIndex struct {
	FormatVersion int               `json:"formatVersion"`
	Name          string            `json:"name"`
	Version       string            `json:"version"`
	Dependencies  map[string]string `json:"dependencies"`
	Files         []SBFile          `json:"files"`
}

type SBFile struct {
	Path      string            `json:"path"`
	Hashes    map[string]string `json:"hashes"`
	Downloads []string          `json:"downloads,omitempty"`
	FileSize  int64             `json:"fileSize"`
	Env       *SBEnvironment    `json:"env,omitempty"`
}

type SBEnvironment struct {
	Client string `json:"client"` // "required", "optional", "unsupported"
	Server string `json:"server"`
}

type SBPatch struct {
	FormatVersion int      `json:"formatVersion"`
	FromVersion   string   `json:"fromVersion"`
	ToVersion     string   `json:"toVersion"`
	NewIndex      SBIndex  `json:"newIndex"`
	RemovedFiles  []string `json:"removedFiles"`
}

type SBRepository struct {
	Name        string        `json:"name"`
	LatestPatch string        `json:"latest_patch"`
	Patches     []SBRepoPatch `json:"patches"`
}

type SBRepoPatch struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"` // "sbpack", "sbpatch"
	Hash       map[string]string `json:"hash"`
	RemotePath string            `json:"remote_path"`
	LocalPath  string            `json:"local_path,omitempty"`
}

func FetchRepository(url string) (*SBRepository, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch repository: %s", resp.Status)
	}

	var repo SBRepository
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

func getRepoPatchLocalPath(p SBRepoPatch) string {
	if p.LocalPath != "" {
		return filepath.Join(DataDir, "cache", filepath.FromSlash(p.LocalPath))
	}
	// Automatic generation: /hash/filename
	hash := p.Hash["sha256"]
	if hash == "" {
		hash = p.Hash["sha1"]
	}
	if hash == "" {
		hash = "unknown"
	}
	filename := path.Base(p.RemotePath)
	return filepath.Join(DataDir, "cache", hash, filename)
}

func downloadAndVerifyRepoPatch(p SBRepoPatch) (string, error) {
	localPath := getRepoPatchLocalPath(p)
	if verifyHashes(localPath, p.Hash) == nil {
		return localPath, nil
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return "", err
	}

	if err := downloadWithVerify(p.RemotePath, localPath, p.Hash); err != nil {
		return "", err
	}
	return localPath, nil
}

func ImportRemoteSBPack(manifestURL string, destDir string, uid uuid.UUID) (*Instance, error) {
	repo, err := FetchRepository(manifestURL)
	if err != nil {
		return nil, err
	}

	// 1. Find initial sbpack. We'll start with the first one or the one matching repo structure.
	// For simplicity, find the latest sbpack that is <= latest_patch.
	var initialPatch *SBRepoPatch
	for i := len(repo.Patches) - 1; i >= 0; i-- {
		if repo.Patches[i].Type == "sbpack" {
			initialPatch = &repo.Patches[i]
			break
		}
	}

	if initialPatch == nil {
		return nil, fmt.Errorf("no base sbpack found in repository")
	}

	localPackPath, err := downloadAndVerifyRepoPatch(*initialPatch)
	if err != nil {
		return nil, err
	}

	inst, err := ImportSBPack(localPackPath, destDir, uid)
	if err != nil {
		return nil, err
	}
	inst.Upstream.ManifestURL = manifestURL
	inst.Upstream.Version = initialPatch.ID

	// 2. Apply patches sequentially up to latest_patch
	if inst.Upstream.Version != repo.LatestPatch {
		if err := UpdateInstanceRemote(inst); err != nil {
			return nil, fmt.Errorf("failed to apply initial patches: %w", err)
		}
	}

	return inst, nil
}

func UpdateInstanceRemote(inst *Instance) error {
	if inst.Upstream == nil || inst.Upstream.ManifestURL == "" {
		return fmt.Errorf("instance does not have a remote manifest")
	}

	repo, err := FetchRepository(inst.Upstream.ManifestURL)
	if err != nil {
		return err
	}

	if inst.Upstream.Version == repo.LatestPatch {
		slog.Info("Instance is already up to date", "name", inst.Name)
		return nil
	}

	// Find the chain of patches from current version to latest
	applying := false
	appliedCount := 0
	for _, p := range repo.Patches {
		if !applying {
			if p.ID == inst.Upstream.Version {
				applying = true
				continue
			}
			continue
		}

		// Apply this patch
		localPath, err := downloadAndVerifyRepoPatch(p)
		if err != nil {
			return err
		}

		if p.Type == "sbpatch" {
			if err := ApplySBPatch(inst, localPath); err != nil {
				return err
			}
		} else if p.Type == "sbpack" {
			if err := ApplySBPack(inst, localPath); err != nil {
				return err
			}
		}
		inst.Upstream.Version = p.ID
		appliedCount++
	}

	if !applying {
		return fmt.Errorf("current version '%s' not found in repository manifest", inst.Upstream.Version)
	}

	if appliedCount == 0 && inst.Upstream.Version != repo.LatestPatch {
		return fmt.Errorf("failed to find update path from '%s' to '%s'", inst.Upstream.Version, repo.LatestPatch)
	}

	return nil
}

// ImportSBPack imports a new instance from an .sbpack ZIP file.
func ImportSBPack(packPath string, destDir string, uid uuid.UUID) (*Instance, error) {
	reader, err := zip.OpenReader(packPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sbpack: %w", err)
	}
	defer reader.Close()

	var index SBIndex
	var indexFound bool

	// Read index first
	for _, f := range reader.File {
		if f.Name == "sb.index.json" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			err = json.NewDecoder(rc).Decode(&index)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to parse sb.index.json: %w", err)
			}
			indexFound = true
			break
		}
	}

	if !indexFound {
		return nil, fmt.Errorf("sb.index.json not found in pack")
	}

	inst := &Instance{
		Name:     index.Name,
		UID:      uid,
		Versions: make([]InstanceVersion, 0, len(index.Dependencies)),
		Mods:     []Mod{},
		Path:     destDir,
		Upstream: &Upstream{
			Version: index.Version,
		},
	}

	for id, ver := range index.Dependencies {
		inst.Versions = append(inst.Versions, InstanceVersion{
			ID:      id,
			Version: ver,
		})
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, err
	}

	// Unzip overrides
	for _, f := range reader.File {
		if after, ok := strings.CutPrefix(f.Name, "overrides/"); ok {
			relPath := after
			if relPath == "" {
				continue
			}

			destPath := filepath.Join(destDir, relPath)
			if f.FileInfo().IsDir() {
				os.MkdirAll(destPath, 0755)
				continue
			}

			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return nil, err
			}

			rc, err := f.Open()
			if err != nil {
				return nil, err
			}

			destFile, err := os.Create(destPath)
			if err != nil {
				rc.Close()
				return nil, err
			}

			_, err = io.Copy(destFile, rc)
			destFile.Close()
			rc.Close()
			if err != nil {
				return nil, err
			}
		}
	}

	// Download and verify files
	for _, fileInfo := range index.Files {
		// Only download if client is not unsupported
		if fileInfo.Env != nil && fileInfo.Env.Client == "unsupported" {
			continue
		}

		destPath := filepath.Join(destDir, fileInfo.Path)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, err
		}

		// Download
		if len(fileInfo.Downloads) > 0 {
			if err := downloadWithVerify(fileInfo.Downloads[0], destPath, fileInfo.Hashes); err != nil {
				slog.Error("Failed to download file", "path", fileInfo.Path, "error", err)
				return nil, err
			}
		}

		// Assume all JARs in mods/ are mods
		if strings.HasPrefix(fileInfo.Path, "mods/") && strings.HasSuffix(fileInfo.Path, ".jar") {
			modName := filepath.Base(fileInfo.Path)
			inst.Mods = append(inst.Mods, Mod{
				Name:     modName,
				File:     fileInfo.Path,
				Version:  "unknown", // Could extract from jar if needed
				UpdateAt: time.Now(),
				Source: &URLSource{
					ModURL: "",
					FileURI: func() string {
						if len(fileInfo.Downloads) > 0 {
							return fileInfo.Downloads[0]
						} else {
							return ""
						}
					}(),
				},
			})
		}
	}

	// Save index to instance dir for future updates
	indexBytes, _ := json.MarshalIndent(index, "", "  ")
	os.WriteFile(filepath.Join(destDir, "sb.index.json"), indexBytes, 0644)

	return inst, nil
}

// ApplySBPack updates an existing instance using a full .sbpack file.
func ApplySBPack(inst *Instance, packPath string) error {
	reader, err := zip.OpenReader(packPath)
	if err != nil {
		return fmt.Errorf("failed to open sbpack: %w", err)
	}
	defer reader.Close()

	var newIndex SBIndex
	var indexFound bool
	for _, f := range reader.File {
		if f.Name == "sb.index.json" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			err = json.NewDecoder(rc).Decode(&newIndex)
			rc.Close()
			if err != nil {
				return fmt.Errorf("failed to parse sb.index.json: %w", err)
			}
			indexFound = true
			break
		}
	}

	if !indexFound {
		return fmt.Errorf("sb.index.json not found in pack")
	}

	// Load old index to find removed files
	var oldIndex SBIndex
	oldIndexBytes, err := os.ReadFile(filepath.Join(inst.Path, "sb.index.json"))
	if err == nil {
		json.Unmarshal(oldIndexBytes, &oldIndex)
	}

	removedFiles := []string{}
	for _, oldF := range oldIndex.Files {
		found := false
		for _, newF := range newIndex.Files {
			if oldF.Path == newF.Path {
				found = true
				break
			}
		}
		if !found {
			removedFiles = append(removedFiles, oldF.Path)
		}
	}

	// Perform update (similar to patch)
	for _, removed := range removedFiles {
		os.Remove(filepath.Join(inst.Path, removed))
	}

	// Unzip overrides from new pack
	for _, f := range reader.File {
		if after, ok := strings.CutPrefix(f.Name, "overrides/"); ok {
			relPath := after
			if relPath == "" {
				continue
			}
			destPath := filepath.Join(inst.Path, relPath)
			if f.FileInfo().IsDir() {
				if err := os.MkdirAll(destPath, 0755); err != nil {
					return err
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return err
			}
			rc, err := f.Open()
			if err != nil {
				return err
			}
			df, err := os.Create(destPath)
			if err != nil {
				rc.Close()
				return err
			}
			_, err = io.Copy(df, rc)
			df.Close()
			rc.Close()
			if err != nil {
				return err
			}
		}
	}

	// Download/Verify files
	newMods := []Mod{}
	for _, fileInfo := range newIndex.Files {
		if fileInfo.Env != nil && fileInfo.Env.Client == "unsupported" {
			continue
		}
		destPath := filepath.Join(inst.Path, fileInfo.Path)
		if verifyHashes(destPath, fileInfo.Hashes) != nil && len(fileInfo.Downloads) > 0 {
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return err
			}
			if err := downloadWithVerify(fileInfo.Downloads[0], destPath, fileInfo.Hashes); err != nil {
				return err
			}
		}
		if strings.HasPrefix(fileInfo.Path, "mods/") && strings.HasSuffix(fileInfo.Path, ".jar") {
			modName := filepath.Base(fileInfo.Path)
			newMods = append(newMods, Mod{
				Name:     modName,
				File:     fileInfo.Path,
				Version:  "unknown",
				UpdateAt: time.Now(),
				Source: &URLSource{
					FileURI: func() string {
						if len(fileInfo.Downloads) > 0 {
							return fileInfo.Downloads[0]
						} else {
							return ""
						}
					}(),
				},
			})
		}
	}

	if inst.Upstream != nil {
		inst.Upstream.Version = newIndex.Version
	}
	inst.Versions = make([]InstanceVersion, 0, len(newIndex.Dependencies))
	for id, ver := range newIndex.Dependencies {
		inst.Versions = append(inst.Versions, InstanceVersion{ID: id, Version: ver})
	}
	inst.Mods = newMods

	// Save new index
	newIndexBytes, _ := json.MarshalIndent(newIndex, "", "  ")
	os.WriteFile(filepath.Join(inst.Path, "sb.index.json"), newIndexBytes, 0644)

	return nil
}

// ApplySBPatch applies an .sbpatch file to an existing instance.
func ApplySBPatch(inst *Instance, patchPath string) error {
	reader, err := zip.OpenReader(patchPath)
	if err != nil {
		return fmt.Errorf("failed to open sbpatch: %w", err)
	}
	defer reader.Close()

	var patch SBPatch
	var patchFound bool

	// Read patch index
	for _, f := range reader.File {
		if f.Name == "sb.patch.json" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			err = json.NewDecoder(rc).Decode(&patch)
			rc.Close()
			if err != nil {
				return fmt.Errorf("failed to parse sb.patch.json: %w", err)
			}
			patchFound = true
			break
		}
	}

	if !patchFound {
		return fmt.Errorf("sb.patch.json not found in patch")
	}

	// Load current index from disk to check modpack version
	var currentIndex SBIndex
	currentIndexBytes, err := os.ReadFile(filepath.Join(inst.Path, "sb.index.json"))
	if err != nil {
		return fmt.Errorf("failed to read current index: %w", err)
	}
	if err := json.Unmarshal(currentIndexBytes, &currentIndex); err != nil {
		return fmt.Errorf("failed to parse current index: %w", err)
	}

	if currentIndex.Version != patch.FromVersion {
		return fmt.Errorf("version mismatch: instance modpack is at %s, patch requires %s",
			currentIndex.Version, patch.FromVersion)
	}

	// 1. Delete removed files
	for _, removed := range patch.RemovedFiles {
		targetPath := filepath.Join(inst.Path, removed)
		os.Remove(targetPath) // Ignore if not exist
	}

	// 2. Unzip overrides from patch
	for _, f := range reader.File {
		if after, ok := strings.CutPrefix(f.Name, "overrides/"); ok {
			relPath := after
			if relPath == "" {
				continue
			}

			destPath := filepath.Join(inst.Path, relPath)
			if f.FileInfo().IsDir() {
				os.MkdirAll(destPath, 0755)
				continue
			}

			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return err
			}

			rc, err := f.Open()
			if err != nil {
				return err
			}

			destFile, err := os.Create(destPath)
			if err != nil {
				rc.Close()
				return err
			}

			_, err = io.Copy(destFile, rc)
			destFile.Close()
			rc.Close()
			if err != nil {
				return err
			}
		}
	}

	// 3. Download/Verify new index files
	newMods := []Mod{}
	for _, fileInfo := range patch.NewIndex.Files {
		if fileInfo.Env != nil && fileInfo.Env.Client == "unsupported" {
			continue
		}

		destPath := filepath.Join(inst.Path, fileInfo.Path)

		// Check if file already exists and hashes match
		if verifyHashes(destPath, fileInfo.Hashes) == nil {
			// Already up to date
		} else if len(fileInfo.Downloads) > 0 {
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return err
			}
			if err := downloadWithVerify(fileInfo.Downloads[0], destPath, fileInfo.Hashes); err != nil {
				slog.Error("Failed to download file during patch", "path", fileInfo.Path, "error", err)
				return err
			}
		}

		if strings.HasPrefix(fileInfo.Path, "mods/") && strings.HasSuffix(fileInfo.Path, ".jar") {
			modName := filepath.Base(fileInfo.Path)
			newMods = append(newMods, Mod{
				Name:     modName,
				File:     fileInfo.Path,
				Version:  "unknown",
				UpdateAt: time.Now(),
				Source: &URLSource{
					ModURL: "",
					FileURI: func() string {
						if len(fileInfo.Downloads) > 0 {
							return fileInfo.Downloads[0]
						} else {
							return ""
						}
					}(),
				},
			})
		}
	}

	// Update instance state
	if inst.Upstream != nil {
		inst.Upstream.Version = patch.ToVersion
	}
	inst.Versions = make([]InstanceVersion, 0, len(patch.NewIndex.Dependencies))
	for id, ver := range patch.NewIndex.Dependencies {
		inst.Versions = append(inst.Versions, InstanceVersion{
			ID:      id,
			Version: ver,
		})
	}
	inst.Mods = newMods

	// Save new index
	newIndexBytes, _ := json.MarshalIndent(patch.NewIndex, "", "  ")
	os.WriteFile(filepath.Join(inst.Path, "sb.index.json"), newIndexBytes, 0644)

	return nil
}

func downloadWithVerify(url, dest string, hashes map[string]string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}

	return verifyHashes(dest, hashes)
}

func verifyHashes(path string, hashes map[string]string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for algo, expectedHash := range hashes {
		f.Seek(0, 0)
		var actualHash string

		switch strings.ToLower(algo) {
		case "sha1":
			h := sha1.New()
			if _, err := io.Copy(h, f); err != nil {
				return err
			}
			actualHash = hex.EncodeToString(h.Sum(nil))
		case "sha256":
			h := sha256.New()
			if _, err := io.Copy(h, f); err != nil {
				return err
			}
			actualHash = hex.EncodeToString(h.Sum(nil))
		default:
			// Unsupported algorithm, skip or error
			continue
		}

		if actualHash != expectedHash {
			return fmt.Errorf("hash mismatch for %s: expected %s, got %s", algo, expectedHash, actualHash)
		}
	}

	return nil
}
