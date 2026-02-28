package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ikafly144/sabalauncher/pkg/resource"
)

type instanceManager struct {
	dataDir   string
	instances []*resource.Instance
	mu        sync.RWMutex
}

func NewInstanceManager(dataDir string) (InstanceManager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	im := &instanceManager{
		dataDir: dataDir,
	}
	if err := im.RefreshInstances(); err != nil {
		return nil, err
	}
	return im, nil
}

func (im *instanceManager) GetInstances() ([]*resource.Instance, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.instances, nil
}

func (im *instanceManager) GetInstance(name string) (*resource.Instance, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	for _, inst := range im.instances {
		if inst.Name == name {
			return inst, nil
		}
	}
	return nil, fmt.Errorf("instance not found: %s", name)
}

func (im *instanceManager) ImportInstance(packPath string) error {
	destDir := filepath.Join(im.dataDir, "instances", strings.TrimSuffix(filepath.Base(packPath), ".sbpack"))
	inst, err := resource.ImportSBPack(packPath, destDir)
	if err != nil {
		return err
	}

	im.mu.Lock()
	im.instances = append(im.instances, inst)
	im.mu.Unlock()

	return im.saveInstances()
}

func (im *instanceManager) UpdateInstance(instanceName string, patchPath string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	var targetInst *resource.Instance
	for _, inst := range im.instances {
		if inst.Name == instanceName {
			targetInst = inst
			break
		}
	}

	if targetInst == nil {
		return fmt.Errorf("instance not found: %s", instanceName)
	}

	if err := resource.ApplySBPatch(targetInst, patchPath); err != nil {
		return err
	}

	return im.saveInstances()
}

func (im *instanceManager) DeleteInstance(name string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	found := false
	for i, inst := range im.instances {
		if inst.Name == name {
			im.instances = append(im.instances[:i], im.instances[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("instance not found: %s", name)
	}

	return im.saveInstances()
}

func (im *instanceManager) RefreshInstances() error {
	im.mu.Lock()
	defer im.mu.Unlock()

	path := filepath.Join(im.dataDir, "instances.json")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			im.instances = []*resource.Instance{}
			return nil
		}
		return err
	}
	defer file.Close()

	var instances []*resource.Instance
	if err := json.NewDecoder(file).Decode(&instances); err != nil {
		return err
	}

	for _, inst := range instances {
		inst.Path = filepath.Join(im.dataDir, "instances", inst.Name)
	}

	sort.SliceStable(instances, func(i, j int) bool {
		return strings.Compare(instances[i].Name, instances[j].Name) < 0
	})

	im.instances = instances
	return nil
}

func (im *instanceManager) saveInstances() error {
	path := filepath.Join(im.dataDir, "instances.json")
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(im.instances)
}
