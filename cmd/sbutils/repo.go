package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
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
	case "validate":
		runRepoValidate(args[1:])
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
	fmt.Println("      Calculate hashes for a local file, add it to the manifest, and set as latest")
	fmt.Println("  set-latest <id>")
	fmt.Println("      Update the latest_patch field in the manifest")
	fmt.Println("  validate")
	fmt.Println("      Check if all patches in the manifest form a valid dependency graph")
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

	// Automatically set latest
	repo.LatestPatch = id
	fmt.Printf("Set latest_patch to '%s'\n", id)

	// Validate before saving
	if err := validateRepoGraph(&repo, filePath); err != nil {
		fmt.Printf("Warning: Repository graph validation failed: %v\n", err)
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
	if err := validateRepoGraph(&repo, ""); err != nil {
		fmt.Printf("Warning: Repository graph validation failed: %v\n", err)
	}

	writeManifest("manifest.json", repo)
	fmt.Printf("Set latest_patch to '%s'\n", id)
}

func runRepoValidate(args []string) {
	repo := readManifest("manifest.json")
	if err := validateRepoGraph(&repo, ""); err != nil {
		fmt.Printf("Validation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Repository graph is valid.")
}

func validateRepoGraph(repo *resource.SBRepository, currentFile string) error {
	// Map patch ID -> metadata
	type patchMeta struct {
		typ    string
		baseID string
	}
	metadata := make(map[string]patchMeta)

	for _, p := range repo.Patches {
		path := p.LocalPath
		if p.ID == repo.LatestPatch && currentFile != "" {
			path = currentFile
		}
		
		if path == "" {
			// If we don't have the file, we can't fully validate.
			// For now, assume it's valid if we can't see it, or require it.
			// Let's try current dir if LocalPath is empty
			if _, err := os.Stat(p.ID + ".sbpack"); err == nil {
				path = p.ID + ".sbpack"
			} else if _, err := os.Stat(p.ID + ".sbpatch"); err == nil {
				path = p.ID + ".sbpatch"
			}
		}

		if path != "" {
			bID, _, err := peekPatchMetadata(path)
			if err == nil {
				metadata[p.ID] = patchMeta{typ: p.Type, baseID: bID.String()}
			}
		} else {
			// Placeholder for missing files
			metadata[p.ID] = patchMeta{typ: p.Type, baseID: ""}
		}
	}
	
	// Check reachability of LatestPatch
	if repo.LatestPatch != "" {
		curr := repo.LatestPatch
		visited := make(map[string]bool)
		for {
			if curr == "" || curr == uuid.Nil.String() {
				return fmt.Errorf("reached dead end before finding an sbpack")
			}
			if visited[curr] {
				return fmt.Errorf("circular dependency detected at %s", curr)
			}
			visited[curr] = true
			
			m, ok := metadata[curr]
			if !ok {
				return fmt.Errorf("patch %s is missing from manifest or file not found", curr)
			}
			
			if m.typ == "sbpack" {
				// Success!
				break
			}
			
			curr = m.baseID
		}
	}

	return nil
}

func peekPatchMetadata(path string) (baseID uuid.UUID, indexID uuid.UUID, err error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "sb.patch.json" {
			rc, err := f.Open()
			if err != nil {
				return uuid.Nil, uuid.Nil, err
			}
			var p resource.SBPatch
			err = json.NewDecoder(rc).Decode(&p)
			rc.Close()
			if err != nil {
				return uuid.Nil, uuid.Nil, err
			}
			return p.BaseID, p.Index.ID, nil
		}
		if f.Name == "sb.index.json" {
			rc, err := f.Open()
			if err != nil {
				return uuid.Nil, uuid.Nil, err
			}
			var idx resource.SBIndex
			err = json.NewDecoder(rc).Decode(&idx)
			rc.Close()
			if err != nil {
				return uuid.Nil, uuid.Nil, err
			}
			return uuid.Nil, idx.ID, nil
		}
	}
	return uuid.Nil, uuid.Nil, fmt.Errorf("metadata not found in %s", path)
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
