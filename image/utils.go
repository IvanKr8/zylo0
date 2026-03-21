package image

import (
	"fmt"
	"os"
)

func openTTY(tty string) (*os.File, error) {
	f, err := os.OpenFile(tty, os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open tty: %v", err)
	}
	return f, nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
