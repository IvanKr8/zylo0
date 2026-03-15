package image

import (
	"fmt"
	"os"
	"path/filepath"

	"zylo/global"
	"zylo/internal/container"
)

func Delete(tty, name string) error {
	if name == "" {
		return fmt.Errorf("image name required")
	}

	ttyFile, err := os.OpenFile(tty, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open tty: %v", err)
	}
	defer ttyFile.Close()

	imagePath := filepath.Join(global.ImgsPth, name)
	if _, err := os.Stat(imagePath); err != nil {
		fmt.Fprintf(ttyFile, "Error: image %s not found\n", name)
		return fmt.Errorf("image not found")
	}

	containers := container.ListContainers()
	var usingContainers []string

	for _, c := range containers {
		if c.Image == name {
			usingContainers = append(usingContainers, c.ID[:12])
		}
	}

	if len(usingContainers) > 0 {
		fmt.Fprintf(ttyFile, "Error: image %s is used by containers: %v\n",
			name, usingContainers)
		return fmt.Errorf("image in use")
	}

	if err := os.RemoveAll(imagePath); err != nil {
		fmt.Fprintf(ttyFile, "Error: failed to delete image: %v\n", err)
		return err
	}

	fmt.Fprintf(ttyFile, "Image %s deleted\n", name)
	return nil
}
