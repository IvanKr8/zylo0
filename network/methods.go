package network

import (
	"fmt"
	"time"
	"zylo/global"
	"zylo/network/dns"
)

func (nm *NetManager) CreateNetwork(name string) (*Net, error) {
	subnet, gateway, err := nm.findFreeSubnet()
	if err != nil {
		return nil, err
	}

	netInfo := &Net{
		Name:    name,
		ID:      generateID(),
		Subnet:  subnet,
		Gateway: gateway,
	}

	netInfo.DNS = dns.NewDNS(netInfo.Gateway+global.DnsPort, func(name string) (string, bool) {
		for _, ctr := range netInfo.Containers {
			if ctr.Name == name {
				return ctr.IP, true
			}
		}
		return "", false
	})

	if err := nm.SaveConfig(netInfo); err != nil {
		return nil, err
	}

	if err := nm.restoreNetwork(netInfo); err != nil {
		return nil, err
	}

	netInfo.DNS.Start()

	timeout := time.After(5 * time.Second)
	tick := time.Tick(50 * time.Millisecond)
	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("DNS failed to start within 5 seconds")
		case <-tick:
			if netInfo.DNS.IsRunning() {
				return netInfo, nil
			}
		}
	}
}

func (nm *NetManager) CreateDefaultNetwork() error {
	if nm.NetworkExists(global.MainNetName) {
		return nil
	}

	_, err := nm.CreateNetwork(global.MainNetName)
	return err
}

func (nm *NetManager) SetupIPTables() error {
	return InitZyloNat()
}

func (nm *NetManager) PortForward(containerIP string, hostPort, containerPort int) error {
	return AddPortForward(hostPort, containerIP, containerPort)
}
