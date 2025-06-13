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
	"path/filepath"

	"github.com/ikafly144/sabalauncher/pkg/msa"
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

	state *SetupState
	err   error

	manifest *ClientManifest `json:"-"`
}

func (v *VanillaManifestLoader) VersionName() string {
	return v.VersionID
}

func (v *VanillaManifestLoader) StartSetup(dataPath string, profile string) {
	v.state = NewState("Vanillaのセットアップ", "vanilla_setup")
	go func() {
		ver, err := GetVersion(v.VersionID)
		if err != nil {
			slog.Error("Failed to get version", "error", err)
			v.err = err
			return
		}
		m, err := GetClientManifest(ver)
		if err != nil {
			slog.Error("Failed to get client manifest", "error", err)
			v.err = err
			return
		}
		v.manifest = m
		v.state.AddStep(&JavaSetupStep{
			manifest: m,
		})
		v.state.AddStep(&ClientDownloadStep{
			manifest: m,
		})
		v.state.AddStep(&AssetsDownloadStep{
			manifest: m,
		})
		v.state.AddStep(&LibraryDownloadStep{
			manifest: m,
		})
		if err := v.state.Do(&SetupContext{
			dataPath:    dataPath,
			profilePath: profile,
		}); err != nil {
			slog.Error("Failed to run setup state", "error", err)
		}
	}()
}

func (v *VanillaManifestLoader) Boot(dataPath string, profile *Profile, account *msa.MinecraftAccount) error {
	if v.manifest == nil {
		return fmt.Errorf("manifest is not set")
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
	if v.state != nil {
		return v.state.FriendlyName()
	}
	return "初期化中"
}

func (v *VanillaManifestLoader) CurrentProgress() float64 {
	if v.state != nil {
		return float64(v.state.CurrentProgress())
	}
	return 0.0
}

func (v *VanillaManifestLoader) TotalProgress() float64 {
	return v.CurrentProgress()
}

func (v *VanillaManifestLoader) IsDone() bool {
	if v.err != nil {
		return true
	}
	if v.state != nil {
		return v.state.IsDone()
	}
	return false
}

func (v *VanillaManifestLoader) Error() error {
	if v.err != nil {
		return v.err
	}
	if v.state != nil {
		return v.state.Error()
	}
	return nil
}

var _ ManifestLoader = (*ForgeManifestLoader)(nil)

func NewForge(version string, forgeVersion string) (*ForgeManifestLoader, error) {
	f := &ForgeManifestLoader{
		VanillaVersion: version,
		ForgeVersion:   forgeVersion,
	}
	return f, nil
}

type ForgeManifestLoader struct {
	VanillaVersion string     `json:"version"`
	ForgeVersion   string     `json:"forgeVersion"`
	PackURL        string     `json:"packUrl"`
	Pack           *modLoader `json:"pack"`

	state        *SetupState
	bootManifest *ClientManifest
	err          error
}

func (f *ForgeManifestLoader) fullForgeVersion() string {
	return f.VanillaVersion + "-" + f.ForgeVersion
}

func (f *ForgeManifestLoader) forgeVersionDirName() string {
	return f.VanillaVersion + "-forge-" + f.ForgeVersion
}

func (f *ForgeManifestLoader) IsDone() bool {
	return f.state.IsDone()
}

func (f *ForgeManifestLoader) Error() error {
	if f.err != nil {
		return f.err
	}
	if f.state != nil {
		return f.state.Error()
	}
	return nil
}

func (f *ForgeManifestLoader) StartSetup(dataPath string, profilePath string) {
	f.state = NewState("Forgeのセットアップ", "forge_setup")
	go func() {
		ver, err := GetVersion(f.VanillaVersion)
		if err != nil {
			slog.Error("Failed to get version", "error", err)
			f.err = err
			return
		}
		m, err := GetClientManifest(ver)
		if err != nil {
			slog.Error("Failed to get client manifest", "error", err)
			f.err = err
			return
		}

		// マニフェストファイルを開く

		manifestFile, err := os.OpenFile(filepath.Join(profilePath, "manifest.json"), os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			slog.Error("Failed to open manifest file", "error", err)
			f.err = err
			return
		}
		defer manifestFile.Close()
		var oldPack modLoader
		if info, err := manifestFile.Stat(); err == nil && info.Size() == 0 {
			if err := json.NewEncoder(manifestFile).Encode(&oldPack); err != nil {
				slog.Error("Failed to encode mod loader manifest", "error", err)
				f.err = err
				return
			}
			_, _ = manifestFile.Seek(0, 0)
		} else if err != nil {
			slog.Error("Failed to stat manifest file", "error", err)
			f.err = err
			return
		}
		if err := json.NewDecoder(manifestFile).Decode(&oldPack); err != nil {
			slog.Error("Failed to decode mod loader manifest", "error", err)
			f.err = err
			return
		}

		// ZIPリーダーを初期化する

		var zipReader *zip.Reader = nil
		if f.PackURL != "" {
			resp, err := http.Get(f.PackURL)
			if err != nil {
				slog.Error("Failed to download mod pack", "error", err)
				f.err = err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				slog.Error("Failed to download mod pack", "error", resp.Status)
				f.err = fmt.Errorf("failed to download mod pack: %s", resp.Status)
				return
			}
			zipFile, err := os.CreateTemp(os.TempDir(), f.forgeVersionDirName()+"-*.zip")
			if err != nil {
				slog.Error("Failed to create temp zip file", "error", err)
				f.err = err
				return
			}
			if _, err = io.Copy(zipFile, resp.Body); err != nil {
				_ = zipFile.Close()
				slog.Error("Failed to copy response body to zip file", "error", err)
				f.err = err
				return
			}
			if err := zipFile.Close(); err != nil {
				slog.Error("Failed to close zip file", "error", err)
				f.err = err
				return
			}
			r, err := zip.OpenReader(zipFile.Name())
			if err != nil {
				slog.Error("Failed to open zip file", "error", err)
				f.err = err
				return
			}
			defer os.Remove(zipFile.Name())
			defer r.Close()
			zipReader = &r.Reader
		}

		f.state.AddStep(&JavaSetupStep{
			manifest: m,
		})
		var forgeManifest ClientManifest
		f.bootManifest = &forgeManifest
		f.state.AddStep(NewForgeSetupStep(f.VanillaVersion, f.ForgeVersion, m, &forgeManifest))
		f.state.AddStep(&AssetsDownloadStep{
			manifest: &forgeManifest,
		})
		f.state.AddStep(&LibraryDownloadStep{
			manifest: &forgeManifest,
		})
		f.state.AddStep(&ModDownloadStep{
			zipReader: zipReader,
			oldMods:   &oldPack,
			newMods:   f.Pack,
		})
		if err := f.state.Do(&SetupContext{
			dataPath:    dataPath,
			profilePath: profilePath,
		}); err != nil {
			slog.Error("Failed to run setup state", "error", err)
		}
	}()
}

func (f *ForgeManifestLoader) Boot(dataPath string, profile *Profile, account *msa.MinecraftAccount) error {
	if f.bootManifest == nil {
		return fmt.Errorf("boot manifest is not set")
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

	if err := BootGame(f.bootManifest, profile, auth, dataPath); err != nil {
		return err
	}
	f.bootManifest = nil
	return nil
}

func (f *ForgeManifestLoader) CurrentStatus() string {
	return f.state.FriendlyName()
}

func (f *ForgeManifestLoader) CurrentProgress() float64 {
	if f.state != nil {
		return float64(f.state.CurrentProgress())
	}
	return 0.0
}

func (f *ForgeManifestLoader) TotalProgress() float64 {
	if f.state != nil {
		return float64(f.state.Progress())
	}
	return 0.0
}

const forgeDownloadURL = "https://maven.minecraftforge.net/net/minecraftforge/forge/${version}/forge-${version}-installer.jar"

func (f *ForgeManifestLoader) VersionName() string {
	return f.fullForgeVersion()
}
