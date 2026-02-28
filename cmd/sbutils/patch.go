package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ikafly144/sabalauncher/pkg/resource"
)

func runPatch(args []string) {
	if len(args) < 3 {
		fmt.Println("Usage: sbutils patch <base.sbpack> <patch.sbpatch> <output.sbpack>")
		os.Exit(1)
	}

	basePack := args[0]
	patchPack := args[1]
	outPack := args[2]

	tempDir, err := os.MkdirTemp("", "sbutils-patch-*")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	baseDir := filepath.Join(tempDir, "base")
	patchDir := filepath.Join(tempDir, "patch")

	if err := extractZip(basePack, baseDir); err != nil {
		fmt.Printf("Failed to extract base pack: %v\n", err)
		os.Exit(1)
	}
	if err := extractZip(patchPack, patchDir); err != nil {
		fmt.Printf("Failed to extract patch pack: %v\n", err)
		os.Exit(1)
	}

	var baseIndex resource.SBIndex
	baseBytes, _ := os.ReadFile(filepath.Join(baseDir, "sb.index.json"))
	json.Unmarshal(baseBytes, &baseIndex)

	var patch resource.SBPatch
	patchBytes, _ := os.ReadFile(filepath.Join(patchDir, "sb.patch.json"))
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		fmt.Printf("Failed to parse sb.patch.json: %v\n", err)
		os.Exit(1)
	}

	if baseIndex.Version != patch.FromVersion {
		fmt.Printf("Version mismatch: base is %s, patch expects %s\n", baseIndex.Version, patch.FromVersion)
		os.Exit(1)
	}

	// Remove files
	for _, f := range patch.RemovedFiles {
		os.Remove(filepath.Join(baseDir, filepath.FromSlash(f)))
	}

	// Copy overrides from patch to base
	patchOverrides := filepath.Join(patchDir, "overrides")
	if _, err := os.Stat(patchOverrides); err == nil {
		filepath.Walk(patchOverrides, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				rel, _ := filepath.Rel(patchOverrides, path)
				destPath := filepath.Join(baseDir, "overrides", rel)
				os.MkdirAll(filepath.Dir(destPath), 0755)
				copyFile(path, destPath)
			}
			return nil
		})
	}

	// Write new index
	newIndexBytes, _ := json.MarshalIndent(patch.NewIndex, "", "  ")
	os.WriteFile(filepath.Join(baseDir, "sb.index.json"), newIndexBytes, 0644)

	// Create output pack
	outFile, err := os.Create(outPack)
	if err != nil {
		fmt.Printf("Failed to create output file: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	defer w.Close()

	// Add new sb.index.json
	addFileToZip(w, filepath.Join(baseDir, "sb.index.json"), "sb.index.json")

	// Add updated overrides
	baseOverrides := filepath.Join(baseDir, "overrides")
	if _, err := os.Stat(baseOverrides); err == nil {
		filepath.Walk(baseOverrides, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				rel, _ := filepath.Rel(baseDir, path)
				rel = filepath.ToSlash(rel)
				addFileToZip(w, path, rel)
			}
			return nil
		})
	}

	fmt.Printf("Successfully patched to %s\n", outPack)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
