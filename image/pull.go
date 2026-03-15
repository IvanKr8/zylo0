package image

import (
	"fmt"
	"os"
	"syscall"
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

	client := api.NewClient()
	if err := api.Ping(); err != nil {
		fmt.Fprintf(writer, "Error: store server not available\n")
		return err
	}

	if api.ImageExists(name) {
		fmt.Fprintf(writer, "Image %s already exists\n", name)
		return nil
	}

	exists, size, err := client.VerifyImage(name)
	if err != nil {
		fmt.Fprintf(writer, "Error: failed to verify image\n")
		return err
	}
	if !exists {
		fmt.Fprintf(writer, "Error: image %s not found on server\n", name)
		return fmt.Errorf("image not found")
	}

	freeSpace, err := getFreeDiskPath(global.ImgsPth)
	if err != nil {
		fmt.Fprintf(writer, "Warning: cannot check disk space: %v\n", err)
	} else {
		neededSpace := uint64(size) * 2
		if freeSpace < neededSpace {
			fmt.Fprintf(writer, "Error: not enough disk space\n")
			fmt.Fprintf(writer, "  Required: %d MB\n", neededSpace/1024/1024)
			fmt.Fprintf(writer, "  Available: %d MB\n", freeSpace/1024/1024)
			return fmt.Errorf("insufficient disk space")
		}
	}

	fmt.Fprintf(writer, "Image size: %d MB\n", size/1024/1024)
	fmt.Fprintf(writer, "Downloading...\n")

	tmpPath, err := client.PullImage(name)
	if err != nil {
		fmt.Fprintf(writer, "Error: download failed: %v\n", err)
		return err
	}
	defer os.Remove(tmpPath)

	fmt.Fprintf(writer, "Installing...\n")

	if err := api.InstallImage(tmpPath, name); err != nil {
		fmt.Fprintf(writer, "Error: installation failed: %v\n", err)
		return err
	}

	fmt.Fprintf(writer, "✅ Image %s installed successfully\n", name)
	return nil
}

func getFreeDiskPath(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}
