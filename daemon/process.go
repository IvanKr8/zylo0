package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	image2 "zylo/image"
	"zylo/internal/container"
	"zylo/internal/system"
)

func daemon() error {
	fmt.Println("DAEMON: starting...")

	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("Zylo daemon started")
	fmt.Println("DAEMON: logger created")

	l, err := sockCreate(SocketPath)
	if err != nil {
		logger.Printf("Failed to create socket: %v", err)
		fmt.Printf("DAEMON: sockCreate error: %v\n", err)
		return err
	}
	defer l.Close()
	fmt.Println("DAEMON: socket created")

	err = system.Chmod(SocketPath, 0777)
	if err != nil {
		logger.Printf("Failed to chmod socket: %v", err)
		fmt.Printf("DAEMON: chmod error: %v\n", err)
		return err
	}
	fmt.Println("DAEMON: socket permissions set")

	logger.Println("Daemon ready, accepting connections")
	fmt.Println("DAEMON: entering accept loop")

	for {
		conn, err := l.Accept()
		if err != nil {
			logger.Printf("Error accepting connection: %v", err)
			fmt.Printf("DAEMON: accept error: %v\n", err)
			continue
		}
		fmt.Println("DAEMON: new connection accepted")
		go router(conn, logger)
	}
}

func router(conn net.Conn, logger *log.Logger) {
	defer conn.Close()

	logger.Println("New connection accepted")

	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		logger.Printf("Error reading from connection: %v", err)
		return
	}

	command := strings.TrimSpace(string(buffer[:n]))
	logger.Printf("Received command: %s", command)

	switch command {
	case "ping":
		logger.Println("Handling ping command")
		conn.Write([]byte("pong"))
		return
	case "status":
		logger.Println("Handling status command")
		conn.Write([]byte("ok"))
		return
	}

	container.RuntimeCleanup()

	var da daemonAction
	err = json.Unmarshal([]byte(command), &da)
	if err != nil {
		logger.Printf("Invalid JSON format: %v", err)
		conn.Write([]byte("Invalid JSON format\n"))
		return
	}

	logger.Printf("Parsed action: Op=%s, TTY=%s", da.Op, da.TTY)

	switch da.Op {
	case "UP":
		if da.TTY == "" {
			log.Printf(">>> DEBUG: action.TTY = '%s'", da.TTY)
			log.Printf("ERROR: TTY is empty for UP command")
			da.TTY = "/dev/stdout"
		}
		logger.Println("Executing container.Up()")
		err = container.Up(da.Path, da.TTY)
		if err != nil {
			logger.Printf("Container.Up() failed: %v", err)
			conn.Write([]byte(err.Error() + "\n"))
			return
		}
		logger.Println("Container.Up() completed successfully")
		conn.Write([]byte("Container started successfully\n"))
	case "DOWN":
		logger.Println("Executing container.Down()")
		err = container.Down(da.Hash, da.Path)
		if err != nil {
			logger.Printf("Container.Down() failed: %v", err)
			conn.Write([]byte(err.Error() + "\n"))
			return
		}
		logger.Println("Container.Down() completed successfully")
		conn.Write([]byte("Container started successfully\n"))
	case "PS":
		logger.Println("Executing container.PS()")
		err = container.Ps(da.TTY)
		if err != nil {
			logger.Printf("Container.Ps() failed: %v", err)
			conn.Write([]byte(err.Error() + "\n"))
			return
		}
		logger.Println("Container.Ps() completed successfully")
	case "LOGS":
		logger.Println("Executing container.Logs()")
		err = container.Logs(da.Path, da.TTY, da.Hash)
		if err != nil {
			logger.Printf("Container.Logs() failed: %v", err)
			conn.Write([]byte(err.Error() + "\n"))
			return
		}
	case "VOLUME_LIST":
		logger.Println("Executing volume.List()")
		err = container.VolumeList(da.TTY)
		if err != nil {
			logger.Printf("Container.VolumeList() failed: %v", err)
			conn.Write([]byte(err.Error() + "\n"))
			return
		}
	case "VOLUME_DELETE":
		logger.Println("Executing volume.Delete()")
		err = container.VolumeDelete(da.TTY, da.Name, da.Path)
		if err != nil {
			logger.Printf("Container.VolumeDelete() failed: %v", err)
			conn.Write([]byte(err.Error() + "\n"))
			return
		}
	case "IMAGE_LIST":
		logger.Println("Executing image.List()")
		err = image2.List(da.TTY)

	case "IMAGE_PULL":
		logger.Println("Executing image.Pull()")
		err = image2.Pull(da.TTY, da.Name)

	case "IMAGE_DELETE":
		logger.Println("Executing image.Delete()")
		err = image2.Delete(da.TTY, da.Name)
	default:
		logger.Printf("Unknown operation: %s", da.Op)
		conn.Write([]byte("Unknown operation\n"))
	}
}
