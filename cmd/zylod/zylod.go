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
		fmt.Println("Usage: zylod <up|down|status>")
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "up":
		err = daemon.Up()
	case "down":
		err = daemon.Down()
	case "status":
		err = daemon.Status()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
