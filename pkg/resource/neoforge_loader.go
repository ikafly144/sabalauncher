package resource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	"github.com/ikafly144/sabalauncher/v2/pkg/runcmd"
)

// NeoForgeLoader implements the ModLoader interface for the NeoForge mod loader.
type NeoForgeLoader struct {
	VanillaVersion  string
	NeoForgeVersion string
	progress        float32
}

var ErrNeoForgeManifestNotFound = errors.New("neoforge manifest not found")

func (n *NeoForgeLoader) Progress() float32 {
	return n.progress
}

func (n *NeoForgeLoader) findManifestID(dataPath string) (string, error) {
	preferred := n.VanillaVersion + "-neoforge-" + n.NeoForgeVersion
	preferredPath := filepath.Join(dataPath, "versions", preferred, preferred+".json")
	if _, err := os.Stat(preferredPath); err == nil {
		return preferred, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	fallback := "neoforge-" + n.NeoForgeVersion
	fallbackPath := filepath.Join(dataPath, "versions", fallback, fallback+".json")
	if _, err := os.Stat(fallbackPath); err == nil {
		return fallback, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	return "", fmt.Errorf("%w: tried %s and %s", ErrNeoForgeManifestNotFound, preferred, fallback)
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
	if manifestID, err := n.findManifestID(dataPath); err == nil {
		slog.Info("NeoForge is already installed", "version", n.NeoForgeVersion, "id", manifestID)
		n.progress = 1.0
		return nil
	} else if !errors.Is(err, ErrNeoForgeManifestNotFound) {
		return err
	}

	slog.Info("Installing NeoForge", "vanilla", n.VanillaVersion, "neoforge", n.NeoForgeVersion)
	n.progress = 0.0

	// 1. Download Installer
	installerURL := fmt.Sprintf("%s/%s/neoforge-%s-installer.jar", NeoForgeMavenURL, n.NeoForgeVersion, n.NeoForgeVersion)
	tmpFile, err := os.CreateTemp("", "neoforge-installer-*.jar")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	n.progress = 0.1
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

	n.progress = 0.5
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

	n.progress = 1.0
	return nil
}

// GenerateLaunchConfig produces the configuration required to launch the game with NeoForge.
func (n *NeoForgeLoader) GenerateLaunchConfig(inst *Instance, features map[string]bool, memory uint64) (*LaunchConfig, error) {
	dataPath := DataDir
	neoforgeDir, err := n.findManifestID(dataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to locate neoforge manifest: %w", err)
	}

	// 1. Load NeoForge Manifest recursively (handles inheritance from vanilla)
	manifest, err := GetClientManifestRecursive(dataPath, neoforgeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load neoforge manifest: %w", err)
	}

	// 2. Generate Classpath
	var classpath []string
	classpath = append(classpath, filepath.Join(dataPath, "versions", manifest.ID, manifest.ID+".jar"))
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

	var jvmArgs []string
	jvmArgs = append(jvmArgs, "-Xmx"+fmt.Sprintf("%d", memory)+"M")
	jvmArgs = append(jvmArgs, defaultJvmArgs...)

	for _, arg := range manifest.Arguments.Jvm {
		if arg == nil {
			continue
		}
		switch arg := arg.(type) {
		case JvmArgumentString:
			jvmArgs = append(jvmArgs, arg.String())
		case JvmArgumentRule:
			if !slices.ContainsFunc(arg.Rules, func(rule JvmArgumentRuleType) bool {
				return rule.Action.Allowed() != rule.OS.Matched()
			}) {
				continue
			}
			jvmArgs = append(jvmArgs, arg.Value...)
		}
	}

	gameArgs := EvaluateGameArguments(manifest.Arguments.Game, features)

	config := &LaunchConfig{
		MainClass:     manifest.MainClass,
		JVMArguments:  jvmArgs,
		GameArguments: gameArgs,
		Classpath:     classpath,
	}

	return config, nil
}
