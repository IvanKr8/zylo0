package kernel

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"zylo/global"
	"zylo/internal/container"
	"zylo/network"
)

func TestContainerWithNetwork(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("Test requires root privileges. Run with sudo go test")
	}

	fmt.Println("asda")

	fmt.Println("🧪 Testing container with network...")

	// 1. Создаем/проверяем сеть zylo0
	nm, err := network.NewNetworkManager()
	if err != nil {
		t.Fatalf("❌ Failed to create network manager: %v", err)
	}

	if !nm.NetworkExists("zylo0") {
		fmt.Println("   Creating zylo0 network...")
		if err := nm.CreateDefaultNetwork(); err != nil {
			t.Fatalf("❌ Failed to create network: %v", err)
		}
	}

	netConfig, err := nm.GetNetwork("zylo0")
	if err != nil {
		t.Fatalf("❌ Failed to get network config: %v", err)
	}
	fmt.Printf("   ✅ Using network: %s (gateway: %s)\n", netConfig.Name, netConfig.Gateway)

	cfg := &container.Config{
		Image:     "postgres",
		CopyDir:   "",
		OpenPorts: []string{"5432:5432"},
		Env: map[string]string{
			"POSTGRES_PASSWORD": "testpass",
			"POSTGRES_USER":     "testuser",
			"POSTGRES_DB":       "testdb",
		},
		Cmd: []string{"/bin/bash"},
	}

	// 3. Проверяем что образ существует
	imagePath := filepath.Join(global.ImgsPth, cfg.Image)
	if _, err := os.Stat(imagePath); err != nil {
		t.Fatalf("❌ Image not found at %s", imagePath)
	}

	// 4. Создаем контейнер
	ctr := NewContainer(cfg)
	if ctr == nil {
		t.Fatal("❌ Failed to create container")
	}
	fmt.Printf("   ✅ Container created with ID: %s\n", ctr.ID)

	// 5. Настраиваем контейнер
	if err := ctr.Setup(); err != nil {
		t.Fatalf("❌ Setup failed: %v", err)
	}
	fmt.Println("   ✅ Container setup completed")

	// 6. Сохраняем порты в контейнер (из конфига)
	ctr.Ports = cfg.OpenPorts

	// 7. Запускаем контейнер с сетью
	fmt.Println("\n   Starting container with network...")

	// Запускаем в фоне, так как PostgreSQL будет работать долго
	errChan := make(chan error)
	go func() {
		errChan <- ctr.Run()
	}()

	// 8. Ждем запуска PostgreSQL
	fmt.Println("   Waitiang for PostgreSQL to start...")
	time.Sleep(5 * time.Second)

	// 9. Проверяем что контейнер жив
	if ctr.pid == 0 {
		t.Fatal("❌ Container PID is 0")
	}

	proc, err := os.FindProcess(ctr.pid)
	if err != nil {
		t.Fatalf("❌ Process вфыnot found: %v", err)
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("❌ Container died: %v", err)
	}
	fmt.Printf("   ✅ Container running with PID: %d\n", ctr.pid)
	fmt.Printf("   ✅ Container IP: %s\n", ctr.IP)

	time.Sleep(2 * time.Second)

	// 10. Проверяем что интерфейс в контейнере настроен
	checkCmd := exec.Command("nsenter", "-t", fmt.Sprintf("%d", ctr.pid), "-n", "ip", "addr", "show")
	if output, err := checkCmd.CombinedOutput(); err == nil {
		fmt.Printf("   ✅ Container network:\n%s", output)
	}

	// 11. Проверяем доступность PostgreSQL с хоста
	fmt.Println("\n   Testing PostgreSQL connection...")

	// Пробуем подключиться несколько раз (PostgreSQL может еще стартовать)
	maxAttempts := 10
	for i := 1; i <= maxAttempts; i++ {
		pgIsReady := exec.Command("pg_isready", "-h", "localhost", "-p", "5432", "-U", "testuser")
		if output, err := pgIsReady.CombinedOutput(); err == nil {
			fmt.Printf("   ✅ PostgreSQL is ready (attempt %d)\n", i)
			fmt.Printf("      Response: %s", output)
			break
		}

		if i == maxAttempts {
			t.Log("   ⚠️ PostgreSQL not ready after 10 attempts")
		} else {
			fmt.Printf("   Waiting for PostgreSQL... (%d/%d)\n", i, maxAttempts)
			time.Sleep(2 * time.Second)
		}
	}

	// 12. Проверяем прямой доступ по IP контейнера
	pingCmd := exec.Command("ping", "-c", "2", "-W", "1", ctr.IP)
	if err := pingCmd.Run(); err != nil {
		t.Logf("   ⚠️ Cannot ping container IP: %v", err)
	} else {
		fmt.Printf("   ✅ Container IP %s is reachable\n", ctr.IP)
	}

	// 13. Проверяем что порт проброшен
	ncCmd := exec.Command("nc", "-zv", "localhost", "5432")
	if err := ncCmd.Run(); err != nil {
		t.Logf("   ⚠️ Port 5432 not accessible: %v", err)
	} else {
		fmt.Println("   ✅ Port 5432 is forwarded correctly")
	}

	// 14. Даем контейнеру поработать 30 секунд
	fmt.Println("\n⏳ Container will run for 30 seconds...")
	for i := 30; i > 0; i-- {
		fmt.Printf("\r   %d seconds remaining... ", i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println()

	// 15. Останавливаем контейнер
	fmt.Println("\n   Stopping container...")
	if err := proc.Kill(); err != nil {
		t.Logf("   ⚠️ Failed to kill process: %v", err)
	}

	// Ждем завершения
	select {
	case err := <-errChan:
		if err != nil {
			t.Logf("Container exited: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Log("Container did not exit cleanly")
	}

	// 16. Очистка сети
	fmt.Println("\n   Cleaning up network...")
	if ctr.networkManager != nil && ctr.IP != "" {
		ipam := network.NewIPAM(ctr.networkManager)
		ipam.ReleaseIP("zylo0", ctr.ID)

		for _, port := range ctr.Ports {
			parts := strings.Split(port, ":")
			if len(parts) == 2 {
				hostPort, _ := strconv.Atoi(parts[0])
				containerPort, _ := strconv.Atoi(parts[1])
				ctr.networkManager.RemovePortForward(ctr.IP, hostPort, containerPort)
			}
		}
	}

	// 17. Очистка файловой системы
	fmt.Println("   Cleaning up filesystem...")

	mergedRoot := filepath.Join(ctr.Rootfs, "merged")
	syscall.Unmount(filepath.Join(mergedRoot, "proc"), syscall.MNT_DETACH)
	syscall.Unmount(filepath.Join(mergedRoot, "sys"), syscall.MNT_DETACH)
	syscall.Unmount(filepath.Join(mergedRoot, "dev/shm"), syscall.MNT_DETACH)
	syscall.Unmount(mergedRoot, syscall.MNT_DETACH)

	if err := os.RemoveAll(ctr.Rootfs); err != nil {
		t.Logf("   ⚠️ Failed to remove rootfs: %v", err)
	} else {
		fmt.Println("   ✅ Rootfs cleaned up")
	}

	fmt.Println("✅ TestContainerWithNetwork passed successfully!")
}
