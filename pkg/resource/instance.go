package resource

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

var (
	DataDir = filepath.Join(os.Getenv("APPDATA"), "SabaLauncher")
)

type Upstream struct {
	PackURL     string `json:"pack_url,omitempty"`
	ManifestURL string `json:"manifest_url,omitempty"`
	Version     string `json:"version"`
}

type Instance struct {
	Name     string            `json:"name"`
	UID      uuid.UUID         `json:"uid"`
	Versions []InstanceVersion `json:"versions"`
	Mods     []Mod             `json:"mods"`
	Upstream *Upstream         `json:"upstream,omitempty"`
	
	// Internal runtime fields
	Path string `json:"-"`
}

type InstanceVersion struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type Mod struct {
	Name     string    `json:"name"`
	File     string    `json:"file"`
	Version  string    `json:"version"`
	UpdateAt time.Time `json:"update_at"`
	Source   Source    `json:"source"`
}

type Source interface {
	Type() string
	URL() string
	FileURL() string
}

type ModUnmarshal struct {
	Name     string          `json:"name"`
	File     string          `json:"file"`
	Version  string          `json:"version"`
	UpdateAt time.Time       `json:"update_at"`
	Source   json.RawMessage `json:"source"`
}

func (m *Mod) UnmarshalJSON(data []byte) error {
	var mu ModUnmarshal
	if err := json.Unmarshal(data, &mu); err != nil {
		return err
	}
	m.Name = mu.Name
	m.File = mu.File
	m.Version = mu.Version
	m.UpdateAt = mu.UpdateAt

	if len(mu.Source) > 0 {
		var typeExtract struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(mu.Source, &typeExtract); err != nil {
			return err
		}

		switch typeExtract.Type {
		case "curseforge":
			var c CurseForgeSource
			if err := json.Unmarshal(mu.Source, &c); err != nil {
				return err
			}
			m.Source = &c
		case "modrinth":
			var mr ModrinthSource
			if err := json.Unmarshal(mu.Source, &mr); err != nil {
				return err
			}
			m.Source = &mr
		default:
			// Fallback or generic URL source
			var u URLSource
			if err := json.Unmarshal(mu.Source, &u); err != nil {
				return err
			}
			m.Source = &u
		}
	}

	return nil
}

// Example Implementations of Source

type CurseForgeSource struct {
	ProjectID int `json:"project_id"`
	FileID    int `json:"file_id"`
}

func (c *CurseForgeSource) Type() string { return "curseforge" }
func (c *CurseForgeSource) URL() string {
	return fmt.Sprintf("https://www.curseforge.com/projects/%d", c.ProjectID)
}
func (c *CurseForgeSource) FileURL() string {
	return "" // Needs API resolution
}

type ModrinthSource struct {
	ProjectID string `json:"project_id"`
	VersionID string `json:"version_id"`
}

func (m *ModrinthSource) Type() string { return "modrinth" }
func (m *ModrinthSource) URL() string {
	return fmt.Sprintf("https://modrinth.com/project/%s", m.ProjectID)
}
func (m *ModrinthSource) FileURL() string {
	return "" // Needs API resolution
}

type URLSource struct {
	ModURL  string `json:"url"`
	FileURI string `json:"file_url"`
}

func (u *URLSource) Type() string    { return "url" }
func (u *URLSource) URL() string     { return u.ModURL }
func (u *URLSource) FileURL() string { return u.FileURI }
