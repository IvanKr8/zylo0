package network

import (
	"fmt"
	"os"
	"testing"
)

func TestIPAMAllocateIP(t *testing.T) {
	fmt.Println("\nTesting IPAM AllocateIP...")

	tmpDir, err := os.MkdirTemp("", "zylo-ipam-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	nm := &NetManager{
		networksDir: tmpDir,
	}

	testNetwork := &Net{
		Name:       "test-net",
		Gateway:    "10.20.1.1",
		Subnet:     "10.20.1.0/24",
		Containers: make(map[string]string),
	}
	nm.saveConfig(testNetwork)

	ipam := NewIPAM(nm)

	ip1, err := ipam.AllocateIP("test-net", "container-1")
	if err != nil {
		t.Fatalf("Failed to allocate first IP: %v", err)
	}
	if ip1 != "10.20.1.2" {
		t.Errorf("Expected first IP 10.20.1.2, got %s", ip1)
	}
	fmt.Printf("   Allocated first IP: %s\n", ip1)

	ip2, err := ipam.AllocateIP("test-net", "container-2")
	if err != nil {
		t.Fatalf("Failed to allocate second IP: %v", err)
	}
	if ip2 != "10.20.1.3" {
		t.Errorf("Expected second IP 10.20.1.3, got %s", ip2)
	}
	fmt.Printf("   Allocated second IP: %s\n", ip2)

	if ip1 == ip2 {
		t.Error("IP addresses are not unique")
	}

	_, err = ipam.AllocateIP("non-existent", "container-3")
	if err == nil {
		t.Error("Expected error for non-existent network, got nil")
	} else {
		fmt.Printf("   Correctly failed for non-existent network: %v\n", err)
	}

	fmt.Println("TestIPAMAllocateIP passed")
}

func TestIPAMGetContainerIP(t *testing.T) {
	fmt.Println("\nTesting IPAM GetContainerIP...")

	tmpDir, _ := os.MkdirTemp("", "zylo-ipam-test-*")
	defer os.RemoveAll(tmpDir)

	nm := &NetManager{
		networksDir: tmpDir,
	}

	testNetwork := &Net{
		Name:       "test-net",
		Gateway:    "10.20.2.1",
		Subnet:     "10.20.2.0/24",
		Containers: map[string]string{"container-1": "10.20.2.2"},
	}
	nm.saveConfig(testNetwork)

	ipam := NewIPAM(nm)

	ip, err := ipam.GetContainerIP("test-net", "container-1")
	if err != nil {
		t.Fatalf("Failed to get container IP: %v", err)
	}
	if ip != "10.20.2.2" {
		t.Errorf("Expected IP 10.20.2.2, got %s", ip)
	}
	fmt.Printf("   Got container IP: %s\n", ip)

	_, err = ipam.GetContainerIP("test-net", "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent container, got nil")
	} else {
		fmt.Printf("   Correctly failed for non-existent container: %v\n", err)
	}

	_, err = ipam.GetContainerIP("non-existent", "container-1")
	if err == nil {
		t.Error("Expected error for non-existent network, got nil")
	} else {
		fmt.Printf("   Correctly failed for non-existent network: %v\n", err)
	}

	fmt.Println("TestIPAMGetContainerIP passed")
}

func TestIPAMReleaseIP(t *testing.T) {
	fmt.Println("\nTesting IPAM ReleaseIP...")

	tmpDir, _ := os.MkdirTemp("", "zylo-ipam-test-*")
	defer os.RemoveAll(tmpDir)

	nm := &NetManager{
		networksDir: tmpDir,
	}

	testNetwork := &Net{
		Name:       "test-net",
		Gateway:    "10.20.3.1",
		Subnet:     "10.20.3.0/24",
		Containers: map[string]string{"container-1": "10.20.3.2"},
	}
	nm.saveConfig(testNetwork)

	ipam := NewIPAM(nm)

	err := ipam.ReleaseIP("test-net", "container-1")
	if err != nil {
		t.Fatalf("Failed to release IP: %v", err)
	}
	fmt.Println("   Released container-1 IP")

	config, _ := nm.GetNetwork("test-net")
	if _, exists := config.Containers["container-1"]; exists {
		t.Error("Container still exists after release")
	}

	err = ipam.ReleaseIP("test-net", "non-existent")
	if err != nil {
		t.Errorf("Releasing non-existent container should not error: %v", err)
	} else {
		fmt.Println("   Releasing non-existent container handled correctly")
	}

	err = ipam.ReleaseIP("non-existent", "container-1")
	if err == nil {
		t.Error("Expected error for non-existent network, got nil")
	} else {
		fmt.Printf("   Correctly failed for non-existent network: %v\n", err)
	}

	fmt.Println("TestIPAMReleaseIP passed")
}

func TestIPAMFullCycle(t *testing.T) {
	fmt.Println("\nTesting IPAM full cycle...")

	tmpDir, _ := os.MkdirTemp("", "zylo-ipam-test-*")
	defer os.RemoveAll(tmpDir)

	nm := &NetManager{
		networksDir: tmpDir,
	}

	testNetwork := &Net{
		Name:       "test-net",
		Gateway:    "10.20.4.1",
		Subnet:     "10.20.4.0/24",
		Containers: make(map[string]string),
	}
	nm.saveConfig(testNetwork)

	ipam := NewIPAM(nm)

	ip1, err := ipam.AllocateIP("test-net", "container-1")
	if err != nil {
		t.Fatalf("Failed to allocate IP: %v", err)
	}
	fmt.Printf("   Allocated: %s\n", ip1)

	ip2, err := ipam.GetContainerIP("test-net", "container-1")
	if err != nil {
		t.Fatalf("Failed to get IP: %v", err)
	}
	if ip1 != ip2 {
		t.Errorf("IP mismatch: %s vs %s", ip1, ip2)
	}
	fmt.Printf("   Got: %s\n", ip2)

	err = ipam.ReleaseIP("test-net", "container-1")
	if err != nil {
		t.Fatalf("Failed to release IP: %v", err)
	}
	fmt.Println("   Released")

	ip3, err := ipam.AllocateIP("test-net", "container-2")
	if err != nil {
		t.Fatalf("Failed to allocate IP after release: %v", err)
	}
	if ip3 != ip1 {
		t.Errorf("Expected same IP %s after release, got %s", ip1, ip3)
	}
	fmt.Printf("   Reallocated: %s\n", ip3)

	fmt.Println("TestIPAMFullCycle passed")
}

func TestIPAMConcurrency(t *testing.T) {
	fmt.Println("\nTesting IPAM no duplicates...")

	tmpDir, _ := os.MkdirTemp("", "zylo-ipam-test-*")
	defer os.RemoveAll(tmpDir)

	nm := &NetManager{
		networksDir: tmpDir,
	}

	testNetwork := &Net{
		Name:       "test-net",
		Gateway:    "10.20.5.1",
		Subnet:     "10.20.5.0/24",
		Containers: make(map[string]string),
	}
	nm.saveConfig(testNetwork)

	ipam := NewIPAM(nm)

	ips := make(map[string]bool)
	for i := 1; i <= 10; i++ {
		containerID := fmt.Sprintf("container-%d", i)
		ip, err := ipam.AllocateIP("test-net", containerID)
		if err != nil {
			t.Fatalf("Failed to allocate IP %d: %v", i, err)
		}

		if ips[ip] {
			t.Errorf("Duplicate IP allocated: %s", ip)
		}
		ips[ip] = true
		fmt.Printf("   Allocated %d: %s\n", i, ip)
	}

	if len(ips) != 10 {
		t.Errorf("Expected 10 unique IPs, got %d", len(ips))
	} else {
		fmt.Println("   All 10 IPs are unique")
	}

	fmt.Println("TestIPAMConcurrency passed")
}

func TestIPAMEdgeCases(t *testing.T) {
	fmt.Println("\nTesting IPAM edge cases...")

	tmpDir, _ := os.MkdirTemp("", "zylo-ipam-test-*")
	defer os.RemoveAll(tmpDir)

	nm := &NetManager{
		networksDir: tmpDir,
	}

	badNetwork := &Net{
		Name:       "bad-net",
		Gateway:    "invalid-ip",
		Subnet:     "10.20.6.0/24",
		Containers: make(map[string]string),
	}
	nm.saveConfig(badNetwork)

	ipam := NewIPAM(nm)

	_, err := ipam.AllocateIP("bad-net", "container-1")
	if err == nil {
		t.Error("Expected error for invalid gateway, got nil")
	} else {
		fmt.Printf("   Correctly failed for invalid gateway: %v\n", err)
	}

	fullNetwork := &Net{
		Name:       "full-net",
		Gateway:    "10.20.7.1",
		Subnet:     "10.20.7.0/24",
		Containers: make(map[string]string),
	}
	nm.saveConfig(fullNetwork)

	for i := 2; i <= 254; i++ {
		containerID := fmt.Sprintf("container-%d", i)
		fullNetwork.Containers[containerID] = fmt.Sprintf("10.20.7.%d", i)
	}
	nm.saveConfig(fullNetwork)

	_, err = ipam.AllocateIP("full-net", "container-255")
	if err == nil {
		t.Error("Expected error when no free IPs, got nil")
	} else {
		fmt.Printf("   Correctly failed when no free IPs: %v\n", err)
	}

	fmt.Println("TestIPAMEdgeCases passed")
}
