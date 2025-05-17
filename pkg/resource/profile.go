package resource

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ikafly144/sabalauncher/pkg/msa"

	"gioui.org/op/paint"
	"gioui.org/widget"
)

var (
	DataDir = filepath.Join(os.Getenv("APPDATA"), "SabaLauncher")
)

const (
	CurrentProfileVersion = 1
)

type Profile struct {
	Name      string      `json:"name"`
	Icon      string      `json:"icon"`
	IconImage image.Image `json:"-"`
	// Path to the profile
	Path          string `json:"-"`
	ServerAddress string `json:"server_address,omitempty"`

	Source string `json:"source,omitempty"`

	Manifest ManifestLoader `json:"manifest"`

	Version int `json:"version,omitempty"`
}

func (p *Profile) DeleteManifestCache(profilePath string) error {
	return os.RemoveAll(filepath.Join(profilePath, "manifest.json"))
}

func (p *Profile) UnmarshalJSON(data []byte) error {
	type Alias Profile
	aux := &struct {
		*Alias
		Manifest ManifestLoaderUnmarshal `json:"manifest"`
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	aux.Alias.Manifest = aux.Manifest.ManifestLoader
	if p.Path == "" {
		p.Path = filepath.Join(DataDir, "profile", p.Name)
	}
	if p.Version != CurrentProfileVersion {
		return fmt.Errorf("unsupported profile version: %d", p.Version)
	}
	if p.Icon != "" {
		// load icon as base64 img
		img, _, err := image.Decode(base64.NewDecoder(base64.StdEncoding, strings.NewReader(p.Icon)))
		if err != nil {
			slog.Error("Failed to decode icon", "error", err)
			// placeholder image
			// black square
			img = image.NewUniform(color.RGBA{R: 0, G: 0, B: 0, A: 255})
		}
		p.IconImage = img
	}
	return nil
}

func (p *Profile) GetIcon() *widget.Image {
	return &widget.Image{
		Src: paint.NewImageOp(p.IconImage),
	}
}

type Manifest struct {
	MinecraftVersion string `json:"minecraftVersion"`
	JavaVersion      int    `json:"javaVersion"`
	MaxMemory        int    `json:"maxMemory"`
}

func SaveCredential(msp *msa.MinecraftAccount) error {
	// Save the credential to the profile
	f, err := os.Create(filepath.Join(DataDir, "account.json"))
	if err != nil {
		return err
	}
	defer f.Close()
	if err := json.NewEncoder(f).Encode(msp); err != nil {
		return err
	}
	return nil
}

func LoadCredential() (*msa.MinecraftAccount, error) {
	f, err := os.Open(filepath.Join(DataDir, "account.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var msp msa.MinecraftAccount
	if err := json.NewDecoder(f).Decode(&msp); err != nil {
		return nil, err
	}
	return &msp, nil
}
