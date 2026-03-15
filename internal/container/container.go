package kernel

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"
	"zylo/global"
	"zylo/internal/container"
	"zylo/internal/identifiers"
	"zylo/network"
)

type ContainerDown struct {
	Hash string `json:"hash,omitempty"`
	Name string `json:"name,omitempty"`
}

type Container struct {
	ID      string             `json:"id,omitempty"`
	Rootfs  string             `json:"rootfs,omitempty"`
	Image   string             `json:"image,omitempty"`
	CopyDir string             `json:"copy_dir,omitempty"`
	Volumes []container.Volume `json:"volumes,omitempty"`
	Env     map[string]string  `json:"env,omitempty"`
	Cmd     []string           `json:"cmd,omitempty"`

	Ports []string `json:"ports,omitempty"`
	IP    string   `json:"ip,omitempty"`
	Pid   int      `json:"pid,omitempty"`

	hostVeth       string
	peerName       string
	containerIP    string
	mountMgr       *MountManager
	deviceMgr      *DeviceManager
	userMgr        *UserManager
	networkManager *network.NetManager
}

func NewContainer(cfg *container.Config) *Container {
	hash := identifiers.Hash()
	return &Container{
		ID:      hash,
		Rootfs:  filepath.Join(global.CtrsPth, hash),
		Image:   cfg.Image,
		CopyDir: cfg.CopyDir,
		Volumes: cfg.Volumes,
		Env:     cfg.Env,
		Cmd:     cfg.Cmd,
	}
}

func (c *Container) GetPid() int {
	return c.Pid
}

func (c *Container) GetIP() string {
	return c.IP
}

func runtimeCleanup() {
	paths := []string{
		"/var/run/zylo/containers",
		"/var/lib/zylo/containers",
	}

	for _, baseDir := range paths {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			fmt.Printf("failed to read %s: %v\n", baseDir, err)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			ctrDir := filepath.Join(baseDir, entry.Name())
			pidFile := filepath.Join(ctrDir, entry.Name()+".pid")
			containerFile := filepath.Join(ctrDir, "container.json")

			pid := 0
			if data, err := os.ReadFile(pidFile); err == nil {
				if p, err := strconv.Atoi(string(data)); err == nil {
					pid = p
				}
			}

			alive := false
			if pid != 0 {
				if syscall.Kill(pid, 0) == nil {
					alive = true
				}
			}

			if !alive {
				var c Container
				if data, err := os.ReadFile(containerFile); err == nil {
					json.Unmarshal(data, &c)
					c.CleanupNetwork()
				}
				os.RemoveAll(ctrDir)
			}
		}
	}
}

func Ps() {
	baseDir := "/var/run/zylo/containers"

	runtimeCleanup()

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		fmt.Printf("failed to read containers dir: %v\n", err)
		return
	}

	var liveContainers []Container
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		containerFile := filepath.Join(baseDir, entry.Name(), "container.json")
		var c Container
		if data, err := os.ReadFile(containerFile); err == nil {
			if err := json.Unmarshal(data, &c); err == nil {
				liveContainers = append(liveContainers, c)
			}
		}
	}

	if len(liveContainers) == 0 {
		fmt.Println("No live containers.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tIMAGE\tVOLUMES\tCMD\tPORTS\tIP")
	for _, c := range liveContainers {
		volumes := ""
		for i, v := range c.Volumes {
			if i > 0 {
				volumes += ", "
			}
			volumes += fmt.Sprintf("%s->%s", v.HostPath, v.ContainerPath)
		}

		cmd := ""
		if len(c.Cmd) > 0 {
			cmd = c.Cmd[0]
		}

		ports := ""
		if len(c.Ports) > 0 {
			ports = c.Ports[0]
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			c.ID, c.Image, volumes, cmd, ports, c.IP)
	}
	w.Flush()
}

func Down(downCfg ContainerDown) error {
	if downCfg.Hash == "" {
		return fmt.Errorf("hash is required")
	}

	containerDir := filepath.Join(global.CtrsCfgPth, downCfg.Hash)
	containerFile := filepath.Join(containerDir, "container.json")

	data, err := os.ReadFile(containerFile)
	if err != nil {
		return fmt.Errorf("failed to read container.json: %v", err)
	}

	var c Container
	if err := json.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("failed to parse container.json: %v", err)
	}

	if c.Pid != 0 {
		fmt.Printf("Stopping container process %d...\n", c.Pid)
		syscall.Kill(c.Pid, syscall.SIGTERM)

		timeout := time.After(3 * time.Second)
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()

	waitLoop:
		for {
			select {
			case <-timeout:
				fmt.Printf("⚠️ Process %d did not exit, sending SIGKILL\n", c.Pid)
				syscall.Kill(c.Pid, syscall.SIGKILL)
			case <-tick.C:
				if syscall.Kill(c.Pid, 0) != nil {
					break waitLoop
				}
			}
		}
	}

	// Очистка сети
	if err := c.CleanupNetwork(); err != nil {
		fmt.Printf("⚠️ Warning: failed to cleanup network: %v\n", err)
	}

	// Удаляем veth интерфейсы на хосте
	if c.hostVeth != "" {
		exec.Command("ip", "link", "del", c.hostVeth).Run()
	}

	// Размонтируем все точки контейнера
	if c.mountMgr != nil {
		c.mountMgr.UnmountAll()
	} else {
		// fallback: пробуем размонтировать merged и tmp
		exec.Command("umount", "-l", filepath.Join(c.Rootfs, "merged")).Run()
		exec.Command("umount", "-l", filepath.Join(c.Rootfs, "work")).Run()
	}

	// Удаляем директорию контейнера
	if err := os.RemoveAll(containerDir); err != nil {
		return fmt.Errorf("failed to remove container dir: %v", err)
	}

	// Удаляем rootfs
	if err := os.RemoveAll(c.Rootfs); err != nil {
		return fmt.Errorf("failed to remove rootfs: %v", err)
	}

	fmt.Printf("✅ Container %s fully down\n", downCfg.Hash)
	return nil
}

func (c *Container) CleanupNetwork() error {
	if c.networkManager == nil || c.IP == "" {
		return nil
	}
	ipam := network.NewIPAM(c.networkManager)
	ipam.ReleaseIP("zylo0", c.ID)
	for _, port := range c.Ports {
		parts := strings.Split(port, ":")
		if len(parts) != 2 {
			continue
		}
		hostPort, _ := strconv.Atoi(parts[0])
		containerPort, _ := strconv.Atoi(parts[1])
		c.networkManager.RemovePortForward(c.IP, hostPort, containerPort)

		// Удаляем правила для localhost
		exec.Command("iptables", "-t", "nat", "-D", "OUTPUT",
			"-p", "tcp", "--dport", strconv.Itoa(hostPort),
			"-j", "DNAT", "--to-destination", c.IP+":"+strconv.Itoa(containerPort)).Run()

		exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING",
			"-d", c.IP, "-p", "tcp", "--dport", strconv.Itoa(containerPort),
			"-j", "MASQUERADE").Run()
	}
	return nil
}

func (c *Container) Setup() error {
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

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s",
		imagePath,
		filepath.Join(c.Rootfs, "upper"),
		filepath.Join(c.Rootfs, "work"))
	if err := syscall.Mount("overlay", filepath.Join(c.Rootfs, "merged"), "overlay", 0, opts); err != nil {
		return fmt.Errorf("overlay mount failed: %v", err)
	}

	mergedRoot := filepath.Join(c.Rootfs, "merged")

	c.deviceMgr = NewDeviceManager(mergedRoot)
	if err := c.deviceMgr.CreateAll(); err != nil {
		return err
	}

	tmpDir := filepath.Join(mergedRoot, "tmp")
	os.MkdirAll(tmpDir, 01777)
	os.Chmod(tmpDir, 01777)

	c.userMgr = NewUserManager(mergedRoot)

	dataDir := filepath.Join(mergedRoot, "var/lib/postgresql/data")
	os.MkdirAll(dataDir, 0700)
	os.Chown(dataDir, 1001, 1001)

	meta, err := LoadImageMetadata(imagePath)
	if err != nil {
		return fmt.Errorf("failed to load image metadata: %v", err)
	}
	c.userMgr.AddUsersFromMetadata(meta)
	c.userMgr.AddDefaultRoot()
	if err := c.userMgr.WriteFiles(); err != nil {
		return err
	}

	c.mountMgr = NewMountManager(mergedRoot)
	c.addMounts()
	if err := c.mountMgr.MountAll(); err != nil {
		return err
	}

	if err := c.writeEnvFile(); err != nil {
		return fmt.Errorf("failed to write env file: %v", err)
	}

	return nil
}

func (c *Container) addMounts() {
	c.mountMgr.Add(SysMount())
	c.mountMgr.Add(TmpfsMount("/dev/shm", "256m"))
	if c.CopyDir != "" {
		c.mountMgr.Add(BindMount(c.CopyDir, "/app"))
	}
	for _, vol := range c.Volumes {
		c.mountMgr.Add(BindMount(vol.HostPath, vol.ContainerPath))
	}
}

func (c *Container) Run() error {
	if err := c.SetupNetwork(&NetworkConfig{
		NetworkName: "zylo0",
		Ports:       c.Ports,
	}); err != nil {
		return fmt.Errorf("network setup failed: %v", err)
	}

	mergedRoot := filepath.Join(c.Rootfs, "merged")

	// формируем env сразу как строку
	envStr := strings.Join(c.buildEnv(), "\n")

	cmd := exec.Command("/proc/self/exe")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Env = append(os.Environ(),
		"_ZYLO_INIT=1",
		"ZYLO_ROOTFS="+mergedRoot,
		"ZYLO_CMD=./entrypoint.sh",
		"_ZYLO_ENV="+envStr,
	)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWPID,
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	c.Pid = cmd.Process.Pid

	if err := c.persistContainerState(); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to persist container state: %v", err)
	}

	if err := c.AttachNetwork(); err != nil {
		cmd.Process.Kill()
		return err
	}

	return cmd.Wait()
}
func ContainerInit() {
	rootfs := os.Getenv("ZYLO_ROOTFS")
	cmdStr := os.Getenv("ZYLO_CMD")
	envStr := os.Getenv("_ZYLO_ENV")

	if rootfs == "" {
		panic("no rootfs")
	}

	// hostname
	syscall.Sethostname([]byte("zylo"))

	// mounts private
	must(syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""))

	// bind mount rootfs
	must(syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND|syscall.MS_REC, ""))

	putOld := filepath.Join(rootfs, ".pivot_root")
	must(os.MkdirAll(putOld, 0700))

	// pivot_root
	must(syscall.PivotRoot(rootfs, putOld))
	must(os.Chdir("/"))

	must(syscall.Unmount("/.pivot_root", syscall.MNT_DETACH))
	os.RemoveAll("/.pivot_root")

	must(syscall.Mount("proc", "/proc", "proc", 0, ""))

	// формируем env
	var envSlice []string
	if envStr != "" {
		envSlice = strings.Split(envStr, "\n")
	} else {
		envSlice = os.Environ()
	}

	if cmdStr == "" {
		cmdStr = "/bin/sh"
	}
	args := []string{"/bin/sh", "-c", cmdStr}
	if err := syscall.Exec("/bin/sh", args, envSlice); err != nil {
		panic(err)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func (c *Container) writeEnvFile() error {
	mergedRoot := filepath.Join(c.Rootfs, "merged")

	envFile := filepath.Join(mergedRoot, ".zylo_env")

	content := strings.Join(c.buildEnv(), "\n")

	return os.WriteFile(envFile, []byte(content), 0644)
}

func (c *Container) buildEnv() []string {
	env := []string{"PATH=/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/bin"}
	for k, v := range c.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}
