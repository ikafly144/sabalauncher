package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
)

type stringListFlag []string

func (f *stringListFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringListFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

type fileEditSpec struct {
	Path string
	URL  string
}

func runEdit(args []string) {
	if err := executeEdit(args, os.Stdout); err != nil {
		fmt.Printf("Error: %v\n", err)
		printEditUsage()
		os.Exit(1)
	}
}

func printEditUsage() {
	fmt.Println("Usage: sbutils edit [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -indexfile <path>")
	fmt.Println("      Target sb.index.json path (default: sb.index.json)")
	fmt.Println("  -name <name>")
	fmt.Println("      Set pack name")
	fmt.Println("  -version <version>")
	fmt.Println("      Set pack version")
	fmt.Println("  -require <id=version>")
	fmt.Println("      Set dependency (repeatable, also supports id@version)")
	fmt.Println("  -droprequire <id>")
	fmt.Println("      Remove dependency by id (repeatable)")
	fmt.Println("  -file <path> <url>")
	fmt.Println("      Upsert files entry by path using metadata fetched from URL (repeatable)")
	fmt.Println("  -dropfile <path>")
	fmt.Println("      Remove files entry by path (repeatable)")
	fmt.Println("  -print")
	fmt.Println("      Print resulting JSON to stdout without writing file")
}

func executeEdit(args []string, out io.Writer) error {
	args, fileEdits, err := extractFileEditSpecs(args)
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("edit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	indexPath := "sb.index.json"
	name := ""
	version := ""
	printOnly := false
	var requireSpecs stringListFlag
	var dropRequires stringListFlag
	var dropFiles stringListFlag

	fs.StringVar(&indexPath, "indexfile", "sb.index.json", "target sb.index.json file")
	fs.StringVar(&indexPath, "index", "sb.index.json", "target sb.index.json file")
	fs.StringVar(&name, "name", "", "set pack name")
	fs.StringVar(&version, "version", "", "set pack version")
	fs.Var(&requireSpecs, "require", "set dependency id=version (repeatable)")
	fs.Var(&dropRequires, "droprequire", "remove dependency id (repeatable)")
	fs.Var(&dropFiles, "dropfile", "remove files entry by path (repeatable)")
	fs.BoolVar(&printOnly, "print", false, "print resulting JSON without writing file")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) > 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	if name == "" && version == "" && len(requireSpecs) == 0 && len(dropRequires) == 0 &&
		len(fileEdits) == 0 && len(dropFiles) == 0 {
		return fmt.Errorf("no edit flags provided")
	}

	indexBytes, err := os.ReadFile(indexPath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", indexPath, err)
	}

	var index resource.SBIndex
	if err := json.Unmarshal(indexBytes, &index); err != nil {
		return fmt.Errorf("failed to parse %s: %w", indexPath, err)
	}

	if index.Dependencies == nil {
		index.Dependencies = map[string]string{}
	}

	if name != "" {
		index.Name = name
	}
	if version != "" {
		index.Version = version
	}

	for _, spec := range requireSpecs {
		depID, depVersion, err := parseRequireSpec(spec)
		if err != nil {
			return err
		}
		index.Dependencies[depID] = depVersion
	}

	for _, depID := range dropRequires {
		depID = strings.TrimSpace(depID)
		if depID == "" {
			return fmt.Errorf("droprequire cannot be empty")
		}
		delete(index.Dependencies, depID)
	}

	for _, spec := range fileEdits {
		meta, err := fetchFileMetadata(spec.URL)
		if err != nil {
			return fmt.Errorf("failed to fetch file metadata for %s: %w", spec.URL, err)
		}
		index.Files = upsertSBFile(index.Files, spec.Path, spec.URL, meta)
	}

	if len(dropFiles) > 0 {
		dropSet := map[string]struct{}{}
		for _, rawPath := range dropFiles {
			dropPath, err := normalizeSBFilePath(rawPath)
			if err != nil {
				return fmt.Errorf("invalid -dropfile path %q: %w", rawPath, err)
			}
			dropSet[dropPath] = struct{}{}
		}
		index.Files = removeSBFiles(index.Files, dropSet)
	}

	outBytes, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated index: %w", err)
	}

	if printOnly {
		_, err := fmt.Fprintln(out, string(outBytes))
		return err
	}

	if err := os.WriteFile(indexPath, outBytes, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", indexPath, err)
	}
	_, err = fmt.Fprintf(out, "Successfully updated %s.\n", indexPath)
	return err
}

func parseRequireSpec(spec string) (string, string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return "", "", fmt.Errorf("require cannot be empty")
	}

	parts := []string{}
	if strings.Contains(spec, "=") {
		parts = strings.SplitN(spec, "=", 2)
	} else if strings.Contains(spec, "@") {
		parts = strings.SplitN(spec, "@", 2)
	} else {
		return "", "", fmt.Errorf("require must be in id=version or id@version format: %q", spec)
	}

	depID := strings.TrimSpace(parts[0])
	depVersion := strings.TrimSpace(parts[1])
	if depID == "" || depVersion == "" {
		return "", "", fmt.Errorf("require must include both id and version: %q", spec)
	}
	return depID, depVersion, nil
}

func extractFileEditSpecs(args []string) ([]string, []fileEditSpec, error) {
	remaining := make([]string, 0, len(args))
	fileEdits := []fileEditSpec{}

	for i := 0; i < len(args); i++ {
		if args[i] != "-file" {
			remaining = append(remaining, args[i])
			continue
		}
		if i+2 >= len(args) {
			return nil, nil, fmt.Errorf("-file requires <path> <url>")
		}

		filePath, err := normalizeSBFilePath(args[i+1])
		if err != nil {
			return nil, nil, fmt.Errorf("invalid -file path %q: %w", args[i+1], err)
		}

		downloadURL := strings.TrimSpace(args[i+2])
		parsedURL, err := url.ParseRequestURI(downloadURL)
		if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
			return nil, nil, fmt.Errorf("invalid -file URL %q", downloadURL)
		}

		fileEdits = append(fileEdits, fileEditSpec{
			Path: filePath,
			URL:  downloadURL,
		})
		i += 2
	}

	return remaining, fileEdits, nil
}

func normalizeSBFilePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	p = filepath.ToSlash(filepath.Clean(p))
	if p == "." || p == "/" {
		return "", fmt.Errorf("path must point to a file")
	}
	return p, nil
}

func upsertSBFile(files []resource.SBFile, filePath string, downloadURL string, meta downloadedFileMetadata) []resource.SBFile {
	newFile := resource.SBFile{
		Path: filePath,
		Hashes: map[string]string{
			"sha1":   meta.SHA1,
			"sha256": meta.SHA256,
		},
		Downloads: []string{downloadURL},
		FileSize:  meta.Size,
	}

	for i := range files {
		existingPath, err := normalizeSBFilePath(files[i].Path)
		if err != nil {
			existingPath = files[i].Path
		}
		if existingPath == filePath {
			newFile.Env = files[i].Env
			files[i] = newFile
			return files
		}
	}

	return append(files, newFile)
}

func removeSBFiles(files []resource.SBFile, dropSet map[string]struct{}) []resource.SBFile {
	filtered := make([]resource.SBFile, 0, len(files))
	for _, f := range files {
		normalizedPath, err := normalizeSBFilePath(f.Path)
		if err != nil {
			normalizedPath = f.Path
		}
		if _, shouldDrop := dropSet[normalizedPath]; shouldDrop {
			continue
		}
		filtered = append(filtered, f)
	}
	return filtered
}
