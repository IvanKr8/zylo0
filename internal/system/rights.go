package system

import "os"

func Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}
