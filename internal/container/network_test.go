package kernel

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"

	"zylo/internal/container"
)

// TestSetupNetwork проверяет создание veth пары и подключение к bridge
func TestSetupNetwork(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	// 1. Создаем тестовый контейнер
	cfg := &container.Config{
		Image:   "test-image",
		CopyDir: "",
		Env:     map[string]string{},
		Cmd:     []string{"/bin/sh"},
	}

	ctr := NewContainer(cfg)
	if ctr == nil {
		t.Fatal("Failed to create container")
	}

	// 2. Создаем структуру директорий
	ctr.Rootfs = filepath.Join("/tmp", "test-"+ctr.ID)
	os.MkdirAll(ctr.Rootfs, 0755)

	// 3. Тестируем SetupNetwork
	netCfg := &NetworkConfig{
		NetworkName: "zylo0",
		Ports:       []string{"5432:5432"},
	}

	err := ctr.SetupNetwork(netCfg)
	if err != nil {
		t.Fatalf("SetupNetwork failed: %v", err)
	}

	// 4. Проверяем что veth интерфейс создался
	hostVeth := fmt.Sprintf("veth%s", ctr.ID[:8])
	link, err := netlink.LinkByName(hostVeth)
	if err != nil {
		t.Fatalf("veth interface %s not found: %v", hostVeth, err)
	}

	// 5. Проверяем что это veth
	if link.Type() != "veth" {
		t.Fatalf("interface is not veth, it's %s", link.Type())
	}

	// 6. Проверяем что интерфейс UP
	if link.Attrs().Flags&net.FlagUp == 0 {
		t.Fatal("host veth is not UP")
	}

	// 7. Проверяем что подключен к bridge
	bridge, err := netlink.LinkByName("zylo0")
	if err != nil {
		t.Fatalf("bridge zylo0 not found: %v", err)
	}

	if link.Attrs().MasterIndex != bridge.Attrs().Index {
		t.Fatal("veth not attached to bridge")
	}

	fmt.Println("✅ SetupNetwork test passed")

	// Cleanup
	netlink.LinkDel(link)
}

// TestAttachNetwork проверяет перемещение интерфейса в netns контейнера
func TestAttachNetwork(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	// 1. Создаем тестовый процесс в отдельном netns
	cmd := exec.Command("unshare", "--net", "--fork", "sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}
	defer cmd.Process.Kill()

	time.Sleep(500 * time.Millisecond)
	pid := cmd.Process.Pid

	// 2. Создаем контейнер
	cfg := &container.Config{
		Image:   "test-image",
		CopyDir: "",
		Env:     map[string]string{},
		Cmd:     []string{"/bin/sh"},
	}

	ctr := NewContainer(cfg)
	ctr.pid = pid
	ctr.containerIP = "10.20.1.100"
	ctr.peerName = "eth0"
	ctr.hostVeth = "veth-test"
	ctr.Rootfs = filepath.Join("/tmp", "test-"+ctr.ID)

	// 3. Создаем veth пару заранее
	hostVeth := "veth-test"
	peerName := "eth0"

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: hostVeth,
			MTU:  1500,
		},
		PeerName: peerName,
	}

	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("Failed to create veth: %v", err)
	}
	defer netlink.LinkDel(veth)

	// 4. Подключаем к bridge
	bridge, err := netlink.LinkByName("zylo0")
	if err == nil {
		hostLink, _ := netlink.LinkByName(hostVeth)
		netlink.LinkSetMaster(hostLink, bridge.(*netlink.Bridge))
		netlink.LinkSetUp(hostLink)
	}

	// 5. Тестируем AttachNetwork
	err = ctr.AttachNetwork()
	if err != nil {
		t.Fatalf("AttachNetwork failed: %v", err)
	}

	// 6. Проверяем что интерфейс переместился в контейнер
	containerNs, _ := netns.GetFromPid(pid)
	defer containerNs.Close()

	origns, _ := netns.Get()
	defer origns.Close()

	netns.Set(containerNs)
	defer netns.Set(origns)

	// Проверяем что eth0 есть в контейнере
	link, err := netlink.LinkByName("eth0")
	if err != nil {
		t.Fatalf("eth0 not found in container: %v", err)
	}

	// Проверяем что есть IP
	addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
	if len(addrs) == 0 {
		t.Fatal("eth0 has no IP address")
	}

	fmt.Printf("✅ AttachNetwork test passed, IP: %s\n", addrs[0].IPNet.String())
}

// TestFullNetworkFlow проверяет полный цикл создания сети
func TestFullNetworkFlow(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	// 1. Создаем тестовый процесс
	cmd := exec.Command("unshare", "--net", "--fork", "sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}
	defer cmd.Process.Kill()

	time.Sleep(500 * time.Millisecond)
	pid := cmd.Process.Pid

	// 2. Создаем контейнер
	cfg := &container.Config{
		Image:   "test-image",
		CopyDir: "",
		Env:     map[string]string{},
		Cmd:     []string{"/bin/sh"},
	}

	ctr := NewContainer(cfg)
	ctr.pid = pid
	ctr.Ports = []string{"5432:5432"}
	ctr.Rootfs = filepath.Join("/tmp", "test-"+ctr.ID)

	// 3. SetupNetwork
	netCfg := &NetworkConfig{
		NetworkName: "zylo0",
		Ports:       ctr.Ports,
	}

	if err := ctr.SetupNetwork(netCfg); err != nil {
		t.Fatalf("SetupNetwork failed: %v", err)
	}

	// 4. Проверяем что veth создался на хосте
	hostVeth := fmt.Sprintf("veth%s", ctr.ID[:8])
	hostLink, err := netlink.LinkByName(hostVeth)
	if err != nil {
		t.Fatalf("Host veth not found: %v", err)
	}

	// 5. AttachNetwork
	if err := ctr.AttachNetwork(); err != nil {
		t.Fatalf("AttachNetwork failed: %v", err)
	}

	// 6. Проверяем что интерфейс в контейнере
	containerNs, _ := netns.GetFromPid(pid)
	defer containerNs.Close()

	origns, _ := netns.Get()
	defer origns.Close()

	netns.Set(containerNs)
	defer netns.Set(origns)

	containerLink, err := netlink.LinkByName("eth0")
	if err != nil {
		t.Fatalf("eth0 not found in container: %v", err)
	}

	addrs, _ := netlink.AddrList(containerLink, netlink.FAMILY_V4)
	if len(addrs) == 0 {
		t.Fatal("eth0 has no IP")
	}

	// 7. Проверяем что маршрут добавился
	routes, _ := netlink.RouteList(containerLink, netlink.FAMILY_V4)
	foundDefault := false
	for _, r := range routes {
		if r.Gw != nil {
			foundDefault = true
			break
		}
	}
	if !foundDefault {
		t.Fatal("No default route found in container")
	}

	// 8. Проверяем что iptables правила добавились
	ip := ctr.IP
	checkIPtables := exec.Command("iptables", "-t", "nat", "-L", "PREROUTING", "-n")
	output, _ := checkIPtables.CombinedOutput()
	if !strings.Contains(string(output), ip) {
		t.Logf("Warning: iptables rule for %s not found", ip)
	} else {
		fmt.Printf("✅ iptables rule for %s found\n", ip)
	}

	// Cleanup
	netlink.LinkDel(hostLink)
	fmt.Println("✅ Full network flow test passed")
}

func TestMain(m *testing.M) {
	fmt.Println("🔧 Setting up network tests...")

	// Проверяем что zylo0 существует
	_, err := netlink.LinkByName("zylo0")
	if err != nil {
		fmt.Println("⚠️ zylo0 bridge not found, some tests may fail")
	}

	code := m.Run()
	os.Exit(code)
}
