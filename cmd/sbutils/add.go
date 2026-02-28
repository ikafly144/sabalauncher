package main

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/ikafly144/sabalauncher/pkg/resource"
)

func runAdd(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sbutils add <url>")
		os.Exit(1)
	}

	downloadURL := args[0]
	fmt.Printf("Fetching: %s\n", downloadURL)

	resp, err := http.Get(downloadURL)
	if err != nil {
		fmt.Printf("Failed to download: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Bad status code: %s\n", resp.Status)
		os.Exit(1)
	}

	// Determine filename
	filename := ""
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		_, params, err := mime.ParseMediaType(cd)
		if err == nil {
			filename = params["filename"]
		}
	}
	if filename == "" {
		u, err := url.Parse(downloadURL)
		if err == nil {
			filename = path.Base(u.Path)
		}
	}
	if filename == "" || filename == "/" || filename == "." {
		filename = "unknown.jar"
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "sbutils-dl-*")
	if err != nil {
		fmt.Printf("Failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Calculate hashes while downloading
	h1 := sha1.New()
	h256 := sha256.New()
	w := io.MultiWriter(tmpFile, h1, h256)

	size, err := io.Copy(w, resp.Body)
	if err != nil {
		fmt.Printf("Failed to save and hash file: %v\n", err)
		os.Exit(1)
	}

	sha1Hash := hex.EncodeToString(h1.Sum(nil))
	sha256Hash := hex.EncodeToString(h256.Sum(nil))

	fmt.Printf("Downloaded %s (%d bytes)\n", filename, size)
	fmt.Printf("SHA1:   %s\n", sha1Hash)
	fmt.Printf("SHA256: %s\n", sha256Hash)

	// Load sb.index.json
	indexPath := "sb.index.json"
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		fmt.Printf("Failed to read sb.index.json: %v\n", err)
		os.Exit(1)
	}

	var index resource.SBIndex
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		fmt.Printf("Failed to parse sb.index.json: %v\n", err)
		os.Exit(1)
	}

	// Check if already exists
	modPath := filepath.ToSlash(filepath.Join("mods", filename))
	exists := false
	for i, f := range index.Files {
		if f.Path == modPath {
			// Update existing
			index.Files[i].Hashes = map[string]string{
				"sha1":   sha1Hash,
				"sha256": sha256Hash,
			}
			index.Files[i].Downloads = []string{downloadURL}
			index.Files[i].FileSize = size
			exists = true
			fmt.Printf("Updated existing entry for %s\n", modPath)
			break
		}
	}

	if !exists {
		index.Files = append(index.Files, resource.SBFile{
			Path: modPath,
			Hashes: map[string]string{
				"sha1":   sha1Hash,
				"sha256": sha256Hash,
			},
			Downloads: []string{downloadURL},
			FileSize:  size,
		})
		fmt.Printf("Added new entry for %s\n", modPath)
	}

	// Save sb.index.json
	outBytes, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal updated index: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(indexPath, outBytes, 0644); err != nil {
		fmt.Printf("Failed to write sb.index.json: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully updated sb.index.json.")
}
