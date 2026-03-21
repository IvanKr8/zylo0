package network

import (
	"fmt"
	"strings"
	"zylo/hosts"
)

func NewIPAM(nm *NetManager) *IPAM {
	return &IPAM{nm: nm}
}

func (ipam *IPAM) AllocateIP(networkName, containerID, containerName string) (string, error) {
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

		for _, ctr := range config.Containers {
			if ctr.IP == ip {
				taken = true
				break
			}
		}

		if !taken {
			if config.Containers == nil {
				config.Containers = make(map[string]ContainerNetInfo)
			}

			config.Containers[containerID] = ContainerNetInfo{
				ID:   containerID,
				Name: containerName,
				IP:   ip,
			}

			if err := ipam.nm.SaveConfig(config); err != nil {
				return "", fmt.Errorf("failed to save config: %v", err)
			}

			hostsMgr, err := hosts.NewHostsManager(networkName)
			if err != nil {
				fmt.Printf("Warning: failed to init hosts manager: %v\n", err)
			} else {
				if err := hostsMgr.AddContainer(ip, containerName); err != nil {
					fmt.Printf("Warning: failed to add container to hosts: %v\n", err)
				}
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

	ctr, exists := config.Containers[containerID]
	if !exists {
		return "", fmt.Errorf("container %s not found in network %s", containerID, networkName)
	}

	return ctr.IP, nil
}

func (ipam *IPAM) ReleaseIP(networkName, containerID string) error {
	config, err := ipam.nm.GetNetwork(networkName)
	if err != nil {
		return err
	}

	var containerInfo ContainerNetInfo
	if ctr, ok := config.Containers[containerID]; ok {
		containerInfo = ctr
		delete(config.Containers, containerID)

		if containerInfo.Name != "" {
			hostsMgr, err := hosts.NewHostsManager(networkName)
			if err != nil {
				fmt.Printf("Warning: failed to init hosts manager: %v\n", err)
			} else {
				if err := hostsMgr.RemoveContainer(containerInfo.IP, containerInfo.Name); err != nil {
					fmt.Printf("Warning: failed to remove container from hosts: %v\n", err)
				}
			}
		}
	}

	return ipam.nm.SaveConfig(config)
}

func (ipam *IPAM) GetGateway(networkName string) (string, error) {
	config, err := ipam.nm.GetNetwork(networkName)
	if err != nil {
		return "", err
	}
	return config.Gateway, nil
}
