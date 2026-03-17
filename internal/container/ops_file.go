package container

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

type file struct {
	Reader io.Reader
	Closer io.Closer
}

func readFile(path string) (*file, error) {
	fl, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(fl)
	return &file{Reader: reader, Closer: fl}, nil
}

func (fl *file) getEnv() ([]string, error) {
	var envVars []string

	scanner := bufio.NewScanner(fl.Reader)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" {
			continue
		}

		parts := strings.Split(line, "=")
		if len(parts) == 2 {
			envVars = append(envVars, parts[0]+"="+parts[1])
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading a file: %w", err)
	}

	return envVars, nil
}
