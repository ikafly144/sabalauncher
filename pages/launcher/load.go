package launcher

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/ikafly144/sabalauncher/pkg/resource"
)

var (
	dataDir = resource.DataDir
)

var (
	profilesLocal = []Profile{}
	sources       = []string{}
)

func (p *Page) loadProfiles() {
	if !p.loading.TryLock() {
		return
	}
	defer p.loading.Unlock()
	profilesLocal = []Profile{}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		slog.Error("Failed to create data directory", "error", err)
		return
	}
	profilesTP := p.loadFromProfileSources()
	file, err := os.OpenFile(filepath.Join(dataDir, "profiles.json"), os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		slog.Error("Failed to open profiles.json", "error", err)
		return
	}
	defer file.Close()
	if stat, err := file.Stat(); err == nil && stat.Size() == 0 {
		_, err := file.WriteString("[]")
		if err != nil {
			slog.Error("Failed to write empty profiles.json", "error", err)
			return
		}
		_, _ = file.Seek(0, 0)
	}
	if err := json.NewDecoder(file).Decode(&profilesLocal); err != nil {
		slog.Error("Failed to load profiles", "error", err)
		return
	}
	newProfiles := make([]Profile, 0, len(profilesLocal)+len(profilesTP))
	newProfiles = append(newProfiles, profilesLocal...)
	newProfiles = append(newProfiles, profilesTP...)
	sort.SliceStable(newProfiles, func(i, j int) bool {
		cmp := strings.Compare(newProfiles[i].Name, newProfiles[j].Name)
		if cmp != 0 {
			return cmp < 0
		}
		return newProfiles[i].Source < newProfiles[j].Source
	})
	p.Profiles = newProfiles
	slog.Info("Loaded profiles", "profiles", p.Profiles)
}

func (p *Page) loadFromProfileSources() (result []Profile) {
	source, err := os.OpenFile(filepath.Join(dataDir, "sources.json"), os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		slog.Error("Failed to open sources.json", "error", err)
		return nil
	}
	defer source.Close()

	if stat, err := source.Stat(); err == nil && stat.Size() == 0 {
		_, err := source.WriteString("[]")
		if err != nil {
			slog.Error("Failed to write empty sources.json", "error", err)
			return nil
		}
		_, _ = source.Seek(0, 0)
	}
	if err := json.NewDecoder(source).Decode(&sources); err != nil {
		slog.Error("Failed to load sources", "error", err)
		return nil
	}
	for _, source := range sources {
		httpClient := &http.Client{}
		resp, err := httpClient.Get(source)
		if err != nil {
			slog.Error("Failed to get source", "error", err)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			slog.Error("Failed to get source", "status", resp.StatusCode)
			continue
		}
		var profiles []resource.Profile
		{
			var publicProfiles resource.PublicProfiles
			if err := json.NewDecoder(resp.Body).Decode(&publicProfiles); err != nil {
				slog.Error("Failed to decode source", "error", err)
				continue
			}
			p, err := publicProfiles.Convert()
			if err != nil {
				slog.Error("Failed to convert public profiles", "error", err)
				continue
			}
			profiles = p
		}
		p := make([]Profile, len(profiles))
		for i := range profiles {
			p[i] = Profile{
				Profile: profiles[i],
			}
		}
		for i := range p {
			p[i].Source = source
			u, err := url.Parse(p[i].Source)
			if err != nil {
				slog.Error("Failed to parse source URL", "error", err)
				continue
			}
			if u.Scheme == "" {
				slog.Error("Invalid source URL", "url", p[i].Source)
				continue
			}
			if u.Host == "" {
				slog.Error("Invalid source URL", "url", p[i].Source)
				continue
			}
			p[i].Path = filepath.Join(dataDir, "profiles", u.Host, p[i].Name)
		}
		result = append(result, p...)
	}
	return result
}

func (p *Page) addProfileSource(url url.URL) {
	if slices.Contains(sources, url.String()) {
		return
	}
	sources = append(sources, url.String())
	file, err := os.OpenFile(filepath.Join(dataDir, "sources.json"), os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		slog.Error("Failed to open sources.json", "error", err)
		return
	}
	defer file.Close()
	if stat, err := file.Stat(); err == nil && stat.Size() == 0 {
		_, err := file.WriteString("[]")
		if err != nil {
			slog.Error("Failed to write empty sources.json", "error", err)
			return
		}
		_, _ = file.Seek(0, 0)
	}
	if err := json.NewEncoder(file).Encode(sources); err != nil {
		slog.Error("Failed to encode sources", "error", err)
		return
	}
	if err := file.Sync(); err != nil {
		slog.Error("Failed to sync sources.json", "error", err)
		return
	}

	go p.loadProfiles()
}

func (p *Page) removeProfileSource(url string) {
	p.loading.Lock()
	defer p.loading.Unlock()
	if url == "" {
		return
	}
	if !slices.Contains(sources, url) {
		return
	}
	for i, source := range sources {
		if source == url {
			sources = append(sources[:i], sources[i+1:]...)
			break
		}
	}
	file, err := os.OpenFile(filepath.Join(dataDir, "sources.json"), os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		slog.Error("Failed to open sources.json", "error", err)
		return
	}
	defer file.Close()
	if stat, err := file.Stat(); err == nil && stat.Size() == 0 {
		_, err := file.WriteString("[]")
		if err != nil {
			slog.Error("Failed to write empty sources.json", "error", err)
			return
		}
		_, _ = file.Seek(0, 0)
	}
	if err := json.NewEncoder(file).Encode(sources); err != nil {
		slog.Error("Failed to encode sources", "error", err)
		return
	}
	if err := file.Sync(); err != nil {
		slog.Error("Failed to sync sources.json", "error", err)
		return
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		p.loadProfiles()
	}()
}
