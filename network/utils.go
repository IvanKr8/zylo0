package network

import (
	"fmt"
	"net"
	"os/exec"
	"time"
	"zylo/global"

	"github.com/vishvananda/netlink"
)

func generateID() string {
	return fmt.Sprintf("zylo-net-%d", time.Now().UnixNano())
}

func IsPortFree(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
	if err != nil {
		return true
	}
	conn.Close()
	return false
}

func (nm *NetManager) testNetwork(gateway string) error {
	cmd := exec.Command("ping", "-c", "1", "-W", "1", gateway)
	return cmd.Run()
}

func (nm *NetManager) bridgeExists(bridgeName string) bool {
	cmd := exec.Command("ip", "link", "show", bridgeName)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func (nm *NetManager) restoreNetwork(n *Net) error {
	bridge, err := netlink.LinkByName(n.Name)
	if err != nil {
		bridge = &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: n.Name, MTU: 1500}}
		if err := netlink.LinkAdd(bridge); err != nil {
			return err
		}
	}

	// Ensure IP is assigned
	addr, _ := netlink.ParseAddr(n.Gateway + "/24")
	addrs, _ := netlink.AddrList(bridge, netlink.FAMILY_V4)
	hasIP := false
	for _, a := range addrs {
		if a.IPNet.String() == addr.IPNet.String() {
			hasIP = true
			break
		}
	}
	if !hasIP {
		_ = netlink.AddrAdd(bridge, addr)
	}

	_ = netlink.LinkSetUp(bridge)

	if n.Name != global.MainNetName {
		mainBridge, err := netlink.LinkByName(global.MainNetName)
		if err != nil {
			return fmt.Errorf("zylo0 not found")
		}

		veth0Name := n.Name + "-veth0"
		veth1Name := n.Name + "-veth1"

		if _, err := netlink.LinkByName(veth0Name); err != nil {
			veth := &netlink.Veth{
				LinkAttrs: netlink.LinkAttrs{Name: veth0Name},
				PeerName:  veth1Name,
			}
			if err := netlink.LinkAdd(veth); err != nil {
				return err
			}

			veth0, _ := netlink.LinkByName(veth0Name)
			veth1, _ := netlink.LinkByName(veth1Name)

			_ = netlink.LinkSetMaster(veth0, bridge)
			_ = netlink.LinkSetMaster(veth1, mainBridge)

			_ = netlink.LinkSetUp(veth0)
			_ = netlink.LinkSetUp(veth1)
		}
	}

	return nm.setupNetworkIPTables(n.Subnet, n.Name)
}
