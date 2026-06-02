package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
	"github.com/kr/binarydist"
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
	baseBytes, err := os.ReadFile(filepath.Join(baseDir, "sb.index.json"))
	if err != nil {
		fmt.Printf("Failed to read base index: %v\n", err)
		os.Exit(1)
	}
	if err := json.Unmarshal(baseBytes, &baseIndex); err != nil {
		fmt.Printf("Failed to parse base index: %v\n", err)
		os.Exit(1)
	}

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
		_ = os.Remove(filepath.Join(baseDir, filepath.FromSlash(f)))
	}

	// Copy overrides from patch to base
	patchOverrides := filepath.Join(patchDir, "overrides")
	if _, err := os.Stat(patchOverrides); err == nil {
		if err := filepath.Walk(patchOverrides, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				rel, _ := filepath.Rel(patchOverrides, path)
				destPath := filepath.Join(baseDir, "overrides", rel)
				if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
					return err
				}
				return copyFile(path, destPath)
			}
			return nil
		}); err != nil {
			fmt.Printf("Failed to copy overrides: %v\n", err)
			os.Exit(1)
		}
	}

	// Apply patches from patch to base
	if patch.FormatVersion >= resource.SBPatchFormatVersion {
		patchPatches := filepath.Join(patchDir, "patches")
		if _, err := os.Stat(patchPatches); err == nil {
			if err := filepath.Walk(patchPatches, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					rel, _ := filepath.Rel(patchPatches, path)
					targetPath := filepath.Join(baseDir, "overrides", rel)

					if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
						return err
					}

					oldFile, err := os.Open(targetPath)
					if err != nil {
						fmt.Printf("Warning: failed to open base file %s for patching: %v\n", rel, err)
						return nil
					}
					patchFile, err := os.Open(path)
					if err != nil {
						oldFile.Close()
						fmt.Printf("Warning: failed to open patch file %s: %v\n", rel, err)
						return nil
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
						fmt.Printf("Warning: failed to apply patch to %s: %v\n", rel, err)
						return nil
					}

					oldFile.Close()
					patchFile.Close()
					tempFile.Close()

					_ = os.Remove(targetPath)
					if err := os.Rename(tempFile.Name(), targetPath); err != nil {
						fmt.Printf("Warning: failed to rename patched file %s: %v\n", rel, err)
					}
				}
				return nil
			}); err != nil {
				fmt.Printf("Failed to apply patches: %v\n", err)
				os.Exit(1)
			}
		}
	}

	// Write new index
	newIndexBytes, _ := json.MarshalIndent(patch.NewIndex, "", "  ")
	if err := os.WriteFile(filepath.Join(baseDir, "sb.index.json"), newIndexBytes, 0644); err != nil {
		fmt.Printf("Failed to write new index: %v\n", err)
		os.Exit(1)
	}

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
	if err := addFileToZip(w, filepath.Join(baseDir, "sb.index.json"), "sb.index.json"); err != nil {
		fmt.Printf("Failed to add index to pack: %v\n", err)
	}

	// Add updated overrides
	baseOverrides := filepath.Join(baseDir, "overrides")
	if _, err := os.Stat(baseOverrides); err == nil {
		if err := filepath.Walk(baseOverrides, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				rel, _ := filepath.Rel(baseDir, path)
				rel = filepath.ToSlash(rel)
				return addFileToZip(w, path, rel)
			}
			return nil
		}); err != nil {
			fmt.Printf("Failed to walk base overrides: %v\n", err)
		}
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
