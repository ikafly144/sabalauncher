package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "init":
		runInit(os.Args[2:])
	case "add":
		runAdd(os.Args[2:])
	case "pack":
		runPack(os.Args[2:])
	case "diff":
		runDiff(os.Args[2:])
	case "patch":
		runPatch(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: sbutils <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init <name> <version> [minecraft_version] [loader_id] [loader_version]")
	fmt.Println("      Initialize a new sb.index.json workspace")
	fmt.Println("  add <url>")
	fmt.Println("      Download a mod from URL, hash it, and add to sb.index.json")
	fmt.Println("  pack <dir> <output.sbpack>")
	fmt.Println("      Package a directory (containing sb.index.json and overrides/) into an .sbpack")
	fmt.Println("  diff <old.sbpack> <new.sbpack> <output.sbpatch>")
	fmt.Println("      Create a patch from old to new sbpack")
	fmt.Println("  patch <base.sbpack> <patch.sbpatch> <output.sbpack>")
	fmt.Println("      Apply a patch to a base sbpack to generate a new sbpack")
}
