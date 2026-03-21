package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"zylo/global"
)

type Command struct {
	Op   string      `json:"op"`
	TTY  string      `json:"tty,omitempty"`
	Path string      `json:"path,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

func Run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("command required")
	}

	cmd := args[0]
	tty, err := getTTY()
	if err != nil {
		return err
	}

	conn, err := net.Dial(global.SocketNetworkType, global.SocketPath)
	if err != nil {
		return fmt.Errorf("daemon not running: %v", err)
	}
	defer conn.Close()

	setupSignalHandler(conn)

	var command Command

	switch cmd {
	case "up":
		command = Command{
			Op:   "UP",
			TTY:  tty,
			Path: getCwd(),
		}
	case "down":
		command = buildDownCommand(args[1:], tty)
	case "ps":
		command = Command{Op: "PS", TTY: tty}
	case "logs":
		command = buildLogsCommand(args[1:], tty)
	case "exec":
		command = buildExecCommand(args[1:], tty)
	case "volume":
		command = buildVolumeCommand(args[1:], tty)
	case "image":
		command = buildImageCommand(args[1:], tty)
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}

	data, err := json.Marshal(command)
	if err != nil {
		return err
	}

	if _, err := conn.Write(data); err != nil {
		return err
	}

	_, err = io.Copy(os.Stdout, conn)
	return err
}

func getCwd() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

func getTTY() (string, error) {
	ttyFile, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	defer ttyFile.Close()
	return ttyFile.Name(), nil
}

func setupSignalHandler(conn net.Conn) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		conn.Close()
	}()
}
