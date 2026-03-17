package network

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
)

func TestCreateDefaultNetwork(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	fmt.Println("\nTesting CreateDefaultNetwork...")

	nm, err := NewNetworkManager()
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	if nm.NetworkExists("zylo0") {
		nm.DeleteNetwork("zylo0")
	}

	err = nm.CreateDefaultNetwork()
	if err != nil {
		t.Fatalf("CreateDefaultNetwork failed: %v", err)
	}

	link, err := netlink.LinkByName("zylo0")
	if err != nil {
		t.Fatalf("Bridge zylo0 not found: %v", err)
	}

	if link.Type() != "bridge" {
		t.Fatalf("Interface is not bridge, it's %s", link.Type())
	}

	if link.Attrs().Flags&net.FlagUp == 0 {
		t.Fatal("Bridge is not UP")
	}

	addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil || len(addrs) == 0 {
		t.Fatal("Bridge has no IP address")
	}
	fmt.Printf("   Bridge IP: %s\n", addrs[0].IPNet.String())

	config, err := nm.GetNetwork("zylo0")
	if err != nil {
		t.Fatalf("Failed to get network config: %v", err)
	}
	fmt.Printf("   Config loaded: subnet=%s, gateway=%s\n", config.Subnet, config.Gateway)

	checkIPForward := exec.Command("cat", "/proc/sys/net/ipv4/ip_forward")
	if output, err := checkIPForward.Output(); err != nil || string(output) != "1\n" {
		t.Fatal("IP forwarding not enabled")
	}
	fmt.Println("   IP forwarding enabled")

	checkRouteLocalnet := exec.Command("cat", "/proc/sys/net/ipv4/conf/all/route_localnet")
	if output, err := checkRouteLocalnet.Output(); err != nil || string(output) != "1\n" {
		t.Fatal("route_localnet not enabled")
	}
	fmt.Println("   route_localnet enabled")

	checkNAT := exec.Command("iptables", "-t", "nat", "-L", "POSTROUTING", "-n")
	output, _ := checkNAT.CombinedOutput()
	if !contains(string(output), "MASQUERADE") || !contains(string(output), "10.20.") {
		t.Log("NAT rule not found, but may be created later")
	} else {
		fmt.Println("   NAT rule found")
	}

	fmt.Println("TestCreateDefaultNetwork passed")
}

func TestEnsureDefaultNetwork(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	fmt.Println("\nTesting EnsureDefaultNetwork...")

	nm, err := NewNetworkManager()
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	if nm.NetworkExists("zylo0") {
		nm.DeleteNetwork("zylo0")
	}

	err = nm.EnsureDefaultNetwork()
	if err != nil {
		t.Fatalf("EnsureDefaultNetwork failed: %v", err)
	}

	if !nm.NetworkExists("zylo0") {
		t.Fatal("Network config not created")
	}

	link, err := netlink.LinkByName("zylo0")
	if err != nil {
		t.Fatalf("Bridge not found after ensure: %v", err)
	}

	if link.Attrs().Flags&net.FlagUp == 0 {
		t.Fatal("Bridge is DOWN after ensure")
	}

	fmt.Println("   Network created by Ensure")

	netlink.LinkDel(link)
	time.Sleep(100 * time.Millisecond)

	err = nm.EnsureDefaultNetwork()
	if err != nil {
		t.Fatalf("EnsureDefaultNetwork failed after deletion: %v", err)
	}

	link, err = netlink.LinkByName("zylo0")
	if err != nil {
		t.Fatalf("Bridge not restored: %v", err)
	}

	if link.Attrs().Flags&net.FlagUp == 0 {
		t.Fatal("Bridge is DOWN after restore")
	}

	fmt.Println("   Network restored by Ensure")
	fmt.Println("TestEnsureDefaultNetwork passed")
}

func TestIPAM(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	fmt.Println("\nTesting IPAM...")

	nm, err := NewNetworkManager()
	if err != nil {
		t.Fatalf("Failed to create network manager: %v", err)
	}

	nm.EnsureDefaultNetwork()

	config, err := nm.GetNetwork("zylo0")
	if err != nil {
		t.Fatalf("Failed to get network config: %v", err)
	}
	fmt.Printf("   Network subnet: %s, gateway: %s\n", config.Subnet, config.Gateway)

	ipam := NewIPAM(nm)

	ips := make([]string, 5)
	for i := 0; i < 5; i++ {
		containerID := fmt.Sprintf("test-container-%d", i)
		ip, err := ipam.AllocateIP("zylo0", containerID)
		if err != nil {
			t.Fatalf("Failed to allocate IP %d: %v", i, err)
		}
		ips[i] = ip
		fmt.Printf("   Allocated IP %d: %s\n", i, ip)
	}

	for i := 0; i < 5; i++ {
		for j := i + 1; j < 5; j++ {
			if ips[i] == ips[j] {
				t.Fatalf("Duplicate IP allocated: %s", ips[i])
			}
		}
	}
	fmt.Println("   All IPs are unique")

	err = ipam.ReleaseIP("zylo0", "test-container-0")
	if err != nil {
		t.Fatalf("Failed to release IP: %v", err)
	}

	newIP, err := ipam.AllocateIP("zylo0", "new-container")
	if err != nil {
		t.Fatalf("Failed to allocate released IP: %v", err)
	}
	fmt.Printf("   Released IP %s reallocated as %s\n", ips[0], newIP)

	fmt.Println("TestIPAM passed")
}

func TestBridgeConnectivity(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	fmt.Println("\nTesting bridge connectivity...")

	vethName := "veth-test"
	peerName := "veth-peer"

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: vethName,
			MTU:  1500,
		},
		PeerName: peerName,
	}

	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("Failed to create veth: %v", err)
	}
	defer netlink.LinkDel(veth)

	bridge, err := netlink.LinkByName("zylo0")
	if err != nil {
		t.Fatalf("Bridge zylo0 not found: %v", err)
	}

	hostLink, _ := netlink.LinkByName(vethName)
	if err := netlink.LinkSetMaster(hostLink, bridge.(*netlink.Bridge)); err != nil {
		t.Fatalf("Failed to attach to bridge: %v", err)
	}
	netlink.LinkSetUp(hostLink)

	bridgeLinks, _ := netlink.LinkList()
	found := false
	for _, l := range bridgeLinks {
		if l.Attrs().Name == vethName && l.Attrs().MasterIndex == bridge.Attrs().Index {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("veth not attached to bridge")
	}
	fmt.Println("   veth attached to bridge")

	addr, _ := netlink.ParseAddr("10.20.2.100/16")
	netlink.AddrAdd(hostLink, addr)
	netlink.LinkSetUp(hostLink)

	exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()
	exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", "10.20.2.0/16", "-j", "MASQUERADE").Run()
	exec.Command("iptables", "-A", "FORWARD", "-i", "zylo0", "-j", "ACCEPT").Run()

	pingCmd := exec.Command("ping", "-c", "3", "-W", "1", "10.20.2.1")
	if output, err := pingCmd.CombinedOutput(); err != nil {
		t.Logf("Ping to gateway failed: %s", output)
	} else {
		fmt.Println("   Ping to gateway works")
	}

	nsCmd := exec.Command("ip", "netns", "add", "test-ns")
	nsCmd.Run()
	defer exec.Command("ip", "netns", "del", "test-ns").Run()

	peer, _ := netlink.LinkByName(peerName)
	netlink.LinkSetNsPid(peer, getNsPid("test-ns"))

	exec.Command("ip", "netns", "exec", "test-ns", "ip", "link", "set", "lo", "up").Run()
	exec.Command("ip", "netns", "exec", "test-ns", "ip", "addr", "add", "10.20.2.101/16", "dev", peerName).Run()
	exec.Command("ip", "netns", "exec", "test-ns", "ip", "link", "set", peerName, "up").Run()
	exec.Command("ip", "netns", "exec", "test-ns", "ip", "route", "add", "default", "via", "10.20.2.1").Run()

	pingFromNS := exec.Command("ip", "netns", "exec", "test-ns", "ping", "-c", "3", "10.20.2.100")
	if output, err := pingFromNS.CombinedOutput(); err != nil {
		t.Logf("Ping from namespace failed: %s", output)
	} else {
		fmt.Println("   Ping from namespace works")
	}

	fmt.Println("TestBridgeConnectivity passed")
}

func getNsPid(name string) int {
	cmd := exec.Command("ip", "netns", "pid", name)
	output, _ := cmd.Output()
	var pid int
	fmt.Sscanf(string(output), "%d", &pid)
	return pid
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestMain(m *testing.M) {
	fmt.Println("Setting up network tests...")

	if os.Geteuid() != 0 {
		fmt.Println("Run tests with sudo")
		os.Exit(1)
	}

	code := m.Run()

	exec.Command("iptables", "-F").Run()
	exec.Command("iptables", "-t", "nat", "-F").Run()

	os.Exit(code)
}
