package resource

import (
	"archive/zip"
	"context"
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

const uiMaxFilenameLength = 30

const (
	SBPackFormatVersion  = 2
	SBPatchFormatVersion = 3
)

// SBPackIndex represents the content of sb.index.json
type SBPackIndex struct {
	FormatVersion int                   `json:"formatVersion"`
	Name          string                `json:"name"`
	ID            uuid.UUID             `json:"id"`
	Properties    SBPackIndexProperties `json:"properties,omitzero"`
	Dependencies  map[string]string     `json:"dependencies"`
	Files         []SBFile              `json:"files"`
	Hashes        map[string]string     `json:"hashes,omitempty"`
}

type SBPackIndexProperties struct {
	// Path to a preview image for this pack, relative to the instance directory. Optional.
	Icon string `json:"icon,omitempty"`
	// A short description of the pack. Optional.
	Description string `json:"description,omitempty"`

	QuickLaunch SBQuickLaunch `json:"quickLaunch,omitzero"`

	// Optional recommended memory in MB
	// min(max(this value, user setting), machine memory) will be used as the instance memory when this pack is applied
	Memory int `json:"memory,omitempty"`
}

type SBQuickLaunch struct {
	MultiPlayer  string `json:"multiplayer,omitempty"`  // Server address for quick multiplayer
	SinglePlayer string `json:"singleplayer,omitempty"` // Optional command or identifier for quick singleplayer launch
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
	FormatVersion int         `json:"formatVersion"`
	BaseID        uuid.UUID   `json:"baseID"`
	Index         SBPackIndex `json:"index"`
	RemovedFiles  []string    `json:"removedFiles"`
}

type SBRepository struct {
	Name    string        `json:"name"`
	Patches []SBRepoPatch `json:"patches"`
}

type SBRepoPatch struct {
	ID         string            `json:"id"`
	Type       SBPatchType       `json:"type"` // "sbpack", "sbpatch"
	Hash       map[string]string `json:"hash"`
	RemotePath string            `json:"remote_path"`
	LocalPath  string            `json:"local_path,omitempty"`
	Timestamp  int64             `json:"timestamp"`
}

type SBPatchType string

const (
	SBPatchTypePack  SBPatchType = "sbpack"
	SBPatchTypePatch SBPatchType = "sbpatch"
)

func FetchRepository(ctx context.Context, url string) (*SBRepository, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
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

func downloadAndVerifyRepoPatch(ctx context.Context, p SBRepoPatch, observer ProgressObserver) (string, error) {
	localPath := getRepoPatchLocalPath(p)
	if verifyHashes(localPath, p.Hash) == nil {
		return localPath, nil
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return "", err
	}

	taskName := "Downloading " + filepath.Base(localPath)
	if err := downloadWithVerify(ctx, p.RemotePath, localPath, p.Hash, observer, taskName, "download"); err != nil {
		return "", err
	}

	return localPath, nil
}

func ImportRemoteSBPack(ctx context.Context, manifestURL string, destDir string, uid uuid.UUID, observer ProgressObserver) (*Instance, error) {
	if observer == nil {
		observer = &NopProgressObserver{}
	}

	observer.OnProgress("Fetching repository manifest", 0, "", "main")
	repo, err := FetchRepository(ctx, manifestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository manifest: %w", err)
	}

	// 1. Find initial sbpack. We'll start with the first one or the one matching repo structure.
	// For simplicity, find the latest sbpack that is <= latest_patch.
	var initialPatch *SBRepoPatch
	for i := len(repo.Patches) - 1; i >= 0; i-- {
		if repo.Patches[i].Type == SBPatchTypePack {
			initialPatch = &repo.Patches[i]
			break
		}
	}

	if initialPatch == nil {
		return nil, fmt.Errorf("no base sbpack found in repository")
	}

	observer.OnProgress("Downloading initial base pack", 10, "", "main")
	localPackPath, err := downloadAndVerifyRepoPatch(ctx, *initialPatch, observer)
	if err != nil {
		return nil, fmt.Errorf("failed to download and verify initial patch: %w", err)
	}

	observer.OnProgress("Importing base pack", 30, "", "main")
	inst, err := ImportSBPack(ctx, localPackPath, destDir, uid, observer)
	if err != nil {
		return nil, fmt.Errorf("failed to import initial sbpack: %w", err)
	}
	inst.Upstream.ManifestURL = manifestURL
	inst.Upstream.Version = initialPatch.ID

	// 2. Apply patches sequentially up to latest_patch
	latestPatchID := repo.Patches[len(repo.Patches)-1].ID
	if inst.Upstream.Version != latestPatchID {
		observer.OnProgress("Applying updates", 50, "", "main")
		if err := UpdateInstanceRemoteWithObserver(ctx, inst, observer); err != nil {
			return nil, fmt.Errorf("failed to apply initial patches: %w", err)
		}
	}

	observer.OnProgress("Registration complete", 100, "", "main")
	return inst, nil
}

func UpdateInstanceRemote(ctx context.Context, inst *Instance) error {
	return UpdateInstanceRemoteWithObserver(ctx, inst, &NopProgressObserver{})
}

func UpdateInstanceRemoteWithObserver(ctx context.Context, inst *Instance, observer ProgressObserver) error {
	if inst.Upstream == nil || inst.Upstream.ManifestURL == "" {
		return fmt.Errorf("instance does not have a remote manifest")
	}

	repo, err := FetchRepository(ctx, inst.Upstream.ManifestURL)
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
			_, err := downloadAndVerifyRepoPatch(ctx, p, observer)
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

	// Check if context was cancelled during download
	if err := ctx.Err(); err != nil {
		return err
	}

	// 2. Sequential Apply
	for i, p := range patchesToApply {
		progress := (float64(i) / float64(totalPatches)) * 100.0
		observer.OnProgress(fmt.Sprintf("Applying patch %d/%d (%s)", i+1, totalPatches, p.ID), progress, "", "main")

		localPath := getRepoPatchLocalPath(p)

		switch p.Type {
		case SBPatchTypePatch:
			if err := ApplySBPatch(ctx, inst, localPath, observer); err != nil {
				return err
			}
		case SBPatchTypePack:
			if err := ApplySBPack(ctx, inst, localPath, observer); err != nil {
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
func ImportSBPack(ctx context.Context, packPath string, destDir string, uid uuid.UUID, observer ProgressObserver) (*Instance, error) {
	if observer == nil {
		observer = &NopProgressObserver{}
	}
	reader, err := zip.OpenReader(packPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sbpack: %w", err)
	}
	defer reader.Close()

	var index SBPackIndex
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
		Name:       index.Name,
		UID:        uid,
		Properties: index.Properties,
		Versions:   make([]InstanceVersion, 0, len(index.Dependencies)),
		Path:       destDir,
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
		filename := filepath.Base(relPath)
		if len(filename) > uiMaxFilenameLength {
			filename = filename[:uiMaxFilenameLength] + "..."
		}
		observer.OnProgress("Extracting "+filename, percentage, fmt.Sprintf("%d/%d", i+1, totalExtract), "main")

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
			if err := downloadWithVerify(ctx, fileInfo.Downloads[0], destPath, fileInfo.Hashes, observer, taskName, "main"); err != nil {
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
func ApplySBPack(ctx context.Context, inst *Instance, packPath string, observer ProgressObserver) error {
	if observer == nil {
		observer = &NopProgressObserver{}
	}

	backup, err := newInstanceBackup(inst.Path)
	if err != nil {
		return err
	}
	defer backup.Cleanup()

	err = func() error { // TODO: refactor to avoid this closure by making backup.Restore() more flexible
		reader, err := zip.OpenReader(packPath)
		if err != nil {
			return fmt.Errorf("failed to open sbpack: %w", err)
		}
		defer reader.Close()

		var newIndex SBPackIndex
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
		var oldIndex SBPackIndex
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
			_ = backup.Backup(removed)
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
			if err := ctx.Err(); err != nil {
				return err
			}
			f, _ := reader.Open(fName)
			relPath := strings.TrimPrefix(fName, "overrides/")
			if relPath == "" {
				f.Close()
				continue
			}

			percentage := float64(i) / float64(totalExtract) * 100.0
			filename := filepath.Base(relPath)
			if len(filename) > uiMaxFilenameLength {
				filename = filename[:uiMaxFilenameLength] + "..."
			}
			observer.OnProgress("Extracting "+filename, percentage, fmt.Sprintf("%d/%d", i+1, totalExtract), "main")

			_ = backup.Backup(relPath)
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
			if err := ctx.Err(); err != nil {
				return err
			}
			if fileInfo.Env != nil && fileInfo.Env.Client == SBEnvUnsupported {
				continue
			}
			destPath := filepath.Join(inst.Path, fileInfo.Path)
			if verifyHashes(destPath, fileInfo.Hashes) != nil && len(fileInfo.Downloads) > 0 {
				_ = backup.Backup(fileInfo.Path)
				if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
					return err
				}
				taskName := "Downloading " + filepath.Base(fileInfo.Path)
				if err := downloadWithVerify(ctx, fileInfo.Downloads[0], destPath, fileInfo.Hashes, observer, taskName, "main"); err != nil {
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
		inst.Name = newIndex.Name
		inst.Properties = newIndex.Properties
		inst.Versions = make([]InstanceVersion, 0, len(newIndex.Dependencies))
		for id, ver := range newIndex.Dependencies {
			inst.Versions = append(inst.Versions, InstanceVersion{ID: id, Version: ver})
		}
		inst.Mods = newMods

		// Save new index
		_ = backup.Backup("sb.index.json")
		newIndexBytes, _ := json.MarshalIndent(newIndex, "", "  ")
		if err := os.WriteFile(filepath.Join(inst.Path, "sb.index.json"), newIndexBytes, 0644); err != nil {
			return fmt.Errorf("failed to save new index: %w", err)
		}

		return nil
	}()

	if err != nil {
		_ = backup.Restore()
		return err
	}
	return nil
}

// ApplySBPatch applies an .sbpatch file to an existing instance.
func ApplySBPatch(ctx context.Context, inst *Instance, patchPath string, observer ProgressObserver) error {
	if observer == nil {
		observer = &NopProgressObserver{}
	}

	backup, err := newInstanceBackup(inst.Path)
	if err != nil {
		return err
	}
	defer backup.Cleanup()

	err = func() error { // TODO: refactor to avoid this closure by making backup.Restore() more flexible
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
		var currentIndex SBPackIndex
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
			_ = backup.Backup(cleanPath)
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
			if err := ctx.Err(); err != nil {
				return err
			}
			f := t.f
			percentage := float64(i) / float64(totalTasks) * 100.0

			if t.mode == "extract" {
				relPath := strings.TrimPrefix(f.Name, "overrides/")
				filename := filepath.Base(relPath)
				if len(filename) > uiMaxFilenameLength {
					filename = filename[:uiMaxFilenameLength] + "..."
				}
				observer.OnProgress("Extracting "+filename, percentage, fmt.Sprintf("%d/%d", i+1, totalTasks), "main")

				_ = backup.Backup(relPath)
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

				_ = backup.Backup(relPath)
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
			if err := ctx.Err(); err != nil {
				return err
			}
			if fileInfo.Env != nil && fileInfo.Env.Client == SBEnvUnsupported {
				continue
			}

			destPath := filepath.Join(inst.Path, fileInfo.Path)

			// Check if file already exists and hashes match
			if verifyHashes(destPath, fileInfo.Hashes) == nil {
				// Already up to date
			} else if len(fileInfo.Downloads) > 0 {
				_ = backup.Backup(fileInfo.Path)
				if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
					return err
				}
				taskName := "Downloading " + filepath.Base(fileInfo.Path)
				if err := downloadWithVerify(ctx, fileInfo.Downloads[0], destPath, fileInfo.Hashes, observer, taskName, "main"); err != nil {
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
		inst.Name = patch.Index.Name
		inst.Properties = patch.Index.Properties
		inst.Versions = make([]InstanceVersion, 0, len(patch.Index.Dependencies))
		for id, ver := range patch.Index.Dependencies {
			inst.Versions = append(inst.Versions, InstanceVersion{
				ID:      id,
				Version: ver,
			})
		}
		inst.Mods = newMods

		// Save new index
		_ = backup.Backup("sb.index.json")
		newIndexBytes, _ := json.MarshalIndent(patch.Index, "", "  ")
		if err := os.WriteFile(filepath.Join(inst.Path, "sb.index.json"), newIndexBytes, 0644); err != nil {
			return fmt.Errorf("failed to save new index: %w", err)
		}

		return nil
	}()

	if err != nil {
		_ = backup.Restore()
		return err
	}
	return nil
}

func downloadWithVerify(ctx context.Context, url, dest string, hashes map[string]string, observer ProgressObserver, taskName string, category string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
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

// RepairInstance verifies all files in the instance index and re-downloads any that are missing or corrupted.
func RepairInstance(ctx context.Context, inst *Instance, observer ProgressObserver) error {
	if observer == nil {
		observer = &NopProgressObserver{}
	}

	// 1. Load sb.index.json
	indexPath := filepath.Join(inst.Path, "sb.index.json")
	var index SBPackIndex
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("failed to read sb.index.json: %w", err)
	}
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		return fmt.Errorf("failed to parse sb.index.json: %w", err)
	}

	// 2. Identify corrupted/missing files from index
	toRepair := []SBFile{}
	for _, f := range index.Files {
		targetPath := filepath.Join(inst.Path, f.Path)
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			toRepair = append(toRepair, f)
			continue
		}

		if err := verifyHashes(targetPath, f.Hashes); err != nil {
			slog.Warn("File corruption detected", "path", f.Path, "err", err)
			toRepair = append(toRepair, f)
		}
	}

	corruptedOverrides := []string{}
	for rel, expectedHash := range index.Hashes {
		targetPath := filepath.Join(inst.Path, rel)
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			corruptedOverrides = append(corruptedOverrides, rel)
			continue
		}

		if err := verifyHashes(targetPath, map[string]string{"sha256": expectedHash}); err != nil {
			slog.Warn("Override corruption detected", "path", rel, "err", err)
			corruptedOverrides = append(corruptedOverrides, rel)
		}
	}

	if len(toRepair) == 0 && len(corruptedOverrides) == 0 {
		observer.OnProgress("All files verified, no repair needed", 100, "Done", "main")
		return nil
	}

	// 3. Repair SBFiles (downloads)
	totalRepair := len(toRepair)
	for i, f := range toRepair {
		if err := ctx.Err(); err != nil {
			return err
		}

		percentage := float64(i) / float64(totalRepair) * 100.0
		observer.OnProgress(fmt.Sprintf("Repairing mod %d/%d: %s", i+1, totalRepair, filepath.Base(f.Path)), percentage, "", "main")

		targetPath := filepath.Join(inst.Path, f.Path)
		_ = os.MkdirAll(filepath.Dir(targetPath), 0755)

		var downloadErr error
		for _, url := range f.Downloads {
			downloadErr = downloadWithVerify(ctx, url, targetPath, f.Hashes, observer, "Downloading "+filepath.Base(f.Path), "repair")
			if downloadErr == nil {
				break
			}
		}

		if downloadErr != nil {
			return fmt.Errorf("failed to repair file %s: %w", f.Path, downloadErr)
		}
	}

	// 4. Repair Overrides (for remote instances)
	if len(corruptedOverrides) > 0 {
		if inst.Upstream == nil || inst.Upstream.ManifestURL == "" {
			slog.Warn("Cannot repair local overrides without a remote repository", "count", len(corruptedOverrides))
		} else {
			observer.OnProgress("Repairing local files from repository", 0, "", "main")
			repo, err := FetchRepository(ctx, inst.Upstream.ManifestURL)
			if err != nil {
				return fmt.Errorf("failed to fetch repository for repair: %w", err)
			}

			// We need to re-download patches and re-extract corrupted files.
			// This is a bit heavy, but safe.
			// TODO: Optimize by only downloading relevant patches if possible.
			for _, p := range repo.Patches {
				if err := ctx.Err(); err != nil {
					return err
				}

				// Check if this patch contains any of our corrupted files
				// Since we don't have a file-to-patch map, we have to check them all or re-apply.
				// For simplicity, re-extract from all patches if needed.
				localPath, err := downloadAndVerifyRepoPatch(ctx, p, observer)
				if err != nil {
					return fmt.Errorf("failed to download patch for repair: %w", err)
				}

				reader, err := zip.OpenReader(localPath)
				if err != nil {
					continue
				}

				for _, zf := range reader.File {
					var rel string
					if name, ok := strings.CutPrefix(zf.Name, "overrides/"); ok {
						rel = name
					} else if _, ok := strings.CutPrefix(zf.Name, "patches/"); ok {
						// Binary patches are tricky because they depend on the base.
						// If an override is corrupted, and it was last modified by a patch,
						// we might need to re-apply the patch chain.
						// For now, let's focus on direct overrides.
						continue
					} else {
						continue
					}

					isCorrupted := slices.Contains(corruptedOverrides, rel)

					if isCorrupted {
						targetPath := filepath.Join(inst.Path, rel)
						_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
						rc, _ := zf.Open()
						out, _ := os.Create(targetPath)
						io.Copy(out, rc)
						rc.Close()
						out.Close()
						slog.Info("Repaired override from patch", "path", rel, "patch", p.ID)
					}
				}
				reader.Close()
			}
		}
	}

	observer.OnProgress("Repair complete", 100, "Done", "main")
	return nil
}
