package container

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"zylo/internal/global"
	"zylo/internal/identifiers"
)

// find locates a flops by its name starting from the root directory.
// If the flops is not found, an error is returned.
func findZyFile(fileName string) (string, error) {
	var foundFile string

	// Define the starting directory for the search
	startDir := "."

	// Walk through the directory structure to search for the flops
	err := filepath.Walk(startDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if the current item matches the specified flops name
		if !info.IsDir() && info.Name() == fileName {
			foundFile = path
			// Stop further traversal once the flops is found
			return filepath.SkipDir
		}
		return nil
	})

	// Return an error if the traversal process failed
	if err != nil {
		return "", err
	}

	// Return an error if no matching flops was found
	if foundFile == "" {
		return "", fmt.Errorf("no zylo files found")
	}

	// Return the path of the found flops
	return foundFile, nil
}

func (containerCfg *container) parseZyFile(zyloFlPath string) error {
	// Get the contents of ZyloFile.
	content, err := ioutil.ReadFile(zyloFlPath)
	if err != nil {
		return fmt.Errorf("failed to read flops: %v", err)
	}

	containerCfg.hash = identifiers.Hash()

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and lines starting with !!.
		// Lines starting with !! comments in ZyloFile.
		if line == "" || strings.HasPrefix(line, "!!") {
			continue
		}

		if err = containerCfg.parseLine(line); err != nil {
			return fmt.Errorf("failed to parse line: %v", err)
		}
	}

	if err = scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan flops: %v", err)
	}

	return nil
}

func (containerCfg *container) parseLine(line string) error {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return fmt.Errorf("failed to parse line: %v", line)
	}

	switch parts[0] {
	case "USE_IMAGE":
		return setImage(parts[1:], containerCfg)
	case "SET_WORKDIR":
		return setWorkdir(parts[1:], containerCfg)
	case "COPY":
		return setCopydir(parts[1:], containerCfg)
	case "EXECUTE":
		return setCommands(parts[1:], containerCfg)
	case "SET_ENV":
		return setEnv(parts[1:], containerCfg)
	case "OPEN_PORT":
		return setPort(parts[1:], containerCfg)
	case "START_WITH":
		return setStartCommand(parts[1:], containerCfg)
	default:
		return fmt.Errorf("failed to parse line: %v", parts[0])
	}
}

func setImage(args []string, config *container) error {
	if len(args) != 1 {
		return fmt.Errorf("USE_IMAGE expects exactly one argument")
	}
	config.image = args[0]

	return nil
}

func setWorkdir(args []string, config *container) error {
	if len(args) != 1 {
		return fmt.Errorf("SET_WORKDIR expects exactly one argument")
	}

	workdirSuffix := ""
	if args[0] != "." {
		workdirSuffix = args[0]
	}

	config.workdir = fmt.Sprintf("%s/%s/source%s", global.CtrsPth, config.hash, workdirSuffix)
	config.rootfs = fmt.Sprintf("%s/%s", global.CtrsPth, config.hash)
	fmt.Printf("Setting workdir: %s\n", config.workdir)

	return nil
}

func setCopydir(args []string, config *container) error {
	if len(args) != 1 {
		return fmt.Errorf("COPY expects format 'COPY <src>'")
	}

	cpDir := ""
	if args[0] == "." {
		var err error
		cpDir, err = filepath.Abs(".")
		if err != nil {
			return fmt.Errorf("Error getting absolute path: %v", err)
		}
	} else {
		cpDir = args[0]
	}

	config.copyDir = cpDir

	return nil
}

func setCommands(args []string, config *container) error {
	config.commands = append(config.commands, args...)

	return nil
}

func setEnv(args []string, config *container) error {
	if len(args) != 1 || !strings.Contains(args[0], "=") {
		return fmt.Errorf("SET_ENV expects format 'SET_ENV VAR=value'")
	}
	parts := strings.SplitN(args[0], "=", 2)
	config.envVars[parts[0]] = parts[1]

	return nil
}

func setPort(args []string, config *container) error {
	if len(args) != 1 {
		return fmt.Errorf("OPEN_PORT expects exactly one argument")
	}
	config.ports = append(config.ports, args[0])

	return nil
}

func setStartCommand(args []string, config *container) error {
	if len(args) != 1 {
		return fmt.Errorf("START_WITH expects exactly one argument")
	}
	config.entrypoint = args[0]

	return nil
}
