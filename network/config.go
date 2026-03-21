package network

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"zylo/global"
	"zylo/network/dns"
)

var ErrNetworkNotFound = fmt.Errorf("network not found")

// SaveConfig persists network configuration to disk as JSON
// Each network is stored as a separate file: <name>.json
func (nm *NetManager) SaveConfig(config *Net) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(nm.networksDir, config.Name+".json")
	return os.WriteFile(path, data, 0644)
}

// loadConfig loads a single network configuration by name
func (nm *NetManager) loadConfig(name string) (*Net, error) {
	path := filepath.Join(nm.networksDir, name+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Net
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// ListNetworks returns all available network configs
// Used for CLI / inspection
func (nm *NetManager) ListNetworks() ([]*Net, error) {
	files, err := filepath.Glob(filepath.Join(nm.networksDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %v", err)
	}

	var networks []*Net

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var config Net
		if err := json.Unmarshal(data, &config); err != nil {
			continue
		}

		networks = append(networks, &config)
	}

	return networks, nil
}

// getAllNetworks loads all network configs from disk
// Internal helper used during restore
func (nm *NetManager) getAllNetworks() ([]*Net, error) {
	files, err := filepath.Glob(filepath.Join(nm.networksDir, "*.json"))
	if err != nil {
		return nil, err
	}

	var networks []*Net

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var config Net
		if err := json.Unmarshal(data, &config); err == nil {
			networks = append(networks, &config)
		}
	}

	return networks, nil
}

// NetworkExists checks if network config file exists
func (nm *NetManager) NetworkExists(name string) bool {
	_, err := os.Stat(filepath.Join(nm.networksDir, name+".json"))
	return err == nil
}

// GetNetwork loads network config or returns ErrNetworkNotFound
func (nm *NetManager) GetNetwork(name string) (*Net, error) {
	path := filepath.Join(nm.networksDir, name+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNetworkNotFound
		}
		return nil, err
	}

	var config Net
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// RestoreNetworks recreates all networks from saved configs
// This is called on daemon startup to restore bridges, veth pairs and NAT rules
func (nm *NetManager) RestoreNetworks() error {
	nets, err := nm.getAllNetworks()
	if err != nil {
		return err
	}

	for _, n := range nets {
		// Skip main network
		if n.Name == global.MainNetName {
			continue
		}

		// Recreate bridge, connectivity and iptables rules
		if err := nm.restoreNetwork(n); err != nil {
			fmt.Printf("failed to restore network %s: %v\n", n.Name, err)
		}

		if n.DNS == nil || !n.DNS.IsRunning() {
			fmt.Printf("starting DNS for network %s at %s\n", n.Name, n.Gateway)
			n.DNS = dns.NewDNS(n.Gateway+global.DnsPort, func(name string) (string, bool) {
				for _, ctr := range n.Containers {
					if ctr.Name == name {
						return ctr.IP, true
					}
				}
				return "", false
			})

			n.DNS.Start()

			timeout := time.After(5 * time.Second)
			tick := time.Tick(50 * time.Millisecond)
			ready := false
			for !ready {
				select {
				case <-timeout:
					fmt.Printf("DNS failed to start within 5 seconds for network %s\n", n.Name)
					ready = true
				case <-tick:
					if n.DNS.IsRunning() {
						fmt.Printf("DNS started for network %s\n", n.Name)
						ready = true
					}
				}
			}
		}
	}

	return nil
}
