package network

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

func (nm *NetManager) PortForward(containerIP string, hostPort, containerPort int) error {
	preroutingRule := []string{
		"-p", "tcp",
		"--dport", strconv.Itoa(hostPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort),
	}

	exists, err := nm.ipt.Exists("nat", "PREROUTING", preroutingRule...)
	if err != nil {
		return fmt.Errorf("failed to check PREROUTING rule: %v", err)
	}
	if !exists {
		if err := nm.ipt.Append("nat", "PREROUTING", preroutingRule...); err != nil {
			return fmt.Errorf("failed to add PREROUTING rule: %v", err)
		}
	}

	outputRule := []string{
		"-p", "tcp",
		"--dport", strconv.Itoa(hostPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort),
	}

	exists, err = nm.ipt.Exists("nat", "OUTPUT", outputRule...)
	if err != nil {
		return fmt.Errorf("failed to check OUTPUT rule: %v", err)
	}
	if !exists {
		if err := nm.ipt.Append("nat", "OUTPUT", outputRule...); err != nil {
			return fmt.Errorf("failed to add OUTPUT rule: %v", err)
		}
	}

	masqueradeRule := []string{
		"-d", containerIP,
		"-p", "tcp",
		"--dport", strconv.Itoa(containerPort),
		"-j", "MASQUERADE",
	}

	exists, err = nm.ipt.Exists("nat", "POSTROUTING", masqueradeRule...)
	if err != nil {
		return fmt.Errorf("failed to check MASQUERADE rule: %v", err)
	}
	if !exists {
		if err := nm.ipt.Append("nat", "POSTROUTING", masqueradeRule...); err != nil {
			return fmt.Errorf("failed to add MASQUERADE rule: %v", err)
		}
	}

	forwardRule := []string{
		"-p", "tcp",
		"-d", containerIP,
		"--dport", strconv.Itoa(containerPort),
		"-j", "ACCEPT",
	}

	exists, err = nm.ipt.Exists("filter", "FORWARD", forwardRule...)
	if err != nil {
		return fmt.Errorf("failed to check FORWARD rule: %v", err)
	}
	if !exists {
		if err := nm.ipt.Append("filter", "FORWARD", forwardRule...); err != nil {
			return fmt.Errorf("failed to add FORWARD rule: %v", err)
		}
	}

	return nil
}

func (nm *NetManager) RemovePortForward(containerIP string, hostPort, containerPort int) error {
	preroutingRule := []string{
		"-p", "tcp",
		"--dport", strconv.Itoa(hostPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort),
	}
	nm.ipt.Delete("nat", "PREROUTING", preroutingRule...)

	outputRule := []string{
		"-p", "tcp",
		"--dport", strconv.Itoa(hostPort),
		"-j", "DNAT",
		"--to-destination", fmt.Sprintf("%s:%d", containerIP, containerPort),
	}
	nm.ipt.Delete("nat", "OUTPUT", outputRule...)

	masqueradeRule := []string{
		"-d", containerIP,
		"-p", "tcp",
		"--dport", strconv.Itoa(containerPort),
		"-j", "MASQUERADE",
	}
	nm.ipt.Delete("nat", "POSTROUTING", masqueradeRule...)

	forwardRule := []string{
		"-p", "tcp",
		"-d", containerIP,
		"--dport", strconv.Itoa(containerPort),
		"-j", "ACCEPT",
	}
	nm.ipt.Delete("filter", "FORWARD", forwardRule...)

	return nil
}

func IsPortFree(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
	if err != nil {
		return true
	}
	conn.Close()
	return false
}

func ArePortsFree(ports []string) error {
	for _, p := range ports {
		parts := strings.Split(p, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid port format: %s", p)
		}
		hostPort, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid host port: %s", parts[0])
		}
		if !IsPortFree(hostPort) {
			return fmt.Errorf("host port %d is already in use", hostPort)
		}
	}
	return nil
}
