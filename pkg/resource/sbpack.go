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

	return inst, nil
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

	if inst.Upstream == nil || inst.Upstream.Version != patch.FromVersion {
		return fmt.Errorf("version mismatch: instance is at %s, patch requires %s",
			func() string {
				if inst.Upstream != nil {
					return inst.Upstream.Version
				} else {
					return "unknown"
				}
			}(),
			patch.FromVersion)
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
	inst.Upstream.Version = patch.ToVersion
	inst.Versions = make([]InstanceVersion, 0, len(patch.NewIndex.Dependencies))
	for id, ver := range patch.NewIndex.Dependencies {
		inst.Versions = append(inst.Versions, InstanceVersion{
			ID:      id,
			Version: ver,
		})
	}
	inst.Mods = newMods

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
