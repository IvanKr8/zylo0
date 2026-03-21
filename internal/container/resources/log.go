package container

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Manager struct {
	containerID string
	userTTY     *os.File
	outputFile  *os.File
	daemonFile  *os.File
}

func NewManager(containerID string, ttyPath string) (*Manager, error) {
	logDir := filepath.Join("/var/run/zylo/containers", containerID, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log dir: %v", err)
	}

	userTTY, err := os.OpenFile(ttyPath, os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open user tty %s: %v", ttyPath, err)
	}

	outputFile, err := os.OpenFile(
		filepath.Join(logDir, "output.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644,
	)
	if err != nil {
		userTTY.Close()
		return nil, fmt.Errorf("failed to create output.log: %v", err)
	}

	daemonFile, err := os.OpenFile(
		filepath.Join(logDir, "daemon.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644,
	)
	if err != nil {
		userTTY.Close()
		outputFile.Close()
		return nil, fmt.Errorf("failed to create daemon.log: %v", err)
	}

	return &Manager{
		containerID: containerID,
		userTTY:     userTTY,
		outputFile:  outputFile,
		daemonFile:  daemonFile,
	}, nil
}

func (lm *Manager) Close() error {
	if lm.userTTY != nil {
		lm.userTTY.Close()
	}
	if lm.outputFile != nil {
		lm.outputFile.Close()
	}
	if lm.daemonFile != nil {
		lm.daemonFile.Close()
	}
	return nil
}

func (lm *Manager) Output(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05")

	if msg == "" {
		fmt.Fprintln(lm.userTTY)
		fmt.Fprintln(lm.outputFile)
		return
	}

	fmt.Fprintf(lm.userTTY, "%s %s\n", timestamp, msg)
	fmt.Fprintf(lm.outputFile, "%s %s\n", timestamp, msg)
}

func (lm *Manager) Daemon(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006/01/02 15:04:05")

	fmt.Fprintf(lm.daemonFile, "%s [%s] %s\n", timestamp, level, msg)
}

func (lm *Manager) GetOutputWriter() io.Writer {
	return io.MultiWriter(lm.userTTY, lm.outputFile)
}
