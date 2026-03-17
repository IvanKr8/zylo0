package container

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"zylo/global"
)

func (c *Container) persistContainerState() error {
	baseDir := global.CtrsCfgPth
	containerDir := filepath.Join(baseDir, c.ID)

	if err := os.MkdirAll(containerDir, 0755); err != nil {
		return fmt.Errorf("failed to create container dir: %v", err)
	}

	writeJSON := func() error {
		jsonFile := filepath.Join(containerDir, "container.json")
		data, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal container: %v", err)
		}
		if err := os.WriteFile(jsonFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write container.json: %v", err)
		}
		return nil
	}

	writePID := func() error {
		pidFile := filepath.Join(containerDir, c.ID+".pid")
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(c.Pid)), 0644); err != nil {
			return fmt.Errorf("failed to write pid file: %v", err)
		}
		return nil
	}

	if err := writeJSON(); err != nil {
		return err
	}

	if err := writePID(); err != nil {
		return err
	}

	return nil
}
