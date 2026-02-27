package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ikafly144/sabalauncher/pkg/resource"
)

type profileManager struct {
	dataDir string
	sources []string
	profiles []Profile
	mu sync.RWMutex
}

func NewProfileManager(dataDir string) (ProfileManager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	pm := &profileManager{
		dataDir: dataDir,
	}
	if err := pm.loadSources(); err != nil {
		return nil, err
	}
	if err := pm.RefreshProfiles(); err != nil {
		return nil, err
	}
	return pm, nil
}

func (pm *profileManager) loadSources() error {
	path := filepath.Join(pm.dataDir, "sources.json")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			pm.sources = []string{}
			return nil
		}
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(&pm.sources)
}

func (pm *profileManager) saveSources() error {
	path := filepath.Join(pm.dataDir, "sources.json")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(pm.sources)
}

func (pm *profileManager) GetProfiles() ([]Profile, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.profiles, nil
}

func (pm *profileManager) AddProfile(sourceURL string) error {
	// Relax validation to allow local file paths
	if strings.HasPrefix(sourceURL, "http://") || strings.HasPrefix(sourceURL, "https://") {
		u, err := url.Parse(sourceURL)
		if err != nil {
			return err
		}
		if u.Host == "" {
			return fmt.Errorf("invalid URL: %s", sourceURL)
		}
	} else {
		// Assume local file path
		if _, err := os.Stat(sourceURL); err != nil {
			return fmt.Errorf("local file not found: %w", err)
		}
	}

	pm.mu.Lock()
	for _, s := range pm.sources {
		if s == sourceURL {
			pm.mu.Unlock()
			return nil // Already added
		}
	}
	pm.sources = append(pm.sources, sourceURL)
	pm.mu.Unlock()

	if err := pm.saveSources(); err != nil {
		return err
	}
	return pm.RefreshProfiles()
}

func (pm *profileManager) DeleteProfile(sourceURL string) error {
	pm.mu.Lock()
	found := false
	for i, s := range pm.sources {
		if s == sourceURL {
			pm.sources = append(pm.sources[:i], pm.sources[i+1:]...)
			found = true
			break
		}
	}
	pm.mu.Unlock()

	if !found {
		return nil
	}

	if err := pm.saveSources(); err != nil {
		return err
	}
	return pm.RefreshProfiles()
}

func (pm *profileManager) RefreshProfiles() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var allProfiles []Profile
	for _, source := range pm.sources {
		var reader io.ReadCloser
		if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
			resp, err := http.Get(source)
			if err != nil {
				continue // Skip failing sources
			}
			reader = resp.Body
		} else {
			file, err := os.Open(source)
			if err != nil {
				continue // Skip failing files
			}
			reader = file
		}
		
		var publicProfiles resource.PublicProfiles
		if err := json.NewDecoder(reader).Decode(&publicProfiles); err != nil {
			reader.Close()
			continue
		}
		reader.Close()
		
		resProfiles, err := publicProfiles.Convert()
		if err != nil {
			continue
		}
		
		for _, rp := range resProfiles {
			allProfiles = append(allProfiles, Profile{
				Name:        rp.Name,
				DisplayName: rp.DisplayName,
				Description: rp.Description,
				IconImage:   rp.IconImage,
				IsActive:    false,
				Source:      source,
			})
		}
	}

	sort.SliceStable(allProfiles, func(i, j int) bool {
		return strings.Compare(allProfiles[i].Name, allProfiles[j].Name) < 0
	})

	pm.profiles = allProfiles
	return nil
}
