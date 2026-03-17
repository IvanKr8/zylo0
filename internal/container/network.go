package container

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
	mainNet := global.MainNetName
	nm, err := network.NewNetworkManager()
	if err != nil {
		return fmt.Errorf("failed to create network manager: %v", err)
	}
	c.networkManager = nm

	if !nm.NetworkExists(mainNet) {
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

	ipam := network.NewIPAM(nm)
	containerIP, err := ipam.AllocateIP(mainNet, c.ID)
	if err != nil {
		return fmt.Errorf("failed to allocate IP: %v", err)
	}
	c.IP = containerIP
	c.containerIP = containerIP

	uniqueSuffix := c.ID[len(c.ID)-12:]

	if len(c.ID) >= 16 {
		prefix := c.ID[:4]
		suffix := c.ID[len(c.ID)-4:]
		uniqueSuffix = prefix + suffix
	}

	hostVeth := fmt.Sprintf("veth-%s", uniqueSuffix)
	peerName := fmt.Sprintf("eth-%s", uniqueSuffix)

	if len(hostVeth) > 15 {
		hostVeth = fmt.Sprintf("v%s", c.ID[len(c.ID)-10:])
	}
	if len(peerName) > 15 {
		peerName = fmt.Sprintf("e%s", c.ID[len(c.ID)-10:])
	}

	exec.Command("ip", "link", "del", hostVeth).Run()
	exec.Command("ip", "link", "del", peerName).Run()
	time.Sleep(100 * time.Millisecond)

	out, err := exec.Command("ip", "link", "add", hostVeth, "type", "veth", "peer", "name", peerName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create veth: %s", out)
	}

	out, err = exec.Command("ip", "link", "set", hostVeth, "master", global.MainNetName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to attach to bridge: %s", out)
	}

	out, err = exec.Command("ip", "link", "set", hostVeth, "up").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set host veth up: %s", out)
	}

	time.Sleep(100 * time.Millisecond)
	if out, err := exec.Command("bridge", "link", "show", hostVeth).CombinedOutput(); err != nil {
		return fmt.Errorf("veth not attached to bridge: %s", out)
	}

	c.hostVeth = hostVeth
	c.peerName = peerName

	exec.Command("sysctl", "-w", "net.ipv4.conf.all.route_localnet=1").Run()

	networkConfig, _ := nm.GetNetwork(mainNet)
	if networkConfig != nil {
		// gateway используется в PortForward для логов, но мы убрали логи
		_ = networkConfig.Gateway
	}

	for _, port := range netCfg.Ports {
		parts := strings.Split(port, ":")
		if len(parts) != 2 {
			continue
		}
		hostPort, _ := strconv.Atoi(parts[0])
		containerPort, _ := strconv.Atoi(parts[1])

		if err := nm.PortForward(containerIP, hostPort, containerPort); err != nil {
			return fmt.Errorf("failed to forward port %s: %v", port, err)
		}
	}

	return nil
}

func (c *Container) AttachNetwork() error {
	if err := syscall.Kill(c.Pid, 0); err != nil {
		return fmt.Errorf("container process %d is dead: %v", c.Pid, err)
	}

	networkConfig, err := c.networkManager.GetNetwork(global.MainNetName)
	if err != nil {
		return fmt.Errorf("failed to get network config: %v", err)
	}
	gateway := networkConfig.Gateway

	out, err := exec.Command("ip", "link", "set", c.peerName, "netns", strconv.Itoa(c.Pid)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to move interface to container: %s", out)
	}

	nsenter := func(cmd ...string) error {
		args := append([]string{"-t", strconv.Itoa(c.Pid), "-n"}, cmd...)
		if out, err := exec.Command("nsenter", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("nsenter %v failed: %s", cmd, out)
		}
		return nil
	}

	if err := nsenter("ip", "addr", "add", c.containerIP+"/16", "dev", c.peerName); err != nil {
		return fmt.Errorf("failed to add IP: %v", err)
	}

	if err := nsenter("ip", "link", "set", c.peerName, "up"); err != nil {
		return fmt.Errorf("failed to set eth0 up: %v", err)
	}

	if err := nsenter("ip", "link", "set", "lo", "up"); err != nil {
		return fmt.Errorf("failed to set lo up: %v", err)
	}

	if err := nsenter("ip", "route", "add", "default", "via", gateway); err != nil {
		return fmt.Errorf("failed to add default route: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if err := nsenter("ip", "link", "show", c.peerName); err != nil {
		return fmt.Errorf("eth0 verification failed: %v", err)
	}

	return nil
}
