package container

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"zylo/volume"

	"zylo/global"
	"zylo/internal/identifiers"
)

func findZyFile(p string, fileName string) (string, error) {
	path := filepath.Join(p, fileName)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no zylo files found")
		}
		return "", err
	}

	if info.IsDir() {
		return "", fmt.Errorf("expected file but got directory")
	}

	return path, nil
}

func (containerCfg *container) parseZyFile(zyloFlPath string) error {
	content, err := ioutil.ReadFile(zyloFlPath)
	if err != nil {
		return fmt.Errorf("failed to read ZyloFile: %v", err)
	}

	containerCfg.hash = identifiers.Hash()
	containerCfg.envVars = make(map[string]string)
	containerCfg.ports = []string{}
	containerCfg.commands = []string{}
	containerCfg.volumes = []Volume{}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "!!") {
			continue
		}

		if err = containerCfg.parseLine(line); err != nil {
			return fmt.Errorf("failed to parse line '%s': %v", line, err)
		}
	}

	if err = scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan ZyloFile: %v", err)
	}

	return nil
}

func (containerCfg *container) parseLine(line string) error {
	idx := strings.Index(line, " ")
	if idx == -1 {
		return fmt.Errorf("invalid instruction: %s", line)
	}

	instruction := line[:idx]
	rawArgs := strings.TrimSpace(line[idx+1:])

	switch instruction {
	case "USE_IMAGE":
		return setImage(rawArgs, containerCfg)
	case "SET_WORKDIR":
		return setWorkdir(rawArgs, containerCfg)
	case "COPY":
		return setCopydir(rawArgs, containerCfg)
	case "NETWORK":
		return setNetwork(rawArgs, containerCfg)
	case "EXECUTE":
		return setCommands(rawArgs, containerCfg)
	case "VOLUME":
		return setVolume(rawArgs, containerCfg)
	case "SET_ENV":
		return setEnv(rawArgs, containerCfg)
	case "OPEN_PORT":
		return setPort(rawArgs, containerCfg)
	case "START_WITH":
		return setStartCommand(rawArgs, containerCfg)
	default:
		return fmt.Errorf("unknown instruction: %s", instruction)
	}
}

func setImage(arg string, config *container) error {
	config.image = arg
	return nil
}

func setVolume(arg string, config *container) error {
	if !strings.Contains(arg, ":") {
		containerPath := arg
		volName := identifiers.Hash()
		hostPath := filepath.Join(global.VolPth, config.hash, volName)

		volume.RegisterVolume(volName, hostPath)

		if err := os.MkdirAll(hostPath, 0755); err != nil {
			return fmt.Errorf("failed to create anonymous volume: %v", err)
		}

		config.volumes = append(config.volumes, Volume{
			Name:          volName,
			HostPath:      hostPath,
			ContainerPath: containerPath,
		})

		return nil
	}

	parts := strings.SplitN(arg, ":", 2)
	left := parts[0]
	right := parts[1]

	if strings.HasPrefix(left, "/") {
		containerPath := left
		hostPath := right

		existing, _ := volume.FindVolume("", hostPath)
		if existing == nil {
			volume.RegisterVolume("", hostPath)
		}

		if err := os.MkdirAll(hostPath, 0755); err != nil {
			return fmt.Errorf("failed to create host volume path: %v", err)
		}

		config.volumes = append(config.volumes, Volume{
			HostPath:      hostPath,
			ContainerPath: containerPath,
		})

		return nil
	}

	volName := left
	containerPath := right
	hostPath := filepath.Join(global.VolPth, volName)

	existing, _ := volume.FindVolume(volName, "")
	if existing == nil {
		volume.RegisterVolume(volName, hostPath)
	}

	if err := os.MkdirAll(hostPath, 0755); err != nil {
		return fmt.Errorf("failed to create named volume: %v", err)
	}

	config.volumes = append(config.volumes, Volume{
		Name:          volName,
		HostPath:      hostPath,
		ContainerPath: containerPath,
	})

	return nil
}

func setWorkdir(arg string, config *container) error {
	basePath := "/source"

	if arg == "." || arg == "" {
		config.workdir = basePath
	} else {
		trimmed := strings.TrimPrefix(arg, "/")
		config.workdir = filepath.Join(basePath, trimmed)
	}

	return nil
}

func setCopydir(arg string, config *container) error {
	if arg == "." {
		config.copyDir = config.cmdPath
		return nil
	}

	config.copyDir = fmt.Sprintf("%s%s", config.cmdPath, arg)
	return nil
}

func setCommands(arg string, config *container) error {
	config.commands = append(config.commands, arg)
	return nil
}

func setNetwork(arg string, config *container) error {
	if arg == global.MainNetName {
		return fmt.Errorf("network %s is reserved", global.MainNetName)
	}

	config.cNetwork.name = arg

	return nil
}

func setEnv(arg string, config *container) error {
	parts := strings.SplitN(arg, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("SET_ENV expects format KEY:value")
	}

	config.envVars[parts[0]] = parts[1]
	return nil
}

func setPort(arg string, config *container) error {
	config.ports = append(config.ports, arg)
	return nil
}

func setStartCommand(arg string, config *container) error {
	var cmd []string
	if err := json.Unmarshal([]byte(arg), &cmd); err != nil {
		return fmt.Errorf("START_WITH must be JSON array: %v", err)
	}
	config.commands = cmd
	return nil
}
