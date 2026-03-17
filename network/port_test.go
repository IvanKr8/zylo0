package network

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestPortForwardAdd(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	fmt.Println("\nTesting PortForward Add...")

	nm, err := NewNetworkManager()
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	containerIP := "10.20.1.100"
	hostPort := 8080
	containerPort := 80

	err = nm.PortForward(containerIP, hostPort, containerPort)
	if err != nil {
		t.Fatalf("PortForward failed: %v", err)
	}
	fmt.Println("   PortForward rule added")

	checkPREROUTING := exec.Command("iptables", "-t", "nat", "-L", "PREROUTING", "-n")
	output, _ := checkPREROUTING.CombinedOutput()
	outputStr := string(output)

	if !strings.Contains(outputStr, containerIP) ||
		!strings.Contains(outputStr, fmt.Sprintf("dpt:%d", hostPort)) {
		t.Errorf("PREROUTING rule not found for %s:%d", containerIP, hostPort)
	} else {
		fmt.Println("   PREROUTING rule verified")
	}

	checkOUTPUT := exec.Command("iptables", "-t", "nat", "-L", "OUTPUT", "-n")
	output, _ = checkOUTPUT.CombinedOutput()
	outputStr = string(output)

	if !strings.Contains(outputStr, containerIP) ||
		!strings.Contains(outputStr, fmt.Sprintf("dpt:%d", hostPort)) {
		t.Errorf("OUTPUT rule not found for %s:%d", containerIP, hostPort)
	} else {
		fmt.Println("   OUTPUT rule verified")
	}

	checkFORWARD := exec.Command("iptables", "-L", "FORWARD", "-n")
	output, _ = checkFORWARD.CombinedOutput()
	outputStr = string(output)

	if !strings.Contains(outputStr, containerIP) ||
		!strings.Contains(outputStr, fmt.Sprintf("dpt:%d", containerPort)) {
		t.Errorf("FORWARD rule not found for %s port %d", containerIP, containerPort)
	} else {
		fmt.Println("   FORWARD rule verified")
	}

	err = nm.PortForward(containerIP, hostPort, containerPort)
	if err != nil {
		t.Errorf("Adding duplicate rule should not fail: %v", err)
	} else {
		fmt.Println("   Duplicate rule handled correctly")
	}

	fmt.Println("TestPortForwardAdd passed")
}

func TestPortForwardRemove(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	fmt.Println("\nTesting PortForward Remove...")

	nm, err := NewNetworkManager()
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	containerIP := "10.20.1.101"
	hostPort := 9090
	containerPort := 443

	err = nm.PortForward(containerIP, hostPort, containerPort)
	if err != nil {
		t.Fatalf("Failed to add port forward for removal test: %v", err)
	}
	fmt.Println("   Added rule for removal test")

	err = nm.RemovePortForward(containerIP, hostPort, containerPort)
	if err != nil {
		t.Fatalf("RemovePortForward failed: %v", err)
	}
	fmt.Println("   Rule removed")

	checkPREROUTING := exec.Command("iptables", "-t", "nat", "-L", "PREROUTING", "-n")
	output, _ := checkPREROUTING.CombinedOutput()
	if strings.Contains(string(output), containerIP) {
		t.Error("PREROUTING rule still exists after removal")
	} else {
		fmt.Println("   PREROUTING rule removed")
	}

	checkOUTPUT := exec.Command("iptables", "-t", "nat", "-L", "OUTPUT", "-n")
	output, _ = checkOUTPUT.CombinedOutput()
	if strings.Contains(string(output), containerIP) {
		t.Error("OUTPUT rule still exists after removal")
	} else {
		fmt.Println("   OUTPUT rule removed")
	}

	checkFORWARD := exec.Command("iptables", "-L", "FORWARD", "-n")
	output, _ = checkFORWARD.CombinedOutput()
	if strings.Contains(string(output), containerIP) {
		t.Error("FORWARD rule still exists after removal")
	} else {
		fmt.Println("   FORWARD rule removed")
	}

	err = nm.RemovePortForward(containerIP, hostPort, containerPort)
	if err != nil {
		t.Errorf("Removing non-existent rule should not fail: %v", err)
	} else {
		fmt.Println("   Removing non-existent rule handled correctly")
	}

	fmt.Println("TestPortForwardRemove passed")
}

func TestPortForwardMultiple(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	fmt.Println("\nTesting PortForward Multiple...")

	nm, err := NewNetworkManager()
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	rules := []struct {
		containerIP   string
		hostPort      int
		containerPort int
	}{
		{"10.20.1.10", 8080, 80},
		{"10.20.1.11", 8443, 443},
		{"10.20.1.12", 5432, 5432},
	}

	for i, rule := range rules {
		err := nm.PortForward(rule.containerIP, rule.hostPort, rule.containerPort)
		if err != nil {
			t.Fatalf("Failed to add rule %d: %v", i, err)
		}
		fmt.Printf("   Added rule %d: %d -> %s:%d\n", i+1, rule.hostPort, rule.containerIP, rule.containerPort)
	}

	checkPREROUTING := exec.Command("iptables", "-t", "nat", "-L", "PREROUTING", "-n")
	output, _ := checkPREROUTING.CombinedOutput()
	outputStr := string(output)

	for _, rule := range rules {
		if !strings.Contains(outputStr, rule.containerIP) {
			t.Errorf("Rule for %s not found in PREROUTING", rule.containerIP)
		}
	}
	fmt.Println("   All PREROUTING rules verified")

	checkFORWARD := exec.Command("iptables", "-L", "FORWARD", "-n")
	output, _ = checkFORWARD.CombinedOutput()
	outputStr = string(output)

	for _, rule := range rules {
		if !strings.Contains(outputStr, rule.containerIP) {
			t.Errorf("FORWARD rule for %s not found", rule.containerIP)
		}
	}
	fmt.Println("   All FORWARD rules verified")

	fmt.Println("TestPortForwardMultiple passed")
}

func TestPortForwardInvalidParams(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	fmt.Println("\nTesting PortForward Invalid Params...")

	nm, err := NewNetworkManager()
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	err = nm.PortForward("invalid-ip", 8080, 80)
	if err == nil {
		t.Error("Expected error for invalid IP, got nil")
	} else {
		fmt.Printf("   Invalid IP correctly handled: %v\n", err)
	}

	err = nm.PortForward("10.20.1.100", 0, 80)
	if err != nil {
		t.Logf("   Port 0 resulted in error: %v", err)
	} else {
		fmt.Println("   Port 0 accepted")
		nm.RemovePortForward("10.20.1.100", 0, 80)
	}

	err = nm.PortForward("10.20.1.100", -1, 80)
	if err == nil {
		t.Error("Expected error for negative port, got nil")
	} else {
		fmt.Printf("   Negative port correctly handled: %v\n", err)
	}

	fmt.Println("TestPortForwardInvalidParams passed")
}

func TestPortForwardLocalhost(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	fmt.Println("\nTesting PortForward Localhost...")

	nm, err := NewNetworkManager()
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	containerIP := "10.20.1.200"
	hostPort := 8888
	containerPort := 8080

	exec.Command("sysctl", "-w", "net.ipv4.conf.all.route_localnet=1").Run()

	err = nm.PortForward(containerIP, hostPort, containerPort)
	if err != nil {
		t.Fatalf("Failed to add port forward: %v", err)
	}

	checkOUTPUT := exec.Command("iptables", "-t", "nat", "-L", "OUTPUT", "-n")
	output, _ := checkOUTPUT.CombinedOutput()
	outputStr := string(output)

	if !strings.Contains(outputStr, containerIP) ||
		!strings.Contains(outputStr, fmt.Sprintf("dpt:%d", hostPort)) {
		t.Error("OUTPUT rule for localhost not found")
	} else {
		fmt.Println("   OUTPUT rule for localhost verified")
	}

	nm.RemovePortForward(containerIP, hostPort, containerPort)

	fmt.Println("TestPortForwardLocalhost passed")
}

func cleanupPortForwardRules() {
	exec.Command("iptables", "-F").Run()
	exec.Command("iptables", "-t", "nat", "-F").Run()
}
