package core

import (
	"testing"

	"github.com/ikafly144/sabalauncher/pkg/msa"
	"github.com/ikafly144/sabalauncher/pkg/resource"
)

// Mock objects for testing
type mockAuth struct {
	Authenticator
}

func (m *mockAuth) GetMinecraftAccount() (*msa.MinecraftAccount, error) {
	return &msa.MinecraftAccount{}, nil
}

type mockProfileManager struct {
	ProfileManager
	profile *resource.Profile
}

func (m *mockProfileManager) GetFullProfile(name string) (*resource.Profile, error) {
	return m.profile, nil
}

type mockManifestLoader struct {
	resource.ManifestLoader
	loaderType string
	done       bool
}

func (m *mockManifestLoader) Type() string { return m.loaderType }
func (m *mockManifestLoader) StartSetup(dataPath, profilePath string) {
	m.done = true
}
func (m *mockManifestLoader) IsDone() bool             { return m.done }
func (m *mockManifestLoader) CurrentStatus() string    { return "Done" }
func (m *mockManifestLoader) TotalProgress() float64   { return 1.0 }
func (m *mockManifestLoader) CurrentProgress() float64 { return 1.0 }
func (m *mockManifestLoader) Error() error             { return nil }
func (m *mockManifestLoader) GetClientManifest() *resource.ClientManifest {
	return &resource.ClientManifest{
		ID: "1.20.1",
		JavaVersion: resource.JavaVersion{
			Component:    "java-runtime-gamma",
			MajorVersion: 17,
		},
	}
}

func TestGameRunner_LaunchFlow(t *testing.T) {
	loaders := []string{"vanilla", "forge", "fabric", "neoforge", "quilt"}

	for _, loaderType := range loaders {
		t.Run(loaderType, func(t *testing.T) {
			manifest := &mockManifestLoader{loaderType: loaderType}
			profile := &resource.Profile{
				PublicProfile: resource.PublicProfile{
					Name:      "test",
					ModLoader: loaderType,
					Manifest:  manifest,
					Version:   2,
				},
				Path: "test-path",
			}

			auth := &mockAuth{}
			pm := &mockProfileManager{profile: profile}

			runner := NewGameRunner(auth, pm, "data-dir")

			if runner == nil {
				t.Fatal("Failed to create GameRunner")
			}
		})
	}
}
