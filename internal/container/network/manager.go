package network

import (
	"fmt"
	"zylo/internal/system"
)

var checkIface = func(name string) (bool, error) {
	if err := system.Command("ip", "link", "show", name); err != nil {
		return false, nil
	}
	return true, nil
}

func createBridge(name string) error {
	if err := system.Command("ip", "link", "add", name, "type", "bridge"); err != nil {
		return fmt.Errorf("failed to create network %s: %v", name, err)
	}
	return nil
}

func upIface(name string) error {
	if err := system.Command("ip", "link", "set", name, "up"); err != nil {
		return err
	}
	return nil
}

func assignIPtoBridge(name, ip string) error {
	if err := system.Command("ip", "addr", "add", ip, "dev", name); err != nil {
		return fmt.Errorf("failed to assign IP to bridge %s: %v", name, err)
	}
	return nil
}

func createVeth(pair1, pair2 string) error {
	if err := system.Command("ip", "link", "add", pair1, "type", "veth", "peer", "name", pair2); err != nil {
		return fmt.Errorf("failed to create veth pair: %v", err)
	}
	return nil
}

func tiePair(pair1, pair2 string) error {
	if err := system.Command("ip", "link", "set", pair1, "master", pair2); err != nil {
		return fmt.Errorf("failed to add container veth to bridge: %v", err)
	}

	return nil
}
