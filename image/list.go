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

	ttyFile, err := os.OpenFile(tty, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open tty: %v", err)
	}
	defer ttyFile.Close()

	// Пустая строка СРАЗУ в ttyFile
	fmt.Fprintln(ttyFile, "")

	if len(entries) == 0 {
		fmt.Fprintln(ttyFile, "No images found.")
		fmt.Fprintln(ttyFile, "")
		return nil
	}

	fmt.Fprintln(ttyFile, "")

	w := tabwriter.NewWriter(ttyFile, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSIZE\tPATH")

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		imagePath := filepath.Join(imagesDir, entry.Name())
		var totalSize int64

		err := filepath.Walk(imagePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				totalSize += info.Size()
			}
			return nil
		})

		sizeStr := "unknown"
		if err == nil {
			sizeStr = fmt.Sprintf("%d MB", totalSize/1024/1024)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\n", entry.Name(), sizeStr, imagePath)
	}

	w.Flush()
	fmt.Fprintln(ttyFile, "")
	return nil
}
