package container

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"zylo/api"
	"zylo/global"
	"zylo/internal/system"
	"zylo/ui"
)

type Config struct {
	Image      string
	Name       string
	OpenPorts  []string
	CmdPath    string
	UserTTY    string
	SetWorkdir string
	CopyDir    string
	Env        map[string]string
	Network    string
	Volumes    []Volume
	Cmd        []string
}

type container struct {
	hash       string
	tty        string
	name       string
	status     string
	pid        int
	image      string
	workdir    string
	cmdPath    string
	rootfs     string
	fs         filesystem
	envVars    map[string]string
	ports      []string
	commands   []string
	copyDir    string
	entrypoint []string
	modifiedAt time.Time
	resources  resourcesConfig
	volumes    []Volume
	cNetwork
}

type filesystem struct {
	merged string
	work   string
	lower  []string
	upper  string
}

type resourcesConfig struct {
	cpu    int64
	memory int64
}

type cNetwork struct {
	name string
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

func checkVolumes(volumes []Volume) error {
	for _, vol := range volumes {
		if isVolumeInUse(vol.HostPath) {
			return fmt.Errorf("volume %s is already in use", vol.HostPath)
		}
	}
	return nil
}
