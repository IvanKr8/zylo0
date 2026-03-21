package container

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"zylo/global"
	res "zylo/internal/container/resources"
	namee "zylo/name"
	"zylo/network"
	"zylo/volume"
)

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

	if err = checkDiskSpace(ttyFile); err != nil {
		return err
	}

	if err = ensureImage(ttyFile, cfg); err != nil {
		return err
	}

	if err = checkNetwork(nm, ttyFile, cfg); err != nil {
		return err
	}

	if err = checkPorts(cfg.OpenPorts); err != nil {
		return err
	}

	if err = checkVolumes(cfg.Volumes); err != nil {
		return err
	}

	ctr, err := createAndSetupContainer(nm, cfg)
	if err != nil {
		fmt.Fprintf(ttyFile, "Setup failed: %v\n", err)
		return err
	}

	if err = checkName(ctr.Name, ctr.ID); err != nil {
		return err
	}

	runContainer(ctr)

	if err = waitForContainerStart(ctr, ttyFile); err != nil {
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

func (c *Container) run() error {
	logMgr, err := res.NewManager(c.ID, c.UserTTY)
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

func (c *Container) cleanup(logMgr *res.Manager) {
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
		if err := namee.UnregisterName(c.Name); err != nil {
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
