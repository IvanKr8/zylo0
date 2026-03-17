package container

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zylo/global"
	"zylo/network"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func TestSetupNetwork(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	cfg := &Config{
		Image:      "test-image",
		OpenPorts:  []string{"8080:8080"},
		SetWorkdir: "/test",
		CmdPath:    "/tmp",
		Cmd:        []string{"/bin/sh"},
		Env:        map[string]string{},
	}

	ctr := newContainer(cfg)
	if ctr == nil {
		t.Fatal("newContainer failed")
	}

	ctr.Rootfs = filepath.Join("/tmp", "test-"+ctr.ID)
	if err := os.MkdirAll(ctr.Rootfs, 0755); err != nil {
		t.Fatalf("Failed to create rootfs: %v", err)
	}
	defer os.RemoveAll(ctr.Rootfs)

	netCfg := &NetworkConfig{
		NetworkName: global.MainNetName,
		Ports:       cfg.OpenPorts,
	}

	if err := ctr.SetupNetwork(netCfg); err != nil {
		t.Fatalf("SetupNetwork failed: %v", err)
	}
	defer func() {
		exec.Command("ip", "link", "del", ctr.hostVeth).Run()
	}()

	if ctr.IP == "" {
		t.Fatal("Container IP not set")
	}
	if ctr.containerIP == "" {
		t.Fatal("containerIP not set")
	}
	if ctr.hostVeth == "" {
		t.Fatal("hostVeth not set")
	}
	if ctr.peerName == "" {
		t.Fatal("peerName not set")
	}

	link, err := netlink.LinkByName(ctr.hostVeth)
	if err != nil {
		t.Fatalf("Host veth %s not found: %v", ctr.hostVeth, err)
	}

	if link.Attrs().Flags&net.FlagUp == 0 {
		t.Fatal("Host veth is not UP")
	}
}

func TestAttachNetwork(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	cmd := exec.Command("unshare", "--net", "--fork", "sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}
	defer cmd.Process.Kill()

	time.Sleep(500 * time.Millisecond)

	cfg := &Config{
		Image:      "test-image",
		OpenPorts:  []string{},
		SetWorkdir: "/test",
		CmdPath:    "/tmp",
		Cmd:        []string{"/bin/sh"},
		Env:        map[string]string{},
	}

	ctr := newContainer(cfg)
	ctr.Pid = cmd.Process.Pid
	ctr.containerIP = "10.20.1.100"
	ctr.peerName = "eth-test"
	ctr.hostVeth = "veth-test"

	nm, _ := network.NewNetworkManager()
	ctr.networkManager = nm

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: ctr.hostVeth,
			MTU:  1500,
		},
		PeerName: ctr.peerName,
	}

	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("Failed to create veth: %v", err)
	}
	defer netlink.LinkDel(veth)

	bridge, err := netlink.LinkByName(global.MainNetName)
	if err == nil {
		hostLink, _ := netlink.LinkByName(ctr.hostVeth)
		netlink.LinkSetMaster(hostLink, bridge.(*netlink.Bridge))
		netlink.LinkSetUp(hostLink)
	}

	if err := ctr.AttachNetwork(); err != nil {
		t.Fatalf("AttachNetwork failed: %v", err)
	}

	containerNs, err := netns.GetFromPid(ctr.Pid)
	if err != nil {
		t.Fatalf("Failed to get container netns: %v", err)
	}
	defer containerNs.Close()

	origns, _ := netns.Get()
	defer origns.Close()

	netns.Set(containerNs)
	defer netns.Set(origns)

	link, err := netlink.LinkByName(ctr.peerName)
	if err != nil {
		t.Fatalf("Interface %s not found in container: %v", ctr.peerName, err)
	}

	addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
	if len(addrs) == 0 {
		t.Fatal("Interface has no IP address")
	}
}

func TestCleanupNetwork(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	cfg := &Config{
		Image:      "test-image",
		OpenPorts:  []string{"7070:7070"},
		SetWorkdir: "/test",
		CmdPath:    "/tmp",
		Cmd:        []string{"/bin/sh"},
		Env:        map[string]string{},
	}

	ctr := newContainer(cfg)
	ctr.IP = "10.20.1.150"
	ctr.Ports = []string{"7070:7070"}
	ctr.hostVeth = "veth-test"
	ctr.containerIP = "10.20.1.150"

	nm, _ := network.NewNetworkManager()
	ctr.networkManager = nm

	ipam := network.NewIPAM(nm)
	ipam.AllocateIP(global.MainNetName, ctr.ID)
	nm.PortForward(ctr.IP, 7070, 7070)

	if err := ctr.CleanupNetwork(); err != nil {
		t.Fatalf("CleanupNetwork failed: %v", err)
	}

	checkCmd := exec.Command("iptables", "-t", "nat", "-L", "PREROUTING", "-n")
	output, _ := checkCmd.CombinedOutput()
	if strings.Contains(string(output), "10.20.1.150:7070") {
		t.Fatal("PREROUTING rule still exists after cleanup")
	}

	_, err := ipam.GetContainerIP(global.MainNetName, ctr.ID)
	if err == nil {
		t.Fatal("IP still registered in IPAM after cleanup")
	}

	cmd := exec.Command("ip", "link", "show", ctr.hostVeth)
	if err := cmd.Run(); err == nil {
		t.Fatal("Veth interface still exists after cleanup")
	}
}

func TestFullNetworkFlow(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	cmd := exec.Command("unshare", "--net", "--fork", "sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}
	defer cmd.Process.Kill()

	time.Sleep(500 * time.Millisecond)

	cfg := &Config{
		Image:      "test-image",
		OpenPorts:  []string{"9090:9090"},
		SetWorkdir: "/test",
		CmdPath:    "/tmp",
		Cmd:        []string{"/bin/sh"},
		Env:        map[string]string{},
	}

	ctr := newContainer(cfg)
	ctr.Pid = cmd.Process.Pid
	ctr.Rootfs = filepath.Join("/tmp", "test-"+ctr.ID)

	if err := os.MkdirAll(ctr.Rootfs, 0755); err != nil {
		t.Fatalf("Failed to create rootfs: %v", err)
	}
	defer os.RemoveAll(ctr.Rootfs)

	netCfg := &NetworkConfig{
		NetworkName: global.MainNetName,
		Ports:       cfg.OpenPorts,
	}

	if err := ctr.SetupNetwork(netCfg); err != nil {
		t.Fatalf("SetupNetwork failed: %v", err)
	}
	defer func() {
		exec.Command("ip", "link", "del", ctr.hostVeth).Run()
		ctr.CleanupNetwork()
	}()

	if err := ctr.AttachNetwork(); err != nil {
		t.Fatalf("AttachNetwork failed: %v", err)
	}

	if ctr.IP == "" {
		t.Fatal("Container IP not set")
	}

	containerNs, err := netns.GetFromPid(ctr.Pid)
	if err != nil {
		t.Fatalf("Failed to get container netns: %v", err)
	}
	defer containerNs.Close()

	origns, _ := netns.Get()
	defer origns.Close()

	netns.Set(containerNs)
	defer netns.Set(origns)

	link, err := netlink.LinkByName(ctr.peerName)
	if err != nil {
		t.Fatalf("Interface %s not found in container: %v", ctr.peerName, err)
	}

	addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
	if len(addrs) == 0 {
		t.Fatal("Interface has no IP address")
	}
}

func TestMain(m *testing.M) {
	nm, err := network.NewNetworkManager()
	if err != nil {
		os.Exit(1)
	}

	if !nm.NetworkExists(global.MainNetName) {
		if err := nm.CreateDefaultNetwork(); err != nil {
			os.Exit(1)
		}
	}

	code := m.Run()

	exec.Command("iptables", "-t", "nat", "-F", "ZYLO").Run()

	os.Exit(code)
}
