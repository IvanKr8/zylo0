package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"zylo/image"
	"zylo/internal/container"
	"zylo/internal/system"
)

func daemon() error {
	fmt.Println("Zylo daemon starting...")

	l, err := sockCreate(SocketPath)
	if err != nil {
		fmt.Printf("Socket error: %v\n", err)
		return err
	}
	defer l.Close()

	if err := system.Chmod(SocketPath, 0777); err != nil {
		fmt.Printf("Chmod error: %v\n", err)
		return err
	}

	fmt.Println("Daemon ready")

	go startHeartbeat()

	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		go router(conn)
	}
}

func router(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		return
	}

	command := strings.TrimSpace(string(buffer[:n]))

	switch command {

	case "ping":
		conn.Write([]byte("pong"))
		return

	case "status":
		conn.Write([]byte("ok"))
		return
	}

	container.RuntimeCleanup()

	var da daemonAction
	if err := json.Unmarshal([]byte(command), &da); err != nil {
		conn.Write([]byte("Invalid JSON format\n"))
		return
	}

	switch da.Op {

	case "UP":
		if da.TTY == "" {
			da.TTY = "/dev/stdout"
		}

		if err := container.Up(da.Path, da.TTY); err != nil {
			conn.Write([]byte(err.Error() + "\n"))
			return
		}

		conn.Write([]byte("Container started successfully\n"))

	case "DOWN":
		if err := container.Down(da.Hash, da.Path); err != nil {
			conn.Write([]byte(err.Error() + "\n"))
			return
		}

		conn.Write([]byte("Container stopped successfully\n"))

	case "PS":
		if err := container.Ps(da.TTY); err != nil {
			conn.Write([]byte(err.Error() + "\n"))
			return
		}

	case "LOGS":
		if err := container.Logs(da.Path, da.TTY, da.Hash); err != nil {
			conn.Write([]byte(err.Error() + "\n"))
			return
		}

	case "VOLUME_LIST":
		if err := container.VolumeList(da.TTY); err != nil {
			conn.Write([]byte(err.Error() + "\n"))
			return
		}

	case "VOLUME_DELETE":
		if err := container.VolumeDelete(da.TTY, da.Name, da.Path); err != nil {
			conn.Write([]byte(err.Error() + "\n"))
			return
		}

	case "IMAGE_LIST":
		if err := image.List(da.TTY); err != nil {
			conn.Write([]byte(err.Error() + "\n"))
			return
		}

	case "IMAGE_PULL":
		if err := image.Pull(da.TTY, da.Name); err != nil {
			conn.Write([]byte(err.Error() + "\n"))
			return
		}

	case "IMAGE_DELETE":
		if err := image.Delete(da.TTY, da.Name); err != nil {
			conn.Write([]byte(err.Error() + "\n"))
			return
		}

	default:
		conn.Write([]byte("Unknown operation\n"))
	}
}
