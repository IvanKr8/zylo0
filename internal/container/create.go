package container

import (
	"fmt"
	"zylo/network"
)

func createAndSetupContainer(nm *network.NetManager, cfg *Config) (*Container, error) {
	ctr := newContainer(cfg)
	if ctr == nil {
		return nil, fmt.Errorf("failed to create container")
	}

	ctr.NetworkManager = nm

	if err := ctr.setup(); err != nil {
		return nil, err
	}

	return ctr, nil
}
