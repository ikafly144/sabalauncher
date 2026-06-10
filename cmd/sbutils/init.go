package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
)

func runInit(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: sbutils init <name> [minecraft_version] [loader_id] [loader_version]")
		os.Exit(1)
	}

	name := args[0]

	deps := make(map[string]string)
	if len(args) > 1 {
		deps["minecraft"] = args[1]
	}
	if len(args) > 3 {
		deps[args[2]] = args[3]
	}

	newID, err := uuid.NewV7()
	if err != nil {
		fmt.Printf("Failed to generate UUID: %v\n", err)
		os.Exit(1)
	}

	index := resource.SBPackIndex{
		FormatVersion: resource.SBPackFormatVersion,
		Name:          name,
		ID:            newID,
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
