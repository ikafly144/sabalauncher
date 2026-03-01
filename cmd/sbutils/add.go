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

	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
)

type downloadedFileMetadata struct {
	Filename string
	SHA1     string
	SHA256   string
	Size     int64
}

func fetchFileMetadata(downloadURL string) (downloadedFileMetadata, error) {
	resp, err := http.Get(downloadURL)
	if err != nil {
		return downloadedFileMetadata{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return downloadedFileMetadata{}, fmt.Errorf("bad status code: %s", resp.Status)
	}

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

	h1 := sha1.New()
	h256 := sha256.New()
	size, err := io.Copy(io.MultiWriter(h1, h256), resp.Body)
	if err != nil {
		return downloadedFileMetadata{}, err
	}

	return downloadedFileMetadata{
		Filename: filename,
		SHA1:     hex.EncodeToString(h1.Sum(nil)),
		SHA256:   hex.EncodeToString(h256.Sum(nil)),
		Size:     size,
	}, nil
}

func runAdd(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sbutils add <url>")
		os.Exit(1)
	}

	downloadURL := args[0]
	fmt.Printf("Fetching: %s\n", downloadURL)

	meta, err := fetchFileMetadata(downloadURL)
	if err != nil {
		fmt.Printf("Failed to download: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Downloaded %s (%d bytes)\n", meta.Filename, meta.Size)
	fmt.Printf("SHA1:   %s\n", meta.SHA1)
	fmt.Printf("SHA256: %s\n", meta.SHA256)

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
	modPath := filepath.ToSlash(filepath.Join("mods", meta.Filename))
	exists := false
	for i, f := range index.Files {
		if f.Path == modPath {
			// Update existing
			index.Files[i].Hashes = map[string]string{
				"sha1":   meta.SHA1,
				"sha256": meta.SHA256,
			}
			index.Files[i].Downloads = []string{downloadURL}
			index.Files[i].FileSize = meta.Size
			exists = true
			fmt.Printf("Updated existing entry for %s\n", modPath)
			break
		}
	}

	if !exists {
		index.Files = append(index.Files, resource.SBFile{
			Path: modPath,
			Hashes: map[string]string{
				"sha1":   meta.SHA1,
				"sha256": meta.SHA256,
			},
			Downloads: []string{downloadURL},
			FileSize:  meta.Size,
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
