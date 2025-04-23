package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"zylo/internal/container"
	"zylo/internal/system"
)

func daemon() error {
	l, err := sockCreate(SocketPath)
	if err != nil {
		return err
	}
	defer l.Close()

	err = system.Chmod(SocketPath, 0777)
	if err != nil {
		return err
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		go router(conn)
	}
}

func router(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 1024)

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

	var da daemonAction
	err = json.Unmarshal([]byte(command), &da)
	if err != nil {
		conn.Write([]byte("Invalid JSON format\n"))
		return
	}

	switch da.Op {
	case "UP":
		err = container.Up()
		if err != nil {
			return
		}
	default:
		conn.Write([]byte(fmt.Sprintln("Unknown operation")))
	}
}
