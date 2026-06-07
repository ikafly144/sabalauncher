package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/ikafly144/sabalauncher/v2/pkg/resource"
)

type instanceManager struct {
	dataDir      string
	instances    []*resource.Instance
	progressChan chan ProgressEvent
	mu           sync.RWMutex
}

func NewInstanceManager(dataDir string) (InstanceManager, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	im := &instanceManager{
		dataDir:      dataDir,
		progressChan: make(chan ProgressEvent, 100),
	}
	if err := im.RefreshInstances(); err != nil {
		return nil, err
	}
	return im, nil
}

func (im *instanceManager) SubscribeProgress() <-chan ProgressEvent {
	return im.progressChan
}

func (im *instanceManager) GetInstances() ([]*resource.Instance, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.instances, nil
}

func (im *instanceManager) GetInstance(id uuid.UUID) (*resource.Instance, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	for _, inst := range im.instances {
		if inst.UID == id {
			return inst, nil
		}
	}
	return nil, fmt.Errorf("instance not found: %s", id)
}

func (im *instanceManager) ImportInstance(packPath string) error {
	uid := uuid.New()
	destDir := filepath.Join(im.dataDir, "instances", uid.String())
	inst, err := resource.ImportSBPack(packPath, destDir, uid)
	if err != nil {
		return err
	}

	im.mu.Lock()
	im.instances = append(im.instances, inst)
	im.mu.Unlock()

	return im.saveInstances()
}

func (im *instanceManager) AddRemoteInstance(manifestURL string) error {
	uid := uuid.New()
	destDir := filepath.Join(im.dataDir, "instances", uid.String())

	observer := &progressBridge{
		ch: im.progressChan,
	}

	inst, err := resource.ImportRemoteSBPack(manifestURL, destDir, uid, observer)
	if err != nil {
		return err
	}

	im.mu.Lock()
	im.instances = append(im.instances, inst)
	im.mu.Unlock()

	return im.saveInstances()
}

type progressBridge struct {
	ch chan ProgressEvent
}

func (p *progressBridge) OnProgress(taskName string, percentage float64, status string) {
	p.ch <- ProgressEvent{
		TaskName:   taskName,
		Percentage: percentage,
		Status:     status,
	}
}

func (im *instanceManager) UpdateInstance(instanceID uuid.UUID, path string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	var targetInst *resource.Instance
	for _, inst := range im.instances {
		if inst.UID == instanceID {
			targetInst = inst
			break
		}
	}

	if targetInst == nil {
		return fmt.Errorf("instance not found: %s", instanceID)
	}

	var err error
	if path == "" {
		// Remote update
		if targetInst.Upstream == nil || targetInst.Upstream.ManifestURL == "" {
			return fmt.Errorf("instance does not have a remote manifest, please provide a patch file")
		}
		err = resource.UpdateInstanceRemote(targetInst)
	} else if strings.HasSuffix(strings.ToLower(path), ".sbpatch") {
		err = resource.ApplySBPatch(targetInst, path)
	} else if strings.HasSuffix(strings.ToLower(path), ".sbpack") {
		err = resource.ApplySBPack(targetInst, path)
	} else {
		return fmt.Errorf("unsupported file format: %s (expected .sbpack or .sbpatch)", filepath.Base(path))
	}

	if err != nil {
		return err
	}

	return im.saveInstances()
}

func (im *instanceManager) DeleteInstance(instanceID uuid.UUID) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	found := false
	for i, inst := range im.instances {
		if inst.UID == instanceID {
			// Delete files from disk
			if err := os.RemoveAll(inst.Path); err != nil {
				return fmt.Errorf("failed to delete instance files: %w", err)
			}

			im.instances = append(im.instances[:i], im.instances[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("instance not found: %s", instanceID)
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
		inst.Path = filepath.Join(im.dataDir, "instances", inst.UID.String())
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
