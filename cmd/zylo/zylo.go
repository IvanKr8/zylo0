package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"zylo/daemon"
	"zylo/internal/system"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: zylo <command>")
		os.Exit(1)
	}

	cmd := os.Args[1]

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("failed to get working directory:", err)
		os.Exit(1)
	}

	pts, err := system.FindCmd()
	if err != nil {
		fmt.Println("failed to execute 'tty' command:", err)
		return
	}

	conn, err := net.Dial("unix", daemon.SocketPath)
	if err != nil {
		os.Exit(1)
	}
	defer conn.Close()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-signalChan
		conn.Close()
	}()

	switch cmd {

	case "up":
		containerCfg := struct {
			Op   string `json:"op"`
			TTY  string `json:"tty"`
			Path string `json:"path"`
		}{
			Op:   "UP",
			TTY:  pts,
			Path: cwd,
		}

		data, _ := json.Marshal(containerCfg)
		conn.Write(data)

		io.Copy(os.Stdout, conn)

	case "down":
		downCmd := flag.NewFlagSet("down", flag.ExitOnError)
		hash := downCmd.String("hash", "", "container hash")

		downCmd.Parse(os.Args[2:])

		containerCfg := struct {
			Op   string `json:"op"`
			TTY  string `json:"tty"`
			Path string `json:"path"`
			Hash string `json:"hash"`
		}{
			Op:   "DOWN",
			TTY:  pts,
			Path: cwd,
			Hash: *hash,
		}

		data, _ := json.Marshal(containerCfg)
		conn.Write(data)

		io.Copy(os.Stdout, conn)

	case "ps":
		containerCfg := struct {
			Op  string `json:"op"`
			TTY string `json:"tty"`
		}{
			Op:  "PS",
			TTY: pts,
		}

		data, _ := json.Marshal(containerCfg)
		conn.Write(data)

		io.Copy(os.Stdout, conn)

	case "logs":
		logsCmd := flag.NewFlagSet("logs", flag.ExitOnError)
		hash := logsCmd.String("hash", "", "container hash")
		typeStr := logsCmd.String("type", "output", "container type (output or daemon)")

		logsCmd.Parse(os.Args[2:])

		if *typeStr != "output" && *typeStr != "daemon" {
			fmt.Printf("Error: invalid type '%s'. Must be 'output' or 'daemon'\n", *typeStr)
			logsCmd.PrintDefaults()
			os.Exit(1)
		}

		containerCfg := struct {
			Op   string `json:"op"`
			TTY  string `json:"tty"`
			Path string `json:"path"`
			Hash string `json:"hash"`
			Type string `json:"type"`
		}{
			Op:   "LOGS",
			TTY:  pts,
			Path: cwd,
			Hash: *hash,
			Type: *typeStr,
		}

		data, _ := json.Marshal(containerCfg)
		conn.Write(data)

		io.Copy(os.Stdout, conn)

	case "volume":
		if len(os.Args) < 3 {
			fmt.Println("Usage: zylo volume <command>")
			fmt.Println("Commands:")
			fmt.Println("  list              List all volumes")
			fmt.Println("  delete            Delete a volume (use --name or --path)")
			os.Exit(1)
		}

		volumeCmd := os.Args[2]

		switch volumeCmd {
		case "list":
			volumeCfg := struct {
				Op  string `json:"op"`
				TTY string `json:"tty"`
			}{
				Op:  "VOLUME_LIST",
				TTY: pts,
			}

			data, _ := json.Marshal(volumeCfg)
			conn.Write(data)

			io.Copy(os.Stdout, conn)

		case "delete":
			deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)
			name := deleteCmd.String("name", "", "volume name")
			path := deleteCmd.String("path", "", "volume path")

			deleteCmd.Parse(os.Args[3:])

			if *name == "" && *path == "" {
				fmt.Println("Error: specify volume by --name or --path")
				deleteCmd.PrintDefaults()
				os.Exit(1)
			}
			if *name != "" && *path != "" {
				fmt.Println("Error: use either --name or --path, not both")
				os.Exit(1)
			}

			volumeCfg := struct {
				Op   string `json:"op"`
				TTY  string `json:"tty"`
				Name string `json:"name,omitempty"`
				Path string `json:"path,omitempty"`
			}{
				Op:   "VOLUME_DELETE",
				TTY:  pts,
				Name: *name,
				Path: *path,
			}

			data, _ := json.Marshal(volumeCfg)
			conn.Write(data)

			io.Copy(os.Stdout, conn)

		default:
			fmt.Printf("Unknown volume command: %s\n", volumeCmd)
			fmt.Println("Available: list, delete")
			os.Exit(1)
		}

	case "image":
		if len(os.Args) < 3 {
			fmt.Println("Usage: zylo image <command>")
			fmt.Println("Commands:")
			fmt.Println("  list              List all images")
			fmt.Println("  pull              Download an image (use --name)")
			fmt.Println("  delete            Delete an image (use --name)")
			os.Exit(1)
		}

		imageCmd := os.Args[2]

		switch imageCmd {
		case "list":
			imageCfg := struct {
				Op  string `json:"op"`
				TTY string `json:"tty"`
			}{
				Op:  "IMAGE_LIST",
				TTY: pts,
			}

			data, _ := json.Marshal(imageCfg)
			conn.Write(data)

			io.Copy(os.Stdout, conn)

		case "pull":
			pullCmd := flag.NewFlagSet("pull", flag.ExitOnError)
			name := pullCmd.String("name", "", "image name")

			pullCmd.Parse(os.Args[3:])

			if *name == "" {
				fmt.Println("Error: image name required (--name)")
				pullCmd.PrintDefaults()
				os.Exit(1)
			}

			imageCfg := struct {
				Op   string `json:"op"`
				TTY  string `json:"tty"`
				Name string `json:"name"`
			}{
				Op:   "IMAGE_PULL",
				TTY:  pts,
				Name: *name,
			}

			data, _ := json.Marshal(imageCfg)
			conn.Write(data)

			io.Copy(os.Stdout, conn)

		case "delete":
			deleteCmd := flag.NewFlagSet("delete", flag.ExitOnError)
			name := deleteCmd.String("name", "", "image name")

			deleteCmd.Parse(os.Args[3:])

			if *name == "" {
				fmt.Println("Error: image name required (--name)")
				deleteCmd.PrintDefaults()
				os.Exit(1)
			}

			imageCfg := struct {
				Op   string `json:"op"`
				TTY  string `json:"tty"`
				Name string `json:"name"`
			}{
				Op:   "IMAGE_DELETE",
				TTY:  pts,
				Name: *name,
			}

			data, _ := json.Marshal(imageCfg)
			conn.Write(data)

			io.Copy(os.Stdout, conn)

		default:
			fmt.Printf("Unknown image command: %s\n", imageCmd)
			fmt.Println("Available: list, pull, delete")
			os.Exit(1)
		}

	default:
		fmt.Println("Usage: zylo [up|down|ps|logs|volume|image]")
		os.Exit(1)
	}
}
