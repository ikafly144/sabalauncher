package resource

import (
	"archive/zip"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"
)

type Loader struct {
	Override   *OverridesManifest  `json:"override"`
	Initialize *InitializeManifest `json:"initialize"`
}

func (newLoader *Loader) update(oldLoader *Loader, packZip *zip.Reader, profilePath string, worker *DownloadWorker) error {
	worker.addTask(func() error {
		if newLoader.Initialize != nil && (oldLoader.Initialize == nil || oldLoader.Initialize.UpdatedAt.IsZero()) {
			slog.Info("profile override updating")
			if err := newLoader.Initialize.update(packZip, profilePath); err != nil {
				return fmt.Errorf("failed to update initialize: %w", err)
			}
		}
		if newLoader.Override != nil && (oldLoader.Override == nil || oldLoader.Override.UpdatedAt.Before(newLoader.Override.UpdatedAt)) {
			slog.Info("profile initializing")
			if err := newLoader.Override.update(packZip, profilePath); err != nil {
				return fmt.Errorf("failed to update overrides: %w", err)
			}
		}
		return nil
	})
	return nil
}

type OverridesManifest struct {
	UpdatedAt time.Time `json:"updated_at"`
	Overrides string    `json:"overrides"`
}

func (o *OverridesManifest) update(packZip *zip.Reader, profilePath string) error {
	for _, file := range packZip.File {
		slog.Info("file", "name", file.Name)
		if !strings.HasPrefix(file.Name, o.Overrides) {
			slog.Info("file not in overrides", "name", file.Name, "overrides", o.Overrides)
			continue
		}
		name := filepath.Join(profilePath, strings.TrimPrefix(file.Name, o.Overrides))
		if file.Mode().IsDir() {
			continue
		}
		if err := writeZipFile(name, file); err != nil {
			return fmt.Errorf("failed to write file %s: %w", name, err)
		}
	}
	return nil
}

type InitializeManifest struct {
	UpdatedAt   time.Time `json:"updated_at"`
	Initializes string    `json:"initializes"`
}

func (i *InitializeManifest) update(packZip *zip.Reader, profilePath string) error {
	for _, file := range packZip.File {
		if !strings.HasPrefix(file.Name, i.Initializes) {
			continue
		}
		name := filepath.Join(profilePath, strings.TrimPrefix(file.Name, i.Initializes))
		if file.Mode().IsDir() {
			continue
		}
		if err := writeZipFile(name, file); err != nil {
			return fmt.Errorf("failed to write file %s: %w", name, err)
		}
	}
	return nil
}
