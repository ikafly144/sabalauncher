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
	"strings"

	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
	"github.com/kr/binarydist"
)

func runDiff(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: sbutils diff <old.sbpack> <new.sbpack> <output.sbpatch>")
		os.Exit(1)
	}

	oldPackPath := args[0]
	newPackPath := args[1]
	outPatch := args[2]

	oldZip, err := zip.OpenReader(oldPackPath)
	if err != nil {
		fmt.Printf("Failed to open old pack: %v\n", err)
		os.Exit(1)
	}
	defer oldZip.Close()

	newZip, err := zip.OpenReader(newPackPath)
	if err != nil {
		fmt.Printf("Failed to open new pack: %v\n", err)
		os.Exit(1)
	}
	defer newZip.Close()

	oldFiles := mapZipFiles(&oldZip.Reader)
	newFiles := mapZipFiles(&newZip.Reader)

	var oldIndex, newIndex resource.SBPackIndex
	if f, ok := oldFiles["sb.index.json"]; ok {
		rc, _ := f.Open()
		json.NewDecoder(rc).Decode(&oldIndex)
		rc.Close()
	} else {
		fmt.Println("Error: old pack missing sb.index.json")
		os.Exit(1)
	}

	if f, ok := newFiles["sb.index.json"]; ok {
		rc, _ := f.Open()
		json.NewDecoder(rc).Decode(&newIndex)
		rc.Close()
	} else {
		fmt.Println("Error: new pack missing sb.index.json")
		os.Exit(1)
	}

	removedFiles := []string{}
	// Check for removed external mods
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

	addedOverrides := []string{}
	patchedOverrides := []string{}

	for name, newF := range newFiles {
		if !strings.HasPrefix(name, "overrides/") || newF.FileInfo().IsDir() {
			continue
		}
		rel := strings.TrimPrefix(name, "overrides/")

		if oldF, ok := oldFiles[name]; !ok {
			addedOverrides = append(addedOverrides, rel)
		} else {
			// Fast path: check CRC32 and Size
			if oldF.CRC32 != newF.CRC32 || oldF.UncompressedSize64 != newF.UncompressedSize64 {
				patchedOverrides = append(patchedOverrides, rel)
			}
		}
	}

	// Any in oldFiles["overrides/"] NOT in newFiles are removed
	for name := range oldFiles {
		if !strings.HasPrefix(name, "overrides/") || oldFiles[name].FileInfo().IsDir() {
			continue
		}
		if _, ok := newFiles[name]; !ok {
			removedFiles = append(removedFiles, name)
		}
	}

	// Create patch JSON
	// Collect hashes for the new index
	newHashes := make(map[string]string)
	for name, f := range newFiles {
		if !strings.HasPrefix(name, "overrides/") || f.FileInfo().IsDir() {
			continue
		}
		rel := strings.TrimPrefix(name, "overrides/")
		// We could optimize this by not reading everything, but for small files it's fine.
		// Actually, we already have the hash if we extracted it, but we are avoiding extraction.
		// Let's just calculate it for now.
		rc, _ := f.Open()
		h := sha256.New()
		io.Copy(h, rc)
		rc.Close()
		newHashes[rel] = hex.EncodeToString(h.Sum(nil))
	}
	newIndex.Hashes = newHashes

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
	if err := addDataToZip(w, patchBytes, "sb.patch.json"); err != nil {
		fmt.Printf("Failed to add sb.patch.json: %v\n", err)
		os.Exit(1)
	}

	// Add added overrides
	for _, rel := range addedOverrides {
		f := newFiles["overrides/"+rel]
		rc, err := f.Open()
		if err != nil {
			fmt.Printf("Failed to open added override %s: %v\n", rel, err)
			continue
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		if err := addDataToZip(w, data, "overrides/"+rel); err != nil {
			fmt.Printf("Failed to add added override %s: %v\n", rel, err)
		}
	}

	// Add patched overrides (binary diff)
	for _, rel := range patchedOverrides {
		oldF := oldFiles["overrides/"+rel]
		newF := newFiles["overrides/"+rel]

		oldRc, _ := oldF.Open()
		newRc, _ := newF.Open()

		var buf bytes.Buffer
		if err := binarydist.Diff(oldRc, newRc, &buf); err != nil {
			fmt.Printf("Failed to diff %s: %v\n", rel, err)
			oldRc.Close()
			newRc.Close()
			continue
		}
		oldRc.Close()
		newRc.Close()

		if err := addDataToZip(w, buf.Bytes(), "patches/"+rel); err != nil {
			fmt.Printf("Failed to add patch %s: %v\n", rel, err)
		}
	}

	fmt.Printf("Successfully created patch %s\n", outPatch)
}
