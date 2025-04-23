package container

import "zylo/internal/container/network"

func (containerCfg *container) run() error {
	if err := prepareDirs(containerCfg); err != nil {
		return err
	}

	if err := prepareWorkDir(containerCfg); err != nil {
		return err
	}

	if err := network.NewBridge(containerCfg.net.name); err != nil {
		return err
	}

	if err := setNS(containerCfg); err != nil {
		return err
	}

	return nil
}
