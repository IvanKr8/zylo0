package network

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindFreeSubnet(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	nm := &NetManager{
		networksDir: "/tmp/zylo-test-networks",
	}
	os.MkdirAll(nm.networksDir, 0755)
	defer os.RemoveAll(nm.networksDir)

	testConfig := &Net{
		Name:    "test-net",
		Subnet:  "10.20.5.0/24",
		Gateway: "10.20.5.1",
	}
	data, _ := json.Marshal(testConfig)
	os.WriteFile(filepath.Join(nm.networksDir, "test-net.json"), data, 0644)

	subnet, gateway, err := nm.findFreeSubnet()
	if err != nil {
		t.Fatalf("findFreeSubnet failed: %v", err)
	}

	if subnet == "" || gateway == "" {
		t.Fatal("subnet or gateway is empty")
	}
}

func TestSetupIPTables(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	nm, err := NewNetworkManager()
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	subnet := "10.20.3.0/24"

	err = nm.setupIPTables(subnet)
	if err != nil {
		t.Fatalf("setupIPTables failed: %v", err)
	}

	ipForward, _ := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if strings.TrimSpace(string(ipForward)) != "1" {
		t.Error("IP forwarding not enabled")
	}

	rule := []string{"-s", subnet, "!", "-o", "zylo0", "-j", "MASQUERADE"}
	exists, _ := nm.ipt.Exists("nat", "POSTROUTING", rule...)
	if !exists {
		t.Error("MASQUERADE rule not found")
	}

	forwardRules := [][]string{
		{"-i", "zylo0", "-o", "zylo0", "-j", "ACCEPT"},
		{"-i", "zylo0", "!", "-o", "zylo0", "-j", "ACCEPT"},
		{"!", "-i", "zylo0", "-o", "zylo0", "-j", "ACCEPT"},
	}

	for _, rule := range forwardRules {
		exists, _ := nm.ipt.Exists("filter", "FORWARD", rule...)
		if !exists {
			t.Errorf("FORWARD rule not found: %v", rule)
		}
	}
}

func TestSaveLoadConfig(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "zylo-test-*")
	defer os.RemoveAll(tmpDir)

	nm := &NetManager{
		networksDir: tmpDir,
	}

	config := &Net{
		Name:       "test-network",
		ID:         "test-id-123",
		Subnet:     "10.20.4.0/24",
		Gateway:    "10.20.4.1",
		Created:    time.Now(),
		Containers: map[string]string{"container1": "10.20.4.2"},
		Options:    map[string]string{"mtu": "1500"},
	}

	err := nm.saveConfig(config)
	if err != nil {
		t.Fatalf("saveConfig failed: %v", err)
	}

	loaded, err := nm.loadConfig("test-network")
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if loaded.Name != config.Name {
		t.Errorf("Name mismatch: %s != %s", loaded.Name, config.Name)
	}
	if loaded.Subnet != config.Subnet {
		t.Errorf("Subnet mismatch: %s != %s", loaded.Subnet, config.Subnet)
	}
	if loaded.Gateway != config.Gateway {
		t.Errorf("Gateway mismatch: %s != %s", loaded.Gateway, config.Gateway)
	}
	if loaded.Containers["container1"] != "10.20.4.2" {
		t.Error("Container IP not saved correctly")
	}
}

func TestTestNetwork(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	nm := &NetManager{}

	exec.Command("ip", "addr", "add", "10.20.99.1/24", "dev", "lo").Run()
	defer exec.Command("ip", "addr", "del", "10.20.99.1/24", "dev", "lo").Run()

	err := nm.testNetwork("10.20.99.1")
	if err != nil {
		t.Fatalf("testNetwork failed for existing gateway: %v", err)
	}

	err = nm.testNetwork("10.20.99.254")
	if err == nil {
		t.Log("Warning: testNetwork returned success for non-existent IP")
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" || id2 == "" {
		t.Error("Generated ID is empty")
	}

	if id1 == id2 {
		t.Error("Generated IDs are not unique")
	}
}

func TestGetAllNetworks(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "zylo-test-*")
	defer os.RemoveAll(tmpDir)

	nm := &NetManager{
		networksDir: tmpDir,
	}

	for i := 1; i <= 3; i++ {
		config := &Net{
			Name:    fmt.Sprintf("net-%d", i),
			Subnet:  fmt.Sprintf("10.20.%d.0/24", i),
			Gateway: fmt.Sprintf("10.20.%d.1", i),
		}
		nm.saveConfig(config)
	}

	networks, err := nm.getAllNetworks()
	if err != nil {
		t.Fatalf("getAllNetworks failed: %v", err)
	}

	if len(networks) != 3 {
		t.Errorf("Expected 3 networks, got %d", len(networks))
	}
}

func TestNetworkExists(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "zylo-test-*")
	defer os.RemoveAll(tmpDir)

	nm := &NetManager{
		networksDir: tmpDir,
	}

	config := &Net{Name: "test-net"}
	nm.saveConfig(config)

	if !nm.networkExists("test-net") {
		t.Error("networkExists returned false for existing network")
	}

	if nm.networkExists("non-existent") {
		t.Error("networkExists returned true for non-existent network")
	}
}
