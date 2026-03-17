package image

import (
	"fmt"
	"os"
	"syscall"
	"zylo/ui"

	"zylo/api"
	"zylo/global"
)

func Pull(tty, name string) error {

	if name == "" {
		return fmt.Errorf("image name required")
	}

	ttyFile, err := os.OpenFile(tty, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open tty: %v", err)
	}
	defer ttyFile.Close()

	writer := ttyFile

	if err := api.Ping(); err != nil {
		fmt.Fprintf(writer, "Error: registry not available\n")
		return err
	}

	if api.ImageExists(name) {
		fmt.Fprintf(writer, "Image %s already exists\n", name)
		return nil
	}

	freeSpace, err := getFreeDiskPath(global.ImgsPth)
	if err == nil && freeSpace < 512*1024*1024 {
		fmt.Fprintf(writer, "Error: not enough disk space\n")
		return fmt.Errorf("insufficient disk space")
	}

	err = ui.WithSpinner(writer, "Downloading image "+name, func() error {
		return api.PullImage(name)
	})

	if err != nil {
		return err
	}

	fmt.Fprintf(writer, "Image %s installed successfully\n", name)
	return nil
}

func getFreeDiskPath(path string) (uint64, error) {

	var stat syscall.Statfs_t

	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}

	return stat.Bavail * uint64(stat.Bsize), nil
}
