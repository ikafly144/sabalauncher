package resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ikafly144/sabalauncher/v2/pkg/msa"
	"github.com/ikafly144/sabalauncher/v2/pkg/osinfo"
	"github.com/ikafly144/sabalauncher/v2/pkg/runcmd"
)

const (
	ManifestEndpoint = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"
)

type VersionManifest struct {
	Latest   Latest    `json:"latest"`
	Versions []Version `json:"versions"`
}

type Latest struct {
	Release  string `json:"release"`
	Snapshot string `json:"snapshot"`
}

type VersionType string

const (
	Release  VersionType = "release"
	Snapshot VersionType = "snapshot"
)

type Version struct {
	ID              string      `json:"id"`
	Type            VersionType `json:"type"`
	URL             string      `json:"url"`
	Time            time.Time   `json:"time"`
	ReleaseTime     time.Time   `json:"releaseTime"`
	Sha1            string      `json:"sha1"`
	ComplianceLevel int         `json:"complianceLevel"`
}

func GetManifest() (*VersionManifest, error) {
	resp, err := http.Get(ManifestEndpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch manifest")
	}

	var manifest VersionManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func GetVersion(version string) (*Version, error) {
	manifest, err := GetManifest()
	if err != nil {
		return nil, err
	}
	for _, v := range manifest.Versions {
		if v.ID == version {
			return &v, nil
		}
	}
	return nil, nil
}

func GetClientManifest(version *Version) (*ClientManifest, error) {
	if version == nil {
		return nil, errors.New("version is nil")
	}
	resp, err := http.Get(version.URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch client manifest")
	}
	var clientManifest ClientManifest
	if err := json.NewDecoder(resp.Body).Decode(&clientManifest); err != nil {
		return nil, err
	}
	return &clientManifest, nil
}

func GetLocalClientManifest(dataDir, version string) (*ClientManifest, error) {
	manifestPath := filepath.Join(dataDir, "versions", version, version+".json")
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var clientManifest ClientManifest
	if err := json.NewDecoder(file).Decode(&clientManifest); err != nil {
		return nil, err
	}
	return &clientManifest, nil
}

const (
	AssetResourceURL = "https://resources.download.minecraft.net/"
)

type Asset struct {
	Hash string `json:"hash"`
	Size int    `json:"size"`
}

type Assets struct {
	Objects map[string]Asset `json:"objects"`
}

type DownloadWorker struct {
	m      sync.Mutex
	w      sync.WaitGroup
	remain int
	tasks  []func() error
	err    error
}

func (w *DownloadWorker) addTask(task func() error) {
	if task == nil {
		return
	}
	w.m.Lock()
	defer w.m.Unlock()
	w.w.Add(1)
	w.remain++
	w.tasks = append(w.tasks, task)
}

func (w *DownloadWorker) Run() (err error) {
	const (
		maxProcCount = 8
	)
	for range maxProcCount {
		go func() {
			if e := w.run(); e != nil {
				w.err = e
				slog.Error("Download worker encountered an error", "error", e)
				if err != nil {
					e = fmt.Errorf("%w: %w", err, e)
				}
				err = e
			}
		}()
	}
	if err := w.Wait(); err != nil {
		slog.Error("Download worker failed", "error", err)
		w.err = err
	}
	return err
}

func (w *DownloadWorker) run() error {
	retry := 0
	for {
		w.m.Lock()
		if len(w.tasks) == 0 {
			w.m.Unlock()
			break
		}
		task := w.tasks[0]
		w.tasks = w.tasks[1:]
		w.m.Unlock()
		if err := task(); err != nil {
			if retry < 5 {
				w.m.Lock()
				retry++
				w.tasks = append(w.tasks, task)
				slog.Error("Task failed, retrying", "error", err)
				w.m.Unlock()
				time.Sleep(5 * time.Second)
				continue
			}
			return err
		}
		w.m.Lock()
		w.remain--
		w.w.Done()
		if w.remain == 0 {
			w.m.Unlock()
			break
		}
		w.m.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func (w *DownloadWorker) Wait() error {
	wait := func() <-chan struct{} {
		done := make(chan struct{})
		go func() {
			w.w.Wait()
			close(done)
		}()
		return done
	}()
	for {
		select {
		case <-wait:
			return nil
		default:
			if w.err != nil {
				slog.Error("Download worker encountered an error", "error", w.err)
				return w.err
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (w *DownloadWorker) Remain() int {
	w.m.Lock()
	defer w.m.Unlock()
	return w.remain
}

func DownloadAssets(clientManifest *ClientManifest, dataDir string) (*DownloadWorker, error) {
	if clientManifest == nil {
		return nil, errors.New("client manifest is nil")
	}

	resp, err := http.Get(clientManifest.AssetIndex.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch asset index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch assets")
	}
	var assets Assets
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		return nil, err
	}
	var workers DownloadWorker
	for _, asset := range assets.Objects {
		path := filepath.Join(dataDir, "assets", "objects", asset.Hash[:2], asset.Hash)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			workers.addTask(func() error {
				start := time.Now()
				slog.Info("Downloading asset", "path", path, "hash", asset.Hash)
				if err := assetDownloadWorker(asset, path); err != nil {
					return err
				}
				slog.Info("Downloaded asset", "path", path, "duration", time.Since(start))
				return nil
			})
		}
	}
	// Save the asset index to disk
	workers.addTask(func() error {
		assetIndexPath := filepath.Join(dataDir, "assets", "indexes", clientManifest.AssetIndex.ID+".json")
		_ = os.MkdirAll(filepath.Dir(assetIndexPath), os.ModePerm)
		assetIndexFile, err := os.Create(assetIndexPath)
		if err != nil {
			return err
		}
		defer assetIndexFile.Close()
		encoder := json.NewEncoder(assetIndexFile)
		if err := encoder.Encode(assets); err != nil {
			return err
		}
		slog.Info("Saved asset index", "path", assetIndexPath)
		return nil
	})
	// Save logging configuration to disk
	workers.addTask(func() error {
		if clientManifest.Logging.Client.File.URL == "" {
			return nil
		}
		resp, err := http.Get(clientManifest.Logging.Client.File.URL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return errors.New("failed to download logging configuration")
		}
		loggingPath := filepath.Join(dataDir, "assets", "log_configs", clientManifest.Logging.Client.File.ID)
		_ = os.MkdirAll(filepath.Dir(loggingPath), os.ModePerm)
		loggingFile, err := os.Create(loggingPath)
		if err != nil {
			return err
		}
		defer loggingFile.Close()
		_, err = io.Copy(loggingFile, resp.Body)
		if err != nil {
			return err
		}
		slog.Info("Saved logging configuration", "path", loggingPath)
		return nil
	})
	return &workers, nil
}

func assetDownloadWorker(asset Asset, path string) error {
	resp, err := http.Get(AssetResourceURL + asset.Hash[:2] + "/" + asset.Hash)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to download asset")
	}
	// Save the asset to disk
	_ = os.MkdirAll(filepath.Dir(path), os.ModePerm)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	written, err := io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	if written != int64(asset.Size) {
		return errors.New("downloaded asset size does not match expected size")
	}
	return nil
}

func osName() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "osx"
	case "linux":
		return "linux"
	default:
		return ""
	}
}

func osArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "386":
		return "x86"
	case "arm64":
		return "aarch64"
	case "arm":
		return "armv7l"
	default:
		return ""
	}
}

func isMatchArch(arch string) bool {
	osArch := osArch()
	switch arch {
	case "x86_64":
		return osArch == "x86_64"
	case "x86":
		return osArch == "x86_64" || osArch == "x86"
	case "aarch64":
		return osArch == "arm64"
	case "armv7l":
		return osArch == "arm" || osArch == "arm64"
	default:
		return false
	}
}

func DownloadLibraries(clientManifest *ClientManifest, dataDir string) (*DownloadWorker, error) {
	if clientManifest == nil {
		return nil, errors.New("client manifest is nil")
	}
	var workers DownloadWorker
	for _, library := range clientManifest.Libraries {
		if slices.ContainsFunc(library.Rules, func(rule LibraryRule) bool {
			return rule.Action.Allowed() != (rule.Os.Name == osName() && (rule.Os.Arch == "" || isMatchArch(rule.Os.Arch)) && (rule.Os.Version == "" || rule.Os.Version == osinfo.GetOsVersion()))
		}) {
			slog.Info("Skipping library", "name", library.Name, "rules", library.Rules, "library", library)
			continue
		}
		path := filepath.Join(dataDir, "libraries", library.Downloads.Artifact.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			workers.addTask(func() error {
				start := time.Now()
				slog.Info("Downloading library", "path", path, "name", library.Name)
				if err := libraryDownloadWorker(library, dataDir); err != nil {
					return err
				}
				slog.Info("Downloaded library", "path", path, "duration", time.Since(start))
				return nil
			})
		}
	}
	return &workers, nil
}

func libraryDownloadWorker(library Library, dataDir string) error {
	if library.Downloads.Artifact.URL == "" {
		return errors.New("library artifact URL is empty")
	}
	if classifiers, ok := library.Downloads.Classifiers[osName()]; ok {
		resp, err := http.Get(classifiers.URL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return errors.New("failed to download library classifier")
		}
		// Save the classifier to disk
		path := filepath.Join(dataDir, "libraries", classifiers.Path)
		_ = os.MkdirAll(filepath.Dir(path), os.ModePerm)
		_ = os.MkdirAll(filepath.Dir(path), os.ModePerm)
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return err
		}
	}
	resp, err := http.Get(library.Downloads.Artifact.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to download library")
	}
	// Save the library to disk
	path := filepath.Join(dataDir, "libraries", library.Downloads.Artifact.Path)
	_ = os.MkdirAll(filepath.Dir(path), os.ModePerm)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func DownloadClientJar(clientManifest *ClientManifest, dataDir string) (*DownloadWorker, error) {
	if clientManifest == nil {
		return nil, errors.New("client manifest is nil")
	}
	var workers DownloadWorker
	path := filepath.Join(dataDir, "versions", clientManifest.ID, clientManifest.ID+".jar")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		workers.addTask(func() error {
			start := time.Now()
			slog.Info("Downloading client jar", "path", path)
			if err := clientJarDownloadWorker(clientManifest, dataDir); err != nil {
				return err
			}
			slog.Info("Downloaded client jar", "path", path, "duration", time.Since(start))
			return nil
		})
	}
	return &workers, nil
}

func clientJarDownloadWorker(clientManifest *ClientManifest, dataDir string) error {
	if clientManifest.Downloads.Client.URL == "" {
		return errors.New("client jar URL is empty")
	}
	resp, err := http.Get(clientManifest.Downloads.Client.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to download client jar")
	}
	// Save the client jar to disk
	path := filepath.Join(dataDir, "versions", clientManifest.ID, clientManifest.ID+".jar")
	_ = os.MkdirAll(filepath.Dir(path), os.ModePerm)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	// Save the client manifest to disk
	manifestPath := filepath.Join(dataDir, "versions", clientManifest.ID, clientManifest.ID+".json")
	_ = os.MkdirAll(filepath.Dir(manifestPath), os.ModePerm)
	manifestFile, err := os.Create(manifestPath)
	if err != nil {
		return err
	}
	defer manifestFile.Close()
	encoder := json.NewEncoder(manifestFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(clientManifest); err != nil {
		return err
	}
	return nil
}

func DownloadJVM(clientManifest *ClientManifest, dataDir string) (*DownloadWorker, error) {
	if clientManifest == nil {
		return nil, errors.New("client manifest is nil")
	}
	slog.Info("Downloading JVM", "version", clientManifest.JavaVersion.Component)
	var workers DownloadWorker
	if err := installJavaRuntime(clientManifest.JavaVersion.Component, "/", &workers); err != nil {
		return nil, fmt.Errorf("failed to install java runtime: %w", err)
	}
	return &workers, nil
}

var (
	defaultJvmArgs = []string{
		"-XX:+UnlockExperimentalVMOptions",
		"-XX:+UseG1GC",
		"-XX:G1NewSizePercent=20",
		"-XX:G1ReservePercent=20",
		"-XX:MaxGCPauseMillis=50",
		"-XX:G1HeapRegionSize=32M",
	}
)

func BootGame(ctx context.Context, clientManifest *ClientManifest, inst *Instance, account *msa.MinecraftAccountAuthResult, dataDir string, stdout, stderr io.Writer) error {
	if clientManifest == nil {
		return errors.New("client manifest is nil")
	}
	slog.Info("Booting game", "version", clientManifest.ID)

	// Construct a LaunchConfig from the legacy BootGame logic
	javaPath, err := GetJavaExecutablePath(clientManifest.JavaVersion.Component, "C:\\")
	if err != nil {
		return err
	}

	classpath := filepath.Join(dataDir, "versions", clientManifest.ID, clientManifest.ID+".jar")
	classpathSeparator := string(os.PathListSeparator)
	for _, library := range clientManifest.Libraries {
		if library.Downloads.Classifiers != nil {
			for _, classifier := range library.Downloads.Classifiers {
				classpath += classpathSeparator + filepath.Join(dataDir, "libraries", classifier.Path)
			}
		}
		classpath += classpathSeparator + filepath.Join(dataDir, "libraries", library.Downloads.Artifact.Path)
	}

	cmdMap := map[string]string{
		"natives_directory":   filepath.Join(dataDir, "bin", clientManifest.ID),
		"launcher_name":       "SabaLauncher",
		"launcher_version":    "1.0",
		"classpath":           classpath,
		"library_directory":   filepath.Join(dataDir, "libraries"),
		"classpath_separator": classpathSeparator,
	}

	var jvmArgs []string
	memory := uint64(2048) // Fixed for now
	jvmArgs = append(jvmArgs, "-Xmx"+strconv.FormatUint(memory, 10)+"M")
	jvmArgs = append(jvmArgs, defaultJvmArgs...)

	for _, arg := range clientManifest.Arguments.Jvm {
		if arg == nil {
			continue
		}
		switch arg := arg.(type) {
		case JvmArgumentString:
			val := arg.String()
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
				for before, after := range cmdMap {
					a = strings.ReplaceAll(a, fmt.Sprintf("${%s}", before), after)
				}
				jvmArgs = append(jvmArgs, a)
			}
		}
	}

	mcProfile, err := account.GetMinecraftProfile()
	if err != nil {
		return err
	}

	var gameArgs []string
	var gameArgsMap = map[string]string{
		"auth_player_name":      mcProfile.Username,
		"version_name":          clientManifest.ID,
		"game_directory":        inst.Path,
		"assets_root":           filepath.Join(dataDir, "assets"),
		"assets_index_name":     clientManifest.AssetIndex.ID,
		"auth_uuid":             mcProfile.UUID.String(),
		"auth_access_token":     account.AccessToken,
		"clientid":              "launcher",
		"auth_xuid":             mcProfile.UUID.String(),
		"user_type":             "msa",
		"version_type":          clientManifest.Type,
		"resolution_width":      "1280",
		"resolution_height":     "720",
		"quickPlayPath":         "",
		"quickPlayMultiplayer":  "",
		"quickPlayRealms":       "",
		"quickPlaySingleplayer": "",
	}

	for _, arg := range clientManifest.Arguments.Game {
		if arg == nil {
			continue
		}
		switch arg := arg.(type) {
		case GameArgumentString:
			val := arg.String()
			for before, after := range gameArgsMap {
				val = strings.ReplaceAll(val, fmt.Sprintf("${%s}", before), after)
			}
			gameArgs = append(gameArgs, val)
		case GameArgumentRule:
			if !slices.ContainsFunc(arg.Rules, func(rule GameArgumentRuleType) bool {
				if !rule.Action.Allowed() {
					return false
				}
				return false
			}) {
				continue
			}
			for _, a := range arg.Value {
				for before, after := range gameArgsMap {
					a = strings.ReplaceAll(a, fmt.Sprintf("${%s}", before), after)
				}
				gameArgs = append(gameArgs, a)
			}
		}
	}

	config := &LaunchConfig{
		MainClass:     clientManifest.MainClass,
		JVMArguments:  jvmArgs,
		GameArguments: gameArgs,
		Classpath:     []string{classpath}, // BootGameFromConfig handles classpath joining if needed, but here we provide the full string for now
	}

	return BootGameFromConfig(ctx, javaPath, config, clientManifest, inst, account, stdout, stderr)
}

// BootGameFromConfig launches the game using the provided LaunchConfig.
func BootGameFromConfig(ctx context.Context, javaPath string, config *LaunchConfig, clientManifest *ClientManifest, inst *Instance, account *msa.MinecraftAccountAuthResult, stdout, stderr io.Writer) error {
	slog.Info("Booting game from config", "mainClass", config.MainClass)

	mcProfile, err := account.GetMinecraftProfile()
	if err != nil {
		return err
	}

	var gameArgsMap = map[string]string{
		"auth_player_name":      mcProfile.Username,
		"version_name":          clientManifest.ID,
		"game_directory":        inst.Path,
		"assets_root":           filepath.Join(DataDir, "assets"),
		"assets_index_name":     clientManifest.AssetIndex.ID,
		"auth_uuid":             mcProfile.UUID.String(),
		"auth_access_token":     account.AccessToken,
		"clientid":              "launcher",
		"auth_xuid":             mcProfile.UUID.String(),
		"user_type":             "msa",
		"version_type":          clientManifest.Type,
		"resolution_width":      "1280",
		"resolution_height":     "720",
		"quickPlayPath":         "",
		"quickPlayMultiplayer":  "", // TODO: Address ServerAddress later
		"quickPlayRealms":       "",
		"quickPlaySingleplayer": "",
	}

	var resolvedGameArgs []string
	for _, arg := range config.GameArguments {
		val := arg
		for before, after := range gameArgsMap {
			val = strings.ReplaceAll(val, fmt.Sprintf("${%s}", before), after)
		}
		resolvedGameArgs = append(resolvedGameArgs, val)
	}

	var cmds []string
	cmds = append(cmds, javaPath)
	cmds = append(cmds, config.JVMArguments...)

	// Ensure classpath is handled. LaunchConfig.Classpath is a list of JARs.
	// Minecraft expects -cp <joined_classpath>
	if len(config.Classpath) > 0 {
		cp := strings.Join(config.Classpath, string(os.PathListSeparator))
		cmds = append(cmds, "-cp", cp)
	}

	cmds = append(cmds, config.MainClass)
	cmds = append(cmds, resolvedGameArgs...)
	slog.Info("Game command", "cmd", cmds)
	_ = os.MkdirAll(inst.Path, os.ModePerm)

	cmd := exec.CommandContext(ctx, cmds[0], cmds[1:]...)
	cmd.Stdout = io.MultiWriter(stdout, slog.NewLogLogger(slog.Default().Handler(), slog.LevelInfo).Writer())
	cmd.Stderr = io.MultiWriter(stderr, slog.NewLogLogger(slog.Default().Handler(), slog.LevelInfo).Writer())
	cmd.SysProcAttr = runcmd.GetSysProcAttr()
	cmd.Dir = inst.Path

	activity, err := SetActivity(inst, mcProfile)
	if err != nil {
		slog.Error("Failed to set activity", "error", err)
	}

	if err := cmd.Run(); err != nil {
		slog.Error("Failed to run game command", "error", err)
		return err
	}

	if activity != nil {
		if err := ClearActivity(); err != nil {
			slog.Error("Failed to end activity", "error", err)
		}
	}

	return nil
}
