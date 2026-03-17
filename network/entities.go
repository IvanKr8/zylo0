package network

import (
	"time"

	"github.com/coreos/go-iptables/iptables"
)

type Net struct {
	Name       string            `json:"name"`
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Driver     string            `json:"driver"`
	Created    time.Time         `json:"created"`
	Subnet     string            `json:"subnet"`
	Gateway    string            `json:"gateway"`
	IPRange    string            `json:"ip_range"`
	Containers map[string]string `json:"containers"`
	Options    map[string]string `json:"options"`
	Labels     map[string]string `json:"labels"`
	Internal   bool              `json:"internal"`
}

type NetManager struct {
	networksDir string
	ipt         *iptables.IPTables
}
