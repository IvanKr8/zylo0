package container

import (
	"fmt"
	"os"
	"path/filepath"
	"zylo/global"
)

func Logs(cmdPath, tty, hash string) error {
	container, err := findContainer(cmdPath, hash)
	if err != nil {
		return err
	}

	ttyFile, err := os.OpenFile(tty, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open tty %s: %v", tty, err)
	}
	defer ttyFile.Close()

	logFile := filepath.Join(global.CtrsCfgPth, container.ID, "logs", "output.log")

	data, err := os.ReadFile(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(ttyFile, "No logs found for container %s\n", container.ID[:12])
			return nil
		}
		return fmt.Errorf("failed to read logs: %v", err)
	}

	fmt.Fprint(ttyFile, string(data))
	return nil
}
