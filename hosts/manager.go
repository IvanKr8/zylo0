package hosts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"zylo/global"
)

var (
	hostsMutex sync.Mutex
)

type HostsManager struct {
	networkName string
	hostsPath   string
}

func NewHostsManager(networkName string) (*HostsManager, error) {
	hostsPath := filepath.Join(global.HostsCfgPth, networkName, "hosts")

	if err := os.MkdirAll(filepath.Dir(hostsPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create hosts dir: %v", err)
	}

	if _, err := os.Stat(hostsPath); os.IsNotExist(err) {
		baseContent := `127.0.0.1       localhost
::1             localhost ip6-localhost ip6-loopback

# containers zylo-net
`
		if err := os.WriteFile(hostsPath, []byte(baseContent), 0644); err != nil {
			return nil, fmt.Errorf("failed to create hosts file: %v", err)
		}
	}

	return &HostsManager{
		networkName: networkName,
		hostsPath:   hostsPath,
	}, nil
}

func (hm *HostsManager) AddContainer(ip, name string) error {
	hostsMutex.Lock()
	defer hostsMutex.Unlock()

	content, err := os.ReadFile(hm.hostsPath)
	if err != nil {
		return fmt.Errorf("failed to read hosts: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	// Remove existing entries with same name
	for _, line := range lines {
		if !strings.Contains(line, name) {
			newLines = append(newLines, line)
		}
	}

	newEntry := fmt.Sprintf("%s       %s", ip, name)

	// Insert after the comment section
	foundSection := false
	for i, line := range newLines {
		if strings.Contains(line, "# containers zylo-net") {
			foundSection = true
			newLines = append(newLines[:i+1], append([]string{newEntry}, newLines[i+1:]...)...)
			break
		}
	}

	if !foundSection {
		newLines = append(newLines, newEntry)
	}

	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(hm.hostsPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write hosts: %v", err)
	}

	return nil
}

func (hm *HostsManager) RemoveContainer(ip, name string) error {
	hostsMutex.Lock()
	defer hostsMutex.Unlock()

	content, err := os.ReadFile(hm.hostsPath)
	if err != nil {
		return fmt.Errorf("failed to read hosts: %v", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string

	for _, line := range lines {
		if strings.Contains(line, ip) || strings.Contains(line, name) {
			continue
		}
		newLines = append(newLines, line)
	}

	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(hm.hostsPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write hosts: %v", err)
	}

	return nil
}

func (hm *HostsManager) UpdateAllContainers(containers map[string]ContainerInfo) error {
	hostsMutex.Lock()
	defer hostsMutex.Unlock()

	content := `127.0.0.1       localhost
::1             localhost ip6-localhost ip6-loopback

# containers zylo-net
`

	for _, ctr := range containers {
		content += fmt.Sprintf("%s       %s\n", ctr.IP, ctr.Name)
	}

	if err := os.WriteFile(hm.hostsPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write hosts: %v", err)
	}

	return nil
}
