package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
	"github.com/kr/binarydist"
)

func runPatch(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: sbutils patch <base.sbpack> <patch.sbpatch> <output.sbpack>")
		os.Exit(1)
	}

	basePackPath := args[0]
	patchPackPath := args[1]
	outPackPath := args[2]

	baseZip, err := zip.OpenReader(basePackPath)
	if err != nil {
		fmt.Printf("Failed to open base pack: %v\n", err)
		os.Exit(1)
	}
	defer baseZip.Close()

	patchZip, err := zip.OpenReader(patchPackPath)
	if err != nil {
		fmt.Printf("Failed to open patch pack: %v\n", err)
		os.Exit(1)
	}
	defer patchZip.Close()

	baseFiles := mapZipFiles(&baseZip.Reader)
	patchFiles := mapZipFiles(&patchZip.Reader)

	var patch resource.SBPatch
	if f, ok := patchFiles["sb.patch.json"]; ok {
		rc, _ := f.Open()
		if err := json.NewDecoder(rc).Decode(&patch); err != nil {
			fmt.Printf("Failed to decode patch JSON: %v\n", err)
			os.Exit(1)
		}
		rc.Close()
	} else {
		fmt.Println("Error: patch missing sb.patch.json")
		os.Exit(1)
	}

	// Verify base ID
	var baseIndex resource.SBPackIndex
	if f, ok := baseFiles["sb.index.json"]; ok {
		rc, _ := f.Open()
		if err := json.NewDecoder(rc).Decode(&baseIndex); err != nil {
			fmt.Printf("Failed to decode base index: %v\n", err)
			os.Exit(1)
		}
		rc.Close()
	}

	if baseIndex.ID != patch.BaseID {
		fmt.Printf("Version mismatch: base is %s, patch expects %s\n", baseIndex.ID, patch.BaseID)
		os.Exit(1)
	}

	outFile, err := os.Create(outPackPath)
	if err != nil {
		fmt.Printf("Failed to create output file: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	defer w.Close()

	removedSet := make(map[string]struct{})
	for _, f := range patch.RemovedFiles {
		removedSet[f] = struct{}{}
	}

	patchedSet := make(map[string]struct{})
	for name := range patchFiles {
		if after, ok := strings.CutPrefix(name, "patches/"); ok {
			rel := after
			patchedSet["overrides/"+rel] = struct{}{}
		}
	}

	addedSet := make(map[string]struct{})
	for name := range patchFiles {
		if strings.HasPrefix(name, "overrides/") {
			addedSet[name] = struct{}{}
		}
	}

	// 1. Copy unchanged files from base
	for name, f := range baseFiles {
		if name == "sb.index.json" {
			continue
		}
		if _, removed := removedSet[name]; removed {
			continue
		}
		if _, patched := patchedSet[name]; patched {
			continue
		}
		if _, added := addedSet[name]; added {
			// If it's in addedSet, it means the patch overwrites it directly
			continue
		}

		if err := copyZipFile(w, f); err != nil {
			fmt.Printf("Failed to copy %s: %v\n", name, err)
		}
	}

	// 2. Apply patches
	for name, f := range patchFiles {
		if !strings.HasPrefix(name, "patches/") || f.FileInfo().IsDir() {
			continue
		}
		rel := strings.TrimPrefix(name, "patches/")
		baseF, ok := baseFiles["overrides/"+rel]
		if !ok {
			fmt.Printf("Warning: base file missing for patch %s\n", rel)
			continue
		}

		baseRc, err := baseF.Open()
		if err != nil {
			fmt.Printf("Failed to open base file %s: %v\n", rel, err)
			continue
		}
		patchRc, err := f.Open()
		if err != nil {
			baseRc.Close()
			fmt.Printf("Failed to open patch file %s: %v\n", name, err)
			continue
		}

		// Use a temp file to apply patch safely
		tempFile, err := os.CreateTemp("", "sbpatch-*")
		if err != nil {
			baseRc.Close()
			patchRc.Close()
			fmt.Printf("Failed to create temp file for patch %s: %v\n", rel, err)
			continue
		}

		if err := binarydist.Patch(baseRc, tempFile, patchRc); err != nil {
			fmt.Printf("Failed to apply patch to %s: %v\n", rel, err)
			baseRc.Close()
			patchRc.Close()
			tempFile.Close()
			_ = os.Remove(tempFile.Name())
			continue
		}
		baseRc.Close()
		patchRc.Close()

		// Read patched data from temp file
		if _, err := tempFile.Seek(0, 0); err != nil {
			fmt.Printf("Failed to seek temp file for %s: %v\n", rel, err)
			tempFile.Close()
			_ = os.Remove(tempFile.Name())
			continue
		}

		patchedData, err := io.ReadAll(tempFile)
		tempFile.Close()
		_ = os.Remove(tempFile.Name())
		if err != nil {
			fmt.Printf("Failed to read patched data for %s: %v\n", rel, err)
			continue
		}

		if err := addDataToZip(w, patchedData, "overrides/"+rel); err != nil {
			fmt.Printf("Failed to add patched file %s to zip: %v\n", rel, err)
		}
	}

	// 3. Add added/overwritten files from patch
	for name, f := range patchFiles {
		if !strings.HasPrefix(name, "overrides/") || f.FileInfo().IsDir() {
			continue
		}
		if err := copyZipFile(w, f); err != nil {
			fmt.Printf("Failed to add %s: %v\n", name, err)
		}
	}

	// 4. Add new sb.index.json
	newIndexBytes, _ := json.MarshalIndent(patch.Index, "", "  ")
	if err := addDataToZip(w, newIndexBytes, "sb.index.json"); err != nil {
		fmt.Printf("Failed to add new index: %v\n", err)
	}

	fmt.Printf("Successfully patched to %s\n", outPackPath)
}
