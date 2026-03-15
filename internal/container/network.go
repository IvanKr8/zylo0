package kernel

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"zylo/global"
	"zylo/network"

	"github.com/vishvananda/netlink"
)

type NetworkConfig struct {
	NetworkName string
	Ports       []string
}

func (c *Container) SetupNetwork(netCfg *NetworkConfig) error {
	fmt.Println("\n🔧 Setting up network...")

	mainNet := global.MainNetName

	nm, err := network.NewNetworkManager()
	if err != nil {
		return fmt.Errorf("failed to create network manager: %v", err)
	}
	c.networkManager = nm

	if !nm.NetworkExists(mainNet) {
		fmt.Println("   Creating zylo0 network...")
		if err := nm.CreateDefaultNetwork(); err != nil {
			return fmt.Errorf("failed to create default network: %v", err)
		}
	}

	bridge, err := netlink.LinkByName(mainNet)
	if err != nil {
		return fmt.Errorf("bridge zylo0 not found: %v", err)
	}
	if bridge.Attrs().Flags&net.FlagUp == 0 {
		if err := netlink.LinkSetUp(bridge); err != nil {
			return fmt.Errorf("failed to set bridge up: %v", err)
		}
	}
	fmt.Println("   ✅ Bridge zylo0 is UP")

	ipam := network.NewIPAM(nm)
	containerIP, err := ipam.AllocateIP(mainNet, c.ID)
	if err != nil {
		return fmt.Errorf("failed to allocate IP: %v", err)
	}
	c.IP = containerIP
	c.containerIP = containerIP
	fmt.Printf("   ✅ Allocated IP: %s\n", containerIP)

	hostVeth := fmt.Sprintf("veth-%s", c.ID[:8])
	peerName := fmt.Sprintf("eth-%s", c.ID[:8])

	// Удаляем старые интерфейсы если есть
	exec.Command("ip", "link", "del", hostVeth).Run()
	exec.Command("ip", "link", "del", peerName).Run()
	time.Sleep(100 * time.Millisecond)

	// Создаём veth пару
	fmt.Printf("   Creating veth pair: %s <-> %s\n", hostVeth, peerName)
	out, err := exec.Command("ip", "link", "add", hostVeth, "type", "veth", "peer", "name", peerName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create veth: %s", out)
	}
	fmt.Println("   ✅ veth pair created")

	// Подключаем к bridge
	out, err = exec.Command("ip", "link", "set", hostVeth, "master", "zylo0").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to attach to bridge: %s", out)
	}
	fmt.Println("   ✅ Attached to bridge")

	// Поднимаем интерфейс на хосте
	out, err = exec.Command("ip", "link", "set", hostVeth, "up").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set host veth up: %s", out)
	}
	fmt.Println("   ✅ Host veth is UP")

	// Проверяем что интерфейс действительно в bridge
	time.Sleep(100 * time.Millisecond)
	if out, err := exec.Command("bridge", "link", "show", hostVeth).CombinedOutput(); err != nil {
		return fmt.Errorf("veth not attached to bridge: %s", out)
	}
	fmt.Println("   ✅ Verified bridge attachment")

	c.hostVeth = hostVeth
	c.peerName = peerName

	// Включение маршрутизации localhost
	exec.Command("sysctl", "-w", "net.ipv4.conf.all.route_localnet=1").Run()
	fmt.Println("   ✅ route_localnet enabled")

	for _, port := range netCfg.Ports {
		parts := strings.Split(port, ":")
		if len(parts) != 2 {
			fmt.Printf("   ⚠️ Invalid port format: %s\n", port)
			continue
		}
		hostPort, _ := strconv.Atoi(parts[0])
		containerPort, _ := strconv.Atoi(parts[1])

		fmt.Printf("   Forwarding port %d -> %d\n", hostPort, containerPort)

		if err := nm.PortForward(containerIP, hostPort, containerPort); err != nil {
			return fmt.Errorf("failed to forward port %s: %v", port, err)
		}

		// Правило для localhost (OUTPUT цепочка)
		exec.Command("iptables", "-t", "nat", "-A", "OUTPUT",
			"-p", "tcp", "--dport", strconv.Itoa(hostPort),
			"-j", "DNAT", "--to-destination", containerIP+":"+strconv.Itoa(containerPort)).Run()

		// MASQUERADE для ответных пакетов
		exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING",
			"-d", containerIP, "-p", "tcp", "--dport", strconv.Itoa(containerPort),
			"-j", "MASQUERADE").Run()
	}
	fmt.Println("   ✅ Port forwarding configured")
	return nil
}

func (c *Container) AttachNetwork() error {
	fmt.Println("\n🔧 Attaching network to container...")

	if err := syscall.Kill(c.Pid, 0); err != nil {
		return fmt.Errorf("container process %d is dead: %v", c.Pid, err)
	}
	fmt.Printf("   ✅ Container process %d is alive\n", c.Pid)

	// Перемещаем eth0 в контейнер
	out, err := exec.Command("ip", "link", "set", c.peerName, "netns", strconv.Itoa(c.Pid)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to move interface to container: %s", out)
	}
	fmt.Println("   ✅ Moved eth0 to container")

	// Функция для выполнения команд внутри контейнера
	nsenter := func(cmd ...string) error {
		args := append([]string{"-t", strconv.Itoa(c.Pid), "-n"}, cmd...)
		if out, err := exec.Command("nsenter", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("nsenter %v failed: %s", cmd, out)
		}
		return nil
	}

	// Назначаем IP
	if err := nsenter("ip", "addr", "add", c.containerIP+"/16", "dev", c.peerName); err != nil {
		return fmt.Errorf("failed to add IP: %v", err)
	}
	fmt.Printf("   ✅ Added IP %s to eth0\n", c.containerIP)

	// Поднимаем интерфейсы
	if err := nsenter("ip", "link", "set", c.peerName, "up"); err != nil {
		return fmt.Errorf("failed to set eth0 up: %v", err)
	}
	fmt.Println("   ✅ eth0 is UP")

	if err := nsenter("ip", "link", "set", "lo", "up"); err != nil {
		return fmt.Errorf("failed to set lo up: %v", err)
	}
	fmt.Println("   ✅ Loopback is UP")

	// Добавляем маршрут по умолчанию
	if err := nsenter("ip", "route", "add", "default", "via", "10.20.1.1"); err != nil {
		return fmt.Errorf("failed to add default route: %v", err)
	}
	fmt.Println("   ✅ Default route added")

	// Проверяем что интерфейс действительно UP
	time.Sleep(100 * time.Millisecond)
	if err := nsenter("ip", "link", "show", c.peerName); err != nil {
		return fmt.Errorf("eth0 verification failed: %v", err)
	}
	fmt.Println("   ✅ Network attached successfully")
	return nil
}
