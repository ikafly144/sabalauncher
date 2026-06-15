package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
)

func runSplit(args []string) {
	if len(args) < 4 {
		fmt.Println("Usage: sbutils split <base.sbpack> <large.sbpatch> <output_prefix> <max_size_mb>")
		os.Exit(1)
	}

	basePackPath := args[0]
	largePatchPath := args[1]
	outPrefix := args[2]
	maxSizeMB, _ := strconv.ParseInt(args[3], 10, 64)
	maxSizeBytes := maxSizeMB * 1024 * 1024

	baseZip, err := zip.OpenReader(basePackPath)
	if err != nil {
		fmt.Printf("Failed to open base pack: %v\n", err)
		os.Exit(1)
	}
	defer baseZip.Close()

	largePatchZip, err := zip.OpenReader(largePatchPath)
	if err != nil {
		fmt.Printf("Failed to open large patch: %v\n", err)
		os.Exit(1)
	}
	defer largePatchZip.Close()

	baseFiles := mapZipFiles(&baseZip.Reader)
	patchFiles := mapZipFiles(&largePatchZip.Reader)

	// Read base index
	var baseIndex resource.SBPackIndex
	if f, ok := baseFiles["sb.index.json"]; ok {
		rc, _ := f.Open()
		if err := json.NewDecoder(rc).Decode(&baseIndex); err != nil {
			fmt.Printf("Failed to decode base index: %v\n", err)
			os.Exit(1)
		}
		rc.Close()
	}

	// Read large patch metadata
	var largeP resource.SBPatch
	if f, ok := patchFiles["sb.patch.json"]; ok {
		rc, _ := f.Open()
		if err := json.NewDecoder(rc).Decode(&largeP); err != nil {
			fmt.Printf("Failed to decode large patch: %v\n", err)
			os.Exit(1)
		}
		rc.Close()
	}

	if baseIndex.ID != largeP.BaseID {
		fmt.Printf("Version mismatch: base is %s, patch expects %s\n", baseIndex.ID, largeP.BaseID)
		os.Exit(1)
	}

	// Collect all files to be included in the patch from the ZIP
	type patchFileEntry struct {
		name    string
		isPatch bool
		size    int64
	}
	allEntries := []patchFileEntry{}

	// 1. Removed Files (from metadata)
	for _, rf := range largeP.RemovedFiles {
		allEntries = append(allEntries, patchFileEntry{
			name: "REMOVE:" + rf,
			size: 100,
		})
	}

	// 2. Overrides and Patches (from ZIP)
	for name, f := range patchFiles {
		if strings.HasPrefix(name, "overrides/") || strings.HasPrefix(name, "patches/") {
			if f.FileInfo().IsDir() {
				continue
			}
			allEntries = append(allEntries, patchFileEntry{
				name:    name,
				isPatch: strings.HasPrefix(name, "patches/"),
				size:    int64(f.UncompressedSize64),
			})
		}
	}

	// Split into chunks
	var chunks [][]patchFileEntry
	var currentChunk []patchFileEntry
	var currentSize int64

	for _, e := range allEntries {
		if currentSize > 0 && currentSize+e.size > maxSizeBytes {
			chunks = append(chunks, currentChunk)
			currentChunk = nil
			currentSize = 0
		}
		currentChunk = append(currentChunk, e)
		currentSize += e.size
	}
	if len(currentChunk) > 0 {
		chunks = append(chunks, currentChunk)
	}

	fmt.Printf("Splitting into %d patches...\n", len(chunks))

	currentBaseID := largeP.BaseID
	finalTargetID := largeP.Index.ID
	currentIndex := baseIndex

	// Map of final index files for easy lookup
	finalFilesMap := make(map[string]resource.SBFile)
	for _, f := range largeP.Index.Files {
		finalFilesMap[filepath.ToSlash(f.Path)] = f
	}

	for i, chunk := range chunks {
		isLast := (i == len(chunks)-1)
		var nextID uuid.UUID
		if isLast {
			nextID = finalTargetID
		} else {
			nextID, _ = uuid.NewV7()
		}

		outName := fmt.Sprintf("%s_%d.sbpatch", outPrefix, i+1)
		fmt.Printf("Creating %s...\n", outName)

		// Create patch data for this chunk
		var chunkP resource.SBPatch
		chunkP.FormatVersion = resource.SBPatchFormatVersion
		chunkP.BaseID = currentBaseID
		chunkP.Index = currentIndex
		chunkP.Index.ID = nextID
		chunkP.RemovedFiles = []string{}

		// If it's the last chunk, ensure all other metadata fields are updated to final state
		if isLast {
			chunkP.Index.Name = largeP.Index.Name
			chunkP.Index.FormatVersion = largeP.Index.FormatVersion
			chunkP.Index.Properties = largeP.Index.Properties
			chunkP.Index.Dependencies = largeP.Index.Dependencies
			chunkP.Index.Hashes = largeP.Index.Hashes
		}

		// Update index files and collect items for ZIP
		newFiles := make(map[string]resource.SBFile)
		for _, f := range currentIndex.Files {
			newFiles[filepath.ToSlash(f.Path)] = f
		}

		// Prepare ZIP
		outFile, _ := os.Create(outName)
		w := zip.NewWriter(outFile)

		for _, e := range chunk {
			if after, ok := strings.CutPrefix(e.name, "REMOVE:"); ok {
				path := filepath.ToSlash(after)
				chunkP.RemovedFiles = append(chunkP.RemovedFiles, path)
				delete(newFiles, path)
			} else {
				f := patchFiles[e.name]
				_ = copyZipFile(w, f)

				// Update index file entry
				instPath := e.name
				if after, ok := strings.CutPrefix(instPath, "overrides/"); ok {
					instPath = after
				} else if after, ok := strings.CutPrefix(instPath, "patches/"); ok {
					instPath = after
				}
				instPath = filepath.ToSlash(instPath)

				if finalF, ok := finalFilesMap[instPath]; ok {
					newFiles[instPath] = finalF
				}
			}
		}

		// Rebuild index files slice
		chunkP.Index.Files = make([]resource.SBFile, 0, len(newFiles))
		// Sort keys for deterministic output
		paths := slices.Sorted(maps.Keys(newFiles))
		for _, p := range paths {
			chunkP.Index.Files = append(chunkP.Index.Files, newFiles[p])
		}

		// Add patch JSON to ZIP
		pb, _ := json.MarshalIndent(chunkP, "", "  ")
		_ = addDataToZip(w, pb, "sb.patch.json")

		w.Close()
		outFile.Close()

		// Prepare for next chunk
		currentBaseID = nextID
		currentIndex = chunkP.Index
	}

	fmt.Println("Successfully split patch.")
}
