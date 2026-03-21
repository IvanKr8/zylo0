package container

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"zylo/global"
	namee "zylo/name"
	"zylo/volume"
)

type ContainerDown struct {
	Hash string `json:"hash,omitempty"`
	Name string `json:"name,omitempty"`
}

func RuntimeCleanup() {
	paths := []string{
		global.CtrsCfgPth,
		global.CtrsPth,
	}

	for _, baseDir := range paths {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			ctrDir := filepath.Join(baseDir, entry.Name())
			containerFile := filepath.Join(ctrDir, "container.json")

			data, err := os.ReadFile(containerFile)
			if err != nil {
				continue
			}

			var c Container
			if err := json.Unmarshal(data, &c); err != nil {
				continue
			}

			alive := false
			if c.Pid != 0 {
				if syscall.Kill(c.Pid, 0) == nil {
					alive = true
				}
			}

			if !alive {
				c.CleanupNetwork()
				os.RemoveAll(ctrDir)
			}
		}
	}
}

func down(downCfg ContainerDown) error {
	if downCfg.Hash == "" {
		return fmt.Errorf("hash is required")
	}

	containerDir := filepath.Join(global.CtrsCfgPth, downCfg.Hash)
	containerFile := filepath.Join(containerDir, "container.json")

	data, err := os.ReadFile(containerFile)
	if err != nil {
		return fmt.Errorf("failed to read container.json: %v", err)
	}

	var c Container
	if err := json.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("failed to parse container.json: %v", err)
	}

	if c.Pid != 0 {
		syscall.Kill(c.Pid, syscall.SIGTERM)

		timeout := time.After(10 * time.Second)
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()

	waitLoop:
		for {
			select {
			case <-timeout:
				syscall.Kill(c.Pid, syscall.SIGKILL)
			case <-tick.C:
				if syscall.Kill(c.Pid, 0) != nil {
					break waitLoop
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	for _, vol := range c.Volumes {
		if vol.Name != "" {
			volume.MarkVolumeUnused(vol.Name, "", c.ID)
		} else {
			volume.MarkVolumeUnused("", vol.HostPath, c.ID)
		}
	}

	if err = c.CleanupNetwork(); err != nil {
		fmt.Printf("⚠️ Warning: failed to cleanup network: %v\n", err)
	}

	if c.Name != "" {
		err := namee.UnregisterName(c.ID)
		if err != nil {
			fmt.Printf("⚠Warning: failed to unregister name %s: %v\n", c.Name, err)
		}
	}

	if c.MountMgr != nil {
		c.MountMgr.UnmountAll()
	} else {
		exec.Command("umount", "-l", filepath.Join(c.Rootfs, "merged")).Run()
		exec.Command("umount", "-l", filepath.Join(c.Rootfs, "work")).Run()
	}

	if err := os.RemoveAll(containerDir); err != nil {
		return fmt.Errorf("failed to remove container dir: %v", err)
	}

	if err := os.RemoveAll(c.Rootfs); err != nil {
		return fmt.Errorf("failed to remove rootfs: %v", err)
	}

	return nil
}

func findContainer(cmdPath, hash string) (*Container, error) {
	containers := ListContainers()
	if len(containers) == 0 {
		return nil, fmt.Errorf("no containers found")
	}

	if hash != "" {
		for _, c := range containers {
			if c.ID == hash {
				return &c, nil
			}
		}
		return nil, fmt.Errorf("container with hash '%s' not found", hash)
	}

	if cmdPath != "" {
		var matches []Container
		for _, c := range containers {
			if c.CmdPath == cmdPath {
				matches = append(matches, c)
			}
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no containers started from path '%s'", cmdPath)
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("multiple containers started from path '%s'", cmdPath)
		}
		return &matches[0], nil
	}

	currentPath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current path: %v", err)
	}

	var matches []Container
	for _, c := range containers {
		if c.CmdPath == currentPath {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no containers started from current path '%s'", currentPath)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple containers started from current path '%s'", currentPath)
	}
	return &matches[0], nil
}
