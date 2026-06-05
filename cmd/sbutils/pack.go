package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
)

func runPack(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: sbutils pack <dir> <output.sbpack>")
		os.Exit(1)
	}

	dir := args[0]
	outPath := args[1]

	// Verify dir has sb.index.json
	indexPath := filepath.Join(dir, "sb.index.json")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		fmt.Printf("Error: %s does not contain sb.index.json\n", dir)
		os.Exit(1)
	}

	// Read index and update ID
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		fmt.Printf("Failed to read index: %v\n", err)
		os.Exit(1)
	}

	var index resource.SBIndex
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		fmt.Printf("Failed to parse index: %v\n", err)
		os.Exit(1)
	}

	newID, err := uuid.NewV7()
	if err != nil {
		fmt.Printf("Failed to generate new ID: %v\n", err)
		os.Exit(1)
	}
	index.ID = newID

	// Write updated index back to source
	updatedIndexBytes, _ := json.MarshalIndent(index, "", "  ")
	if err := os.WriteFile(indexPath, updatedIndexBytes, 0644); err != nil {
		fmt.Printf("Failed to update index file: %v\n", err)
		os.Exit(1)
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		fmt.Printf("Failed to create output file: %v\n", err)
		os.Exit(1)
	}
	defer outFile.Close()

	w := zip.NewWriter(outFile)
	defer w.Close()

	// Add sb.index.json (re-read from updated file)
	if err := addFileToZip(w, indexPath, "sb.index.json"); err != nil {
		fmt.Printf("Failed to add sb.index.json to zip: %v\n", err)
		os.Exit(1)
	}

	// Add overrides
	overridesDir := filepath.Join(dir, "overrides")
	if _, err := os.Stat(overridesDir); err == nil {
		err = filepath.Walk(overridesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			relPath = filepath.ToSlash(relPath)

			return addFileToZip(w, path, relPath)
		})
		if err != nil {
			fmt.Printf("Failed to add overrides to zip: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("Successfully packed (ID: %s) to %s\n", newID, outPath)
}

func addFileToZip(w *zip.Writer, srcPath string, zipPath string) error {
	fileToZip, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer fileToZip.Close()

	info, err := fileToZip.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	header.Name = zipPath
	header.Method = zip.Deflate

	writer, err := w.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(writer, fileToZip)
	return err
}
