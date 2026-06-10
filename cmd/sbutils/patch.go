package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
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

	baseFiles := mapZipFiles(baseZip.Reader)
	patchFiles := mapZipFiles(patchZip.Reader)

	var patch resource.SBPatch
	if f, ok := patchFiles["sb.patch.json"]; ok {
		rc, _ := f.Open()
		json.NewDecoder(rc).Decode(&patch)
		rc.Close()
	} else {
		fmt.Println("Error: patch missing sb.patch.json")
		os.Exit(1)
	}

	// Verify base ID
	var baseIndex resource.SBPackIndex
	if f, ok := baseFiles["sb.index.json"]; ok {
		rc, _ := f.Open()
		json.NewDecoder(rc).Decode(&baseIndex)
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
		if strings.HasPrefix(name, "patches/") {
			rel := strings.TrimPrefix(name, "patches/")
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

		baseRc, _ := baseF.Open()
		patchRc, _ := f.Open()

		// Use a pipe or buffer to apply patch
		// For safety with binarydist, let's use a buffer or temp file if needed.
		// binarydist.Patch(old, new, patch)
		var outBuf bytes.Buffer
		if err := binarydist.Patch(baseRc, &outBuf, patchRc); err != nil {
			fmt.Printf("Failed to apply patch to %s: %v\n", rel, err)
			baseRc.Close()
			patchRc.Close()
			continue
		}
		baseRc.Close()
		patchRc.Close()

		if err := addDataToZip(w, outBuf.Bytes(), "overrides/"+rel); err != nil {
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
