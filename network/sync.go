package network

import (
	"fmt"
	"zylo/hosts"
)

func SyncAllHosts(nm *NetManager) error {
	networks, err := nm.ListNetworks()
	if err != nil {
		return err
	}

	for _, net := range networks {
		config, err := nm.GetNetwork(net.Name)
		if err != nil {
			continue
		}

		hostsMgr, err := hosts.NewHostsManager(net.Name)
		if err != nil {
			continue
		}

		containers := make(map[string]hosts.ContainerInfo)
		for id, ctr := range config.Containers {
			containers[id] = hosts.ContainerInfo{
				ID:   ctr.ID,
				Name: ctr.Name,
				IP:   ctr.IP,
			}
		}

		if err := hostsMgr.UpdateAllContainers(containers); err != nil {
			fmt.Printf("Warning: failed to update hosts for %s: %v\n", net.Name, err)
		}
	}

	return nil
}
