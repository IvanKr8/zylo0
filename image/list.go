package image

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"zylo/global"
)

func List(tty string) error {
	imagesDir := global.ImgsPth

	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		return fmt.Errorf("failed to read images dir: %v", err)
	}

	ttyFile, err := openTTY(tty)
	if err != nil {
		return err
	}
	defer ttyFile.Close()

	fmt.Fprintln(ttyFile, "")

	if len(entries) == 0 {
		fmt.Fprintln(ttyFile, "No images found.")
		fmt.Fprintln(ttyFile, "")
		return nil
	}

	w := tabwriter.NewWriter(ttyFile, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSIZE\tPATH")

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		imagePath := filepath.Join(imagesDir, entry.Name())
		var totalSize int64

		filepath.Walk(imagePath, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				totalSize += info.Size()
			}
			return nil
		})

		sizeStr := formatBytes(totalSize)
		fmt.Fprintf(w, "%s\t%s\t%s\n", entry.Name(), sizeStr, imagePath)
	}

	w.Flush()
	fmt.Fprintln(ttyFile, "")
	return nil
}
