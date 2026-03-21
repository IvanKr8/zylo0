package container

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"zylo/global"
	"zylo/volume"
)

func volumeList(tty string) error {
	volumes, err := volume.ListVolumes()
	if err != nil {
		return err
	}

	ttyFile, _ := os.OpenFile(tty, os.O_WRONLY, 0644)
	defer ttyFile.Close()

	fmt.Fprintln(ttyFile, "")

	if len(volumes) == 0 {
		fmt.Fprintln(ttyFile, "No volumes found.")
		fmt.Fprintln(ttyFile, "")
		return nil
	}

	w := tabwriter.NewWriter(ttyFile, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPATH\tCREATED\tUSED BY")

	for _, v := range volumes {
		usedBy := fmt.Sprintf("%d container(s)", len(v.UsedBy))
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			v.Name, v.Path, v.Created[:10], usedBy)
	}

	w.Flush()
	fmt.Fprintln(ttyFile, "")
	return nil
}

func volumeDelete(name, path string) error {
	if name != "" && path != "" {
		return fmt.Errorf("use either --name or --path, not both")
	}
	if name == "" && path == "" {
		return fmt.Errorf("specify volume by --name or --path")
	}

	vol, err := volume.FindVolume(name, path)
	if err != nil {
		return err
	}

	if vol == nil {
		return fmt.Errorf("volume not found")
	}

	if len(vol.UsedBy) > 0 {
		return fmt.Errorf(
			"volume '%s' is in use by container(s): %s",
			vol.Name,
			strings.Join(vol.UsedBy, ", "),
		)
	}

	if err := volume.UnregisterVolume(name, path); err != nil {
		return err
	}

	if err := os.RemoveAll(vol.Path); err != nil {
		_ = volume.RegisterVolume(vol.Name, vol.Path)
		return err
	}

	return nil
}

func isVolumeInUse(hostPath string) bool {
	baseDirs := []string{global.CtrsCfgPth, global.CtrsPth}

	for _, baseDir := range baseDirs {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			containerFile := filepath.Join(baseDir, entry.Name(), "container.json")
			pidFile := filepath.Join(baseDir, entry.Name(), entry.Name()+".pid")

			data, err := os.ReadFile(containerFile)
			if err != nil {
				continue
			}

			var c Container
			if err := json.Unmarshal(data, &c); err != nil {
				continue
			}

			alive := false
			if pidData, err := os.ReadFile(pidFile); err == nil {
				if pid, err := strconv.Atoi(strings.TrimSpace(string(pidData))); err == nil {
					if syscall.Kill(pid, 0) == nil {
						alive = true
					}
				}
			}

			if !alive {
				continue
			}

			for _, v := range c.Volumes {
				if v.HostPath == hostPath {
					return true
				}
			}
		}
	}

	return false
}

func VolumeList(tty string) error {
	return volumeList(tty)
}

func VolumeDelete(tty, name, path string) error {
	return volumeDelete(name, path)
}
