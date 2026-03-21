package network

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"zylo/global"

	"github.com/coreos/go-iptables/iptables"
)

const (
	MIN_SUBNET = 1
	MAX_SUBNET = 254
	BASE_IP    = "10.20"
)

func (nm *NetManager) EnsureDefaultNetwork() error {
	netName := global.MainNetName

	exists := nm.NetworkExists(netName)
	if !exists {
		subnet, gateway, err := nm.findFreeSubnet()
		if err != nil {
			return fmt.Errorf("cannot find free subnet: %v", err)
		}

		netCfg := &Net{
			Name:    netName,
			ID:      generateID(),
			Subnet:  subnet,
			Gateway: gateway,
			Driver:  "bridge",
			Type:    "bridge",
			Created: time.Now(),
		}

		if err := nm.SaveConfig(netCfg); err != nil {
			return fmt.Errorf("cannot save main network config: %v", err)
		}

		if err := nm.restoreNetwork(netCfg); err != nil {
			return fmt.Errorf("cannot create main network bridge: %v", err)
		}

		return nil
	}

	cfg, err := nm.GetNetwork(netName)
	if err != nil {
		return fmt.Errorf("cannot load main network config: %v", err)
	}

	if !nm.bridgeExists(cfg.Name) {
		if err := nm.restoreNetwork(cfg); err != nil {
			return fmt.Errorf("cannot restore main network bridge: %v", err)
		}
	}

	return nil
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
		subnet := fmt.Sprintf("%s.%d.0/24", BASE_IP, i)

		if !used[subnet] {
			gateway := fmt.Sprintf("%s.%d.1", BASE_IP, i)
			return subnet, gateway, nil
		}
	}

	return "", "", fmt.Errorf("no free subnet found in range 10.20.1.0/24 - 10.20.254.0/24")
}
