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

type mockInstanceManager struct {
	InstanceManager
	inst *resource.Instance
}

func (m *mockInstanceManager) GetInstance(name string) (*resource.Instance, error) {
	return m.inst, nil
}

func TestGameRunner_LaunchFlow(t *testing.T) {
	loaders := []string{"vanilla", "forge", "fabric-loader", "neoforge", "quilt-loader"}

	for _, loaderType := range loaders {
		t.Run(loaderType, func(t *testing.T) {
			inst := &resource.Instance{
				Name: "test",
				Versions: []resource.InstanceVersion{
					{ID: "minecraft", Version: "1.20.1"},
					{ID: loaderType, Version: "1.0.0"},
				},
				Path: "test-path",
			}

			auth := &mockAuth{}
			im := &mockInstanceManager{inst: inst}

			runner := NewGameRunner(auth, im, "data-dir")

			if runner == nil {
				t.Fatal("Failed to create GameRunner")
			}
		})
	}
}
