package main

import (
	"fmt"
	"os"
	"zylo/daemon"
)

func main() {
	switch os.Args[1] {
	case "up":
		if err := daemon.Up(); err != nil {
			os.Exit(1)
		}
	case "down":
		if err := daemon.Down(); err != nil {
			os.Exit(1)
		}
	case "status":
		if _, err := daemon.Status(); err != nil {
			os.Exit(1)
		}
	default:
		fmt.Println("Usage: zylod [up|down|status]")
	}
}
