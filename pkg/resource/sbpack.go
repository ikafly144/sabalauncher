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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kr/binarydist"
)

const (
	SBPackFormatVersion  = 2
	SBPatchFormatVersion = 3
)

//TODO: rename SBIndex to SBPackIndex

type SBIndex struct {
	FormatVersion int               `json:"formatVersion"`
	Name          string            `json:"name"`
	ID            uuid.UUID         `json:"id"`
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

type SBEnvSide string

const (
	SBEnvRequired    SBEnvSide = "required"
	SBEnvOptional    SBEnvSide = "optional"
	SBEnvUnsupported SBEnvSide = "unsupported"
)

type SBEnvironment struct {
	Client SBEnvSide `json:"client"`
	Server SBEnvSide `json:"server"`
}

type SBPatch struct {
	FormatVersion int       `json:"formatVersion"`
	BaseID        uuid.UUID `json:"baseID"`
	Index         SBIndex   `json:"index"`
	RemovedFiles  []string  `json:"removedFiles"`
}

type SBRepository struct {
	Name    string        `json:"name"`
	Patches []SBRepoPatch `json:"patches"`
}

type SBRepoPatch struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"` // "sbpack", "sbpatch"
	Hash       map[string]string `json:"hash"`
	RemotePath string            `json:"remote_path"`
	LocalPath  string            `json:"local_path,omitempty"`
	Timestamp  int64             `json:"timestamp"`
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

	// Sort patches by timestamp (stable sort)
	slices.SortStableFunc(repo.Patches, func(a, b SBRepoPatch) int {
		if a.Timestamp < b.Timestamp {
			return -1
		}
		if a.Timestamp > b.Timestamp {
			return 1
		}
		return 0
	})

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

func downloadAndVerifyRepoPatch(p SBRepoPatch, observer ProgressObserver) (string, error) {
	localPath := getRepoPatchLocalPath(p)
	if verifyHashes(localPath, p.Hash) == nil {
		return localPath, nil
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return "", err
	}

	taskName := "Downloading " + filepath.Base(localPath)
	if err := downloadWithVerify(p.RemotePath, localPath, p.Hash, observer, taskName, "download"); err != nil {
		return "", err
	}

	return localPath, nil
}

func ImportRemoteSBPack(manifestURL string, destDir string, uid uuid.UUID, observer ProgressObserver) (*Instance, error) {
	if observer == nil {
		observer = &NopProgressObserver{}
	}

	observer.OnProgress("Fetching repository manifest", 0, "", "main")
	repo, err := FetchRepository(manifestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository manifest: %w", err)
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

	observer.OnProgress("Downloading initial base pack", 10, "", "main")
	localPackPath, err := downloadAndVerifyRepoPatch(*initialPatch, observer)
	if err != nil {
		return nil, fmt.Errorf("failed to download and verify initial patch: %w", err)
	}

	observer.OnProgress("Importing base pack", 30, "", "main")
	inst, err := ImportSBPack(localPackPath, destDir, uid, observer)
	if err != nil {
		return nil, fmt.Errorf("failed to import initial sbpack: %w", err)
	}
	inst.Upstream.ManifestURL = manifestURL
	inst.Upstream.Version = initialPatch.ID

	// 2. Apply patches sequentially up to latest_patch
	latestPatchID := repo.Patches[len(repo.Patches)-1].ID
	if inst.Upstream.Version != latestPatchID {
		observer.OnProgress("Applying updates", 50, "", "main")
		if err := UpdateInstanceRemoteWithObserver(inst, observer); err != nil {
			return nil, fmt.Errorf("failed to apply initial patches: %w", err)
		}
	}

	observer.OnProgress("Registration complete", 100, "", "main")
	return inst, nil
}

func UpdateInstanceRemote(inst *Instance) error {
	return UpdateInstanceRemoteWithObserver(inst, &NopProgressObserver{})
}

func UpdateInstanceRemoteWithObserver(inst *Instance, observer ProgressObserver) error {
	if inst.Upstream == nil || inst.Upstream.ManifestURL == "" {
		return fmt.Errorf("instance does not have a remote manifest")
	}

	repo, err := FetchRepository(inst.Upstream.ManifestURL)
	if err != nil {
		return err
	}

	if len(repo.Patches) == 0 {
		return nil
	}

	latestPatchID := repo.Patches[len(repo.Patches)-1].ID
	if inst.Upstream.Version == latestPatchID {
		slog.Info("Instance is already up to date", "name", inst.Name)
		return nil
	}

	// Find the chain of patches from current version to latest
	applying := false
	appliedCount := 0
	patchesToApply := []SBRepoPatch{}
	for _, p := range repo.Patches {
		if !applying {
			if p.ID == inst.Upstream.Version {
				applying = true
			}
			continue
		}
		patchesToApply = append(patchesToApply, p)
	}

	if !applying {
		return fmt.Errorf("current version '%s' not found in repository manifest", inst.Upstream.Version)
	}

	totalPatches := len(patchesToApply)

	// 1. Parallel Download all patches
	var wg sync.WaitGroup
	downloadErrs := make(chan error, totalPatches)

	for _, p := range patchesToApply {
		wg.Add(1)
		go func(p SBRepoPatch) {
			defer wg.Done()
			_, err := downloadAndVerifyRepoPatch(p, observer)
			if err != nil {
				downloadErrs <- fmt.Errorf("failed to download patch %s: %w", p.ID, err)
			}
		}(p)
	}

	wg.Wait()
	close(downloadErrs)

	for err := range downloadErrs {
		if err != nil {
			return err
		}
	}

	// 2. Sequential Apply
	for i, p := range patchesToApply {
		progress := (float64(i) / float64(totalPatches)) * 100.0
		observer.OnProgress(fmt.Sprintf("Applying patch %d/%d (%s)", i+1, totalPatches, p.ID), progress, "", "main")

		localPath := getRepoPatchLocalPath(p)

		switch p.Type {
		case "sbpatch":
			if err := ApplySBPatch(inst, localPath, observer); err != nil {
				return err
			}
		case "sbpack":
			if err := ApplySBPack(inst, localPath, observer); err != nil {
				return err
			}
		}
		inst.Upstream.Version = p.ID
		appliedCount++
	}

	if appliedCount == 0 && inst.Upstream.Version != latestPatchID {
		return fmt.Errorf("failed to find update path from '%s' to '%s'", inst.Upstream.Version, latestPatchID)
	}

	return nil
}

// ImportSBPack imports a new instance from an .sbpack ZIP file.
func ImportSBPack(packPath string, destDir string, uid uuid.UUID, observer ProgressObserver) (*Instance, error) {
	if observer == nil {
		observer = &NopProgressObserver{}
	}
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

	if index.FormatVersion < SBPackFormatVersion {
		return nil, fmt.Errorf("unsupported sbpack format version: %d (requires %d)", index.FormatVersion, SBPackFormatVersion)
	}

	inst := &Instance{
		Name:     index.Name,
		UID:      uid,
		Versions: make([]InstanceVersion, 0, len(index.Dependencies)),
		Mods:     []Mod{},
		Path:     destDir,
		Upstream: &Upstream{
			Version: index.ID.String(),
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
	overrideFiles := []string{}
	for _, f := range reader.File {
		if strings.HasPrefix(f.Name, "overrides/") && !f.FileInfo().IsDir() {
			overrideFiles = append(overrideFiles, f.Name)
		}
	}
	totalExtract := len(overrideFiles)

	for i, fName := range overrideFiles {
		f, _ := reader.Open(fName) // fName exists
		relPath := strings.TrimPrefix(fName, "overrides/")
		if relPath == "" {
			f.Close()
			continue
		}

		percentage := float64(i) / float64(totalExtract) * 100.0
		observer.OnProgress("Extracting "+filepath.Base(relPath), percentage, fmt.Sprintf("%d/%d", i+1, totalExtract), "main")

		destPath := filepath.Join(destDir, relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			f.Close()
			return nil, err
		}

		destFile, err := os.Create(destPath)
		if err != nil {
			f.Close()
			return nil, err
		}

		_, err = io.Copy(destFile, f)
		destFile.Close()
		f.Close()
		if err != nil {
			return nil, err
		}
	}

	// For compatibility with previous loop structure if directories needed
	for _, f := range reader.File {
		if after, ok := strings.CutPrefix(f.Name, "overrides/"); ok {
			if f.FileInfo().IsDir() && after != "" {
				_ = os.MkdirAll(filepath.Join(destDir, after), 0755)
			}
		}
	}

	// Download and verify files
	for _, fileInfo := range index.Files {
		// Only download if client is not unsupported
		if fileInfo.Env != nil && fileInfo.Env.Client == SBEnvUnsupported {
			continue
		}

		destPath := filepath.Join(destDir, fileInfo.Path)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, err
		}

		// Download
		if len(fileInfo.Downloads) > 0 {
			taskName := "Downloading " + filepath.Base(fileInfo.Path)
			if err := downloadWithVerify(fileInfo.Downloads[0], destPath, fileInfo.Hashes, observer, taskName, "main"); err != nil {
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
	if err := os.WriteFile(filepath.Join(destDir, "sb.index.json"), indexBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to save index: %w", err)
	}

	return inst, nil
}

// ApplySBPack updates an existing instance using a full .sbpack file.
func ApplySBPack(inst *Instance, packPath string, observer ProgressObserver) error {
	if observer == nil {
		observer = &NopProgressObserver{}
	}
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

	if newIndex.FormatVersion < SBPackFormatVersion {
		return fmt.Errorf("unsupported sbpack format version: %d (requires %d)", newIndex.FormatVersion, SBPackFormatVersion)
	}

	// Load old index to find removed files
	var oldIndex SBIndex
	oldIndexBytes, err := os.ReadFile(filepath.Join(inst.Path, "sb.index.json"))
	if err == nil {
		_ = json.Unmarshal(oldIndexBytes, &oldIndex)
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
		_ = os.Remove(filepath.Join(inst.Path, removed))
	}

	// Unzip overrides from new pack
	overrideFiles := []string{}
	for _, f := range reader.File {
		if strings.HasPrefix(f.Name, "overrides/") && !f.FileInfo().IsDir() {
			overrideFiles = append(overrideFiles, f.Name)
		}
	}
	totalExtract := len(overrideFiles)

	for i, fName := range overrideFiles {
		f, _ := reader.Open(fName)
		relPath := strings.TrimPrefix(fName, "overrides/")
		if relPath == "" {
			f.Close()
			continue
		}

		percentage := float64(i) / float64(totalExtract) * 100.0
		observer.OnProgress("Extracting "+filepath.Base(relPath), percentage, fmt.Sprintf("%d/%d", i+1, totalExtract), "main")

		destPath := filepath.Join(inst.Path, relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			f.Close()
			return err
		}

		df, err := os.Create(destPath)
		if err != nil {
			f.Close()
			return err
		}

		_, err = io.Copy(df, f)
		df.Close()
		f.Close()
		if err != nil {
			return err
		}
	}

	// For compatibility with directories
	for _, f := range reader.File {
		if after, ok := strings.CutPrefix(f.Name, "overrides/"); ok {
			if f.FileInfo().IsDir() && after != "" {
				_ = os.MkdirAll(filepath.Join(inst.Path, after), 0755)
			}
		}
	}

	// Download/Verify files
	newMods := []Mod{}
	for _, fileInfo := range newIndex.Files {
		if fileInfo.Env != nil && fileInfo.Env.Client == SBEnvUnsupported {
			continue
		}
		destPath := filepath.Join(inst.Path, fileInfo.Path)
		if verifyHashes(destPath, fileInfo.Hashes) != nil && len(fileInfo.Downloads) > 0 {
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return err
			}
			taskName := "Downloading " + filepath.Base(fileInfo.Path)
			if err := downloadWithVerify(fileInfo.Downloads[0], destPath, fileInfo.Hashes, observer, taskName, "main"); err != nil {
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
		inst.Upstream.Version = newIndex.ID.String()
	}
	inst.Versions = make([]InstanceVersion, 0, len(newIndex.Dependencies))
	for id, ver := range newIndex.Dependencies {
		inst.Versions = append(inst.Versions, InstanceVersion{ID: id, Version: ver})
	}
	inst.Mods = newMods

	// Save new index
	newIndexBytes, _ := json.MarshalIndent(newIndex, "", "  ")
	if err := os.WriteFile(filepath.Join(inst.Path, "sb.index.json"), newIndexBytes, 0644); err != nil {
		return fmt.Errorf("failed to save new index: %w", err)
	}

	return nil
}

// ApplySBPatch applies an .sbpatch file to an existing instance.
func ApplySBPatch(inst *Instance, patchPath string, observer ProgressObserver) error {
	if observer == nil {
		observer = &NopProgressObserver{}
	}
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

	if patch.FormatVersion < SBPatchFormatVersion {
		return fmt.Errorf("unsupported sbpatch format version: %d (requires %d)", patch.FormatVersion, SBPatchFormatVersion)
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

	if currentIndex.ID != patch.BaseID {
		return fmt.Errorf("version mismatch: instance modpack is at %s, patch requires %s",
			currentIndex.ID, patch.BaseID)
	}

	// 1. Delete removed files
	for _, removed := range patch.RemovedFiles {
		// Sanitize path: overrides/ in zip is extracted to instance root
		cleanPath := strings.TrimPrefix(removed, "overrides/") // TODO: remove this hack by standardizing patch format to not include "overrides/" prefix
		targetPath := filepath.Join(inst.Path, cleanPath)
		if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove file %s: %w", targetPath, err)
		}
	}

	// 2. Unzip overrides and apply patches
	type patchTask struct {
		f    *zip.File
		mode string // "extract" or "patch"
	}
	tasks := []patchTask{}
	for _, f := range reader.File {
		if strings.HasPrefix(f.Name, "overrides/") && !f.FileInfo().IsDir() {
			tasks = append(tasks, patchTask{f, "extract"})
		} else if strings.HasPrefix(f.Name, "patches/") && !f.FileInfo().IsDir() {
			tasks = append(tasks, patchTask{f, "patch"})
		}
	}
	totalTasks := len(tasks)

	for i, t := range tasks {
		f := t.f
		percentage := float64(i) / float64(totalTasks) * 100.0

		if t.mode == "extract" {
			relPath := strings.TrimPrefix(f.Name, "overrides/")
			observer.OnProgress("Extracting "+filepath.Base(relPath), percentage, fmt.Sprintf("%d/%d", i+1, totalTasks), "main")

			destPath := filepath.Join(inst.Path, relPath)
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
		} else {
			relPath := strings.TrimPrefix(f.Name, "patches/")
			observer.OnProgress("Patching "+filepath.Base(relPath), percentage, fmt.Sprintf("%d/%d", i+1, totalTasks), "main")

			targetPath := filepath.Join(inst.Path, relPath)
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}

			oldFile, err := os.Open(targetPath)
			if err != nil {
				return fmt.Errorf("failed to open old file for patching %s: %w", relPath, err)
			}

			patchFile, err := f.Open()
			if err != nil {
				oldFile.Close()
				return err
			}

			tempFile, err := os.CreateTemp("", "sbpatch-*")
			if err != nil {
				oldFile.Close()
				patchFile.Close()
				return err
			}

			if err := binarydist.Patch(oldFile, tempFile, patchFile); err != nil {
				oldFile.Close()
				patchFile.Close()
				tempFile.Close()
				_ = os.Remove(tempFile.Name())
				return fmt.Errorf("failed to apply binary patch to %s: %w", relPath, err)
			}

			oldFile.Close()
			patchFile.Close()
			tempFile.Close()

			if err := os.Remove(targetPath); err != nil {
				_ = os.Remove(tempFile.Name())
				return err
			}
			if err := os.Rename(tempFile.Name(), targetPath); err != nil {
				return err
			}
		}
	}

	// For compatibility with directories
	for _, f := range reader.File {
		if after, ok := strings.CutPrefix(f.Name, "overrides/"); ok {
			if f.FileInfo().IsDir() && after != "" {
				_ = os.MkdirAll(filepath.Join(inst.Path, after), 0755)
			}
		}
	}

	// 3. Download/Verify new index files
	newMods := []Mod{}
	for _, fileInfo := range patch.Index.Files {
		if fileInfo.Env != nil && fileInfo.Env.Client == SBEnvUnsupported {
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
			taskName := "Downloading " + filepath.Base(fileInfo.Path)
			if err := downloadWithVerify(fileInfo.Downloads[0], destPath, fileInfo.Hashes, observer, taskName, "main"); err != nil {
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
		inst.Upstream.Version = patch.Index.ID.String()
	}
	inst.Versions = make([]InstanceVersion, 0, len(patch.Index.Dependencies))
	for id, ver := range patch.Index.Dependencies {
		inst.Versions = append(inst.Versions, InstanceVersion{
			ID:      id,
			Version: ver,
		})
	}
	inst.Mods = newMods

	// Save new index
	newIndexBytes, _ := json.MarshalIndent(patch.Index, "", "  ")
	if err := os.WriteFile(filepath.Join(inst.Path, "sb.index.json"), newIndexBytes, 0644); err != nil {
		return fmt.Errorf("failed to save new index: %w", err)
	}

	return nil
}

func downloadWithVerify(url, dest string, hashes map[string]string, observer ProgressObserver, taskName string, category string) error {
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

	if observer == nil {
		observer = &NopProgressObserver{}
	}

	total := resp.ContentLength
	pw := &progressWriter{
		total:    total,
		observer: observer,
		taskName: taskName,
		category: category,
		writer:   out,
	}

	if _, err := io.Copy(pw, resp.Body); err != nil {
		return err
	}

	return verifyHashes(dest, hashes)
}

type progressWriter struct {
	total    int64
	current  int64
	observer ProgressObserver
	taskName string
	category string
	writer   io.Writer
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if err != nil {
		return n, err
	}
	pw.current += int64(n)
	if pw.total > 0 {
		percentage := float64(pw.current) / float64(pw.total) * 100.0
		pw.observer.OnProgress(pw.taskName, percentage, fmt.Sprintf("%.1f%%", percentage), pw.category)
	}
	return n, nil
}

func verifyHashes(path string, hashes map[string]string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for algo, expectedHash := range hashes {
		if _, err := f.Seek(0, 0); err != nil {
			return err
		}
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
