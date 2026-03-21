package container

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type Mount struct {
	Source string
	Target string
	FSType string
	Flags  uintptr
	Data   string
	Create bool
}

type MountManager struct {
	Rootfs string
	mounts []*Mount
}

func NewMountManager(rootfs string) *MountManager {
	return &MountManager{
		Rootfs: rootfs,
		mounts: []*Mount{},
	}
}

func (m *MountManager) Add(mnt *Mount) {
	m.mounts = append(m.mounts, mnt)
}

func (m *MountManager) MountAll() error {
	for _, mnt := range m.mounts {
		if err := m.Mount(mnt); err != nil {
			return err
		}
	}
	return nil
}

func (m *MountManager) Mount(mnt *Mount) error {
	target := filepath.Join(m.Rootfs, mnt.Target)

	if mnt.Create {
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %v", target, err)
		}
	}

	var source string
	if mnt.Source != "" && !filepath.IsAbs(mnt.Source) {
		source = filepath.Join(m.Rootfs, mnt.Source)
	} else {
		source = mnt.Source
	}

	if err := syscall.Mount(source, target, mnt.FSType, mnt.Flags, mnt.Data); err != nil {
		return fmt.Errorf("mount %s -> %s failed: %v", source, target, err)
	}

	return nil
}

func (m *MountManager) UnmountAll() error {
	for i := len(m.mounts) - 1; i >= 0; i-- {
		target := filepath.Join(m.Rootfs, m.mounts[i].Target)
		syscall.Unmount(target, 0)
	}
	return nil
}

func OverlayMount(target, lower, upper, work string) *Mount {
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	return &Mount{
		Target: target,
		FSType: "overlay",
		Flags:  0,
		Data:   opts,
		Create: true,
	}
}

func BindMount(source, target string) *Mount {
	return &Mount{
		Source: source,
		Target: target,
		Flags:  syscall.MS_BIND | syscall.MS_REC,
		Create: true,
	}
}

func ProcMount() *Mount {
	return &Mount{
		Target: "/proc",
		FSType: "proc",
		Flags:  0,
		Create: true,
	}
}

func SysMount() *Mount {
	return &Mount{
		Target: "/sys",
		FSType: "sysfs",
		Flags:  0,
		Create: true,
	}
}

func TmpfsMount(target, size string) *Mount {
	return &Mount{
		Target: target,
		FSType: "tmpfs",
		Flags:  0,
		Data:   "size=" + size,
		Create: true,
	}
}
