package container

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"
	"zylo/volume"

	"zylo/global"
	"zylo/internal/identifiers"
	"zylo/network"
)

type ContainerDown struct {
	Hash string `json:"hash,omitempty"`
	Name string `json:"name,omitempty"`
}

type Container struct {
	ID             string            `json:"id,omitempty"`
	Name           string            `json:"name,omitempty"`
	Rootfs         string            `json:"rootfs,omitempty"`
	Image          string            `json:"image,omitempty"`
	CopyDir        string            `json:"copy_dir,omitempty"`
	Workdir        string            `json:"workdir,omitempty"`
	Volumes        []Volume          `json:"volume,omitempty"`
	Network        string            `json:"network,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Cmd            []string          `json:"cmd,omitempty"`
	CmdPath        string            `json:"cmd_path,omitempty"`
	UserTTY        string            `json:"user_tty,omitempty"`
	Ports          []string          `json:"ports,omitempty"`
	IP             string            `json:"ip,omitempty"`
	Pid            int               `json:"pid,omitempty"`
	hostVeth       string
	peerName       string
	containerIP    string
	mountMgr       *MountManager
	deviceMgr      *DeviceManager
	userMgr        *UserManager
	networkManager *network.NetManager
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

func (c *Container) cleanup(logMgr *Manager) {
	logMgr.Output("Cleaning up container...")

	downCfg := ContainerDown{
		Hash: c.ID,
	}

	for _, vol := range c.Volumes {
		if vol.Name != "" {
			volume.MarkVolumeUnused(vol.Name, "", c.ID)
		} else {
			volume.MarkVolumeUnused("", vol.HostPath, c.ID)
		}
	}

	if c.Name != "" {
		if err := UnregisterName(c.Name); err != nil {
			logMgr.Daemon("WARN", "Failed to unregister container name %s: %v", c.Name, err)
		} else {
			logMgr.Daemon("INFO", "Unregistered container name %s", c.Name)
		}
	}

	if err := down(downCfg); err != nil {
		logMgr.Daemon("ERROR", "Cleanup failed: %v", err)
		logMgr.Output("Cleanup failed")
		return
	}

	logMgr.Output("Container cleaned up")
}

func volumeDelete(name, path string) error {
	if name != "" && path != "" {
		return fmt.Errorf("use either --name or --path, not both")
	}
	if name == "" && path == "" {
		return fmt.Errorf("specify volume by --name or --path")
	}

	vol, err := volume.FindVolume(name, path)
	if err != nil {
		return err
	}

	if vol == nil {
		return fmt.Errorf("volume not found")
	}

	if len(vol.UsedBy) > 0 {
		return fmt.Errorf(
			"volume '%s' is in use by container(s): %s",
			vol.Name,
			strings.Join(vol.UsedBy, ", "),
		)
	}

	if err := volume.UnregisterVolume(name, path); err != nil {
		return err
	}

	if err := os.RemoveAll(vol.Path); err != nil {
		_ = volume.RegisterVolume(vol.Name, vol.Path)
		return err
	}

	return nil
}

func volumeList(tty string) error {
	volumes, err := volume.ListVolumes()
	if err != nil {
		return err
	}

	ttyFile, _ := os.OpenFile(tty, os.O_WRONLY, 0644)
	defer ttyFile.Close()

	fmt.Fprintln(ttyFile, "")

	if len(volumes) == 0 {
		fmt.Fprintln(ttyFile, "No volumes found.")
		fmt.Fprintln(ttyFile, "")
		return nil
	}

	w := tabwriter.NewWriter(ttyFile, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tPATH\tCREATED\tUSED BY")

	for _, v := range volumes {
		usedBy := fmt.Sprintf("%d container(s)", len(v.UsedBy))
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			v.Name, v.Path, v.Created[:10], usedBy)
	}

	w.Flush()
	fmt.Fprintln(ttyFile, "")
	return nil
}

func findContainer(cmdPath, hash string) (*Container, error) {
	containers := ListContainers()
	if len(containers) == 0 {
		return nil, fmt.Errorf("no containers found")
	}

	if hash != "" {
		for _, c := range containers {
			if c.ID == hash {
				return &c, nil
			}
		}
		return nil, fmt.Errorf("container with hash '%s' not found", hash)
	}

	if cmdPath != "" {
		var matches []Container
		for _, c := range containers {
			if c.CmdPath == cmdPath {
				matches = append(matches, c)
			}
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("no containers started from path '%s'", cmdPath)
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("multiple containers started from path '%s'", cmdPath)
		}
		return &matches[0], nil
	}

	currentPath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current path: %v", err)
	}

	var matches []Container
	for _, c := range containers {
		if c.CmdPath == currentPath {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no containers started from current path '%s'", currentPath)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple containers started from current path '%s'", currentPath)
	}
	return &matches[0], nil
}

func RuntimeCleanup() {
	paths := []string{
		global.CtrsCfgPth,
		global.CtrsPth,
	}

	for _, baseDir := range paths {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			ctrDir := filepath.Join(baseDir, entry.Name())
			containerFile := filepath.Join(ctrDir, "container.json")

			data, err := os.ReadFile(containerFile)
			if err != nil {
				continue
			}

			var c Container
			if err := json.Unmarshal(data, &c); err != nil {
				continue
			}

			alive := false
			if c.Pid != 0 {
				if syscall.Kill(c.Pid, 0) == nil {
					alive = true
				}
			}

			if !alive {
				c.CleanupNetwork()
				os.RemoveAll(ctrDir)
			}
		}
	}
}

func ps(tty string) error {
	baseDir := global.CtrsCfgPth

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return fmt.Errorf("failed to read containers dir: %v", err)
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

	ttyFile, err := os.OpenFile(tty, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open tty %s: %v", tty, err)
	}
	defer ttyFile.Close()

	fmt.Fprintln(ttyFile, "")

	if len(liveContainers) == 0 {
		fmt.Fprintln(ttyFile, "No live containers.")
		fmt.Fprintln(ttyFile, "")
		return nil
	}

	w := tabwriter.NewWriter(ttyFile, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tIMAGE\tVOLUMES\tCMD\tPORTS\tIP")

	for _, c := range liveContainers {
		volumes := ""
		for i, v := range c.Volumes {
			if i > 0 {
				volumes += ", "
			}
			if v.Name != "" {
				volumes += v.Name
			} else {
				volumes += v.HostPath
			}
		}

		cmd := ""
		if len(c.Cmd) > 0 {
			cmd = strings.Join(c.Cmd, " ")
		}

		ports := ""
		if len(c.Ports) > 0 {
			ports = strings.Join(c.Ports, ", ")
		}

		name := c.Name
		if name == "" {
			name = c.ID[:12]
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			name, c.Image, volumes, cmd, ports, c.IP)
	}

	w.Flush()
	fmt.Fprintln(ttyFile, "")

	return nil
}

func ListContainers() []Container {
	baseDir := global.CtrsCfgPth
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}

	var containers []Container
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		containerFile := filepath.Join(baseDir, entry.Name(), "container.json")
		var c Container
		if data, err := os.ReadFile(containerFile); err == nil {
			if err := json.Unmarshal(data, &c); err == nil {
				containers = append(containers, c)
			}
		}
	}

	return containers
}

func isVolumeInUse(hostPath string) bool {
	baseDirs := []string{global.CtrsCfgPth, global.CtrsPth}

	for _, baseDir := range baseDirs {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			containerFile := filepath.Join(baseDir, entry.Name(), "container.json")
			pidFile := filepath.Join(baseDir, entry.Name(), entry.Name()+".pid")

			data, err := os.ReadFile(containerFile)
			if err != nil {
				continue
			}

			var c Container
			if err := json.Unmarshal(data, &c); err != nil {
				continue
			}

			alive := false
			if pidData, err := os.ReadFile(pidFile); err == nil {
				if pid, err := strconv.Atoi(strings.TrimSpace(string(pidData))); err == nil {
					if syscall.Kill(pid, 0) == nil {
						alive = true
					}
				}
			}

			if !alive {
				continue
			}

			for _, v := range c.Volumes {
				if v.HostPath == hostPath {
					return true
				}
			}
		}
	}

	return false
}

func arePortsFree(ports []string) error {
	usedPorts := make(map[int]string)

	for _, c := range ListContainers() {
		for _, p := range c.Ports {
			parts := strings.Split(p, ":")
			if len(parts) != 2 {
				continue
			}
			port, _ := strconv.Atoi(parts[0])
			usedPorts[port] = c.ID
		}
	}

	for _, p := range ports {
		parts := strings.Split(p, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid port format: %s", p)
		}

		hostPort, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid host port: %s", parts[0])
		}

		if !network.IsPortFree(hostPort) {
			return fmt.Errorf("host port %d is already in use by system", hostPort)
		}

		if id, ok := usedPorts[hostPort]; ok {
			return fmt.Errorf("host port %d is already used by container %s", hostPort, id)
		}
	}

	return nil
}

func down(downCfg ContainerDown) error {
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
		syscall.Kill(c.Pid, syscall.SIGTERM)

		timeout := time.After(10 * time.Second)
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()

	waitLoop:
		for {
			select {
			case <-timeout:
				syscall.Kill(c.Pid, syscall.SIGKILL)
			case <-tick.C:
				if syscall.Kill(c.Pid, 0) != nil {
					break waitLoop
				}
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	for _, vol := range c.Volumes {
		if vol.Name != "" {
			volume.MarkVolumeUnused(vol.Name, "", c.ID)
		} else {
			volume.MarkVolumeUnused("", vol.HostPath, c.ID)
		}
	}

	if err = c.CleanupNetwork(); err != nil {
		fmt.Printf("⚠️ Warning: failed to cleanup network: %v\n", err)
	}

	if c.Name != "" {
		err := UnregisterName(c.ID)
		if err != nil {
			fmt.Printf("⚠Warning: failed to unregister name %s: %v\n", c.Name, err)
		}
	}

	if c.mountMgr != nil {
		c.mountMgr.UnmountAll()
	} else {
		exec.Command("umount", "-l", filepath.Join(c.Rootfs, "merged")).Run()
		exec.Command("umount", "-l", filepath.Join(c.Rootfs, "work")).Run()
	}

	if err := os.RemoveAll(containerDir); err != nil {
		return fmt.Errorf("failed to remove container dir: %v", err)
	}

	if err := os.RemoveAll(c.Rootfs); err != nil {
		return fmt.Errorf("failed to remove rootfs: %v", err)
	}

	return nil
}

func (c *Container) CleanupNetwork() error {
	if c.IP == "" {
		return nil
	}

	if c.networkManager == nil {
		nm, err := network.NewNetworkManager()
		if err == nil {
			c.networkManager = nm
		}
	}

	if c.networkManager != nil {
		ipam := network.NewIPAM(c.networkManager)
		ipam.ReleaseIP(global.MainNetName, c.ID)
	}

	for _, port := range c.Ports {
		parts := strings.Split(port, ":")
		if len(parts) != 2 {
			continue
		}
		hostPort, _ := strconv.Atoi(parts[0])
		containerPort, _ := strconv.Atoi(parts[1])

		if c.networkManager != nil {
			network.RemovePortForward(strconv.Itoa(hostPort), c.IP, strconv.Itoa(containerPort))
		} else {
			network.RemovePortForward(parts[0], c.IP, parts[1])
		}
	}

	if c.hostVeth != "" {
		exec.Command("ip", "link", "del", c.hostVeth).Run()
	}

	return nil
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

	c.deviceMgr = NewDeviceManager(mergedRoot)
	if err := c.deviceMgr.CreateAll(); err != nil {
		return err
	}

	tmpDir := filepath.Join(mergedRoot, "tmp")
	os.MkdirAll(tmpDir, 01777)
	os.Chmod(tmpDir, 01777)

	c.userMgr = NewUserManager(mergedRoot)

	if c.Workdir != "" {
		workDir := filepath.Join(mergedRoot, c.Workdir)
		os.MkdirAll(workDir, 0755)
	}

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

	if c.Workdir != "" {
		sourceDir := c.CopyDir
		if sourceDir == "" {
			sourceDir = c.CmdPath
		}

		if sourceDir != "" {
			targetDir := filepath.Join(c.mountMgr.rootfs, c.Workdir)
			os.MkdirAll(targetDir, 0755)
			c.mountMgr.Add(BindMount(sourceDir, c.Workdir))
		}
	}

	for _, vol := range c.Volumes {
		targetDir := filepath.Join(c.mountMgr.rootfs, vol.ContainerPath)
		os.MkdirAll(targetDir, 0755)
		c.mountMgr.Add(BindMount(vol.HostPath, vol.ContainerPath))
	}
}

func (c *Container) run() error {
	logMgr, err := NewManager(c.ID, c.UserTTY)
	if err != nil {
		return fmt.Errorf("failed to create log manager: %v", err)
	}
	defer logMgr.Close()

	defer c.cleanup(logMgr)

	for _, vol := range c.Volumes {
		if vol.Name != "" {
			volume.MarkVolumeUsed(vol.Name, "", c.ID)
		} else {
			volume.MarkVolumeUsed("", vol.HostPath, c.ID)
		}
	}

	logMgr.Output("Starting container %s", c.ID[:12])
	logMgr.Daemon("INFO", "Setting up network for container %s", c.ID)

	if err := c.SetupNetwork(&NetworkConfig{
		NetworkName: global.MainNetName,
		Ports:       c.Ports,
	}); err != nil {
		logMgr.Daemon("ERROR", "Network setup failed: %v", err)
		return fmt.Errorf("network setup failed: %v", err)
	}
	logMgr.Output("Network configured, IP: %s", c.IP)

	mergedRoot := filepath.Join(c.Rootfs, "merged")
	envStr := strings.Join(c.buildEnv(), "\n")

	initScript := filepath.Join(mergedRoot, "container_init.sh")
	if _, err := os.Stat(initScript); err == nil {
		os.Chmod(initScript, 0755)
		logMgr.Daemon("DEBUG", "Found container_init.sh")
	}

	cmdArgs := []string{"./container_init.sh"}
	if len(c.Cmd) > 0 {
		cmdArgs = append(cmdArgs, c.Cmd...)
		logMgr.Daemon("DEBUG", "Command: %v", c.Cmd)
	}

	cmd := exec.Command("/proc/self/exe")
	cmd.Args = cmdArgs

	cmd.Stdout = logMgr.GetOutputWriter()
	cmd.Stderr = logMgr.GetOutputWriter()
	cmd.Stdin = os.Stdin

	env := []string{
		"_ZYLO_INIT=1",
		"ZYLO_ROOTFS=" + mergedRoot,
		"_ZYLO_ENV=" + envStr,
	}

	if c.Workdir != "" {
		env = append(env, "ZYLO_WORKDIR="+c.Workdir)
		logMgr.Daemon("DEBUG", "Workdir: %s", c.Workdir)
	}

	cmd.Env = append(os.Environ(), env...)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWPID,
	}

	logMgr.Output("Starting container process...")
	logMgr.Daemon("INFO", "Executing: %v", cmdArgs)

	if err := cmd.Start(); err != nil {
		logMgr.Daemon("ERROR", "Failed to start process: %v", err)
		return err
	}

	c.Pid = cmd.Process.Pid
	logMgr.Output("Container process started with PID: %d", c.Pid)
	logMgr.Daemon("INFO", "Container PID: %d", c.Pid)

	time.Sleep(200 * time.Millisecond)

	if err := syscall.Kill(c.Pid, 0); err != nil {
		logMgr.Daemon("ERROR", "Process died immediately: %v", err)
		logMgr.Output("Container process died immediately")
		return fmt.Errorf("process died immediately: %v", err)
	}
	logMgr.Daemon("INFO", "Process is alive")

	if err := c.persistContainerState(); err != nil {
		logMgr.Daemon("ERROR", "Failed to persist state: %v", err)
		cmd.Process.Kill()
		return fmt.Errorf("failed to persist container state: %v", err)
	}
	logMgr.Daemon("INFO", "Container state persisted")

	logMgr.Output("Attaching network...")
	if err := c.AttachNetwork(); err != nil {
		logMgr.Daemon("ERROR", "Network attachment failed: %v", err)
		logMgr.Output("Network attachment failed")
		cmd.Process.Kill()
		return err
	}
	logMgr.Output("Network attached successfully")
	logMgr.Output("Container %s is running!", c.ID[:12])

	err = cmd.Wait()

	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok && exitErr.ExitCode() == 143 {
			logMgr.Output("Container stopped")
		} else {
			logMgr.Output("Container finished with error: %v", err)
		}
	} else {
		logMgr.Output("Container finished successfully")
	}

	logMgr.Output("")

	return err
}

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
