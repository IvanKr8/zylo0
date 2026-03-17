package network

import (
	"fmt"
	"strings"
)

type IPAM struct {
	nm *NetManager
}

func NewIPAM(nm *NetManager) *IPAM {
	return &IPAM{nm: nm}
}

func (ipam *IPAM) AllocateIP(networkName, containerID string) (string, error) {
	config, err := ipam.nm.GetNetwork(networkName)
	if err != nil {
		return "", err
	}

	parts := strings.Split(config.Gateway, ".")
	if len(parts) != 4 {
		return "", fmt.Errorf("invalid gateway format")
	}

	base := fmt.Sprintf("%s.%s.%s", parts[0], parts[1], parts[2])

	for i := 2; i <= 254; i++ {
		ip := fmt.Sprintf("%s.%d", base, i)
		taken := false

		for _, containerIP := range config.Containers {
			if containerIP == ip {
				taken = true
				break
			}
		}

		if !taken {
			if config.Containers == nil {
				config.Containers = make(map[string]string)
			}
			config.Containers[containerID] = ip

			if err := ipam.nm.saveConfig(config); err != nil {
				return "", fmt.Errorf("failed to save config: %v", err)
			}

			return ip, nil
		}
	}

	return "", fmt.Errorf("no free IP addresses")
}

func (ipam *IPAM) GetContainerIP(networkName, containerID string) (string, error) {
	config, err := ipam.nm.GetNetwork(networkName)
	if err != nil {
		return "", err
	}

	ip, exists := config.Containers[containerID]
	if !exists {
		return "", fmt.Errorf("container %s not found in network %s", containerID, networkName)
	}

	return ip, nil
}

func (ipam *IPAM) ReleaseIP(networkName, containerID string) error {
	config, err := ipam.nm.GetNetwork(networkName)
	if err != nil {
		return err
	}

	delete(config.Containers, containerID)

	return ipam.nm.saveConfig(config)
}

func (ipam *IPAM) GetGateway(networkName string) (string, error) {
	config, err := ipam.nm.GetNetwork(networkName)
	if err != nil {
		return "", err
	}
	return config.Gateway, nil
}
