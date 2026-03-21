package container

import (
	"path/filepath"
	"zylo/global"
	container2 "zylo/internal/container/resources"
	"zylo/internal/identifiers"
	"zylo/network"
)

type Container struct {
	ID             string                    `json:"id,omitempty"`
	Name           string                    `json:"name,omitempty"`
	Rootfs         string                    `json:"rootfs,omitempty"`
	Image          string                    `json:"image,omitempty"`
	CopyDir        string                    `json:"copy_dir,omitempty"`
	Workdir        string                    `json:"workdir,omitempty"`
	Volumes        []Volume                  `json:"volume,omitempty"`
	Network        string                    `json:"network,omitempty"`
	Env            map[string]string         `json:"env,omitempty"`
	Cmd            []string                  `json:"cmd,omitempty"`
	CmdPath        string                    `json:"cmd_path,omitempty"`
	UserTTY        string                    `json:"user_tty,omitempty"`
	Ports          []string                  `json:"ports,omitempty"`
	IP             string                    `json:"ip,omitempty"`
	Pid            int                       `json:"pid,omitempty"`
	HostVeth       string                    `json:"-"`
	PeerName       string                    `json:"-"`
	ContainerIP    string                    `json:"-"`
	MountMgr       *container2.MountManager  `json:"-"`
	DeviceMgr      *container2.DeviceManager `json:"-"`
	UserMgr        *container2.UserManager   `json:"-"`
	NetworkManager *network.NetManager       `json:"-"`
}

type Volume struct {
	Name          string
	HostPath      string
	ContainerPath string
}

func newContainer(cfg *Config) *Container {
	hash := identifiers.Hash()
	return &Container{
		ID:      hash,
		Name:    cfg.Name,
		Rootfs:  filepath.Join(global.CtrsPth, hash),
		Image:   cfg.Image,
		CopyDir: cfg.CopyDir,
		Workdir: cfg.SetWorkdir,
		Volumes: cfg.Volumes,
		Network: cfg.Network,
		Env:     cfg.Env,
		Cmd:     cfg.Cmd,
		Ports:   cfg.OpenPorts,
		CmdPath: cfg.CmdPath,
		UserTTY: cfg.UserTTY,
	}
}

func (c *Container) GetPid() int {
	return c.Pid
}

func (c *Container) GetIP() string {
	return c.IP
}
