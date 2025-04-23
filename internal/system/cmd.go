package system

import (
	"fmt"
	"os"
	"syscall"
)

func FindCmd() (string, error) {
	fd := os.Stdin.Fd()

	var stat syscall.Stat_t
	err := syscall.Fstat(int(fd), &stat)
	if err != nil {
		return "", fmt.Errorf("not connected to a terminal: %w", err)
	}

	resolvedPath, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd))
	if err != nil {
		return "", fmt.Errorf("error reading link: %w", err)
	}

	return resolvedPath, nil
}
