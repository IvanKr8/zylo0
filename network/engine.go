package network

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"zylo/global"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
)

const (
	MIN_SUBNET = 1
	MAX_SUBNET = 254
	BASE_IP    = "10.20"
)

func FlushZyloRules() error {
	return exec.Command("iptables", "-t", "nat", "-F", "ZYLO").Run()
}

func InitZyloNat() error {
	exec.Command("iptables", "-t", "nat", "-N", "ZYLO").Run()

	if err := exec.Command("iptables", "-t", "nat", "-C", "PREROUTING", "-j", "ZYLO").Run(); err != nil {
		if err := exec.Command("iptables", "-t", "nat", "-A", "PREROUTING", "-j", "ZYLO").Run(); err != nil {
			return fmt.Errorf("failed to link PREROUTING to ZYLO: %v", err)
		}
	}

	if err := exec.Command("iptables", "-t", "nat", "-C", "OUTPUT", "-j", "ZYLO").Run(); err != nil {
		if err := exec.Command("iptables", "-t", "nat", "-A", "OUTPUT", "-j", "ZYLO").Run(); err != nil {
			return fmt.Errorf("failed to link OUTPUT to ZYLO: %v", err)
		}
	}

	return nil
}

func RemovePortForward(hostPort string, containerIP string, containerPort string) {
	exec.Command("iptables", "-t", "nat", "-D", "ZYLO", "-p", "tcp", "--dport", hostPort,
		"-j", "DNAT", "--to-destination", containerIP+":"+containerPort).Run()
	exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING",
		"-d", containerIP, "-p", "tcp", "--dport", containerPort, "-j", "MASQUERADE").Run()
}

func AddMasquerade(containerIP string, containerPort int) {
	exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING",
		"-d", containerIP, "-p", "tcp", "--dport", strconv.Itoa(containerPort), "-j", "MASQUERADE").Run()
}

func AddPortForward(hostPort int, containerIP string, containerPort int) error {
	cmd := exec.Command("iptables", "-t", "nat", "-A", "ZYLO", "-p", "tcp", "--dport", strconv.Itoa(hostPort),
		"-j", "DNAT", "--to-destination", containerIP+":"+strconv.Itoa(containerPort))
	return cmd.Run()
}

func NewNetworkManager() (*NetManager, error) {
	ipt, err := iptables.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize iptables: %v", err)
	}

	if err := os.MkdirAll(global.NetPth, 0755); err != nil {
		return nil, err
	}

	return &NetManager{
		networksDir: global.NetPth,
		ipt:         ipt,
	}, nil
}

func (nm *NetManager) rebuildConfigFromBridge(link netlink.Link) error {
	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil || len(addrs) == 0 {
		return fmt.Errorf("cannot rebuild config: no IP on bridge")
	}

	ip := addrs[0].IP
	gateway := ip.String()

	subnet := gateway + "/16"

	config := &Net{
		Name:       global.MainNetName,
		ID:         generateID(),
		Subnet:     subnet,
		Gateway:    gateway,
		Created:    time.Now(),
		Containers: make(map[string]string),
		Options: map[string]string{
			"mtu":     "1500",
			"icc":     "true",
			"ip_masq": "true",
		},
	}

	return nm.saveConfig(config)
}

func (nm *NetManager) SyncExistingBridge(link netlink.Link) error {
	if link.Attrs().Flags&net.FlagUp == 0 {
		if err := netlink.LinkSetUp(link); err != nil {
			return err
		}
	}

	addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)

	if len(addrs) == 0 {
		config, err := nm.GetNetwork(global.MainNetName)
		if err != nil {
			return nm.rebuildConfigFromBridge(link)
		}

		addr, err := netlink.ParseAddr(config.Gateway + "/16")
		if err == nil {
			netlink.AddrAdd(link, addr)
		}
	}

	if nm.NetworkExists(global.MainNetName) {
		config, err := nm.GetNetwork(global.MainNetName)
		if err == nil {
			nm.ensureIPTables(config.Subnet)
		}
	}

	return nil
}

func (nm *NetManager) CreateDefaultNetwork() error {
	if link, err := netlink.LinkByName(global.MainNetName); err == nil && link != nil {
		return nm.SyncExistingBridge(link)
	}

	subnet, gateway, err := nm.findFreeSubnet()
	if err != nil {
		return fmt.Errorf("failed to find free subnet: %v", err)
	}

	bridge := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: global.MainNetName,
			MTU:  1500,
		},
	}

	if err := netlink.LinkAdd(bridge); err != nil {
		return fmt.Errorf("failed to create bridge: %v", err)
	}

	ipAddr, err := netlink.ParseAddr(gateway + "/16")
	if err != nil {
		netlink.LinkDel(bridge)
		return fmt.Errorf("failed to parse IP: %v", err)
	}

	if err := netlink.AddrAdd(bridge, ipAddr); err != nil {
		netlink.LinkDel(bridge)
		return fmt.Errorf("failed to add IP: %v", err)
	}

	if err := netlink.LinkSetUp(bridge); err != nil {
		netlink.LinkDel(bridge)
		return fmt.Errorf("failed to set bridge up: %v", err)
	}

	os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
	os.WriteFile("/proc/sys/net/ipv4/conf/all/route_localnet", []byte("1"), 0644)

	if err := nm.setupBaseIPTables(subnet); err != nil {
		return fmt.Errorf("failed to setup iptables: %v", err)
	}

	config := &Net{
		Name:       global.MainNetName,
		ID:         generateID(),
		Subnet:     subnet,
		Gateway:    gateway,
		Created:    time.Now(),
		Containers: make(map[string]string),
		Options: map[string]string{
			"mtu":     "1500",
			"icc":     "true",
			"ip_masq": "true",
		},
	}

	return nm.saveConfig(config)
}

func (nm *NetManager) setupNetworkIPTables(subnet, bridgeName string) error {

	// 1️⃣ Включаем IP forwarding
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
		return fmt.Errorf("failed to enable ip_forward: %v", err)
	}

	// 2️⃣ MASQUERADE (NAT в интернет)
	masqRule := []string{
		"-s", subnet,
		"!", "-o", global.MainNetName,
		"-j", "MASQUERADE",
	}

	exists, _ := nm.ipt.Exists("nat", "POSTROUTING", masqRule...)
	if !exists {
		if err := nm.ipt.Append("nat", "POSTROUTING", masqRule...); err != nil {
			return fmt.Errorf("failed to add MASQUERADE: %v", err)
		}
	}

	// 3️⃣ FORWARD правила

	// Разрешаем уже установленные соединения
	established := []string{
		"-m", "conntrack",
		"--ctstate", "ESTABLISHED,RELATED",
		"-j", "ACCEPT",
	}
	nm.ipt.Append("filter", "FORWARD", established...)

	// Разрешаем контейнерам выход в интернет
	out := []string{
		"-s", subnet,
		"-o", global.MainNetName,
		"-j", "ACCEPT",
	}
	nm.ipt.Append("filter", "FORWARD", out...)

	in := []string{
		"-d", subnet,
		"-i", global.MainNetName,
		"-j", "ACCEPT",
	}
	nm.ipt.Append("filter", "FORWARD", in...)

	return nil
}

func (nm *NetManager) setupBaseIPTables(subnet string) error {

	masqRule := []string{"-s", subnet, "!", "-o", global.MainNetName, "-j", "MASQUERADE"}
	exists, _ := nm.ipt.Exists("nat", "POSTROUTING", masqRule...)
	if !exists {
		if err := nm.ipt.Append("nat", "POSTROUTING", masqRule...); err != nil {
			return fmt.Errorf("failed to add MASQUERADE: %v", err)
		}
	}

	forwardRules := [][]string{
		{"-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
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

func (nm *NetManager) ListNetworks() ([]*Net, error) {
	files, err := filepath.Glob(filepath.Join(nm.networksDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %v", err)
	}

	var networks []*Net
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var config Net
		if err := json.Unmarshal(data, &config); err != nil {
			continue
		}

		networks = append(networks, &config)
	}

	return networks, nil
}

func (nm *NetManager) GetNetwork(name string) (*Net, error) {
	path := filepath.Join(nm.networksDir, name+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNetworkNotFound
		}
		return nil, err
	}

	var config Net
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (nm *NetManager) NetworkExists(name string) bool {
	_, err := os.Stat(filepath.Join(nm.networksDir, name+".json"))
	return err == nil
}

func (nm *NetManager) DeleteNetwork(name string) error {
	if link, err := netlink.LinkByName(name); err == nil {
		if err := netlink.LinkDel(link); err != nil {
			return fmt.Errorf("failed to delete bridge %s: %v", name, err)
		}
	}

	path := filepath.Join(nm.networksDir, name+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete config: %v", err)
	}

	return nil
}

func (nm *NetManager) CleanupNetworkRules(subnet, bridgeName string) error {
	rule := []string{"-s", subnet, "!", "-o", bridgeName, "-j", "MASQUERADE"}
	nm.ipt.Delete("nat", "POSTROUTING", rule...)

	forwardRules := [][]string{
		{"-i", bridgeName, "-o", bridgeName, "-j", "ACCEPT"},
		{"-i", bridgeName, "!", "-o", bridgeName, "-j", "ACCEPT"},
		{"!", "-i", bridgeName, "-o", bridgeName, "-j", "ACCEPT"},
	}

	for _, rule := range forwardRules {
		nm.ipt.Delete("filter", "FORWARD", rule...)
	}

	return nil
}

func (nm *NetManager) EnsureDefaultNetwork() error {
	configPath := filepath.Join(nm.networksDir, global.MainNetName+".json")

	_, configErr := os.Stat(configPath)
	link, linkErr := netlink.LinkByName(global.MainNetName)

	if os.IsNotExist(configErr) && linkErr != nil {
		return nm.CreateDefaultNetwork()
	}

	if linkErr == nil && os.IsNotExist(configErr) {
		return nm.rebuildConfigFromBridge(link)
	}

	if linkErr != nil && !os.IsNotExist(configErr) {
		return nm.CreateDefaultNetwork()
	}

	if link.Type() != "bridge" {
		if err := netlink.LinkDel(link); err != nil {
			return err
		}
		return nm.CreateDefaultNetwork()
	}

	return nm.SyncExistingBridge(link)
}

func (nm *NetManager) ensureIPTables(subnet string) error {
	os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
	os.WriteFile("/proc/sys/net/ipv4/conf/all/route_localnet", []byte("1"), 0644)

	rule := []string{"-s", subnet, "!", "-o", global.MainNetName, "-j", "MASQUERADE"}
	exists, _ := nm.ipt.Exists("nat", "POSTROUTING", rule...)
	if !exists {
		nm.ipt.Append("nat", "POSTROUTING", rule...)
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
