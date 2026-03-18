package container

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"zylo/global"
	"zylo/network"
)

type Config struct {
	Image      string
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

type Volume struct {
	Name          string
	HostPath      string
	ContainerPath string
}

type cNetwork struct {
	name string
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

type manifest struct {
	Layers []layer `json:"layers,omitempty"`
}

type layer struct {
	Digest string `json:"digest,omitempty"`
	Size   int64  `json:"size,omitempty"`
}

func Up(p string, tty string) error {
	if os.Getenv("_ZYLO_INIT") == "1" {
		ContainerInit()
		return nil
	}

	nm, err := network.NewNetworkManager()
	if err != nil {
		return err
	}

	cfg, err := loadConfig(p, tty)
	if err != nil {
		return err
	}

	ttyFile, err := os.OpenFile(tty, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open tty: %v", err)
	}
	defer ttyFile.Close()

	if err := checkDiskSpace(ttyFile); err != nil {
		return err
	}

	if err := ensureImage(ttyFile, cfg); err != nil {
		return err
	}

	if err := checkNetwork(nm, ttyFile, cfg); err != nil {
		return err
	}

	if err := checkPorts(cfg.OpenPorts); err != nil {
		return err
	}

	if err := checkVolumes(cfg.Volumes); err != nil {
		return err
	}

	ctr, err := createAndSetupContainer(cfg)
	if err != nil {
		fmt.Fprintf(ttyFile, "Setup failed: %v\n", err)
		return err
	}

	runContainer(ctr)

	if err := waitForContainerStart(ctr, ttyFile); err != nil {
		return err
	}

	fmt.Fprintf(ttyFile, "Container %s started\n", ctr.ID[:12])
	return nil
}

func Down(hash, cmdPath string) error {
	if hash != "" {
		cfg := ContainerDown{Hash: hash}
		return down(cfg)
	}

	containers := ListContainers()
	var matched []Container
	for _, c := range containers {
		if c.CmdPath == cmdPath {
			matched = append(matched, c)
		}
	}

	if len(matched) == 0 {
		return fmt.Errorf("no containers started from this directory")
	}

	for _, ctr := range matched {
		cfg := ContainerDown{Hash: ctr.ID}
		if err := down(cfg); err != nil {
			fmt.Printf("⚠️ failed to stop %s: %v\n", ctr.ID[:12], err)
		}
	}

	return nil
}

func Ps(tty string) error {
	return ps(tty)
}

func Logs(cmdPath, tty, hash string) error {
	container, err := findContainer(cmdPath, hash)
	if err != nil {
		return err
	}

	ttyFile, err := os.OpenFile(tty, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open tty %s: %v", tty, err)
	}
	defer ttyFile.Close()

	logFile := filepath.Join(global.CtrsCfgPth, container.ID, "logs", "output.log")

	data, err := os.ReadFile(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(ttyFile, "No logs found for container %s\n", container.ID[:12])
			return nil
		}
		return fmt.Errorf("failed to read logs: %v", err)
	}

	fmt.Fprint(ttyFile, string(data))
	return nil
}

func VolumeList(tty string) error {
	return volumeList(tty)
}

func VolumeDelete(tty, name, path string) error {
	return volumeDelete(name, path)
}
