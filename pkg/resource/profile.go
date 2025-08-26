package resource

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gioui.org/op/paint"
	"gioui.org/widget"
	sigar "github.com/cloudfoundry/gosigar"
)

var (
	DataDir = filepath.Join(os.Getenv("APPDATA"), "SabaLauncher")
)

const (
	CurrentProfileVersion = 1
)

type PublicProfile struct {
	Name                string         `json:"name"`
	DisplayName         string         `json:"display_name,omitempty"`
	Description         string         `json:"description,omitempty"`
	Icon                string         `json:"icon,omitempty"`
	ServerAddress       string         `json:"server_address,omitempty"`
	Manifest            ManifestLoader `json:"manifest"`
	RecommendedMemoryMB uint64         `json:"recommended_memory_mb,omitempty"`
	Version             int            `json:"version"`
}

func (p *PublicProfile) UnmarshalJSON(data []byte) error {
	type Alias PublicProfile
	aux := &struct {
		*Alias
		Manifest ManifestLoaderUnmarshal `json:"manifest"`
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if p.Version != CurrentProfileVersion {
		return fmt.Errorf("unsupported profile version: %d", p.Version)
	}
	aux.Alias.Manifest = aux.Manifest.ManifestLoader
	return nil
}

type PublicProfiles []PublicProfile

func (p PublicProfiles) Convert() ([]Profile, error) {
	var profiles []Profile
	for _, pub := range p {
		profile, err := convert(pub)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, *profile)
	}
	return profiles, nil
}

func convert(pub PublicProfile) (p *Profile, err error) {
	p = &Profile{
		PublicProfile: pub,
	}
	if p.Path == "" {
		p.Path = filepath.Join(DataDir, "profiles", "local", p.Name)
	}

	if p.Source != "" {
		u, err := url.Parse(p.Source)
		if err != nil {
			return nil, fmt.Errorf("invalid source URL: %w", err)
		}
		if u.Scheme == "" {
			return nil, fmt.Errorf("invalid source URL: %w", err)
		}
		if u.Host == "" {
			return nil, fmt.Errorf("invalid source URL: %w", err)
		}
		p.Path = filepath.Join(DataDir, "profiles", u.Host, p.Name)
	}
	p.IconImage = defaultIconImage
	if p.Icon != "" {
		// load icon as base64 img
		img, _, err := image.Decode(base64.NewDecoder(base64.StdEncoding, strings.NewReader(p.Icon)))
		if err != nil {
			slog.Error("Failed to decode icon", "error", err)
			// placeholder image
			// black square
			img = defaultIconImage
		}
		p.IconImage = img
	}
	return p, nil
}

type Profile struct {
	PublicProfile
	IconImage image.Image `json:"-"`
	// Path to the profile
	Path string `json:"-"`

	Source string `json:"source,omitempty"`
}

func (p *Profile) UnmarshalJSON(data []byte) error {
	type Alias Profile
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	profile, err := convert(aux.PublicProfile)
	if err != nil {
		return err
	}
	*p = *profile
	return nil
}

func (p *Profile) Display() string {
	if p.DisplayName != "" {
		return p.DisplayName
	}
	return p.Name
}

func (p *Profile) DeleteManifestCache(profilePath string) error {
	return os.RemoveAll(filepath.Join(profilePath, "manifest.json"))
}

//go:embed default_icon.png
var defaultIconData []byte
var defaultIconImage image.Image

func init() {
	img, _, err := image.Decode(bytes.NewReader(defaultIconData))
	if err != nil {
		slog.Error("Failed to decode default icon", "error", err)
		// fallback to a black square
		defaultIconImage = image.NewUniform(color.RGBA{R: 0, G: 0, B: 0, A: 255})
	} else {
		defaultIconImage = img
	}
}

func (p *Profile) Fetch() error {
	if p.Source == "" {
		return nil // No source to fetch from
	}
	resp, err := http.Get(p.Source)
	if err != nil {
		slog.Error("Failed to fetch profile from source", "source", p.Source, "error", err)
		return err
	}
	defer resp.Body.Close()
	var publicProfiles []PublicProfile
	if err := json.NewDecoder(resp.Body).Decode(&publicProfiles); err != nil {
		slog.Error("Failed to decode fetched profile", "source", p.Source, "error", err)
		return err
	}
	profiles := make([]Profile, len(publicProfiles))
	for i, publicProfile := range publicProfiles {
		profiles[i] = Profile{
			PublicProfile: publicProfile,
			Path:          filepath.Join(DataDir, "profiles", publicProfile.Name),
		}
	}
	for _, profile := range profiles {
		if profile.Name == p.Name {
			profile.Source = p.Source // Ensure the source is set
			u, err := url.Parse(profile.Source)
			if err != nil {
				slog.Error("Invalid source URL", "source", profile.Source, "error", err)
				return fmt.Errorf("invalid source URL: %w", err)
			}
			if u.Scheme == "" {
				slog.Error("Invalid source URL scheme", "source", profile.Source)
				return fmt.Errorf("invalid source URL scheme: %s", profile.Source)
			}
			if u.Host == "" {
				slog.Error("Invalid source URL host", "source", profile.Source)
				return fmt.Errorf("invalid source URL host: %s", profile.Source)
			}
			profile.Path = filepath.Join(DataDir, "profiles", u.Host, profile.Name)
			*p = profile // Update the current profile with the fetched one
			return nil
		}
	}
	slog.Error("Profile not found in fetched data", "name", p.Name, "source", p.Source)
	return fmt.Errorf("profile %s not found in fetched data from %s", p.Name, p.Source)
}

func (p *Profile) GetIcon() *widget.Image {
	return &widget.Image{Src: paint.NewImageOp(p.IconImage)}
}

func getMem() (sigar.Mem, error) {
	var s sigar.ConcreteSigar
	return s.GetMem()
}

func (p *Profile) CheckMemory() (bool, error) {
	mem, err := getMem()
	if err != nil {
		return false, err
	}
	return mem.Total > 2*(p.RecommendedMemoryMB*1024*1024), nil
}

const DEFAULT_MEMORY_SIZE = 2048

func (p *Profile) ActualMemory() (uint64, error) {
	if p.RecommendedMemoryMB == 0 {
		return DEFAULT_MEMORY_SIZE, nil
	}
	mem, err := getMem()
	if err != nil {
		return 0, err
	}
	if mem.Total > 2*(p.RecommendedMemoryMB*1024*1024) {
		return p.RecommendedMemoryMB, nil
	}
	return mem.Total / (2 * 1024 * 1024), nil
}

type Manifest struct {
	MinecraftVersion string `json:"minecraftVersion"`
	JavaVersion      int    `json:"javaVersion"`
	MaxMemory        int    `json:"maxMemory"`
}
