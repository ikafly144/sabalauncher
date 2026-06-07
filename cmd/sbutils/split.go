package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	basePack := args[0]
	largePatch := args[1]
	outPrefix := args[2]
	maxSizeMB, _ := strconv.ParseInt(args[3], 10, 64)
	maxSizeBytes := maxSizeMB * 1024 * 1024

	tempDir, err := os.MkdirTemp("", "sbutils-split-*")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	baseDir := filepath.Join(tempDir, "base")
	patchDir := filepath.Join(tempDir, "patch")

	fmt.Println("Extracting base pack...")
	if err := extractZip(basePack, baseDir); err != nil {
		fmt.Printf("Failed to extract base pack: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Extracting large patch...")
	if err := extractZip(largePatch, patchDir); err != nil {
		fmt.Printf("Failed to extract large patch: %v\n", err)
		os.Exit(1)
	}

	// Read base index
	var baseIndex resource.SBIndex
	baseIndexBytes, err := os.ReadFile(filepath.Join(baseDir, "sb.index.json"))
	if err != nil {
		fmt.Printf("Failed to read base index: %v\n", err)
		os.Exit(1)
	}
	if err := json.Unmarshal(baseIndexBytes, &baseIndex); err != nil {
		fmt.Printf("Failed to parse base index: %v\n", err)
		os.Exit(1)
	}

	// Read large patch metadata
	var largeP resource.SBPatch
	largePBytes, err := os.ReadFile(filepath.Join(patchDir, "sb.patch.json"))
	if err != nil {
		fmt.Printf("Failed to read patch metadata: %v\n", err)
		os.Exit(1)
	}
	if err := json.Unmarshal(largePBytes, &largeP); err != nil {
		fmt.Printf("Failed to parse patch metadata: %v\n", err)
		os.Exit(1)
	}

	if baseIndex.ID != largeP.BaseID {
		fmt.Printf("Version mismatch: base is %s, patch expects %s\n", baseIndex.ID, largeP.BaseID)
		os.Exit(1)
	}

	// Collect all files to be included in the patch
	type patchFile struct {
		relPath string // Relative to instance root (e.g., "overrides/config/test.txt")
		isPatch bool   // True if it's in patches/ (binary diff)
		size    int64
	}
	allFiles := []patchFile{}

	// 1. Overrides
	patchOverridesDir := filepath.Join(patchDir, "overrides")
	if _, err := os.Stat(patchOverridesDir); err == nil {
		_ = filepath.WalkDir(patchOverridesDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(patchOverridesDir, path)
			info, _ := d.Info()
			allFiles = append(allFiles, patchFile{
				relPath: filepath.ToSlash(filepath.Join("overrides", rel)),
				isPatch: false,
				size:    info.Size(),
			})
			return nil
		})
	}

	// 2. Binary Patches
	patchPatchesDir := filepath.Join(patchDir, "patches")
	if _, err := os.Stat(patchPatchesDir); err == nil {
		_ = filepath.WalkDir(patchPatchesDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(patchPatchesDir, path)
			info, _ := d.Info()
			allFiles = append(allFiles, patchFile{
				relPath: filepath.ToSlash(filepath.Join("patches", rel)),
				isPatch: true,
				size:    info.Size(),
			})
			return nil
		})
	}

	// 3. Removed Files (treat as small size)
	for _, rf := range largeP.RemovedFiles {
		allFiles = append(allFiles, patchFile{
			relPath: "REMOVE:" + rf,
			size:    100, // Minimal overhead
		})
	}

	// Split into chunks
	var chunks [][]patchFile
	var currentChunk []patchFile
	var currentSize int64

	for _, f := range allFiles {
		if currentSize > 0 && currentSize+f.size > maxSizeBytes {
			chunks = append(chunks, currentChunk)
			currentChunk = nil
			currentSize = 0
		}
		currentChunk = append(currentChunk, f)
		currentSize += f.size
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
		finalFilesMap[f.Path] = f
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
		chunkP.Index = currentIndex // Start with current
		chunkP.Index.ID = nextID
		chunkP.RemovedFiles = []string{}

		// Update index files and collect items for ZIP
		newFiles := make(map[string]resource.SBFile)
		for _, f := range currentIndex.Files {
			newFiles[f.Path] = f
		}

		// Prepare ZIP
		outFile, _ := os.Create(outName)
		w := zip.NewWriter(outFile)

		for _, f := range chunk {
			if strings.HasPrefix(f.relPath, "REMOVE:") {
				path := strings.TrimPrefix(f.relPath, "REMOVE:")
				chunkP.RemovedFiles = append(chunkP.RemovedFiles, path)
				delete(newFiles, path)
			} else {
				srcPath := filepath.Join(patchDir, filepath.FromSlash(f.relPath))
				_ = addFileToZip(w, srcPath, f.relPath)

				// Update index file entry from final state
				// We need to strip "overrides/" or "patches/" to get instance path
				instPath := f.relPath
				if strings.HasPrefix(instPath, "overrides/") {
					instPath = strings.TrimPrefix(instPath, "overrides/")
				} else if strings.HasPrefix(instPath, "patches/") {
					instPath = strings.TrimPrefix(instPath, "patches/")
				}

				if finalF, ok := finalFilesMap[instPath]; ok {
					newFiles[instPath] = finalF
				}
			}
		}

		// Rebuild index files slice
		chunkP.Index.Files = make([]resource.SBFile, 0, len(newFiles))
		for _, f := range newFiles {
			chunkP.Index.Files = append(chunkP.Index.Files, f)
		}

		// Add patch JSON to ZIP
		pb, _ := json.MarshalIndent(chunkP, "", "  ")
		header := &zip.FileHeader{Name: "sb.patch.json", Method: zip.Deflate}
		pw, _ := w.CreateHeader(header)
		_, _ = pw.Write(pb)

		w.Close()
		outFile.Close()

		// Prepare for next chunk
		currentBaseID = nextID
		currentIndex = chunkP.Index
	}

	fmt.Println("Successfully split patch.")
}
