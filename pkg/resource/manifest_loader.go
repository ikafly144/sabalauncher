package resource

import (
	"archive/zip"
	"context"
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
	Type() string
	VersionName() string
	StartSetup(dataPath string, profilePath string)
	IsDone() bool
	CurrentStatus() string
	CurrentProgress() float64
	TotalProgress() float64
	Error() error
	Boot(dataPath string, profile *Profile, account *msa.MinecraftAccount, stdout, stderr io.Writer) error
	GetClientManifest() *ClientManifest
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
	case "fabric":
		var f FabricManifestLoader
		if err := json.Unmarshal(data, &f); err != nil {
			return err
		}
		m.ManifestLoader = &f
	case "neoforge":
		var n NeoForgeManifestLoader
		if err := json.Unmarshal(data, &n); err != nil {
			return err
		}
		m.ManifestLoader = &n
	case "quilt":
		var q QuiltManifestLoader
		if err := json.Unmarshal(data, &q); err != nil {
			return err
		}
		m.ManifestLoader = &q
	case "custom":
		var c CustomManifestLoader
		if err := json.Unmarshal(data, &c); err != nil {
			return err
		}
		m.ManifestLoader = &c
	default:
		return fmt.Errorf("unknown loader type: %s", m.LoaderType)
	}
	return nil
}

var _ ManifestLoader = (*CustomManifestLoader)(nil)

type CustomManifestLoader struct {
	VanillaManifestLoader
}

func (v *CustomManifestLoader) Type() string {
	return "custom"
}

func (v *CustomManifestLoader) StartSetup(dataPath string, profile string) {
	v.state = NewState("Customのセットアップ", "custom_setup")
	go func() {
		_ = os.MkdirAll(dataPath, 0755)
		_ = os.MkdirAll(profile, 0755)
		m, err := GetLocalClientManifest(dataPath, v.VersionID)
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

func (v *VanillaManifestLoader) Type() string {
	return "vanilla"
}

func (v *VanillaManifestLoader) VersionName() string {
	return v.VersionID
}

func (v *VanillaManifestLoader) StartSetup(dataPath string, profile string) {
	v.state = NewState("Vanillaのセットアップ", "vanilla_setup")
	go func() {
		_ = os.MkdirAll(dataPath, 0755)
		_ = os.MkdirAll(profile, 0755)
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

func (v *VanillaManifestLoader) Boot(dataPath string, profile *Profile, account *msa.MinecraftAccount, stdout, stderr io.Writer) error {
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

	if err := BootGame(v.manifest, profile, auth, dataPath, stdout, stderr); err != nil {
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
	return 1.0
}

func (v *VanillaManifestLoader) TotalProgress() float64 {
	if v.state != nil {
		return float64(v.state.Progress())
	}
	return 0.0
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

func (f *ForgeManifestLoader) Type() string {
	return "forge"
}

func (f *ForgeManifestLoader) fullForgeVersion() string {
	return f.VanillaVersion + "-" + f.ForgeVersion
}

func (f *ForgeManifestLoader) forgeVersionDirName() string {
	return f.VanillaVersion + "-forge-" + f.ForgeVersion
}

func (f *ForgeManifestLoader) IsDone() bool {
	if f.err != nil {
		return true
	}
	if f.state != nil {
		return f.state.IsDone()
	}
	return false
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
		_ = os.MkdirAll(dataPath, 0755)
		_ = os.MkdirAll(profilePath, 0755)
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

func (f *ForgeManifestLoader) Boot(dataPath string, profile *Profile, account *msa.MinecraftAccount, stdout, stderr io.Writer) error {
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

	if err := BootGame(f.bootManifest, profile, auth, dataPath, stdout, stderr); err != nil {
		return err
	}
	f.bootManifest = nil
	return nil
}

func (f *ForgeManifestLoader) CurrentStatus() string {
	if f.state != nil {
		return f.state.FriendlyName()
	}
	return "初期化中"
}

func (f *ForgeManifestLoader) CurrentProgress() float64 {
	if f.state != nil {
		return float64(f.state.CurrentProgress())
	}
	return 1.0
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

func (v *VanillaManifestLoader) GetClientManifest() *ClientManifest {
	return v.manifest
}

func (f *ForgeManifestLoader) GetClientManifest() *ClientManifest {
	return f.bootManifest
}

func (c *CustomManifestLoader) GetClientManifest() *ClientManifest {
	return c.manifest
}

type FabricManifestLoader struct {
	VanillaVersion string     `json:"version"`
	LoaderVersion  string     `json:"loaderVersion"`
	PackURL        string     `json:"packUrl"`
	Pack           *modLoader `json:"pack"`

	state    *SetupState
	manifest *ClientManifest
	err      error
}

func (f *FabricManifestLoader) Type() string {
	return "fabric"
}

func (f *FabricManifestLoader) VersionName() string {
	return f.VanillaVersion + "-fabric-" + f.LoaderVersion
}

func (f *FabricManifestLoader) StartSetup(dataPath string, profilePath string) {
	f.state = NewState("Fabricのセットアップ", "fabric_setup")
	go func() {
		_ = os.MkdirAll(dataPath, 0755)
		_ = os.MkdirAll(profilePath, 0755)
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
		f.manifest = m

		f.state.AddStep(&JavaSetupStep{
			manifest: m,
		})
		f.state.AddStep(&ClientDownloadStep{
			manifest: m,
		})
		f.state.AddStep(&AssetsDownloadStep{
			manifest: m,
		})
		f.state.AddStep(&LibraryDownloadStep{
			manifest: m,
		})

		// Fabric specific installation (Loader and Libraries)
		fabricLoader := NewFabricLoader(f.VanillaVersion, f.LoaderVersion)
		f.state.AddStep(&FabricSetupStep{
			loader: fabricLoader,
		})

		if err := f.state.Do(&SetupContext{
			dataPath:    dataPath,
			profilePath: profilePath,
		}); err != nil {
			slog.Error("Failed to run setup state", "error", err)
		}
	}()
}

func (f *FabricManifestLoader) IsDone() bool {
	if f.err != nil {
		return true
	}
	if f.state != nil {
		return f.state.IsDone()
	}
	return false
}

func (f *FabricManifestLoader) CurrentStatus() string {
	if f.state != nil {
		return f.state.FriendlyName()
	}
	return "初期化中"
}

func (f *FabricManifestLoader) CurrentProgress() float64 {
	if f.state != nil {
		return float64(f.state.CurrentProgress())
	}
	return 1.0
}

func (f *FabricManifestLoader) TotalProgress() float64 {
	if f.state != nil {
		return float64(f.state.Progress())
	}
	return 0.0
}

func (f *FabricManifestLoader) Error() error {
	if f.err != nil {
		return f.err
	}
	if f.state != nil {
		return f.state.Error()
	}
	return nil
}

func (f *FabricManifestLoader) Boot(dataPath string, profile *Profile, account *msa.MinecraftAccount, stdout, stderr io.Writer) error {
	// Legacy boot logic, now handled by GameRunner using ModLoader
	return nil
}

func (f *FabricManifestLoader) GetClientManifest() *ClientManifest {
	return f.manifest
}

type FabricSetupStep struct {
	loader *FabricLoader
}

func (s *FabricSetupStep) FriendlyName() string {
	return "Fabricのインストール"
}

func (s *FabricSetupStep) Name() string {
	return "fabric_setup"
}

func (s *FabricSetupStep) Progress() float32 {
	return 0.0
}

func (s *FabricSetupStep) Do(ctx *SetupContext) error {
	profile := &Profile{
		Path: ctx.profilePath,
	}
	return s.loader.Install(context.Background(), profile)
}

type NeoForgeManifestLoader struct {
	VanillaVersion  string     `json:"version"`
	NeoForgeVersion string     `json:"neoforgeVersion"`
	PackURL         string     `json:"packUrl"`
	Pack            *modLoader `json:"pack"`

	state    *SetupState
	manifest *ClientManifest
	err      error
}

func (n *NeoForgeManifestLoader) Type() string {
	return "neoforge"
}

func (n *NeoForgeManifestLoader) VersionName() string {
	return n.VanillaVersion + "-neoforge-" + n.NeoForgeVersion
}

func (n *NeoForgeManifestLoader) StartSetup(dataPath string, profilePath string) {
	n.state = NewState("NeoForgeのセットアップ", "neoforge_setup")
	go func() {
		_ = os.MkdirAll(dataPath, 0755)
		_ = os.MkdirAll(profilePath, 0755)
		ver, err := GetVersion(n.VanillaVersion)
		if err != nil {
			slog.Error("Failed to get version", "error", err)
			n.err = err
			return
		}
		m, err := GetClientManifest(ver)
		if err != nil {
			slog.Error("Failed to get client manifest", "error", err)
			n.err = err
			return
		}
		n.manifest = m

		n.state.AddStep(&JavaSetupStep{
			manifest: m,
		})
		n.state.AddStep(&ClientDownloadStep{
			manifest: m,
		})
		n.state.AddStep(&AssetsDownloadStep{
			manifest: m,
		})
		n.state.AddStep(&LibraryDownloadStep{
			manifest: m,
		})

		// NeoForge specific installation
		neoforgeLoader := NewNeoForgeLoader(n.VanillaVersion, n.NeoForgeVersion)
		n.state.AddStep(&NeoForgeSetupStep{
			loader: neoforgeLoader,
		})

		if err := n.state.Do(&SetupContext{
			dataPath:    dataPath,
			profilePath: profilePath,
		}); err != nil {
			slog.Error("Failed to run setup state", "error", err)
		}
	}()
}

func (n *NeoForgeManifestLoader) IsDone() bool {
	if n.err != nil {
		return true
	}
	if n.state != nil {
		return n.state.IsDone()
	}
	return false
}

func (n *NeoForgeManifestLoader) CurrentStatus() string {
	if n.state != nil {
		return n.state.FriendlyName()
	}
	return "初期化中"
}

func (n *NeoForgeManifestLoader) CurrentProgress() float64 {
	if n.state != nil {
		return float64(n.state.CurrentProgress())
	}
	return 1.0
}

func (n *NeoForgeManifestLoader) TotalProgress() float64 {
	if n.state != nil {
		return float64(n.state.Progress())
	}
	return 0.0
}

func (n *NeoForgeManifestLoader) Error() error {
	if n.err != nil {
		return n.err
	}
	if n.state != nil {
		return n.state.Error()
	}
	return nil
}

func (n *NeoForgeManifestLoader) Boot(dataPath string, profile *Profile, account *msa.MinecraftAccount, stdout, stderr io.Writer) error {
	return nil
}

func (n *NeoForgeManifestLoader) GetClientManifest() *ClientManifest {
	return n.manifest
}

type NeoForgeSetupStep struct {
	loader *NeoForgeLoader
}

func (s *NeoForgeSetupStep) FriendlyName() string {
	return "NeoForgeのインストール"
}

func (s *NeoForgeSetupStep) Name() string {
	return "neoforge_setup"
}

func (s *NeoForgeSetupStep) Progress() float32 {
	return 0.0
}

func (s *NeoForgeSetupStep) Do(ctx *SetupContext) error {
	profile := &Profile{
		Path: ctx.profilePath,
	}
	return s.loader.Install(context.Background(), profile)
}

type QuiltManifestLoader struct {
	VanillaVersion string     `json:"version"`
	LoaderVersion  string     `json:"loaderVersion"`
	PackURL        string     `json:"packUrl"`
	Pack           *modLoader `json:"pack"`

	state    *SetupState
	manifest *ClientManifest
	err      error
}

func (q *QuiltManifestLoader) Type() string {
	return "quilt"
}

func (q *QuiltManifestLoader) VersionName() string {
	return q.VanillaVersion + "-quilt-" + q.LoaderVersion
}

func (q *QuiltManifestLoader) StartSetup(dataPath string, profilePath string) {
	q.state = NewState("Quiltのセットアップ", "quilt_setup")
	go func() {
		_ = os.MkdirAll(dataPath, 0755)
		_ = os.MkdirAll(profilePath, 0755)
		ver, err := GetVersion(q.VanillaVersion)
		if err != nil {
			slog.Error("Failed to get version", "error", err)
			q.err = err
			return
		}
		m, err := GetClientManifest(ver)
		if err != nil {
			slog.Error("Failed to get client manifest", "error", err)
			q.err = err
			return
		}
		q.manifest = m

		q.state.AddStep(&JavaSetupStep{
			manifest: m,
		})
		q.state.AddStep(&ClientDownloadStep{
			manifest: m,
		})
		q.state.AddStep(&AssetsDownloadStep{
			manifest: m,
		})
		q.state.AddStep(&LibraryDownloadStep{
			manifest: m,
		})

		// Quilt specific installation
		quiltLoader := NewQuiltLoader(q.VanillaVersion, q.LoaderVersion)
		q.state.AddStep(&QuiltSetupStep{
			loader: quiltLoader,
		})

		if err := q.state.Do(&SetupContext{
			dataPath:    dataPath,
			profilePath: profilePath,
		}); err != nil {
			slog.Error("Failed to run setup state", "error", err)
		}
	}()
}

func (q *QuiltManifestLoader) IsDone() bool {
	if q.err != nil {
		return true
	}
	if q.state != nil {
		return q.state.IsDone()
	}
	return false
}

func (q *QuiltManifestLoader) CurrentStatus() string {
	if q.state != nil {
		return q.state.FriendlyName()
	}
	return "初期化中"
}

func (q *QuiltManifestLoader) CurrentProgress() float64 {
	if q.state != nil {
		return float64(q.state.CurrentProgress())
	}
	return 1.0
}

func (q *QuiltManifestLoader) TotalProgress() float64 {
	if q.state != nil {
		return float64(q.state.Progress())
	}
	return 0.0
}

func (q *QuiltManifestLoader) Error() error {
	if q.err != nil {
		return q.err
	}
	if q.state != nil {
		return q.state.Error()
	}
	return nil
}

func (q *QuiltManifestLoader) Boot(dataPath string, profile *Profile, account *msa.MinecraftAccount, stdout, stderr io.Writer) error {
	return nil
}

func (q *QuiltManifestLoader) GetClientManifest() *ClientManifest {
	return q.manifest
}

type QuiltSetupStep struct {
	loader *QuiltLoader
}

func (s *QuiltSetupStep) FriendlyName() string {
	return "Quiltのインストール"
}

func (s *QuiltSetupStep) Name() string {
	return "quilt_setup"
}

func (s *QuiltSetupStep) Progress() float32 {
	return 0.0
}

func (s *QuiltSetupStep) Do(ctx *SetupContext) error {
	profile := &Profile{
		Path: ctx.profilePath,
	}
	return s.loader.Install(context.Background(), profile)
}
