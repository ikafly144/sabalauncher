package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
)

func runRepo(args []string) {
	if len(args) < 1 {
		printRepoUsage()
		os.Exit(1)
	}

	command := args[0]
	switch command {
	case "init":
		runRepoInit(args[1:])
	case "add":
		runRepoAdd(args[1:])
	case "set-latest":
		runRepoSetLatest(args[1:])
	default:
		fmt.Printf("Unknown repo command: %s\n", command)
		printRepoUsage()
		os.Exit(1)
	}
}

func printRepoUsage() {
	fmt.Println("Usage: sbutils repo <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init <name>")
	fmt.Println("      Initialize a new repository manifest.json")
	fmt.Println("  add <id> <type(sbpack|sbpatch)> <file_path> <remote_url> [local_path]")
	fmt.Println("      Calculate hashes for a local file and add it to the manifest")
	fmt.Println("  set-latest <id>")
	fmt.Println("      Update the latest_patch field in the manifest")
}

func runRepoInit(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sbutils repo init <name>")
		os.Exit(1)
	}

	repo := resource.SBRepository{
		Name:    args[0],
		Patches: []resource.SBRepoPatch{},
	}

	writeManifest("manifest.json", repo)
	fmt.Printf("Initialized manifest.json for repository '%s'\n", repo.Name)
}

func runRepoAdd(args []string) {
	if len(args) < 4 {
		fmt.Println("Usage: sbutils repo add <id> <type(sbpack|sbpatch)> <file_path> <remote_url> [local_path]")
		os.Exit(1)
	}

	id := args[0]
	typ := args[1]
	filePath := args[2]
	remoteURL := args[3]
	var localPath string
	if len(args) > 4 {
		localPath = args[4]
	}

	if typ != "sbpack" && typ != "sbpatch" {
		fmt.Printf("Invalid type: %s. Must be 'sbpack' or 'sbpatch'\n", typ)
		os.Exit(1)
	}

	hash, err := hashFile(filePath)
	if err != nil {
		fmt.Printf("Failed to hash file: %v\n", err)
		os.Exit(1)
	}

	repo := readManifest("manifest.json")

	// Update existing or add new
	found := false
	for i, p := range repo.Patches {
		if p.ID == id {
			repo.Patches[i] = resource.SBRepoPatch{
				ID:         id,
				Type:       typ,
				Hash:       map[string]string{"sha256": hash},
				RemotePath: remoteURL,
				LocalPath:  localPath,
			}
			found = true
			fmt.Printf("Updated existing patch entry '%s'\n", id)
			break
		}
	}

	if !found {
		repo.Patches = append(repo.Patches, resource.SBRepoPatch{
			ID:         id,
			Type:       typ,
			Hash:       map[string]string{"sha256": hash},
			RemotePath: remoteURL,
			LocalPath:  localPath,
		})
		fmt.Printf("Added new patch entry '%s'\n", id)
	}

	writeManifest("manifest.json", repo)
}

func runRepoSetLatest(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sbutils repo set-latest <id>")
		os.Exit(1)
	}

	id := args[0]
	repo := readManifest("manifest.json")

	// Verify ID exists
	found := false
	for _, p := range repo.Patches {
		if p.ID == id {
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("Warning: Patch ID '%s' not found in manifest\n", id)
	}

	repo.LatestPatch = id
	writeManifest("manifest.json", repo)
	fmt.Printf("Set latest_patch to '%s'\n", id)
}

func readManifest(path string) resource.SBRepository {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Failed to read %s: %v\n", path, err)
		os.Exit(1)
	}
	var repo resource.SBRepository
	if err := json.Unmarshal(b, &repo); err != nil {
		fmt.Printf("Failed to parse %s: %v\n", path, err)
		os.Exit(1)
	}
	return repo
}

func writeManifest(path string, repo resource.SBRepository) {
	b, err := json.MarshalIndent(repo, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal manifest: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		fmt.Printf("Failed to write %s: %v\n", path, err)
		os.Exit(1)
	}
}
