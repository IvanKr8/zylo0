package container

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"zylo/network"
	"zylo/ui"

	"zylo/api"
	"zylo/global"
	"zylo/internal/system"
)

func (c *Container) addResolv() error {
	mergedRoot := filepath.Join(c.Rootfs, "merged")
	resolvPath := filepath.Join(mergedRoot, "etc/resolv.conf")

	if err := os.MkdirAll(filepath.Dir(resolvPath), 0755); err != nil {
		return fmt.Errorf("failed to create /etc: %v", err)
	}

	netObj, err := c.networkManager.GetNetwork(c.Network)
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

	// Путь к hosts файлу на хосте для данной сети
	// /var/run/zylo/hosts/{network_name}/hosts
	hostHostsPath := filepath.Join(global.HostsCfgPth, c.Network, "hosts")

	if _, err := os.Stat(hostHostsPath); err != nil {
		if err := os.MkdirAll(filepath.Dir(hostHostsPath), 0755); err != nil {
			return fmt.Errorf("failed to create hosts dir: %v", err)
		}

		baseContent := `127.0.0.1       localhost
::1             localhost ip6-localhost ip6-loopback

# контейнеры zylo-сети
`
		if err := os.WriteFile(hostHostsPath, []byte(baseContent), 0644); err != nil {
			return fmt.Errorf("failed to create base hosts file: %v", err)
		}
	}

	// Создаём директорию /etc в rootfs контейнера
	if err := os.MkdirAll(filepath.Dir(hostsPath), 0755); err != nil {
		return fmt.Errorf("failed to create /etc: %v", err)
	}

	// Биндим файл с хоста в контейнер
	if err := syscall.Mount(hostHostsPath, hostsPath, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("failed to bind mount hosts: %v", err)
	}

	return nil
}

func loadConfig(p, tty string) (*Config, error) {
	zyFl, err := findZyFile(p, global.ZyloFile)
	if err != nil {
		return nil, fmt.Errorf("error finding Zylofile: %v", err)
	}

	c := &container{cmdPath: p}
	if err := c.parseZyFile(zyFl); err != nil {
		return nil, fmt.Errorf("error parsing Zylofile: %v", err)
	}

	return &Config{
		Name:       c.name,
		Image:      c.image,
		CmdPath:    c.cmdPath,
		OpenPorts:  c.ports,
		Env:        c.envVars,
		Volumes:    c.volumes,
		Network:    c.cNetwork.name,
		Cmd:        c.commands,
		SetWorkdir: c.workdir,
		UserTTY:    tty,
	}, nil
}

func checkDiskSpace(ttyFile *os.File) error {
	freeSpace, err := system.GetFreeDiskSpace(global.ZyPth)
	if err != nil {
		fmt.Fprintf(ttyFile, "Warning: cannot check disk space\n")
		return nil
	}

	if freeSpace < 1024*1024*1024 {
		fmt.Fprintf(ttyFile, "Error: need at least 1GB free space (have %d MB)\n", freeSpace/1024/1024)
		return fmt.Errorf("insufficient disk space")
	}

	return nil
}

func ensureImage(ttyFile *os.File, cfg *Config) error {
	imagePath := filepath.Join(global.ImgsPth, api.Sanitize(cfg.Image))

	if _, err := os.Stat(imagePath); err == nil {
		return nil
	}

	fmt.Fprintf(ttyFile, "Image %s not found locally, downloading...\n", cfg.Image)

	err := ui.WithSpinner(ttyFile, "Downloading image "+cfg.Image, func() error {
		return api.PullImage(cfg.Image)
	})

	if err != nil {
		fmt.Fprintf(ttyFile, "Download failed: %v\n", err)
		return err
	}

	fmt.Fprintf(ttyFile, "Image %s ready\n", cfg.Image)
	return nil
}

func checkPorts(ports []string) error {
	return arePortsFree(ports)
}

func checkVolumes(volumes []Volume) error {
	for _, vol := range volumes {
		if isVolumeInUse(vol.HostPath) {
			return fmt.Errorf("volume %s is already in use", vol.HostPath)
		}
	}
	return nil
}

func checkName(name, containerID string) error {
	var n string

	if name != "" {
		ok, err := NameExists(name)
		if err != nil {
			return fmt.Errorf("failed to check name existence: %v", err)
		}
		if ok {
			return fmt.Errorf("name '%s' already exists", name)
		}
		n = name
	} else {
		n = generateContainerName()
	}

	if err := RegisterName(n, containerID); err != nil {
		return fmt.Errorf("failed to register name: %v", err)
	}

	return nil
}

func createAndSetupContainer(nm *network.NetManager, cfg *Config) (*Container, error) {
	ctr := newContainer(cfg)
	if ctr == nil {
		return nil, fmt.Errorf("failed to create container")
	}

	ctr.networkManager = nm

	if err := ctr.setup(); err != nil {
		return nil, err
	}

	return ctr, nil
}

func runContainer(ctr *Container) {
	_ = ctr.run()
}

func waitForContainerStart(ctr *Container, ttyFile *os.File) error {
	for i := 0; i < 50; i++ {
		if ctr.GetPid() != 0 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Fprintf(ttyFile, "Container failed to start\n")
	return fmt.Errorf("container failed to start")
}
