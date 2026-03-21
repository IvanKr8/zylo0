package container

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func ContainerInit() {
	rootfs := os.Getenv("ZYLO_ROOTFS")
	workdir := os.Getenv("ZYLO_WORKDIR")
	envStr := os.Getenv("_ZYLO_ENV")

	if rootfs == "" {
		panic("no rootfs")
	}

	syscall.Sethostname([]byte("zylo"))

	must(syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""))

	must(syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""))

	putOld := filepath.Join(rootfs, ".pivot_root")
	must(os.MkdirAll(putOld, 0700))

	must(syscall.PivotRoot(rootfs, putOld))
	must(os.Chdir("/"))
	must(syscall.Unmount("/.pivot_root", syscall.MNT_DETACH))
	os.RemoveAll("/.pivot_root")

	must(syscall.Mount("proc", "/proc", "proc", 0, ""))

	if workdir != "" {
		os.Chdir(workdir)
	}

	envSlice := []string{}
	if envStr != "" {
		envSlice = strings.Split(envStr, "\n")
	} else {
		envSlice = os.Environ()
	}

	envSlice = append(envSlice, "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin")

	initScript := "/container_init.sh"

	if _, err := os.Stat(initScript); err != nil {
		if _, err := os.Stat("/entrypoint.sh"); err == nil {
			initScript = "/entrypoint.sh"
		} else {
			if err := syscall.Exec("/bin/sh", []string{"/bin/sh"}, envSlice); err != nil {
				panic(err)
			}
			return
		}
	}

	os.Chmod(initScript, 0755)

	if len(os.Args) > 1 {
		cmdArgs := os.Args[1:]
		if err := syscall.Exec(initScript, append([]string{initScript}, cmdArgs...), envSlice); err != nil {
			panic(err)
		}
	} else {
		if err := syscall.Exec(initScript, []string{initScript}, envSlice); err != nil {
			panic(err)
		}
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
