package container

import (
	"fmt"
	"time"
)

type container struct {
	hash            string
	name            string
	status          string
	pid             int
	image           string
	workdir         string
	rootfs          string
	fs              filesystem
	envVars         map[string]string
	ports           []string
	commands        []string
	copyDir         string
	entrypoint      string
	modifiedAt      time.Time
	resourcesConfig resourcesConfig
	net
}

type net struct {
	name string
}

type filesystem struct {
	merged string
	work   string
	lower  string
	upper  string
}

type resourcesConfig struct {
	cpu    int64
	memory int64
}

func Up() error {
	// Find ZyloFile in the project.
	// The zyFl variable stores the path to the file.
	// Instead of string "ZyloFile" you can enter any file name,
	// and it will search for it from the project root
	zyFl, err := findZyFile("ZyloFile")
	if err != nil {
		return fmt.Errorf("could not find ZyloFile")
	}

	var ctr container
	ctr.modifiedAt = time.Now()
	ctr.net.name = "demozylo0"

	// Parse ZyloFile where the function accepts a configuration pointer.
	// Returns an already partially assembled container config.
	err = ctr.parseZyFile(zyFl)
	if err != nil {
		return fmt.Errorf("could not parse ZyloFile: %v", err)
	}

	if err = ctr.run(); err != nil {
		return fmt.Errorf("could not run container: %v", err)
	}

	return nil
}
