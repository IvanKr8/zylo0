package system

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

func CpDir(srcDir string, destDir string) error {
	entries, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %v", srcDir, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		if entry.IsDir() {
			if err = os.MkdirAll(destPath, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", destPath, err)
			}

			if err = CpDir(srcPath, destPath); err != nil {
				return err
			}
		} else {
			if err = CpFile(srcPath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func CpFile(src, dst string) error {
	err := os.MkdirAll(filepath.Dir(dst), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create directory for %s: %v", dst, err)
	}

	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, input)
	return err
}
