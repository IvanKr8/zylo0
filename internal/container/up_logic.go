package container

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"zylo/ui"

	"zylo/api"
	"zylo/global"
	"zylo/internal/system"
)

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
		Image:      c.image,
		CmdPath:    c.cmdPath,
		OpenPorts:  c.ports,
		Env:        c.envVars,
		Volumes:    c.volumes,
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

func createAndSetupContainer(cfg *Config) (*Container, error) {
	ctr := newContainer(cfg)
	if ctr == nil {
		return nil, fmt.Errorf("failed to create container")
	}

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
