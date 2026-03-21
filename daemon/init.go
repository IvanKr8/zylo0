package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"zylo/global"
	"zylo/internal/container"
	"zylo/network"
	"zylo/volume"
)

func initCleanUp() {
	cleanRuntimeContainers()
	cleanLibContainers()
	cleanOrphanedMounts()
	cleanNetworks()
	time.Sleep(50 * time.Millisecond)
}

func cleanRuntimeContainers() {
	containersPath := global.CtrsCfgPth
	entries, err := os.ReadDir(containersPath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("InitCleanUp: cannot read containers dir: %v\n", err)
		}
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		hash := entry.Name()
		pidPath := filepath.Join(containersPath, hash, hash+".pid")
		pidData, err := os.ReadFile(pidPath)
		var pid int
		if err == nil {
			pid, _ = strconv.Atoi(string(pidData))
		}

		if err != nil || pid == 0 || syscall.Kill(pid, 0) != nil {
			fmt.Printf("InitCleanUp: removing dead container %s\n", hash)

			cleanContainerResources(hash)

			os.RemoveAll(filepath.Join(containersPath, hash))
			os.RemoveAll(filepath.Join(global.CtrsPth, hash))
		}
	}
}

func cleanLibContainers() {
	libPath := global.CtrsPth
	entries, err := os.ReadDir(libPath)
	if err != nil {
		return
	}

	nm, _ := network.NewNetworkManager()

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		hash := entry.Name()
		runtimePath := filepath.Join(global.CtrsCfgPth, hash)

		if _, err := os.Stat(runtimePath); os.IsNotExist(err) {
			fmt.Printf("InitCleanUp: removing orphaned lib container %s\n", hash)

			releaseContainerFromAllNetworks(nm, hash)
			cleanContainerResources(hash)

			os.RemoveAll(filepath.Join(libPath, hash))
		}
	}
}

func releaseContainerFromAllNetworks(nm *network.NetManager, containerID string) {
	networks, err := nm.ListNetworks()
	if err != nil {
		return
	}

	for _, net := range networks {
		config, err := nm.GetNetwork(net.Name)
		if err != nil {
			continue
		}

		if _, exists := config.Containers[containerID]; exists {
			ipam := network.NewIPAM(nm)
			ipam.ReleaseIP(net.Name, containerID)
		}
	}
}

func cleanContainerResources(hash string) {
	rootfsPath := filepath.Join(global.CtrsPth, hash)

	unmountAll(rootfsPath)

	if len(hash) >= 8 {
		veth0 := "v" + hash[len(hash)-8:]
		veth1 := "e" + hash[len(hash)-8:]
		exec.Command("ip", "link", "del", veth0).Run()
		exec.Command("ip", "link", "del", veth1).Run()
	} else {
		veth0 := "v" + hash
		veth1 := "e" + hash
		exec.Command("ip", "link", "del", veth0).Run()
		exec.Command("ip", "link", "del", veth1).Run()
	}

	volumes, _ := volume.ListVolumes()
	for _, v := range volumes {
		volume.MarkVolumeUnused(v.Name, v.Path, hash)
	}

	container.UnregisterName(hash)
}

func unmountAll(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	}

	cmd := exec.Command("sh", "-c", fmt.Sprintf("mount | grep '%s' | awk '{print $3}' | sort -r", path))
	output, err := cmd.Output()
	if err != nil {
		return
	}

	mounts := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, mnt := range mounts {
		if mnt != "" {
			exec.Command("umount", "-l", mnt).Run()
		}
	}

	exec.Command("umount", "-l", path).Run()
}

func cleanOrphanedMounts() {
	cmd := exec.Command("sh", "-c", "mount | grep '/var/lib/zylo' | awk '{print $3}' | sort -r")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	mounts := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, mnt := range mounts {
		if mnt != "" && strings.HasPrefix(mnt, "/var/lib/zylo") {
			exec.Command("umount", "-l", mnt).Run()
		}
	}
}

func cleanNetworks() {
	netDir := global.NetPth
	entries, err := os.ReadDir(netDir)
	if err != nil {
		return
	}

	nm, _ := network.NewNetworkManager()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if len(name) < 5 {
			continue
		}

		cfg, err := nm.GetNetwork(name[:len(name)-5])
		if err != nil || (len(cfg.Containers) == 0 && cfg.Name != global.MainNetName) {
			fmt.Printf("InitCleanUp: removing unused network %s\n", cfg.Name)
			exec.Command("ip", "link", "del", cfg.Name).Run()
			os.Remove(filepath.Join(netDir, name))
		}
	}
}
