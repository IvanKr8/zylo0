package container

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"zylo/global"
)

func ps(tty string) error {
	baseDir := global.CtrsCfgPth

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return fmt.Errorf("failed to read containers dir: %v", err)
	}

	var liveContainers []Container
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		containerFile := filepath.Join(baseDir, entry.Name(), "container.json")
		var c Container
		if data, err := os.ReadFile(containerFile); err == nil {
			if err := json.Unmarshal(data, &c); err == nil {
				liveContainers = append(liveContainers, c)
			}
		}
	}

	ttyFile, err := os.OpenFile(tty, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open tty %s: %v", tty, err)
	}
	defer ttyFile.Close()

	fmt.Fprintln(ttyFile, "")

	if len(liveContainers) == 0 {
		fmt.Fprintln(ttyFile, "No live containers.")
		fmt.Fprintln(ttyFile, "")
		return nil
	}

	w := tabwriter.NewWriter(ttyFile, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tIMAGE\tVOLUMES\tCMD\tPORTS\tIP")

	for _, c := range liveContainers {
		volumes := ""
		for i, v := range c.Volumes {
			if i > 0 {
				volumes += ", "
			}
			if v.Name != "" {
				volumes += v.Name
			} else {
				volumes += v.HostPath
			}
		}

		cmd := ""
		if len(c.Cmd) > 0 {
			cmd = strings.Join(c.Cmd, " ")
		}

		ports := ""
		if len(c.Ports) > 0 {
			ports = strings.Join(c.Ports, ", ")
		}

		name := c.Name
		if name == "" {
			name = c.ID[:12]
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			name, c.Image, volumes, cmd, ports, c.IP)
	}

	w.Flush()
	fmt.Fprintln(ttyFile, "")

	return nil
}

func ListContainers() []Container {
	baseDir := global.CtrsCfgPth
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}

	var containers []Container
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		containerFile := filepath.Join(baseDir, entry.Name(), "container.json")
		var c Container
		if data, err := os.ReadFile(containerFile); err == nil {
			if err := json.Unmarshal(data, &c); err == nil {
				containers = append(containers, c)
			}
		}
	}

	return containers
}

func Ps(tty string) error {
	return ps(tty)
}
