package image

import (
	"fmt"
	"zylo/api"
	"zylo/internal/system"
	"zylo/ui"
)

func Pull(tty, name string) error {
	if name == "" {
		return fmt.Errorf("image name required")
	}

	ttyFile, err := openTTY(tty)
	if err != nil {
		return err
	}
	defer ttyFile.Close()

	if err := api.Ping(); err != nil {
		fmt.Fprintf(ttyFile, "Error: registry not available\n")
		return err
	}

	if api.ImageExists(name) {
		fmt.Fprintf(ttyFile, "Image %s already exists\n", name)
		return nil
	}

	freeSpace, err := system.GetFreeDiskSpace(name)
	if err == nil && freeSpace < 512*1024*1024 {
		fmt.Fprintf(ttyFile, "Error: not enough disk space\n")
		return fmt.Errorf("insufficient disk space")
	}

	err = ui.WithSpinner(ttyFile, "Downloading image "+name, func() error {
		return api.PullImage(name)
	})

	if err != nil {
		return err
	}

	fmt.Fprintf(ttyFile, "Image %s installed successfully\n", name)
	return nil
}
