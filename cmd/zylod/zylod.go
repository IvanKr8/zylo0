package main

import (
	"fmt"
	"os"
	"zylo/daemon"
	"zylo/internal/container"
)

func main() {
	if os.Getenv("_ZYLO_INIT") == "1" {
		container.ContainerInit()
		return
	}

	if len(os.Args) < 2 {
		fmt.Println("Usage: zylod [up|down|status]")
		os.Exit(1)
	}

	switch os.Args[1] {

	case "up":
		if err := daemon.Up(); err != nil {
			fmt.Println("up failed:", err)
			os.Exit(1)
		}

	case "down":
		if err := daemon.Down(); err != nil {
			fmt.Println("down failed:", err)
			os.Exit(1)
		}

	case "status":
		if _, err := daemon.Status(); err != nil {
			fmt.Println("status failed:", err)
			os.Exit(1)
		}

	default:
		fmt.Println("Usage: zylod [up|down|status]")
		os.Exit(1)
	}
}
