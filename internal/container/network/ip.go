package network

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"zylo/internal/global"
)

func generateUniqueIP() (string, string, error) {
	used := make(map[string]bool)

	files, _ := os.ReadDir(global.NetPth)
	for _, f := range files {
		data, _ := os.ReadFile(filepath.Join(global.NetPth, f.Name()))
		var netConfig network
		_ = json.Unmarshal(data, &netConfig)
		ip := strings.Split(netConfig.IP, "/")[0]
		used[ip] = true
	}

	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ip := strings.Split(addr.String(), "/")[0]
			used[ip] = true
		}
	}

	for i := MINIPVALUE; i < MAXIPVALUE; i++ {
		baseIP := fmt.Sprintf("10.20.%d.1", i)
		if !used[baseIP] {
			subnet := fmt.Sprintf("10.20.%d.0/24", i)
			return baseIP + "/24", subnet, nil
		}
	}

	return "", "", fmt.Errorf("no available subnets")
}
