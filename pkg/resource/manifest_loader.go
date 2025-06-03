package resource

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ikafly144/sabalauncher/pkg/msa"
	"github.com/ikafly144/sabalauncher/pkg/runcmd"
)

const (
	maxProcCount = 8
)

type ManifestSetupPhase int

type ManifestLoader interface {
	VersionName() string
	StartSetup(dataPath string, profilePath string)
	IsDone() bool
	CurrentStatus() string
	CurrentProgress() float64
	TotalProgress() float64
	Error() error
	Boot(dataPath string, profile *Profile, account *msa.MinecraftAccount) error
}

type ManifestLoaderUnmarshal struct {
	LoaderType string `json:"loaderType"`
	ManifestLoader
}

func (m *ManifestLoaderUnmarshal) UnmarshalJSON(data []byte) error {
	type raw struct {
		LoaderType string `json:"loaderType"`
	}
	var r raw
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	m.LoaderType = r.LoaderType
	switch m.LoaderType {
	case "vanilla":
		var v VanillaManifestLoader
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		m.ManifestLoader = &v
	case "forge":
		var f ForgeManifestLoader
		if err := json.Unmarshal(data, &f); err != nil {
			return err
		}
		m.ManifestLoader = &f
	default:
		return fmt.Errorf("unknown loader type: %s", m.LoaderType)
	}
	return nil
}

var _ ManifestLoader = (*VanillaManifestLoader)(nil)

func NewVanilla(version string) (*VanillaManifestLoader, error) {
	return &VanillaManifestLoader{
		VersionID: version,
	}, nil
}

type VanillaManifestLoader struct {
	VersionID string `json:"version"`
	version   *Version
	manifest  *ClientManifest

	status              int
	currentStatus       string
	currentGoal         int
	currentProgress     int
	currentProgressFunc func() int
	done                bool
	javaPath            string
	err                 error
}

func (v *VanillaManifestLoader) StartSetup(dataPath string, profile string) {
	v.err = nil
	v.done = false
	v.currentProgressFunc = nil
	go func() {
		if err := v.setup(dataPath); err != nil {
			slog.Error("Failed to setup", "error", err)
			v.err = err
		}
		v.done = true
	}()
}

func (v *VanillaManifestLoader) setup(dataPath string) error {
	v.currentGoal = 2
	v.currentProgress = 0
	v.currentStatus = "情報を取得中"
	if v.version == nil {
		ver, err := GetVersion(v.VersionID)
		if err != nil {
			return err
		}
		v.version = ver
	}
	v.currentProgress = 1
	v.status++
	if v.manifest == nil {
		m, err := GetClientManifest(v.version)
		if err != nil {
			return err
		}
		v.manifest = m
	}
	v.currentProgress = 2
	v.status++

	v.currentStatus = "実行環境のセットアップ中"
	v.currentGoal = 1
	v.currentProgress = 0
	worker, err := v.setupJava("C:\\")
	if err != nil {
		return err
	}
	for range maxProcCount {
		go func() {
			if err := worker.Run(); err != nil {
				slog.Error("Unexpected error", "error", err)
				return
			}
		}()
	}
	worker.Wait()
	v.currentProgress = 1
	v.status++

	if v.javaPath, err = GetJavaExecutablePath(v.manifest.JavaVersion.Component, "C:\\"); err != nil {
		return err
	}

	v.currentStatus = "クライアントのダウンロード中"
	v.currentGoal = 1
	worker, err = v.downloadJar(v.manifest, dataPath)
	if err != nil {
		return err
	}
	for range maxProcCount {
		go func() {
			if err := worker.Run(); err != nil {
				slog.Error("Unexpected error", "error", err)
				return
			}
		}()
	}
	worker.Wait()
	v.currentProgress = 1
	v.status++

	worker, err = v.downloadAssets(v.manifest, dataPath)
	if err != nil {
		return err
	}
	v.currentStatus = "アセットのダウンロード中"
	v.currentGoal = worker.Remain()
	v.currentProgressFunc = worker.Remain
	for range maxProcCount {
		go func() {
			if err := worker.Run(); err != nil {
				slog.Error("Unexpected error", "error", err)
				return
			}
		}()
	}
	worker.Wait()
	v.status++

	worker, err = v.downloadLibraries(v.manifest, dataPath)
	if err != nil {
		return err
	}
	v.currentStatus = "ライブラリのダウンロード中"
	v.currentGoal = worker.Remain()
	v.currentProgressFunc = worker.Remain
	for range maxProcCount {
		go func() {
			if err := worker.Run(); err != nil {
				slog.Error("Unexpected error", "error", err)
				return
			}
		}()
	}
	worker.Wait()
	v.status++

	return nil
}

func (v *VanillaManifestLoader) Boot(dataPath string, profile *Profile, account *msa.MinecraftAccount) error {
	if v.javaPath == "" {
		return fmt.Errorf("java path is not set")
	}
	if v.manifest == nil {
		return fmt.Errorf("manifest is not set")
	}
	if v.version == nil {
		return fmt.Errorf("version is not set")
	}
	if account == nil {
		return fmt.Errorf("account is not set")
	}

	auth, err := account.GetMinecraftAccount()
	if err != nil {
		slog.Info("Failed to get Minecraft account", "error", err)
		return errors.New("マイクロソフトアカウントの認証に失敗しました（再ログインしてください）")
	}
	if auth == nil {
		return fmt.Errorf("account is not set")
	}

	if err := BootGame(v.manifest, profile, auth, dataPath); err != nil {
		return err
	}

	return nil
}

func (v *VanillaManifestLoader) CurrentStatus() string {
	return v.currentStatus
}

func (v *VanillaManifestLoader) CurrentProgress() float64 {
	if v.currentProgressFunc != nil {
		v.currentProgress = v.currentGoal - v.currentProgressFunc()
	}
	if v.currentGoal == 0 {
		return 1.0
	}
	return float64(v.currentProgress) / float64(v.currentGoal)
}

func (v *VanillaManifestLoader) TotalProgress() float64 {
	return float64(v.status) / float64(6)
}

func (v *VanillaManifestLoader) setupJava(dataPath string) (*DownloadWorker, error) {
	worker, err := DownloadJVM(v.manifest, dataPath)
	if err != nil {
		return nil, err
	}
	return worker, nil
}

func (v *VanillaManifestLoader) IsDone() bool {
	return v.done
}

func (v *VanillaManifestLoader) Error() error {
	if v.err != nil {
		return v.err
	}
	return nil
}

func (v *VanillaManifestLoader) downloadJar(manifest *ClientManifest, dataPath string) (*DownloadWorker, error) {
	return DownloadClientJar(manifest, dataPath)
}

func (v *VanillaManifestLoader) downloadAssets(manifest *ClientManifest, dataPath string) (*DownloadWorker, error) {
	return DownloadAssets(manifest, dataPath)
}

func (v *VanillaManifestLoader) downloadLibraries(manifest *ClientManifest, dataPath string) (*DownloadWorker, error) {
	return DownloadLibraries(manifest, dataPath)
}

func (v *VanillaManifestLoader) VersionName() string {
	return v.VersionID
}

var _ ManifestLoader = (*ForgeManifestLoader)(nil)

func NewForge(version string, forgeVersion string) (*ForgeManifestLoader, error) {
	v, err := NewVanilla(version)
	if err != nil {
		return nil, err
	}
	f := &ForgeManifestLoader{
		v:            *v,
		ForgeVersion: forgeVersion,
	}
	return f, nil
}

type ForgeManifestLoader struct {
	v                VanillaManifestLoader
	VanillaVersion   string     `json:"version"`
	ForgeVersion     string     `json:"forgeVersion"`
	PackURL          string     `json:"packUrl"`
	Pack             *modLoader `json:"pack"`
	bootManifest     *ClientManifest
	installerJarPath string
	err              error
}

func (f *ForgeManifestLoader) fullForgeVersion() string {
	return f.VanillaVersion + "-" + f.ForgeVersion
}

func (f *ForgeManifestLoader) forgeVersionDirName() string {
	return f.VanillaVersion + "-forge-" + f.ForgeVersion
}

func (f *ForgeManifestLoader) IsDone() bool {
	return f.v.done
}

func (f *ForgeManifestLoader) Error() error {
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *ForgeManifestLoader) StartSetup(dataPath string, profilePath string) {
	f.v.done = false
	f.v.currentProgressFunc = nil
	f.err = nil
	go func() {
		if err := f.setup(dataPath, profilePath); err != nil {
			slog.Error("Failed to setup", "error", err)
			f.err = err
		}
		f.v.done = true
	}()
}

func (f *ForgeManifestLoader) setup(dataPath string, profilePath string) error {
	v := &f.v
	v.currentGoal = 2
	v.currentProgress = 0
	v.currentStatus = "情報を取得中"
	if v.version == nil {
		ver, err := GetVersion(f.VanillaVersion)
		if err != nil {
			return err
		}
		v.version = ver
	}
	v.currentProgress = 1
	v.status++
	if v.manifest == nil {
		m, err := GetClientManifest(v.version)
		if err != nil {
			return err
		}
		v.manifest = m
	}
	v.currentProgress = 2
	v.status++

	v.currentStatus = "実行環境のセットアップ中"
	v.currentGoal = 1
	v.currentProgress = 0
	worker, err := v.setupJava("C:\\")
	if err != nil {
		return err
	}
	for range maxProcCount {
		go func() {
			if err := worker.Run(); err != nil {
				slog.Error("Unexpected error", "error", err)
				return
			}
		}()
	}
	worker.Wait()
	v.currentProgress = 1
	v.status++

	if v.javaPath, err = GetJavaExecutablePath(v.manifest.JavaVersion.Component, "C:\\"); err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(dataPath, "versions", f.forgeVersionDirName(), f.forgeVersionDirName()+".json")); os.IsNotExist(err) {
		worker, err = f.downloadInstaller(dataPath)
		if err != nil {
			return err
		}
		v.currentStatus = "Forgeインストーラーのダウンロード中"
		v.currentGoal = worker.Remain()
		v.currentProgressFunc = worker.Remain
		for range maxProcCount {
			go func() {
				if err := worker.Run(); err != nil {
					slog.Error("Unexpected error", "error", err)
					return
				}
			}()
		}
		worker.Wait()
		v.status++

		v.currentStatus = "Forgeインストーラーの実行中"
		v.currentGoal = 1
		v.currentProgress = 0
		v.currentProgressFunc = nil
		if err := f.installForge(dataPath); err != nil {
			return err
		}
		v.currentProgress = 1
		v.status++
	} else if err != nil {
		return err
	} else {
		slog.Info("Forge installer already exists", "path", filepath.Join(dataPath, "versions", f.forgeVersionDirName(), f.forgeVersionDirName()+".json"))
		v.currentStatus = "Forgeインストーラーの実行済み"
		v.currentGoal = 0
		v.status += 2
	}

	v.currentStatus = "Forgeマニフェストの取得中"
	v.currentGoal = 1
	v.currentProgress = 0
	v.currentProgressFunc = nil
	manifest, err := f.getForgeManifest(dataPath)
	if err != nil {
		return err
	}
	v.currentProgress = 1
	v.status++
	f.bootManifest = manifest

	v.currentStatus = "アセットのダウンロード中"
	worker, err = v.downloadAssets(manifest, dataPath)
	if err != nil {
		return err
	}
	v.currentGoal = worker.Remain()
	v.currentProgressFunc = worker.Remain
	for range maxProcCount {
		go func() {
			if err := worker.Run(); err != nil {
				slog.Error("Unexpected error", "error", err)
				return
			}
		}()
	}
	worker.Wait()
	v.status++

	worker, err = v.downloadLibraries(manifest, dataPath)
	if err != nil {
		return err
	}
	v.currentStatus = "ライブラリのダウンロード中"
	v.currentGoal = worker.Remain()
	v.currentProgressFunc = worker.Remain
	for range maxProcCount {
		go func() {
			if err := worker.Run(); err != nil {
				slog.Error("Unexpected error", "error", err)
				return
			}
		}()
	}
	worker.Wait()
	v.status++

	if f.PackURL != "" || f.Pack != nil {
		v.currentStatus = "Modのダウンロード中"

		if err := os.MkdirAll(profilePath, 0755); err != nil {
			return err
		}

		manifestFile, err := os.OpenFile(filepath.Join(profilePath, "manifest.json"), os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer manifestFile.Close()
		var modLoaderManifest modLoader
		if info, err := manifestFile.Stat(); err == nil && info.Size() == 0 {
			if err := json.NewEncoder(manifestFile).Encode(&modLoaderManifest); err != nil {
				return err
			}
			_, _ = manifestFile.Seek(0, 0)
		} else if err != nil {
			return err
		}
		if err := json.NewDecoder(manifestFile).Decode(&modLoaderManifest); err != nil {
			return err
		}
		var zipReader *zip.Reader = nil
		var manifest modLoader
		if f.Pack != nil {
			manifest = *f.Pack
		} else if f.PackURL != "" {
			resp, err := http.Get(f.PackURL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to download mod pack: %s", resp.Status)
			}

			if filepath.Ext(f.PackURL) == ".zip" {
				zipFile, err := os.CreateTemp(os.TempDir(), f.forgeVersionDirName()+"-*.zip")
				if err != nil {
					return err
				}
				if _, err = io.Copy(zipFile, resp.Body); err != nil {
					_ = zipFile.Close()
					return err
				}
				if err := zipFile.Close(); err != nil {
					return err
				}
				r, err := zip.OpenReader(zipFile.Name())
				if err != nil {
					return err
				}
				defer os.Remove(zipFile.Name())
				defer r.Close()
				zipReader = &r.Reader
				manifestZip, err := r.Open("manifest.json")
				if err != nil {
					return err
				}
				defer manifestZip.Close()
				if err := json.NewDecoder(manifestZip).Decode(&manifest); err != nil {
					return err
				}
			} else {
				manifestResp, err := http.Get(f.PackURL)
				if err != nil {
					return err
				}
				defer manifestResp.Body.Close()
				if manifestResp.StatusCode != http.StatusOK {
					return fmt.Errorf("failed to download mod pack: %s", manifestResp.Status)
				}
				if err := json.NewDecoder(manifestResp.Body).Decode(&manifest); err != nil {
					return err
				}
			}
		} else {
			return fmt.Errorf("mod pack url is not set")
		}
		ov, err := modLoaderManifest.loadMod(zipReader, &manifest, profilePath)
		if err != nil {
			return err
		}

		f.v.currentStatus = "Modのインストール中"
		f.v.currentGoal = ov.Remain()
		f.v.currentProgressFunc = ov.Remain
		for range maxProcCount {
			go func() {
				if err := ov.Run(); err != nil {
					slog.Error("Unexpected error", "error", err)
					return
				}
			}()
		}
		if err := ov.Wait(); err != nil {
			return err
		}

		_, _ = manifestFile.Seek(0, 0)
		_ = manifestFile.Truncate(0)
		if err := json.NewEncoder(manifestFile).Encode(manifest); err != nil {
			return err
		}
	}

	return nil
}

func (f *ForgeManifestLoader) Boot(dataPath string, profile *Profile, account *msa.MinecraftAccount) error {
	if f.v.javaPath == "" {
		return fmt.Errorf("java path is not set")
	}
	if f.v.manifest == nil {
		return fmt.Errorf("manifest is not set")
	}
	if f.v.version == nil {
		return fmt.Errorf("version is not set")
	}
	if account == nil {
		return fmt.Errorf("account is not set")
	}
	if f.bootManifest == nil {
		return fmt.Errorf("boot manifest is not set")
	}
	auth, err := account.GetMinecraftAccount()
	if err != nil {
		slog.Info("Failed to get Minecraft account", "error", err)
		return errors.New("マイクロソフトアカウントの認証に失敗しました（再ログインしてください）")
	}
	if auth == nil {
		return fmt.Errorf("account is not set")
	}
	if err := BootGame(f.bootManifest, profile, auth, dataPath); err != nil {
		return err
	}
	f.bootManifest = nil
	return nil
}

func (f *ForgeManifestLoader) CurrentStatus() string {
	return f.v.currentStatus
}

func (f *ForgeManifestLoader) CurrentProgress() float64 {
	if f.v.currentProgressFunc != nil {
		f.v.currentProgress = f.v.currentGoal - f.v.currentProgressFunc()
	}
	if f.v.currentGoal == 0 {
		return 1.0
	}
	return float64(f.v.currentProgress) / float64(f.v.currentGoal)
}

func (f *ForgeManifestLoader) TotalProgress() float64 {
	return float64(f.v.status) / float64(7)
}

const forgeDownloadURL = "https://maven.minecraftforge.net/net/minecraftforge/forge/${version}/forge-${version}-installer.jar"

func (f *ForgeManifestLoader) downloadInstaller(dataPath string) (*DownloadWorker, error) {
	var worker DownloadWorker
	worker.addTask(func() error {
		httpClient := &http.Client{}
		url := forgeDownloadURL
		url = strings.ReplaceAll(url, "${version}", f.fullForgeVersion())
		resp, err := httpClient.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to download forge jar: %s", resp.Status)
		}

		tmpPath := os.TempDir()
		tmpFile, err := os.CreateTemp(tmpPath, f.forgeVersionDirName()+"-*.jar")
		if err != nil {
			return err
		}
		defer tmpFile.Close()
		_, err = io.Copy(tmpFile, resp.Body)
		if err != nil {
			return err
		}
		f.installerJarPath = tmpFile.Name()
		slog.Info("Forge installer jar downloaded", "path", f.installerJarPath)
		return nil
	})

	return &worker, nil
}

func (f *ForgeManifestLoader) installForge(dataPath string) error {
	if f.installerJarPath == "" {
		return fmt.Errorf("installer jar path is not set")
	}
	profiles, err := os.Create(filepath.Join(dataPath, "launcher_profiles.json"))
	if err != nil {
		return err
	}
	defer os.Remove(profiles.Name())
	defer profiles.Close()
	_, err = profiles.WriteString("{\"profiles\":{}}")
	if err != nil {
		return err
	}
	cmd := exec.Command("java", "-jar", f.installerJarPath, "--installClient", dataPath)
	cmd.Dir = filepath.Join(filepath.Dir(f.installerJarPath))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = runcmd.GetSysProcAttr()
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (f *ForgeManifestLoader) getForgeManifest(dataPath string) (*ClientManifest, error) {
	file, err := os.OpenFile(filepath.Join(dataPath, "versions", f.forgeVersionDirName(), f.forgeVersionDirName()+".json"), os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	var manifest *ClientManifest = &ClientManifest{}
	if err := decoder.Decode(manifest); err != nil {
		return nil, err
	}

	manifest, err = f.v.manifest.InheritsMerge(manifest)
	if err != nil {
		return nil, err
	}

	return manifest, nil
}

func (f *ForgeManifestLoader) VersionName() string {
	return f.fullForgeVersion()
}
