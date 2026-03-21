package network

import (
	"fmt"
	"os/exec"
	"strconv"

	"github.com/coreos/go-iptables/iptables"
)

func FlushZyloRules() error {
	return exec.Command("iptables", "-t", "nat", "-F", "ZYLO").Run()
}

func InitZyloNat() error {
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("failed to initialize iptables: %v", err)
	}

	exists := false
	chains, err := ipt.ListChains("nat")
	if err == nil {
		for _, c := range chains {
			if c == "ZYLO" {
				exists = true
				break
			}
		}
	}
	if !exists {
		if err := ipt.NewChain("nat", "ZYLO"); err != nil {
			return fmt.Errorf("failed to create ZYLO chain: %v", err)
		}
	}

	if err := ipt.AppendUnique("nat", "PREROUTING", "-j", "ZYLO"); err != nil {
		return fmt.Errorf("failed to link PREROUTING to ZYLO: %v", err)
	}

	if err := ipt.AppendUnique("nat", "OUTPUT", "-j", "ZYLO"); err != nil {
		return fmt.Errorf("failed to link OUTPUT to ZYLO: %v", err)
	}

	return nil
}

func AddPortForward(hostPort int, containerIP string, containerPort int) error {
	cmd := exec.Command("iptables", "-t", "nat", "-A", "ZYLO", "-p", "tcp", "--dport", strconv.Itoa(hostPort),
		"-j", "DNAT", "--to-destination", containerIP+":"+strconv.Itoa(containerPort))
	return cmd.Run()
}

func RemovePortForward(hostPort string, containerIP string, containerPort string) {
	exec.Command("iptables", "-t", "nat", "-D", "ZYLO", "-p", "tcp", "--dport", hostPort,
		"-j", "DNAT", "--to-destination", containerIP+":"+containerPort).Run()
	exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING",
		"-d", containerIP, "-p", "tcp", "--dport", containerPort, "-j", "MASQUERADE").Run()
}

func (nm *NetManager) setupNetworkIPTables(subnet, bridgeName string) error {
	if nm.ipt == nil {
		return fmt.Errorf("iptables not initialized")
	}

	_ = nm.ipt.AppendUnique("filter", "FORWARD", "-i", bridgeName, "-j", "ACCEPT")
	_ = nm.ipt.AppendUnique("filter", "FORWARD", "-o", bridgeName, "-j", "ACCEPT")

	_ = nm.ipt.AppendUnique("nat", "POSTROUTING",
		"-s", subnet,
		"!", "-o", bridgeName,
		"-j", "MASQUERADE")

	_ = nm.ipt.AppendUnique("nat", "POSTROUTING",
		"-d", subnet,
		"-j", "MASQUERADE")

	return nil
}
