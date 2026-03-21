package container

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"zylo/global"
	container2 "zylo/internal/container/resources"
)

func (c *Container) setup() error {
	dirs := []string{
		filepath.Join(c.Rootfs, "upper"),
		filepath.Join(c.Rootfs, "work"),
		filepath.Join(c.Rootfs, "merged"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %v", dir, err)
		}
	}

	imagePath := filepath.Join(global.ImgsPth, c.Image)
	if _, err := os.Stat(imagePath); err != nil {
		return fmt.Errorf("image not found: %s", imagePath)
	}

	opts := fmt.Sprintf(
		"lowerdir=%s,upperdir=%s,workdir=%s",
		imagePath,
		filepath.Join(c.Rootfs, "upper"),
		filepath.Join(c.Rootfs, "work"),
	)

	if err := syscall.Mount(
		"overlay",
		filepath.Join(c.Rootfs, "merged"),
		"overlay",
		0,
		opts,
	); err != nil {
		return fmt.Errorf("overlay mount failed: %v", err)
	}

	mergedRoot := filepath.Join(c.Rootfs, "merged")

	initScript := filepath.Join(mergedRoot, "container_init.sh")
	if _, err := os.Stat(initScript); err == nil {
		os.Chmod(initScript, 0755)
	}

	if err := c.addResolv(); err != nil {
		return err
	}

	if err := c.addHosts(); err != nil {
		return err
	}

	c.DeviceMgr = container2.NewDeviceManager(mergedRoot)
	if err := c.DeviceMgr.CreateAll(); err != nil {
		return err
	}

	tmpDir := filepath.Join(mergedRoot, "tmp")
	os.MkdirAll(tmpDir, 01777)
	os.Chmod(tmpDir, 01777)

	c.UserMgr = container2.NewUserManager(mergedRoot)

	if c.Workdir != "" {
		workDir := filepath.Join(mergedRoot, c.Workdir)
		os.MkdirAll(workDir, 0755)
	}

	meta, err := container2.LoadImageMetadata(imagePath)
	if err != nil {
		return fmt.Errorf("failed to load image metadata: %v", err)
	}

	c.UserMgr.AddUsersFromMetadata(meta)
	c.UserMgr.AddDefaultRoot()

	if err := c.UserMgr.WriteFiles(); err != nil {
		return err
	}

	c.MountMgr = container2.NewMountManager(mergedRoot)
	c.addMounts()

	if err := c.MountMgr.MountAll(); err != nil {
		return err
	}

	if err := c.writeEnvFile(); err != nil {
		return fmt.Errorf("failed to write env file: %v", err)
	}

	return nil
}

func (c *Container) addMounts() {
	c.MountMgr.Add(container2.SysMount())
	c.MountMgr.Add(container2.TmpfsMount("/dev/shm", "256m"))

	if c.Workdir != "" {
		sourceDir := c.CopyDir
		if sourceDir == "" {
			sourceDir = c.CmdPath
		}

		if sourceDir != "" {
			targetDir := filepath.Join(c.MountMgr.Rootfs, c.Workdir)
			os.MkdirAll(targetDir, 0755)
			c.MountMgr.Add(container2.BindMount(sourceDir, c.Workdir))
		}
	}

	for _, vol := range c.Volumes {
		targetDir := filepath.Join(c.MountMgr.Rootfs, vol.ContainerPath)
		os.MkdirAll(targetDir, 0755)
		c.MountMgr.Add(container2.BindMount(vol.HostPath, vol.ContainerPath))
	}
}

func (c *Container) addResolv() error {
	mergedRoot := filepath.Join(c.Rootfs, "merged")
	resolvPath := filepath.Join(mergedRoot, "etc/resolv.conf")

	if err := os.MkdirAll(filepath.Dir(resolvPath), 0755); err != nil {
		return fmt.Errorf("failed to create /etc: %v", err)
	}

	netObj, err := c.NetworkManager.GetNetwork(c.Network)
	if err != nil {
		return fmt.Errorf("failed to get network %s: %v", c.Network, err)
	}

	gateway := netObj.Gateway

	content := fmt.Sprintf(
		"nameserver %s\nnameserver 8.8.8.8\nnameserver 8.8.4.4\nsearch %s\n",
		gateway,
		c.Network,
	)

	tempResolv := filepath.Join(mergedRoot, "etc/resolv.conf.zylo")
	if err := os.WriteFile(tempResolv, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temporary resolv.conf: %v", err)
	}

	if err := syscall.Mount(tempResolv, resolvPath, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("failed to bind mount resolv.conf: %v", err)
	}

	return nil
}

func (c *Container) addHosts() error {
	mergedRoot := filepath.Join(c.Rootfs, "merged")
	hostsPath := filepath.Join(mergedRoot, "etc/hosts")

	hostHostsPath := filepath.Join(global.HostsCfgPth, c.Network, "hosts")

	if _, err := os.Stat(hostHostsPath); err != nil {
		if err := os.MkdirAll(filepath.Dir(hostHostsPath), 0755); err != nil {
			return fmt.Errorf("failed to create hosts dir: %v", err)
		}

		baseContent := `127.0.0.1       localhost
::1             localhost ip6-localhost ip6-loopback

# containers zylo-net
`
		if err := os.WriteFile(hostHostsPath, []byte(baseContent), 0644); err != nil {
			return fmt.Errorf("failed to create base hosts file: %v", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(hostsPath), 0755); err != nil {
		return fmt.Errorf("failed to create /etc: %v", err)
	}

	if err := syscall.Mount(hostHostsPath, hostsPath, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("failed to bind mount hosts: %v", err)
	}

	return nil
}

func (c *Container) writeEnvFile() error {
	mergedRoot := filepath.Join(c.Rootfs, "merged")
	envFile := filepath.Join(mergedRoot, ".zylo_env")
	content := strings.Join(c.buildEnv(), "\n")
	return os.WriteFile(envFile, []byte(content), 0644)
}

func (c *Container) buildEnv() []string {
	env := []string{
		"PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/bin",
	}

	for k, v := range c.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

func copyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
