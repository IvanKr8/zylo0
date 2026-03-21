package container

import (
	"fmt"
	"strconv"
	"strings"
	"zylo/network"
)

func arePortsFree(ports []string) error {
	usedPorts := make(map[int]string)

	for _, c := range ListContainers() {
		for _, p := range c.Ports {
			parts := strings.Split(p, ":")
			if len(parts) != 2 {
				continue
			}
			port, _ := strconv.Atoi(parts[0])
			usedPorts[port] = c.ID
		}
	}

	for _, p := range ports {
		parts := strings.Split(p, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid port format: %s", p)
		}

		hostPort, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid host port: %s", parts[0])
		}

		if !network.IsPortFree(hostPort) {
			return fmt.Errorf("host port %d is already in use by system", hostPort)
		}

		if id, ok := usedPorts[hostPort]; ok {
			return fmt.Errorf("host port %d is already used by container %s", hostPort, id)
		}
	}

	return nil
}

func checkPorts(ports []string) error {
	return arePortsFree(ports)
}
