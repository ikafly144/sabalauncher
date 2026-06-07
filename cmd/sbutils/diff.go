package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
	"github.com/kr/binarydist"
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
	oldBytes, err := os.ReadFile(filepath.Join(oldDir, "sb.index.json"))
	if err != nil {
		fmt.Printf("Failed to read old index: %v\n", err)
		os.Exit(1)
	}
	newBytes, err := os.ReadFile(filepath.Join(newDir, "sb.index.json"))
	if err != nil {
		fmt.Printf("Failed to read new index: %v\n", err)
		os.Exit(1)
	}
	if err := json.Unmarshal(oldBytes, &oldIndex); err != nil {
		fmt.Printf("Failed to parse old index: %v\n", err)
		os.Exit(1)
	}
	if err := json.Unmarshal(newBytes, &newIndex); err != nil {
		fmt.Printf("Failed to parse new index: %v\n", err)
		os.Exit(1)
	}

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
	oldOverridesDir := filepath.Join(oldDir, "overrides")
	newOverridesDir := filepath.Join(newDir, "overrides")

	oldOverrides := parallelHashOverrides(oldOverridesDir)
	newOverrides := parallelHashOverrides(newOverridesDir)

	addedOverrides := []string{}
	patchedOverrides := []string{}

	for rel, hash := range newOverrides {
		if oldHash, ok := oldOverrides[rel]; !ok {
			addedOverrides = append(addedOverrides, rel)
		} else if oldHash != hash {
			patchedOverrides = append(patchedOverrides, rel)
		}
		delete(oldOverrides, rel)
	}

	// Any remaining in oldOverrides are removed
	for rel := range oldOverrides {
		removedFiles = append(removedFiles, filepath.ToSlash(rel))
	}
// ... (rest of the function continues as before)

	// Create patch JSON
	patch := resource.SBPatch{
		FormatVersion: resource.SBPatchFormatVersion,
		BaseID:        oldIndex.ID,
		Index:         newIndex,
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
	writer, err := w.CreateHeader(header)
	if err != nil {
		fmt.Printf("Failed to create sb.patch.json header: %v\n", err)
		os.Exit(1)
	}
	if _, err := writer.Write(patchBytes); err != nil {
		fmt.Printf("Failed to write sb.patch.json: %v\n", err)
		os.Exit(1)
	}

	// Add added overrides
	for _, rel := range addedOverrides {
		srcPath := filepath.Join(newOverridesDir, filepath.FromSlash(rel))
		zipPath := filepath.ToSlash(filepath.Join("overrides", rel))
		if err := addFileToZip(w, srcPath, zipPath); err != nil {
			fmt.Printf("Failed to add override %s: %v\n", rel, err)
		}
	}

	// Add patched overrides (parallelize binary diffing)
	type patchResult struct {
		rel  string
		data []byte
		err  error
	}
	patchResults := make(chan patchResult, len(patchedOverrides))
	var patchWg sync.WaitGroup

	for _, rel := range patchedOverrides {
		patchWg.Add(1)
		go func(rel string) {
			defer patchWg.Done()
			oldFilePath := filepath.Join(oldOverridesDir, filepath.FromSlash(rel))
			newFilePath := filepath.Join(newOverridesDir, filepath.FromSlash(rel))

			oldFile, err := os.Open(oldFilePath)
			if err != nil {
				patchResults <- patchResult{rel, nil, err}
				return
			}
			defer oldFile.Close()

			newFile, err := os.Open(newFilePath)
			if err != nil {
				patchResults <- patchResult{rel, nil, err}
				return
			}
			defer newFile.Close()

			var buf bytes.Buffer
			if err := binarydist.Diff(oldFile, newFile, &buf); err != nil {
				patchResults <- patchResult{rel, nil, err}
				return
			}
			patchResults <- patchResult{rel, buf.Bytes(), nil}
		}(rel)
	}

	go func() {
		patchWg.Wait()
		close(patchResults)
	}()

	for res := range patchResults {
		if res.err != nil {
			fmt.Printf("Failed to create binary patch for %s: %v\n", res.rel, res.err)
			continue
		}
		zipPath := filepath.ToSlash(filepath.Join("patches", res.rel))
		if err := addDataToZip(w, res.data, zipPath); err != nil {
			fmt.Printf("Failed to add patch %s to zip: %v\n", res.rel, err)
		}
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
			if err := os.MkdirAll(fpath, os.ModePerm); err != nil {
				return err
			}
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
