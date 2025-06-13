package resource

import (
	"archive/zip"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

func NewState(friendlyName, name string) *SetupState {
	return &SetupState{
		friendlyName: friendlyName,
		name:         name,
		Steps:        []Step{},
		StepCount:    0,
	}
}

type SetupState struct {
	friendlyName string
	name         string
	currentStep  Step
	Steps        []Step
	StepCount    int
	done         bool // Indicates if the setup state is done
	err          error
}

func (s *SetupState) AddStep(step Step) {
	s.Steps = append(s.Steps, step)
	s.StepCount++
}

func (s *SetupState) IsDone() bool {
	return s.done
}

func (s *SetupState) Error() error {
	return s.err // Return the error of the setup state if no step is being processed
}

func (s *SetupState) FriendlyName() string {
	if s.currentStep != nil {
		// If a step is currently being processed, return its friendly name
		return s.currentStep.FriendlyName()
	}
	return s.friendlyName
}

func (s *SetupState) Name() string {
	if s.currentStep != nil {
		// If a step is currently being processed, return its name
		return s.currentStep.Name()
	}
	return s.name
}

func (s *SetupState) Do(ctx *SetupContext) error {
	if s.done {
		return nil // If the setup state is already done, do nothing
	}
	defer func() {
		s.done = true
	}()
	for _, step := range s.Steps {
		if s.currentStep != nil && step.Name() == s.currentStep.Name() {
			// Skip the current step if it's already being processed
			continue
		}
		s.currentStep = step
		if err := step.Do(ctx); err != nil {
			s.err = err
			return err
		}
	}
	s.currentStep = nil // Reset current step after all steps are done
	return nil
}

func (s *SetupState) Progress() float32 {
	if s.StepCount == 0 {
		return 0.0
	}
	var totalProgress float32
	for _, step := range s.Steps {
		totalProgress += step.Progress() * float32(1.0/s.StepCount)
	}
	return totalProgress / float32(s.StepCount)
}

type Step interface {
	FriendlyName() string
	Name() string
	Do(ctx *SetupContext) error
	// Progress returns a value between 0.0 and 1.0 indicating the progress of the step.
	Progress() float32
}

type SetupContext struct {
	dataPath    string
	profilePath string
}

type JavaSetupStep struct {
	manifest *ClientManifest
	worker   *DownloadWorker
	total    int
}

var _ Step = (*JavaSetupStep)(nil)

func (j *JavaSetupStep) FriendlyName() string {
	return "実行環境のセットアップ"
}

func (j *JavaSetupStep) Name() string {
	return "java_setup"
}

func (j *JavaSetupStep) Do(ctx *SetupContext) error {
	worker, err := DownloadJVM(j.manifest, ctx.dataPath)
	if err != nil {
		return err
	}
	j.worker = worker
	j.total = j.worker.Remain()
	if err := j.worker.Run(); err != nil {
		return err
	}
	return nil
}

func (j *JavaSetupStep) Progress() float32 {
	if j.worker == nil {
		return 0.0
	}
	return float32(j.worker.Remain()) / float32(j.total)
}

type ClientDownloadStep struct {
	manifest *ClientManifest
	worker   *DownloadWorker
	total    int
}

var _ Step = (*ClientDownloadStep)(nil)

func (c *ClientDownloadStep) FriendlyName() string {
	return "クライアントのダウンロード"
}

func (c *ClientDownloadStep) Name() string {
	return "client_download"
}

func (c *ClientDownloadStep) Do(ctx *SetupContext) error {
	worker, err := DownloadClientJar(c.manifest, ctx.dataPath)
	if err != nil {
		return err
	}
	c.worker = worker
	c.total = c.worker.Remain()
	if err := c.worker.Run(); err != nil {
		return err
	}
	return nil
}

func (c *ClientDownloadStep) Progress() float32 {
	if c.worker == nil {
		return 0.0
	}
	return float32(c.worker.Remain()) / float32(c.total)
}

type AssetsDownloadStep struct {
	manifest *ClientManifest
	worker   *DownloadWorker
	total    int
}

var _ Step = (*AssetsDownloadStep)(nil)

func (a *AssetsDownloadStep) FriendlyName() string {
	return "アセットのダウンロード"
}

func (a *AssetsDownloadStep) Name() string {
	return "assets_download"
}

func (a *AssetsDownloadStep) Do(ctx *SetupContext) error {
	worker, err := DownloadAssets(a.manifest, ctx.dataPath)
	if err != nil {
		return err
	}
	a.worker = worker
	a.total = a.worker.Remain()
	if err := a.worker.Run(); err != nil {
		return err
	}
	return nil
}

func (a *AssetsDownloadStep) Progress() float32 {
	if a.worker == nil {
		return 0.0
	}
	return float32(a.worker.Remain()) / float32(a.total)
}

type LibraryDownloadStep struct {
	manifest *ClientManifest
	worker   *DownloadWorker
	total    int
}

var _ Step = (*LibraryDownloadStep)(nil)

func (l *LibraryDownloadStep) FriendlyName() string {
	return "ライブラリのダウンロード"
}

func (l *LibraryDownloadStep) Name() string {
	return "library_download"
}

func (l *LibraryDownloadStep) Do(ctx *SetupContext) error {
	worker, err := DownloadLibraries(l.manifest, ctx.dataPath)
	if err != nil {
		return err
	}
	l.worker = worker
	l.total = l.worker.Remain()
	if err := l.worker.Run(); err != nil {
		return err
	}
	return nil
}

func (l *LibraryDownloadStep) Progress() float32 {
	if l.worker == nil {
		return 0.0
	}
	return float32(l.worker.Remain()) / float32(l.total)
}

func NewForgeSetupStep(vanillaVersionName, forgeVersionName string, vanillaManifest *ClientManifest, manifest *ClientManifest) Step {
	state := NewState("Forgeのセットアップ", "forge_setup")
	step := &ForgeDownloadStep{
		vanillaVersionName: vanillaVersionName,
		forgeVersionName:   forgeVersionName,
	}
	state.AddStep(step)
	state.AddStep(&ForgeInstallStep{
		downloadStep:    step,
		vanillaManifest: vanillaManifest,
		manifest:        manifest,
	})
	return state
}

type ForgeDownloadStep struct {
	vanillaVersionName string
	forgeVersionName   string
	installerPath      *string // Path to the downloaded Forge installer
	worker             *DownloadWorker
	total              int
}

var _ Step = (*ForgeDownloadStep)(nil)

func (f *ForgeDownloadStep) FriendlyName() string {
	return "Forgeのセットアップ"
}

func (f *ForgeDownloadStep) Name() string {
	return "forge_setup"
}

func (f *ForgeDownloadStep) Do(ctx *SetupContext) error {
	dir := f.vanillaVersionName + "-forge-" + f.forgeVersionName
	if _, err := os.OpenFile(filepath.Join(ctx.dataPath, "versions", dir, dir+".json"), os.O_RDONLY, 0644); err == nil {
		slog.Info("Forge is already installed", "version", f.forgeVersionName, "path", dir)
		return nil // Forge is already installed, no need to download again
	}
	worker, path, err := DownloadForge(f.vanillaVersionName+"-"+f.forgeVersionName, f.vanillaVersionName+"-forge-"+f.forgeVersionName, ctx.dataPath)
	if err != nil {
		return err
	}
	f.worker = worker
	f.total = f.worker.Remain()
	if err := f.worker.Run(); err != nil {
		return err
	}
	f.installerPath = &path
	return nil
}

func (f *ForgeDownloadStep) Progress() float32 {
	if f.worker == nil {
		return 0.0
	}
	return float32(f.worker.Remain()) / float32(f.total)
}

type ForgeInstallStep struct {
	downloadStep    *ForgeDownloadStep
	done            bool
	vanillaManifest *ClientManifest // The vanilla manifest used for Forge installation
	manifest        *ClientManifest
}

var _ Step = (*ForgeInstallStep)(nil)

func (f *ForgeInstallStep) FriendlyName() string {
	return "Forgeのインストール"
}

func (f *ForgeInstallStep) Name() string {
	return "forge_install"
}

func (f *ForgeInstallStep) Do(ctx *SetupContext) error {
	if f.downloadStep.installerPath != nil {
		err := InstallForge(*f.downloadStep.installerPath, ctx.dataPath)
		if err != nil {
			return err
		}
	}
	dirname := f.downloadStep.vanillaVersionName + "-forge-" + f.downloadStep.forgeVersionName
	file, err := os.OpenFile(filepath.Join(ctx.dataPath, "versions", dirname, dirname+".json"), os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	var manifest ClientManifest
	if err := decoder.Decode(&manifest); err != nil {
		return err
	}
	m, err := f.vanillaManifest.InheritsMerge(&manifest)
	if err != nil {
		return err
	}
	*f.manifest = *m
	return nil
}

func (f *ForgeInstallStep) Progress() float32 {
	if f.done {
		return 1.0
	}
	return 0.0
}

func NewModDownloadStep(zipReader *zip.Reader, oldMods, newMods *modLoader) Step {
	state := NewState("Modのダウンロード", "mod_download")
	state.AddStep(&ModDownloadStep{
		zipReader: zipReader,
		oldMods:   oldMods,
		newMods:   newMods,
	})
	return state
}

type ModDownloadStep struct {
	zipReader *zip.Reader
	oldMods   *modLoader
	newMods   *modLoader
	worker    *DownloadWorker
	total     int
}

var _ Step = (*ModDownloadStep)(nil)

func (m *ModDownloadStep) FriendlyName() string {
	return "Modのダウンロード"
}

func (m *ModDownloadStep) Name() string {
	return "mod_download"
}

func (m *ModDownloadStep) Do(ctx *SetupContext) error {
	worker, err := m.oldMods.loadMod(m.zipReader, m.newMods, ctx.profilePath)
	if err != nil {
		return nil
	}
	m.worker = worker
	m.total = m.worker.Remain()
	if err := m.worker.Run(); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(ctx.profilePath, "manifest.json"))
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(m.newMods); err != nil {
		return err
	}
	return nil
}

func (m *ModDownloadStep) Progress() float32 {
	if m.worker == nil {
		return 0.0
	}
	return float32(m.worker.Remain()) / float32(m.total)
}
