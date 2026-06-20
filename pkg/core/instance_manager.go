package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
		// Reset instances to empty slice on error
		im.instances = []*resource.Instance{}
		slog.Error("Failed to refresh instances", "error", err)
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

func (im *instanceManager) ImportInstance(ctx context.Context, packPath string) error {
	uid := uuid.New()
	destDir := filepath.Join(im.dataDir, "instances", uid.String())
	inst, err := resource.ImportSBPack(ctx, packPath, destDir, uid, nil)
	if err != nil {
		_ = os.RemoveAll(destDir)
		return err
	}

	im.mu.Lock()
	im.instances = append(im.instances, inst)
	im.mu.Unlock()

	return im.saveInstances()
}

func (im *instanceManager) AddRemoteInstance(ctx context.Context, manifestURL string) error {
	uid := uuid.New()
	destDir := filepath.Join(im.dataDir, "instances", uid.String())

	observer := &progressBridge{
		ch: im.progressChan,
	}

	inst, err := resource.ImportRemoteSBPack(ctx, manifestURL, destDir, uid, observer)
	if err != nil {
		_ = os.RemoveAll(destDir)
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

func (p *progressBridge) OnProgress(taskName string, percentage float64, status string, category string) {
	p.ch <- ProgressEvent{
		TaskName:   taskName,
		Percentage: percentage,
		Status:     status,
		Category:   ProgressCategory(category),
	}
}

func (im *instanceManager) CheckUpdate(ctx context.Context, instanceID uuid.UUID) (bool, error) {
	inst, err := im.GetInstance(instanceID)
	if err != nil {
		return false, err
	}

	if inst.Upstream == nil || inst.Upstream.ManifestURL == "" {
		return false, nil
	}

	repo, err := resource.FetchRepository(ctx, inst.Upstream.ManifestURL)
	if err != nil {
		return false, fmt.Errorf("failed to fetch repository manifest: %w", err)
	}

	if len(repo.Patches) == 0 {
		return false, nil
	}

	latestPatchID := repo.Patches[len(repo.Patches)-1].ID
	return inst.Upstream.Version != latestPatchID, nil
}

func (im *instanceManager) RepairInstance(ctx context.Context, instanceID uuid.UUID) error {
	inst, err := im.GetInstance(instanceID)
	if err != nil {
		return err
	}

	return resource.RepairInstance(ctx, inst, &progressBridge{ch: im.progressChan})
}

func (im *instanceManager) SaveInstance(inst *resource.Instance) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	for i, existing := range im.instances {
		if existing.UID == inst.UID {
			im.instances[i] = inst
			return im.saveInstances()
		}
	}
	return fmt.Errorf("instance not found: %s", inst.UID)
}

func (im *instanceManager) UpdateInstance(ctx context.Context, instanceID uuid.UUID, path string) error {
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
	observer := &progressBridge{ch: im.progressChan}

	if path == "" {
		// Remote update
		if targetInst.Upstream == nil || targetInst.Upstream.ManifestURL == "" {
			err = fmt.Errorf("instance does not have a remote manifest, please provide a patch file")
		} else {
			err = resource.UpdateInstanceRemote(ctx, targetInst, observer)
		}
	} else if strings.HasSuffix(strings.ToLower(path), ".sbpatch") {
		err = resource.ApplySBPatch(ctx, targetInst, path, observer)
	} else if strings.HasSuffix(strings.ToLower(path), ".sbpack") {
		err = resource.ApplySBPack(ctx, targetInst, path, observer)
	} else {
		err = fmt.Errorf("unsupported file format: %s (expected .sbpack or .sbpatch)", filepath.Base(path))
	}

	if err != nil {
		// If it's not a context.Canceled error, wrap it
		if !errors.Is(err, context.Canceled) {
			return fmt.Errorf("update failed, rolled back to previous state: %w", err)
		}
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
			go func() {
				if err := os.RemoveAll(inst.Path); err != nil {
					slog.Error("Failed to delete instance files", "error", err)
				}
			}()

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
