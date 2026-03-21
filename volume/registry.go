package volume

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	"zylo/global"
)

var (
	registryPath = fmt.Sprintf("%s/registry.json", global.VolCfgPth)
	mu           sync.RWMutex
)

type Volume struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Created string   `json:"created"`
	UsedBy  []string `json:"used_by"`
}

type Registry struct {
	Volumes []Volume `json:"volumes"`
}

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

func RegisterVolume(name, path string) error {
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}

	for _, v := range reg.Volumes {
		if v.Name == name {
			return fmt.Errorf("volume with name '%s' already exists", name)
		}
		if v.Path == path {
			return fmt.Errorf("volume with path '%s' already exists", path)
		}
	}

	reg.Volumes = append(reg.Volumes, Volume{
		Name:    name,
		Path:    path,
		Created: time.Now().Format(time.RFC3339),
		UsedBy:  []string{},
	})

	return SaveRegistry(reg)
}

func UnregisterVolume(name, path string) error {
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}

	var newVolumes []Volume
	var found bool

	for _, v := range reg.Volumes {
		if (name != "" && v.Name == name) || (path != "" && v.Path == path) {
			found = true

			// Проверяем используется ли том
			if len(v.UsedBy) > 0 {
				return fmt.Errorf("volume is still in use by containers: %v", v.UsedBy)
			}
			continue
		}
		newVolumes = append(newVolumes, v)
	}

	if !found {
		if name != "" {
			return fmt.Errorf("volume with name '%s' not found", name)
		}
		return fmt.Errorf("volume with path '%s' not found", path)
	}

	reg.Volumes = newVolumes
	return SaveRegistry(reg)
}

func MarkVolumeUsed(name, path, containerID string) error {
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}

	for i, v := range reg.Volumes {
		if (name != "" && v.Name == name) || (path != "" && v.Path == path) {
			for _, id := range v.UsedBy {
				if id == containerID {
					return nil
				}
			}
			reg.Volumes[i].UsedBy = append(reg.Volumes[i].UsedBy, containerID)
			return SaveRegistry(reg)
		}
	}
	return nil
}

func MarkVolumeUnused(name, path, containerID string) error {
	reg, err := LoadRegistry()
	if err != nil {
		return err
	}

	for i, v := range reg.Volumes {
		if (name != "" && v.Name == name) || (path != "" && v.Path == path) {
			var newUsed []string
			for _, id := range v.UsedBy {
				if id != containerID {
					newUsed = append(newUsed, id)
				}
			}
			reg.Volumes[i].UsedBy = newUsed
			return SaveRegistry(reg)
		}
	}
	return nil
}

func ListVolumes() ([]Volume, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}
	return reg.Volumes, nil
}

func FindVolume(name, path string) (*Volume, error) {
	reg, err := LoadRegistry()
	if err != nil {
		return nil, err
	}

	for _, v := range reg.Volumes {
		if (name != "" && v.Name == name) || (path != "" && v.Path == path) {
			return &v, nil
		}
	}
	return nil, nil
}
