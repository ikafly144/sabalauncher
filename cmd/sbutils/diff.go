package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ikafly144/sabalauncher/pkg/resource"
)

func runDiff(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: sbutils diff <old.sbpack> <new.sbpack> <output.sbpatch>")
		os.Exit(1)
	}

	oldPack := args[0]
	newPack := args[1]
	outPatch := args[2]

	tempDir, err := os.MkdirTemp("", "sbutils-diff-*")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	oldDir := filepath.Join(tempDir, "old")
	newDir := filepath.Join(tempDir, "new")

	if err := extractZip(oldPack, oldDir); err != nil {
		fmt.Printf("Failed to extract old pack: %v\n", err)
		os.Exit(1)
	}
	if err := extractZip(newPack, newDir); err != nil {
		fmt.Printf("Failed to extract new pack: %v\n", err)
		os.Exit(1)
	}

	var oldIndex, newIndex resource.SBIndex
	oldBytes, _ := os.ReadFile(filepath.Join(oldDir, "sb.index.json"))
	newBytes, _ := os.ReadFile(filepath.Join(newDir, "sb.index.json"))
	json.Unmarshal(oldBytes, &oldIndex)
	json.Unmarshal(newBytes, &newIndex)

	removedFiles := []string{}

	// Check for removed mods
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

	// Check overrides diff
	changedOverrides := []string{} // paths relative to overrides/
	oldOverrides := make(map[string]string)

	oldOverridesDir := filepath.Join(oldDir, "overrides")
	if _, err := os.Stat(oldOverridesDir); err == nil {
		filepath.Walk(oldOverridesDir, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				rel, _ := filepath.Rel(oldOverridesDir, path)
				rel = filepath.ToSlash(rel)
				hash, _ := hashFile(path)
				oldOverrides[rel] = hash
			}
			return nil
		})
	}

	newOverridesDir := filepath.Join(newDir, "overrides")
	if _, err := os.Stat(newOverridesDir); err == nil {
		filepath.Walk(newOverridesDir, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				rel, _ := filepath.Rel(newOverridesDir, path)
				rel = filepath.ToSlash(rel)
				hash, _ := hashFile(path)

				if oldHash, ok := oldOverrides[rel]; !ok || oldHash != hash {
					changedOverrides = append(changedOverrides, rel)
				}
				delete(oldOverrides, rel)
			}
			return nil
		})
	}

	// Any remaining in oldOverrides are removed
	for rel := range oldOverrides {
		removedFiles = append(removedFiles, filepath.ToSlash(filepath.Join("overrides", rel)))
	}

	// Create patch JSON
	patch := resource.SBPatch{
		FormatVersion: 1,
		FromVersion:   oldIndex.Version,
		ToVersion:     newIndex.Version,
		NewIndex:      newIndex,
		RemovedFiles:  removedFiles,
	}

	// Create output zip
	outFile, err := os.Create(outPatch)
	if err != nil {
		fmt.Printf("Failed to create patch file: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	defer w.Close()

	// Add sb.patch.json
	patchBytes, _ := json.MarshalIndent(patch, "", "  ")
	header := &zip.FileHeader{Name: "sb.patch.json", Method: zip.Deflate}
	writer, _ := w.CreateHeader(header)
	writer.Write(patchBytes)

	// Add changed overrides
	for _, rel := range changedOverrides {
		srcPath := filepath.Join(newOverridesDir, filepath.FromSlash(rel))
		zipPath := filepath.ToSlash(filepath.Join("overrides", rel))
		addFileToZip(w, srcPath, zipPath)
	}

	fmt.Printf("Successfully created patch %s\n", outPatch)
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}
		outFile, err := os.Create(fpath)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
