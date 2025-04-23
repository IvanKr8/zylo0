package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"zylo/internal/global"
)

func setNS(cfg *container) error {
	path := "/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/bin"

	if err := mount(cfg); err != nil {
		return err
	}

	fl, err := readFile("/zy_env/zy.env")
	if err != nil {
		return err
	}

	env, err := fl.getEnv()
	if err != nil {
		return err
	}

	Env := append(os.Environ(), "PATH="+path)
	Env = append(Env, env...)

	attr := &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | syscall.CLONE_NEWNET,
	}

	pid, err := syscall.ForkExec("/bin/"+cfg.image, cfg.commands, &syscall.ProcAttr{
		Env:   Env,
		Files: []uintptr{},
		Sys:   attr,
	})

	if err != nil {
		fmt.Println("Error creating process:", err)
		return err
	}

	err = exec.Command("ip", "link", "set", fmt.Sprintf("%s-cont", cfg.net.name), "netns", strconv.Itoa(cfg.pid)).Run()
	if err != nil {
		fmt.Printf("Failed to move interface to netns: %v\n", err)
		return err
	}

	cfg.pid = pid

	f, err := os.Create("pid.txt")
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(strconv.Itoa(cfg.pid))
	if err != nil {
		return err
	}

	_, err = syscall.Wait4(pid, nil, 0, nil)
	if err != nil {
		fmt.Println("Error waiting for process:", err)
		return err
	}

	return nil
}

func mount(cfg *container) error {
	lowerDir := filepath.Join(global.ImgsPth, cfg.image)
	// !tests
	//lowerDir2 := filepath.Join(global.ImgsPth, "Alpine")
	//lowerDir := fmt.Sprintf("%s:%s", lowerDir1, lowerDir2)
	upperDir := cfg.rootfs + "/source"
	workDir := cfg.rootfs + "/work"
	mergedDir := cfg.rootfs + "/merged"

	options := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)

	if err := syscall.Mount("overlay", mergedDir, "overlay", 0, options); err != nil {
		return fmt.Errorf("failed to mount overlay: %w", err)
	}

	if err := syscall.Chroot(mergedDir); err != nil {
		return fmt.Errorf("failed to chroot: %w", err)
	}

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("failed to change dir: %w", err)
	}

	err := syscall.Mknod("/dev/null", 0666|syscall.S_IFCHR, 0x0103)
	if err != nil {
		return fmt.Errorf("mount /dev/null failed: %w", err)
	}
	err = syscall.Chmod("/dev/null", 0666)
	if err != nil {
		return fmt.Errorf("chmod /dev/null failed: %w", err)
	}

	return nil
}
