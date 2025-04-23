package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"zylo/daemon"
	"zylo/internal/system"
)

func main() {
	pts, err := system.FindCmd()
	if err != nil {
		fmt.Println("failed to execute 'tty' command: %w", err)
		return
	}

	fmt.Println("pts", pts)

	switch os.Args[1] {
	case "up":
		conn, err := net.Dial("unix", daemon.SocketPath)
		if err != nil {
			os.Exit(1)
		}
		defer conn.Close()

		containerCfg := struct {
			Op  string `json:"op"`
			TTY string `json:"tty"`
		}{
			Op:  "UP",
			TTY: pts,
		}

		data, err := json.Marshal(containerCfg)
		if err != nil {
			os.Exit(1)
		}

		_, err = conn.Write(data)
		if err != nil {
			os.Exit(1)
		}

	default:
		fmt.Println("Usage: zylo [up]")
	}
}
