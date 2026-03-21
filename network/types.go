package network

import (
	"time"
	"zylo/network/dns"

	"github.com/coreos/go-iptables/iptables"
)

type Net struct {
	Name       string                      `json:"name"`
	ID         string                      `json:"id"`
	Type       string                      `json:"type"`
	Driver     string                      `json:"driver"`
	Created    time.Time                   `json:"created"`
	Subnet     string                      `json:"subnet"`
	Gateway    string                      `json:"gateway"`
	IPRange    string                      `json:"ip_range"`
	Containers map[string]ContainerNetInfo `json:"containers"`
	Options    map[string]string           `json:"options"`
	Labels     map[string]string           `json:"labels"`
	Internal   bool                        `json:"internal"`
	DNS        *dns.DNS                    `json:"-"`
}

type NetManager struct {
	networksDir string
	ipt         *iptables.IPTables
}

type IPAM struct {
	nm *NetManager
}

type ContainerNetInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}
