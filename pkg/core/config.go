package core

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type LauncherConfig struct {
	MaxMemory uint64 `json:"max_memory"`
}

func DefaultConfig() *LauncherConfig {
	return &LauncherConfig{
		MaxMemory: 2048,
	}
}

func LoadConfig(dataPath string) (*LauncherConfig, error) {
	configPath := filepath.Join(dataPath, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}
	return config, nil
}

func (c *LauncherConfig) Save(dataPath string) error {
	configPath := filepath.Join(dataPath, "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}
