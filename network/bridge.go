package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"zylo/global"

	"github.com/vishvananda/netlink"
)

var ErrNetworkNotFound = errors.New("network not found")

func (nm *NetManager) findFreeSubnet() (string, string, error) {
	used := make(map[string]bool)

	files, err := os.ReadDir(nm.networksDir)
	if err == nil {
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".json") {
				continue
			}

			data, err := os.ReadFile(filepath.Join(nm.networksDir, f.Name()))
			if err != nil {
				continue
			}

			var config Net
			if err := json.Unmarshal(data, &config); err != nil {
				continue
			}

			used[config.Subnet] = true
		}
	}

	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok {
					continue
				}
				used[ipNet.String()] = true
			}
		}
	}

	output, err := exec.Command("ip", "-4", "route", "show").Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) > 0 && strings.Contains(line, "/") {
				for _, field := range fields {
					if strings.Contains(field, "/") {
						used[field] = true
					}
				}
			}
		}
	}

	for i := MIN_SUBNET; i <= MAX_SUBNET; i++ {
		subnet := fmt.Sprintf("10.20.%d.0/24", i)

		if !used[subnet] {
			gateway := fmt.Sprintf("10.20.%d.1", i)
			return subnet, gateway, nil
		}
	}

	return "", "", fmt.Errorf("no free subnet found in range 10.20.1.0/24 - 10.20.254.0/24")
}

func (nm *NetManager) CreateNetwork(name string) (*Net, error) {
	if nm.NetworkExists(name) {
		return nil, fmt.Errorf("network already exists")
	}

	subnet, gateway, err := nm.findFreeSubnet()
	if err != nil {
		return nil, err
	}

	bridge := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: name,
			MTU:  1500,
		},
	}

	if err := netlink.LinkAdd(bridge); err != nil {
		return nil, err
	}

	addr, _ := netlink.ParseAddr(gateway + "/24")
	_ = netlink.AddrAdd(bridge, addr)

	if err := netlink.LinkSetUp(bridge); err != nil {
		return nil, err
	}

	mainBridge, err := netlink.LinkByName(global.MainNetName)
	if err != nil {
		return nil, fmt.Errorf("zylo0 not found")
	}

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: name + "-veth0",
		},
		PeerName: name + "-veth1",
	}

	if err := netlink.LinkAdd(veth); err != nil {
		return nil, err
	}

	veth0, _ := netlink.LinkByName(name + "-veth0")
	veth1, _ := netlink.LinkByName(name + "-veth1")

	_ = netlink.LinkSetMaster(veth0, bridge)
	_ = netlink.LinkSetMaster(veth1, mainBridge)

	_ = netlink.LinkSetUp(veth0)
	_ = netlink.LinkSetUp(veth1)

	if err := nm.setupNetworkIPTables(subnet, name); err != nil {
		return nil, fmt.Errorf("failed to setup iptables: %v", err)
	}

	config := &Net{
		Name:       name,
		ID:         generateID(),
		Subnet:     subnet,
		Gateway:    gateway,
		Created:    time.Now(),
		Containers: make(map[string]string),
	}

	if err := nm.saveConfig(config); err != nil {
		return nil, err
	}

	return config, nil
}

func (nm *NetManager) SetupIPTables(subnet string) error {
	err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
	if err != nil {
		return fmt.Errorf("could not setup IP forward: %v", err)
	}

	rule := []string{"-s", subnet, "!", "-o", global.MainNetName, "-j", "MASQUERADE"}
	exists, err := nm.ipt.Exists("nat", "POSTROUTING", rule...)
	if err != nil {
		return fmt.Errorf("failed to check rule: %v", err)
	}

	if !exists {
		if err := nm.ipt.Append("nat", "POSTROUTING", rule...); err != nil {
			return fmt.Errorf("failed to add MASQUERADE: %v", err)
		}
	}

	forwardRules := [][]string{
		{"-i", global.MainNetName, "-o", global.MainNetName, "-j", "ACCEPT"},
		{"-i", global.MainNetName, "!", "-o", global.MainNetName, "-j", "ACCEPT"},
		{"!", "-i", global.MainNetName, "-o", global.MainNetName, "-j", "ACCEPT"},
	}

	for _, rule := range forwardRules {
		exists, _ := nm.ipt.Exists("filter", "FORWARD", rule...)
		if !exists {
			nm.ipt.Append("filter", "FORWARD", rule...)
		}
	}

	return nil
}

func (nm *NetManager) saveConfig(config *Net) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(nm.networksDir, config.Name+".json")
	return os.WriteFile(path, data, 0644)
}

func (nm *NetManager) loadConfig(name string) (*Net, error) {
	path := filepath.Join(nm.networksDir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Net
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (nm *NetManager) testNetwork(gateway string) error {
	cmd := exec.Command("ping", "-c", "1", "-W", "1", gateway)
	return cmd.Run()
}

func generateID() string {
	return fmt.Sprintf("zylo-net-%d", time.Now().UnixNano())
}

func (nm *NetManager) getAllNetworks() ([]*Net, error) {
	files, err := filepath.Glob(filepath.Join(nm.networksDir, "*.json"))
	if err != nil {
		return nil, err
	}

	var networks []*Net
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var config Net
		if err := json.Unmarshal(data, &config); err == nil {
			networks = append(networks, &config)
		}
	}

	return networks, nil
}

func GetNetwork(name string) (*Net, error) {
	path := filepath.Join(global.NetPth, name+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNetworkNotFound
		}
		return nil, err
	}

	var net Net
	if err := json.Unmarshal(data, &net); err != nil {
		return nil, err
	}

	return &net, nil
}

func (nm *NetManager) networkExists(name string) bool {
	_, err := os.Stat(filepath.Join(nm.networksDir, name+".json"))
	return err == nil
}
