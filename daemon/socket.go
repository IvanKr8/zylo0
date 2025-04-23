package daemon

import (
	"fmt"
	"net"
	"os"
)

func sockExists(socketPath string) (error, bool) {
	if _, err := os.Stat(socketPath); err == nil {
		return nil, true
	} else if os.IsNotExist(err) {
		return nil, false
	} else {
		return err, false
	}
}

func SockDelete(socketPath string) error {
	if err, exists := sockExists(socketPath); err != nil {
		return err
	} else if !exists {
		return nil
	}

	err := os.Remove(socketPath)
	if err != nil {
		return fmt.Errorf("failed to delete socket: %v", err)
	}
	return nil
}

func sockCreate(socketPath string) (net.Listener, error) {
	os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create socket: %v", err)
	}

	return listener, nil
}

func SockListener() (net.Listener, error) {
	listener, err := net.Listen("unix", SocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on socket: %v", err)
	}

	return listener, nil
}
