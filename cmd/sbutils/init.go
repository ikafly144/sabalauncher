package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ikafly144/sabalauncher/pkg/resource"
)

func runInit(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: sbutils init <name> <version> [minecraft_version] [loader_id] [loader_version]")
		os.Exit(1)
	}

	name := args[0]
	version := args[1]
	
	deps := make(map[string]string)
	if len(args) > 2 {
		deps["minecraft"] = args[2]
	}
	if len(args) > 4 {
		deps[args[3]] = args[4]
	}

	index := resource.SBIndex{
		FormatVersion: 1,
		Name:          name,
		Version:       version,
		Dependencies:  deps,
		Files:         []resource.SBFile{},
	}

	indexBytes, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal index: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile("sb.index.json", indexBytes, 0644); err != nil {
		fmt.Printf("Failed to write sb.index.json: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll("overrides", 0755); err != nil {
		fmt.Printf("Failed to create overrides directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Initialized sb.index.json and overrides/ directory.")
}
