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
	"os"
	"path/filepath"
	"strings"

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
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Icon        string      `json:"icon"`
	IconImage   image.Image `json:"-"`
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
