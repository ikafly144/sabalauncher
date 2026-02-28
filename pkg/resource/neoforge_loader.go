package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ikafly144/sabalauncher/pkg/runcmd"
)

// NeoForgeLoader implements the ModLoader interface for the NeoForge mod loader.
type NeoForgeLoader struct {
	VanillaVersion  string
	NeoForgeVersion string
}

// NewNeoForgeLoader creates a new NeoForgeLoader instance.
func NewNeoForgeLoader(vanillaVersion, neoforgeVersion string) *NeoForgeLoader {
	return &NeoForgeLoader{
		VanillaVersion:  vanillaVersion,
		NeoForgeVersion: neoforgeVersion,
	}
}

// Install handles the downloading and execution of the NeoForge installer.
func (n *NeoForgeLoader) Install(ctx context.Context, inst *Instance) error {
	dataPath := DataDir
	neoforgeDir := n.VanillaVersion + "-neoforge-" + n.NeoForgeVersion

	// Check if already installed
	manifestPath := filepath.Join(dataPath, "versions", neoforgeDir, neoforgeDir+".json")
	if _, err := os.Stat(manifestPath); err == nil {
		slog.Info("NeoForge is already installed", "version", n.NeoForgeVersion)
		return nil
	}

	slog.Info("Installing NeoForge", "vanilla", n.VanillaVersion, "neoforge", n.NeoForgeVersion)

	// 1. Download Installer
	installerURL := fmt.Sprintf("https://maven.neoforged.net/releases/net/neoforged/neoforge/%s/neoforge-%s-installer.jar", n.NeoForgeVersion, n.NeoForgeVersion)
	tmpFile, err := os.CreateTemp("", "neoforge-installer-*.jar")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	resp, err := http.Get(installerURL)
	if err != nil {
		return fmt.Errorf("failed to download neoforge installer: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("neoforge installer download returned: %s", resp.Status)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("failed to save neoforge installer: %w", err)
	}
	tmpFile.Close()

	// 2. Run Installer
	// NeoForge installer requires a dummy launcher_profiles.json
	dummyProfiles := filepath.Join(dataPath, "launcher_profiles.json")
	if _, err := os.Stat(dummyProfiles); os.IsNotExist(err) {
		if err := os.WriteFile(dummyProfiles, []byte(`{"profiles":{}}`), 0644); err != nil {
			return err
		}
		defer os.Remove(dummyProfiles)
	}

	cmd := exec.Command("java", "-jar", tmpFile.Name(), "--installClient", dataPath)
	cmd.Stdout = slog.NewLogLogger(slog.Default().Handler(), slog.LevelInfo).Writer()
	cmd.Stderr = slog.NewLogLogger(slog.Default().Handler(), slog.LevelInfo).Writer()
	cmd.SysProcAttr = runcmd.GetSysProcAttr()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("neoforge installer failed: %w", err)
	}

	return nil
}

// GenerateLaunchConfig produces the configuration required to launch the game with NeoForge.
func (n *NeoForgeLoader) GenerateLaunchConfig(inst *Instance) (*LaunchConfig, error) {
	dataPath := DataDir
	neoforgeDir := n.VanillaVersion + "-neoforge-" + n.NeoForgeVersion

	// 1. Load NeoForge Manifest
	manifestPath := filepath.Join(dataPath, "versions", neoforgeDir, neoforgeDir+".json")
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open neoforge manifest: %w", err)
	}
	defer file.Close()

	var manifest ClientManifest
	if err := json.NewDecoder(file).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to decode neoforge manifest: %w", err)
	}

	// 2. Generate Classpath
	var classpath []string
	classpath = append(classpath, filepath.Join(dataPath, "versions", manifest.ID, manifest.ID+".jar"))
	classpathSeparator := string(os.PathListSeparator)
	for _, library := range manifest.Libraries {
		if library.Downloads.Classifiers != nil {
			for _, classifier := range library.Downloads.Classifiers {
				classpath = append(classpath, filepath.Join(dataPath, "libraries", classifier.Path))
			}
		}
		if library.Downloads.Artifact.Path != "" {
			classpath = append(classpath, filepath.Join(dataPath, "libraries", library.Downloads.Artifact.Path))
		}
	}

	// 3. Resolve Arguments
	cmdMap := map[string]string{
		"natives_directory":   filepath.Join(dataPath, "bin", manifest.ID),
		"launcher_name":       "SabaLauncher",
		"launcher_version":    "1.0",
		"classpath":           strings.Join(classpath, classpathSeparator),
		"library_directory":   filepath.Join(dataPath, "libraries"),
		"classpath_separator": classpathSeparator,
	}

	var jvmArgs []string
	memory := uint64(2048) // Fixed default memory
	jvmArgs = append(jvmArgs, "-Xmx"+fmt.Sprintf("%d", memory)+"M")
	jvmArgs = append(jvmArgs, defaultJvmArgs...)

	skipNext := false
	for _, arg := range manifest.Arguments.Jvm {
		if skipNext {
			skipNext = false
			continue
		}
		if arg == nil {
			continue
		}
		switch arg := arg.(type) {
		case JvmArgumentString:
			val := arg.String()
			if val == "-cp" {
				skipNext = true
				continue
			}
			if strings.Contains(val, "${classpath}") {
				continue
			}
			for before, after := range cmdMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s}", before), after)
			}
			jvmArgs = append(jvmArgs, val)
		case JvmArgumentRule:
			if !slices.ContainsFunc(arg.Rules, func(rule JvmArgumentRuleType) bool {
				return rule.Action.Allowed() != rule.OS.Matched()
			}) {
				continue
			}
			for _, a := range arg.Value {
				if a == "-cp" {
					continue
				}
				if strings.Contains(a, "${classpath}") {
					continue
				}
				for before, after := range cmdMap {
					a = strings.ReplaceAll(a, fmt.Sprintf("${%s}", before), after)
				}
				jvmArgs = append(jvmArgs, a)
			}
		}
	}

	var gameArgs []string
	for _, arg := range manifest.Arguments.Game {
		if arg == nil {
			continue
		}
		switch arg := arg.(type) {
		case GameArgumentString:
			gameArgs = append(gameArgs, arg.String())
		case GameArgumentRule:
			for _, a := range arg.Value {
				gameArgs = append(gameArgs, a)
			}
		}
	}

	config := &LaunchConfig{
		MainClass:     manifest.MainClass,
		JVMArguments:  jvmArgs,
		GameArguments: gameArgs,
		Classpath:     classpath,
	}

	return config, nil
}
