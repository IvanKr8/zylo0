package kernel

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type Device struct {
	Path  string
	Mode  uint32
	Major int
	Minor int
}

var DefaultDevices = []Device{
	{"/dev/null", 0666, 1, 3},
	{"/dev/zero", 0666, 1, 5},
	{"/dev/random", 0666, 1, 8},
	{"/dev/urandom", 0666, 1, 9},
	{"/dev/tty", 0666, 5, 0},
	{"/dev/console", 0620, 5, 1},
	{"/dev/ptmx", 0666, 5, 2},
	{"/dev/full", 0666, 1, 7},
}

type DeviceManager struct {
	rootfs  string
	devices []Device
}

func NewDeviceManager(rootfs string) *DeviceManager {
	return &DeviceManager{
		rootfs:  rootfs,
		devices: DefaultDevices,
	}
}

func (d *DeviceManager) CreateAll() error {
	devDir := filepath.Join(d.rootfs, "dev")
	if err := os.MkdirAll(devDir, 0755); err != nil {
		return fmt.Errorf("failed to create /dev: %v", err)
	}

	for _, dev := range d.devices {
		if err := d.create(dev); err != nil {
			return fmt.Errorf("failed to create %s: %v", dev.Path, err)
		}
	}

	symlinks := map[string]string{
		"/dev/fd":     "/proc/self/fd",
		"/dev/stdin":  "/proc/self/fd/0",
		"/dev/stdout": "/proc/self/fd/1",
		"/dev/stderr": "/proc/self/fd/2",
	}

	for link, target := range symlinks {
		os.Symlink(target, filepath.Join(d.rootfs, link))
	}

	return nil
}

func (d *DeviceManager) create(dev Device) error {
	path := filepath.Join(d.rootfs, dev.Path)

	if _, err := os.Stat(path); err == nil {
		// Если устройство уже есть, просто меняем права
		os.Chmod(path, os.FileMode(dev.Mode))
		return nil
	}

	mode := dev.Mode | syscall.S_IFCHR
	devNum := makedev(dev.Major, dev.Minor)

	if err := syscall.Mknod(path, mode, devNum); err != nil {
		return err
	}

	os.Chown(path, 0, 0)
	return os.Chmod(path, os.FileMode(dev.Mode))
}

func makedev(major, minor int) int {
	return (major << 8) | (minor & 0xff) | ((minor & 0xff00) << 12)
}
