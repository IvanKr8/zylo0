package system

import (
	"fmt"
	"os"
	"syscall"
)

func DoubleFork() error {
	pid, _, _ := syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
	if pid == 0 {
	} else if pid > 0 {
		os.Exit(0)
	} else {
		return fmt.Errorf("error in first fork")
	}

	_, _, errno := syscall.Syscall(syscall.SYS_SETSID, 0, 0, 0)
	if errno != 0 {
		return fmt.Errorf("error creating session")
	}

	pid, _, _ = syscall.Syscall(syscall.SYS_FORK, 0, 0, 0)
	if pid == 0 {
		return nil
	} else if pid > 0 {
		os.Exit(0)
	} else {
		return fmt.Errorf("error in second fork")
	}

	return nil
}
