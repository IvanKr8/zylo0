package container

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
	nameRegistryPath = filepath.Join(global.NameCfgPth, "registry.json")
	nameMu           sync.RWMutex
)

type NameEntry struct {
	Name      string `json:"name"`
	Container string `json:"container_id"`
	Created   string `json:"created"`
}

type NameRegistry struct {
	Names []NameEntry `json:"names"`
}

func loadNameRegistry() (*NameRegistry, error) {
	nameMu.RLock()
	defer nameMu.RUnlock()

	data, err := os.ReadFile(nameRegistryPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &NameRegistry{Names: []NameEntry{}}, nil
		}
		return nil, err
	}

	var reg NameRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}

	return &reg, nil
}

func saveNameRegistry(reg *NameRegistry) error {
	nameMu.Lock()
	defer nameMu.Unlock()

	dir := filepath.Dir(nameRegistryPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(nameRegistryPath, data, 0644)
}

func NameExists(name string) (bool, error) {
	reg, err := loadNameRegistry()
	if err != nil {
		return false, err
	}

	for _, n := range reg.Names {
		if n.Name == name {
			return true, nil
		}
	}

	return false, nil
}

func RegisterName(name, containerID string) error {
	reg, err := loadNameRegistry()
	if err != nil {
		return err
	}

	for _, n := range reg.Names {
		if n.Name == name {
			return fmt.Errorf("name '%s' already exists", name)
		}
	}

	reg.Names = append(reg.Names, NameEntry{
		Name:      name,
		Container: containerID,
		Created:   time.Now().Format(time.RFC3339),
	})

	return saveNameRegistry(reg)
}

func UnregisterName(containerID string) error {
	reg, err := loadNameRegistry()
	if err != nil {
		return err
	}

	var updated []NameEntry

	for _, n := range reg.Names {
		if n.Container != containerID {
			updated = append(updated, n)
		}
	}

	reg.Names = updated
	return saveNameRegistry(reg)
}

func GetName(containerID string) (string, error) {
	reg, err := loadNameRegistry()
	if err != nil {
		return "", err
	}

	for _, n := range reg.Names {
		if n.Container == containerID {
			return n.Name, nil
		}
	}

	return "", fmt.Errorf("name not found for container %s", containerID)
}

func GetContainerIDByName(name string) (string, error) {
	reg, err := loadNameRegistry()
	if err != nil {
		return "", err
	}

	for _, n := range reg.Names {
		if n.Name == name {
			return n.Container, nil
		}
	}

	return "", fmt.Errorf("container with name '%s' not found", name)
}

func generateContainerName() string {
	return fmt.Sprintf("zylo-%d", time.Now().UnixNano())
}
