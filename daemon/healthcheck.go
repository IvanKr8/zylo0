package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
)

type healthStatus struct {
	Time      string `json:"time"`
	Bridge    int    `json:"bridge"`
	IPForward int    `json:"ip_forward"`
	NAT       int    `json:"nat"`
	Socket    int    `json:"socket"`
	Daemon    int    `json:"daemon"`
}

type networkConfig struct {
	Name    string            `json:"name"`
	Subnet  string            `json:"subnet"`
	Gateway string            `json:"gateway"`
	Options map[string]string `json:"options"`
}

func loadNetworkConfig() (*networkConfig, error) {
	data, err := os.ReadFile("/var/lib/zylo/networks/zylo0.json")
	if err != nil {
		return nil, err
	}

	var cfg networkConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func startHeartbeat() {
	for {
		time.Sleep(5 * time.Second)

		status := checkSystemHealth()

		data, _ := json.Marshal(status)
		fmt.Println(string(data))
	}
}

func checkSystemHealth() healthStatus {
	status := healthStatus{
		Time: time.Now().Format("15:04:05"),
	}

	status.Bridge = checkBridge()
	status.IPForward = checkIPForward()
	status.NAT = checkNAT()
	status.Socket = checkSocket()
	status.Daemon = checkDaemon()

	return status
}

func checkBridge() int {
	_, err := netlink.LinkByName("zylo0")
	if err != nil {
		return 500
	}

	link, err := netlink.LinkByName("zylo0")
	if err != nil {
		return 500
	}

	if link.Attrs().Flags&net.FlagUp == 0 {
		return 500
	}

	return 200
}

func checkIPForward() int {
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return 500
	}

	if strings.TrimSpace(string(data)) != "1" {
		return 500
	}

	return 200
}

func checkNAT() int {
	if err := exec.Command("iptables", "-t", "nat", "-C", "ZYLO").Run(); err != nil {
		out, err := exec.Command("iptables", "-t", "nat", "-S").Output()
		if err != nil {
			return 500
		}
		if !strings.Contains(string(out), "-N ZYLO") {
			return 500
		}
	}

	if err := exec.Command("iptables", "-t", "nat", "-C", "PREROUTING", "-j", "ZYLO").Run(); err != nil {
		return 500
	}

	if err := exec.Command("iptables", "-t", "nat", "-C", "OUTPUT", "-j", "ZYLO").Run(); err != nil {
		return 500
	}

	return 200
}

func checkSocket() int {
	if _, err := os.Stat(SocketPath); err != nil {
		return 500
	}

	conn, err := net.DialTimeout("unix", SocketPath, 1*time.Second)
	if err != nil {
		return 500
	}
	conn.Close()

	return 200
}

func checkDaemon() int {
	conn, err := net.DialTimeout("unix", SocketPath, 1*time.Second)
	if err != nil {
		return 500
	}
	defer conn.Close()

	_, err = conn.Write([]byte("ping"))
	if err != nil {
		return 500
	}

	buffer := make([]byte, 16)

	n, err := conn.Read(buffer)
	if err != nil {
		return 500
	}

	if strings.TrimSpace(string(buffer[:n])) != "pong" {
		return 500
	}

	return 200
}
