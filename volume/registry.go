package volume

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"zylo/global"
)

var (
	registryPath = filepath.Join(global.VolCfgPth, "registry.json")
	mu           sync.RWMutex
)

func LoadRegistry() (*Registry, error) {
	mu.RLock()
	defer mu.RUnlock()

	data, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{Volumes: []Volume{}}, nil
		}
		return nil, err
	}

	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}
	return &reg, nil
}

func SaveRegistry(reg *Registry) error {
	mu.Lock()
	defer mu.Unlock()

	dir := filepath.Dir(registryPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(registryPath, data, 0644)
}
