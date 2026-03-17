package main

import (
	"fmt"
	"os"
	"time"
	"zylo/internal/container"
)

func main() {
	if os.Getenv("_ZYLO_INIT") == "1" {
		container.ContainerInit()
		return
	}

	cfg := &container.Config{
		Image:     "postgres",
		CopyDir:   "",
		OpenPorts: []string{"5460:5460"},
		Env: map[string]string{
			"POSTGRES_PASSWORD": "tpass",
			"POSTGRES_USER":     "tuser",
			"POSTGRES_DB":       "tdb",
			"PG_PORT":           "5460",
		},
		Volumes: []container.Volume{
			{
				HostPath:      "/home/vania/zylo/data",
				ContainerPath: "/var/lib/postgresql/data",
			},
		},
		Cmd: []string{""},
	}

	ctr := container.NewContainer(cfg)
	if ctr == nil {
		fmt.Println("❌ Failed to create container")
		os.Exit(1)
	}
	fmt.Printf("✅ Container created with ID: %s\n", ctr.ID)

	if err := ctr.Setup(); err != nil {
		fmt.Printf("❌ Setup failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Container setup completed")

	ctr.Ports = cfg.OpenPorts

	go func() {
		if err := ctr.Run(); err != nil {
			fmt.Printf("❌ Run failed: %v\n", err)
		}
	}()

	time.Sleep(2 * time.Second)

	// Используем геттеры!
	pid := ctr.GetPid()
	if pid == 0 {
		fmt.Println("❌ Container PID is 0")
		os.Exit(1)
	}

	fmt.Printf("✅ Container running with PID: %d\n", pid)
	fmt.Printf("✅ Container IP: %s\n", ctr.GetIP())

	fmt.Println("✅ Done")
}
