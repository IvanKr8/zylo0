package daemon

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"zylo/internal/system"
)

var (
	pidFl      = "/var/run/zylod.pid"
	SocketPath = "/var/run/zylo-socket.sock"
)

type daemonAction struct {
	Op  string `json:"op"`
	TTY string `json:"tty"`
}

func Up() error {
	if err := system.DoubleFork(); err != nil {
		return fmt.Errorf("error to create a background process: %v", err)
	}

	pid := os.Getpid()
	if err := os.WriteFile(pidFl, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return fmt.Errorf("не удалось записать PID: %v", err)
	}

	if err := daemon(); err != nil {
		return err
	}

	return nil
}

func Down() error {
	if err := os.Remove(SocketPath); err != nil {
		return fmt.Errorf("error removing socket: %v", err)
	}

	data, err := os.ReadFile(pidFl)
	if err != nil {
		return fmt.Errorf("error reading pid: %v", err)
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return fmt.Errorf("error parcing PID: %v", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("error finding process: %v", err)
	}

	err = process.Kill()
	if err != nil {
		return fmt.Errorf("error killing process: %v", err)
	}

	if err := os.Remove(pidFl); err != nil {
		return fmt.Errorf("error removing socket: %v", err)
	}

	return nil
}

func Status() (string, error) {
	conn, err := net.Dial("unix", SocketPath)
	if err != nil {
		fmt.Println("FAIL")
		return "", fmt.Errorf("error dialing socket: %v", err)
	}
	defer conn.Close()

	message := "status"
	_, err = conn.Write([]byte(message))
	if err != nil {
		fmt.Println("FAIL")
		return "", fmt.Errorf("error sending ping: %v", err)
	}

	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		fmt.Println("FAIL")
		return "", fmt.Errorf("error sending ping: %v", err)
	}

	response := string(buffer[:n])
	if response == "ok" {
		fmt.Println("OK")
		return "ok", nil
	}

	fmt.Println("FAIL")
	return "", fmt.Errorf("error sending status OK: response: %v", response)
}
