package resource

import (
	"archive/zip"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed __curseforge_api_key.txt
var curseForgeAPIKey string

type modLoader struct {
	Loader
	Mods []ModInstance `json:"mods"`
}

func (m *modLoader) UnmarshalJSON(data []byte) error {
	type Alias modLoader
	aux := &struct {
		*Alias
		Mods []ModInstanceUnmarshal `json:"mods"`
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	m.Mods = make([]ModInstance, len(aux.Mods))
	for i, mod := range aux.Mods {
		aux.Alias.Mods[i] = mod.ModInstance
	}
	return nil
}

func (oldLoader *modLoader) loadMod(packZip *zip.Reader, newLoader *modLoader, profilePath string) (*DownloadWorker, error) {
	oldMods := make(map[string]ModInstance)
	for _, m := range oldLoader.Mods {
		if m.getCurrentModFileName() != nil {
			oldMods[m.key()] = m
		}
	}
	worker := &DownloadWorker{}
	for i := range newLoader.Mods {
		oldMod, ok := oldMods[newLoader.Mods[i].key()]
		if !ok {
			oldMod = nil
			slog.Info("new mod", "key", newLoader.Mods[i].key())
		}
		worker.addTask(func() error {
			if err := newLoader.Mods[i].update(profilePath, oldMod); err != nil {
				return err
			}
			return nil
		})
	}

	if packZip == nil || (newLoader.Override == nil && newLoader.Initialize == nil) {
		slog.Info("no overrides found")
		return worker, nil
	}
	slog.Info("overrides / initializers found")

	if err := newLoader.update(&oldLoader.Loader, packZip, profilePath, worker); err != nil {
		return nil, fmt.Errorf("failed to update loader: %w", err)
	}

	return worker, nil
}

func writeZipFile(name string, file *zip.File) error {
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	if filepath.Ext(name) == ".delete" {
		if err := os.Remove(strings.TrimSuffix(name, ".delete")); os.IsNotExist(err) {
			return nil
		} else if err != nil {
			return err
		}
	}
	f, err := file.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	out, err := os.Create(name)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, f); err != nil {
		return err
	}
	return nil
}

type ModInstance interface {
	update(profilePath string, oldInstance ModInstance) error
	// getModInfo() (name string, version string)
	getCurrentModFileName() *string
	key() string
}

type ModInstanceUnmarshal struct {
	Type        string `json:"type"`
	ModInstance `json:"-"`
}

func (m *ModInstanceUnmarshal) UnmarshalJSON(data []byte) error {
	type Alias ModInstanceUnmarshal
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch m.Type {
	case "curseforge":
		var curseForge CurseForgeModInstance
		if err := json.Unmarshal(data, &curseForge); err != nil {
			return err
		}
		m.ModInstance = &curseForge
	case "modrinth":
		var modrinth ModrinthModInstance
		if err := json.Unmarshal(data, &modrinth); err != nil {
			return err
		}
		m.ModInstance = &modrinth
	default:
		return fmt.Errorf("unknown mod type: %s", m.Type)
	}
	return nil
}

const (
	CurseForgeBaseURL     = "https://api.curseforge.com"
	CurseForgeModFilePath = "/v1/mods/{modId}/files/{fileId}"
)

var _ ModInstance = (*CurseForgeModInstance)(nil)

type CurseForgeModInstance struct {
	ModId           int    `json:"modId"`
	FileId          int    `json:"fileId"`
	CurrentFileName string `json:"currentFileName,omitempty"`
	CurrentFileId   int    `json:"currentFileId,omitempty"`
}

func (c *CurseForgeModInstance) UnmarshalJSON(data []byte) error {
	type Alias CurseForgeModInstance
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if c.ModId == 0 || c.FileId == 0 {
		return errors.New("modId and fileId are required")
	}
	return nil
}

func (c *CurseForgeModInstance) MarshalJSON() ([]byte, error) {
	type Alias CurseForgeModInstance
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "curseforge",
		Alias: (*Alias)(c),
	})
}

func (c *CurseForgeModInstance) key() string {
	return fmt.Sprintf("curseforge:%d", c.ModId)
}

func (c *CurseForgeModInstance) update(profilePath string, oldInstance ModInstance) error {
	if _, ok := oldInstance.(*CurseForgeModInstance); !ok {
		oldInstance = nil
	}
	if oldInstance != nil && oldInstance.(*CurseForgeModInstance).FileId == c.FileId {
		slog.Info("mod file is already up to date", "fileId", c.FileId)
		return nil
	}
	if oldInstance != nil && oldInstance.getCurrentModFileName() != nil {
		slog.Info("removing old mod file", "fileName", *oldInstance.getCurrentModFileName())
		os.Remove(filepath.Join(profilePath, "mods", *oldInstance.getCurrentModFileName()))
	}
	url := strings.ReplaceAll(CurseForgeBaseURL+CurseForgeModFilePath, "{modId}", fmt.Sprintf("%d", c.ModId))
	url = strings.ReplaceAll(url, "{fileId}", fmt.Sprintf("%d", c.FileId))
	httpClient := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("x-api-key", curseForgeAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to get mod file modId: %d, fileId: %d: %w", c.ModId, c.FileId, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get mod file modId: %d, fileId: %d: %s", c.ModId, c.FileId, resp.Status)
	}
	var modFile CurseForgeModFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&modFile); err != nil {
		return fmt.Errorf("failed to decode mod file: %w", err)
	}
	if modFile.Data.FileName == "" {
		return fmt.Errorf("mod file name is empty")
	}

	if modFile.Data.DownloadURL == "" {
		return fmt.Errorf("mod file url is empty")
	}
	if modFile.Data.ID == c.CurrentFileId {
		slog.Info("mod file is already up to date", "fileId", c.CurrentFileId)
		return nil
	}
	slog.Info("downloading mod file", "fileId", modFile.Data.ID, "fileName", modFile.Data.FileName)

	downloadRequest, err := http.NewRequest(http.MethodGet, modFile.Data.DownloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	downloadRequest.Header.Set("x-api-key", curseForgeAPIKey)
	downloadRequest.Header.Set("Accept", "application/json")
	downloadResponse, err := httpClient.Do(downloadRequest)
	if err != nil {
		return fmt.Errorf("failed to get mod file download url: %w", err)
	}
	defer downloadResponse.Body.Close()
	if downloadResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get mod file download url: %s", downloadResponse.Status)
	}

	if err := os.MkdirAll(filepath.Join(profilePath, "mods"), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	out, err := os.Create(filepath.Join(profilePath, "mods", modFile.Data.FileName))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, downloadResponse.Body); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}
	c.CurrentFileName = modFile.Data.FileName
	c.CurrentFileId = modFile.Data.ID
	return nil
}

func (c *CurseForgeModInstance) getCurrentModFileName() *string {
	if c.CurrentFileName == "" {
		return nil
	}
	ptr := func(s string) *string {
		return &s
	}
	return ptr(c.CurrentFileName)
}

type CurseForgeModFileResponse struct {
	Data CurseForgeModFileResponseData `json:"data"`
}

type CurseForgeModFileResponseHashes struct {
	Value string `json:"value"`
	Algo  int    `json:"algo"`
}

type CurseForgeModFileResponseSortableGameVersions struct {
	GameVersionName        string    `json:"gameVersionName"`
	GameVersionPadded      string    `json:"gameVersionPadded"`
	GameVersion            string    `json:"gameVersion"`
	GameVersionReleaseDate time.Time `json:"gameVersionReleaseDate"`
	GameVersionTypeID      int       `json:"gameVersionTypeId"`
}

type CurseForgeModFileResponseDependencies struct {
	ModID        int `json:"modId"`
	RelationType int `json:"relationType"`
}

type CurseForgeModFileResponseModules struct {
	Name        string `json:"name"`
	Fingerprint int    `json:"fingerprint"`
}

type CurseForgeModFileResponseData struct {
	ID                   int                                             `json:"id"`
	GameID               int                                             `json:"gameId"`
	ModID                int                                             `json:"modId"`
	IsAvailable          bool                                            `json:"isAvailable"`
	DisplayName          string                                          `json:"displayName"`
	FileName             string                                          `json:"fileName"`
	ReleaseType          int                                             `json:"releaseType"`
	FileStatus           int                                             `json:"fileStatus"`
	Hashes               []CurseForgeModFileResponseHashes               `json:"hashes"`
	FileDate             time.Time                                       `json:"fileDate"`
	FileLength           int                                             `json:"fileLength"`
	DownloadCount        int                                             `json:"downloadCount"`
	FileSizeOnDisk       int                                             `json:"fileSizeOnDisk"`
	DownloadURL          string                                          `json:"downloadUrl"`
	GameVersions         []string                                        `json:"gameVersions"`
	SortableGameVersions []CurseForgeModFileResponseSortableGameVersions `json:"sortableGameVersions"`
	Dependencies         []CurseForgeModFileResponseDependencies         `json:"dependencies"`
	ExposeAsAlternative  bool                                            `json:"exposeAsAlternative"`
	ParentProjectFileID  int                                             `json:"parentProjectFileId"`
	AlternateFileID      int                                             `json:"alternateFileId"`
	IsServerPack         bool                                            `json:"isServerPack"`
	ServerPackFileID     int                                             `json:"serverPackFileId"`
	IsEarlyAccessContent bool                                            `json:"isEarlyAccessContent"`
	EarlyAccessEndDate   time.Time                                       `json:"earlyAccessEndDate"`
	FileFingerprint      int                                             `json:"fileFingerprint"`
	Modules              []CurseForgeModFileResponseModules              `json:"modules"`
}

const (
	ModrinthBaseURL     = "https://api.modrinth.com/"
	ModrinthModFilePath = "/project/{projectId}/version/{versionId}"
)

type ModrinthModInstance struct {
	ProjectId        string `json:"project_id"`
	VersionId        string `json:"version_id"`
	FileName         string `json:"file_name"`
	CurrentFileName  string `json:"current_file_name,omitempty"`
	CurrentVersionId string `json:"current_version_id,omitempty"`
}

func (m *ModrinthModInstance) UnmarshalJSON(data []byte) error {
	type Alias ModrinthModInstance
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if m.ProjectId == "" || m.VersionId == "" || m.FileName == "" {
		return errors.New("project_id, version_id and file_name are required")
	}
	return nil
}

func (m *ModrinthModInstance) MarshalJSON() ([]byte, error) {
	type Alias ModrinthModInstance
	return json.Marshal(&struct {
		Type string `json:"type"`
		*Alias
	}{
		Type:  "modrinth",
		Alias: (*Alias)(m),
	})
}

func (m *ModrinthModInstance) key() string {
	return fmt.Sprintf("modrinth:%s", m.ProjectId)
}

func (m *ModrinthModInstance) update(profilePath string, oldLoader ModInstance) error {
	if _, ok := oldLoader.(*ModrinthModInstance); !ok {
		oldLoader = nil
	}
	if oldLoader != nil && oldLoader.(*ModrinthModInstance).VersionId == m.VersionId {
		slog.Info("mod file is already up to date", "versionId", m.VersionId)
		return nil
	}
	if oldLoader != nil && oldLoader.getCurrentModFileName() != nil {
		slog.Info("removing old mod file", "fileName", *oldLoader.getCurrentModFileName())
		os.Remove(filepath.Join(profilePath, "mods", *oldLoader.getCurrentModFileName()))
	}
	url := strings.ReplaceAll(ModrinthBaseURL+ModrinthModFilePath, "{projectId}", m.ProjectId)
	url = strings.ReplaceAll(url, "{versionId}", m.VersionId)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to get mod file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get mod file: %s", resp.Status)
	}
	var modFile ModrinthVersionInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&modFile); err != nil {
		return fmt.Errorf("failed to decode mod file: %w", err)
	}

	if modFile.Files == nil {
		return fmt.Errorf("mod file files is empty")
	}
	if modFile.ID == m.CurrentVersionId {
		slog.Info("mod file is already up to date", "versionId", m.CurrentVersionId)
		return nil
	}
	slog.Info("downloading mod file", "versionId", modFile.ID, "fileName", modFile.Name)

	matchedIndex := -1
	for i, file := range modFile.Files {
		if file.FileName == m.FileName {
			matchedIndex = i
			break
		}
	}
	if matchedIndex == -1 {
		return fmt.Errorf("mod file name %s not found in mod file", m.FileName)
	}
	if modFile.Files[matchedIndex].URL == "" {
		return fmt.Errorf("mod file url is empty")
	}
	download, err := http.Get(modFile.Files[matchedIndex].URL)
	if err != nil {
		return fmt.Errorf("failed to get mod file download url: %w", err)
	}
	defer download.Body.Close()
	if download.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get mod file download url: %s", download.Status)
	}
	if err := os.MkdirAll(profilePath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	out, err := os.Create(filepath.Join(profilePath, "mods", modFile.Files[matchedIndex].FileName))
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, download.Body); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}
	m.CurrentFileName = modFile.Files[matchedIndex].FileName
	m.CurrentVersionId = modFile.ID
	return nil
}

func (m *ModrinthModInstance) getCurrentModFileName() *string {
	if m.CurrentFileName == "" {
		return nil
	}
	ptr := func(s string) *string {
		return &s
	}
	return ptr(m.CurrentFileName)
}

type ModrinthVersionInfoResponse struct {
	Name            string                                  `json:"name"`
	VerseionNumber  string                                  `json:"version_number"`
	ChangeLog       string                                  `json:"changelog"`
	Dependencies    []ModrinthVersionInfoResponseDependency `json:"dependencies"`
	GameVersions    []string                                `json:"game_versions"`
	VersionType     ModrinthVersionInfoResponseVersionType  `json:"version_type"`
	Loaders         []string                                `json:"loaders"`
	Featured        bool                                    `json:"featured"`
	Status          string                                  `json:"status"`
	RequestedStatus string                                  `json:"requested_status"`
	ID              string                                  `json:"id"`
	ProjectID       string                                  `json:"project_id"`
	AuthorID        string                                  `json:"author_id"`
	DatePublished   time.Time                               `json:"date_published"`
	Downloads       int                                     `json:"downloads"`
	ChangeLogURL    string                                  `json:"changelog_url"`
	Files           []ModrinthVersionInfoResponseFile       `json:"files"`
}

type ModrinthVersionInfoResponseDependency struct {
	VersionID      string                                    `json:"version_id"`
	ProjectID      string                                    `json:"project_id"`
	FileName       string                                    `json:"file_name"`
	DependencyType ModrinthVersionInfoResponseDependencyType `json:"dependency_type"`
}

type ModrinthVersionInfoResponseDependencyType string

const (
	ModrinthVersionInfoResponseDependencyTypeRequired     ModrinthVersionInfoResponseDependencyType = "required"
	ModrinthVersionInfoResponseDependencyTypeOptional     ModrinthVersionInfoResponseDependencyType = "optional"
	ModrinthVersionInfoResponseDependencyTypeEmbed        ModrinthVersionInfoResponseDependencyType = "embed"
	ModrinthVersionInfoResponseDependencyTypeIncompatible ModrinthVersionInfoResponseDependencyType = "incompatible"
)

type ModrinthVersionInfoResponseVersionType string

const (
	ModrinthVersionInfoResponseVersionTypeRelease ModrinthVersionInfoResponseVersionType = "release"
	ModrinthVersionInfoResponseVersionTypeBeta    ModrinthVersionInfoResponseVersionType = "beta"
	ModrinthVersionInfoResponseVersionTypeAlpha   ModrinthVersionInfoResponseVersionType = "alpha"
)

type ModrinthVersionInfoResponseFile struct {
	Hashes   ModrinthVersionInfoResponseFileHashes `json:"hashes"`
	URL      string                                `json:"url"`
	FileName string                                `json:"filename"`
	Primary  bool                                  `json:"primary"`
	Size     int                                   `json:"size"`
	FileType ModrinthVersionInfoResponseFileType   `json:"file_type"`
}

type ModrinthVersionInfoResponseFileHashes struct {
	SHA1   string `json:"sha1"`
	SHA512 string `json:"sha512"`
}

type ModrinthVersionInfoResponseFileType string

const (
	ModrinthVersionInfoResponseFileTypeRequiredResourcePack ModrinthVersionInfoResponseFileType = "required-resource-pack"
	ModrinthVersionInfoResponseFileTypeOptionalResourcePack ModrinthVersionInfoResponseFileType = "optional-resource-pack"
)
