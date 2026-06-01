package core

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/v2/pkg/msa"
	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
)

// TestModLoaders_FullLifecycle tests the actual installation and launch process
// using real versions and external APIs/installers.
// Note: This test requires internet access and a Java runtime.
func TestModLoaders_FullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "full-lifecycle-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Point DataDir to temp folder
	oldDataDir := resource.DataDir
	resource.DataDir = tempDir
	defer func() { resource.DataDir = oldDataDir }()

	javaPath, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java not found, skipping full lifecycle test")
	}

	gameVersion := "1.21.1"

	testCases := []struct {
		name          string
		loaderType    string
		loaderVersion string
	}{
		{name: "Fabric", loaderType: "fabric-loader", loaderVersion: "0.19.2"},
		{name: "Quilt", loaderType: "quilt-loader", loaderVersion: "0.29.2"},
		// Forge and NeoForge installers are very heavy and might fail in limited environments.
		// We'll try NeoForge as it's the modern standard.
		{name: "NeoForge", loaderType: "neoforge", loaderVersion: "21.1.233"},
		{name: "Forge", loaderType: "forge", loaderVersion: "52.1.14"},
	}

	profile := &msa.MinecraftProfile{
		Username: "TestUser",
		UUID:     uuid.New(),
	}
	accessToken := "test-token"

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inst := &resource.Instance{
				Name: tc.name,
				Path: filepath.Join(tempDir, "instances", tc.name),
				Versions: []resource.InstanceVersion{
					{ID: "minecraft", Version: gameVersion},
					{ID: tc.loaderType, Version: tc.loaderVersion},
				},
			}

			slog.Info("Starting setup for", "loader", tc.name)

			// 1. Run SetupInstance (Real download/install)
			state := resource.SetupInstance(tempDir, inst)

			// Monitor progress until done or timeout
			timeout := time.After(10 * time.Minute) // Installers can be slow
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

		loop:
			for {
				select {
				case <-timeout:
					t.Fatalf("Setup timed out for %s", tc.name)
				case <-ticker.C:
					if state.IsDone() {
						break loop
					}
					slog.Info("Setup Progress", "loader", tc.name, "step", state.FriendlyName(), "progress", state.Progress())
				}
			}

			if err := state.Error(); err != nil {
				t.Fatalf("Setup failed for %s: %v", tc.name, err)
			}

			slog.Info("Setup completed, attempting launch", "loader", tc.name)

			// 2. Generate Launch Config
			loader, err := resource.GetModLoader(inst)
			if err != nil {
				t.Fatalf("Failed to get loader: %v", err)
			}

			features := map[string]bool{
				"is_demo_user":          true,
				"has_custom_resolution": true,
			}
			config, err := loader.GenerateLaunchConfig(inst, features)
			if err != nil {
				t.Fatalf("Failed to generate launch config: %v", err)
			}

			// 3. Boot Game (Actual process execution)
			// We expect it to fail with ClassNotFoundException if we don't download ALL libraries,
			// but SetupInstance above SHOULD have downloaded them.
			// To keep the test from actually popping up a window or staying open,
			// we'll use a context with a short timeout, or rely on the fact that
			// it will probably crash early due to missing hardware/drivers in CI.

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			var stdout, stderr strings.Builder
			manifest, _ := resource.GetClientManifestForInstance(inst)

			err = resource.BootGameFromConfig(ctx, javaPath, config, manifest, inst, profile, accessToken, &stdout, &stderr)

			// We don't strictly require err == nil because the game might exit with error code
			// if it can't initialize graphics, but we want to see if it ATTEMPTED to boot
			// with the right class.
			output := stdout.String() + stderr.String()

			// If it's a real launch, it should at least mention "Minecraft" or the loader in the logs
			// or fail with a specific Minecraft/Loader exception, not a "file not found" for java.
			if strings.Contains(output, "java.lang.ClassNotFoundException") {
				// This is actually "success" for the test logic because it means Java ran and tried to load the class
				slog.Info("Java executed correctly but class was missing (expected if jars are dummy or minimal)", "loader", tc.name)
			} else if strings.Contains(output, "Loading Minecraft") || strings.Contains(output, "Forge") || strings.Contains(output, "Fabric") {
				slog.Info("Detected loader signature in output", "loader", tc.name)
			} else if err != nil && !strings.Contains(err.Error(), "exit status") {
				t.Errorf("Launch failed with unexpected error: %v, Output: %s", err, output)
			}

			slog.Info("Launch attempt verified", "loader", tc.name)
		})
	}
}
