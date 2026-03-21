package container

import (
	"errors"
	"fmt"
	"net"
	"os"
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

func checkNetwork(nm *network.NetManager, ttyFile *os.File, cfg *Config) error {
	if cfg.Network == "" {
		return nil
	}

	if cfg.Network == global.MainNetName {
		return fmt.Errorf("network %s is reserved", global.MainNetName)
	}

	_, err := nm.GetNetwork(cfg.Network)

	if errors.Is(err, network.ErrNetworkNotFound) {
		fmt.Fprintf(ttyFile, "Creating network %s...\n", cfg.Network)

		netInfo, err := nm.CreateNetwork(cfg.Network)
		if err != nil {
			return fmt.Errorf("failed to create network: %v", err)
		}

		if err := nm.SetupIPTables(); err != nil {
			return fmt.Errorf("failed to setup NAT: %v", err)
		}

		fmt.Fprintf(ttyFile, "Network %s created\n", netInfo.Name)
		return nil
	}

	if err != nil {
		return err
	}

	fmt.Fprintf(ttyFile, "Network %s exists\n", cfg.Network)
	return nil
}

func (c *Container) SetupNetwork(netCfg *NetworkConfig) error {
	if c.networkManager == nil {
		return fmt.Errorf("network manager is not initialized")
	}

	networkName := c.Network
	if networkName == "" {
		networkName = global.MainNetName
	}
	//
	//netObj, err := c.networkManager.GetNetwork(networkName)
	//if err != nil {
	//	return fmt.Errorf("network %s does not exist: %v", networkName, err)
	//}

	//if netObj.DNS == nil || !netObj.DNS.IsRunning() {
	//	return fmt.Errorf("DNS not running for network %s", networkName)
	//}

	bridge, err := netlink.LinkByName(networkName)
	if err != nil {
		return fmt.Errorf("bridge %s not found: %v", networkName, err)
	}

	if bridge.Attrs().Flags&net.FlagUp == 0 {
		if err := netlink.LinkSetUp(bridge); err != nil {
			return fmt.Errorf("failed to set bridge up: %v", err)
		}
	}

	ipam := network.NewIPAM(c.networkManager)
	containerIP, err := ipam.AllocateIP(networkName, c.ID, c.Name)
	if err != nil {
		return fmt.Errorf("failed to allocate IP: %v", err)
	}
	c.IP = containerIP
	c.containerIP = containerIP

	suffix := c.ID[len(c.ID)-8:]
	hostVeth := fmt.Sprintf("v%s", suffix)
	peerName := fmt.Sprintf("e%s", suffix)

	exec.Command("ip", "link", "del", hostVeth).Run()
	exec.Command("ip", "link", "del", peerName).Run()
	time.Sleep(50 * time.Millisecond)

	out, err := exec.Command("ip", "link", "add", hostVeth, "type", "veth", "peer", "name", peerName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create veth pair: %s", out)
	}

	out, err = exec.Command("ip", "link", "set", hostVeth, "master", networkName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to attach host veth to bridge: %s", out)
	}

	out, err = exec.Command("ip", "link", "set", hostVeth, "up").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set host veth up: %s", out)
	}

	c.hostVeth = hostVeth
	c.peerName = peerName

	for _, port := range netCfg.Ports {
		parts := strings.Split(port, ":")
		if len(parts) != 2 {
			continue
		}
		hostPort, _ := strconv.Atoi(parts[0])
		containerPort, _ := strconv.Atoi(parts[1])

		if err := c.networkManager.PortForward(containerIP, hostPort, containerPort); err != nil {
			return fmt.Errorf("failed to forward port %s: %v", port, err)
		}
	}

	return nil
}

func (c *Container) AttachNetwork() error {
	if err := syscall.Kill(c.Pid, 0); err != nil {
		return fmt.Errorf("container process %d is dead: %v", c.Pid, err)
	}

	if c.networkManager == nil {
		return fmt.Errorf("network manager is not initialized")
	}

	networkName := c.Network
	if networkName == "" {
		networkName = global.MainNetName
	}

	networkConfig, err := c.networkManager.GetNetwork(networkName)
	if err != nil {
		return fmt.Errorf("failed to get network config: %v", err)
	}

	if c.containerIP == "" {
		return fmt.Errorf("container IP not allocated")
	}

	gateway := networkConfig.Gateway

	out, err := exec.Command(
		"ip", "link", "set",
		c.peerName,
		"netns",
		strconv.Itoa(c.Pid),
	).CombinedOutput()
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

	_, ipNet, err := net.ParseCIDR(networkConfig.Subnet)
	if err != nil {
		return fmt.Errorf("invalid subnet in network config: %v", err)
	}

	maskSize, _ := ipNet.Mask.Size()
	ipWithMask := fmt.Sprintf("%s/%d", c.containerIP, maskSize)

	if err := nsenter("ip", "addr", "add", ipWithMask, "dev", c.peerName); err != nil {
		return fmt.Errorf("failed to add IP: %v", err)
	}

	if err := nsenter("ip", "link", "set", c.peerName, "up"); err != nil {
		return fmt.Errorf("failed to set eth up: %v", err)
	}

	if err := nsenter("ip", "link", "set", "lo", "up"); err != nil {
		return fmt.Errorf("failed to set lo up: %v", err)
	}

	if err := nsenter("ip", "route", "add", "default", "via", gateway); err != nil {
		return fmt.Errorf("failed to add default route: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if err := nsenter("ip", "link", "show", c.peerName); err != nil {
		return fmt.Errorf("eth verification failed: %v", err)
	}

	return nil
}
