package system

import (
	"os/exec"
)

var Command = func(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}
