package wireguard

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"wg-panel/internal/database"
)

// UpdateServerConfig обновляет конфиг файл сервера
func UpdateServerConfig(server *database.Server, db *database.Database) error {
	configContent := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
ListenPort = %d
PostUp = %s
PostDown = %s
`, server.PrivateKey, server.Address, server.ListenPort, server.PostUp, server.PostDown)

	// Добавляем всех клиентов
	for _, client := range db.Clients {
		if client.ServerID == server.ID && client.Enabled {
			configContent += fmt.Sprintf("\n[Peer]\nPublicKey = %s\nAllowedIPs = %s/32\n",
				client.PublicKey, client.Address)
		}
	}

	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", server.Interface)
	return os.WriteFile(configPath, []byte(configContent), 0600)
}

// CreateServer создает новый WireGuard сервер
func CreateServer(db *database.Database, name, address string, port int, dns string) (*database.Server, error) {
	// Генерируем ключи
	privateKey, publicKey, err := database.GenerateKeys()
	if err != nil {
		return nil, err
	}

	// Определяем имя интерфейса
	interfaceName := fmt.Sprintf("wg%d", len(db.Servers))

	// Получаем основной сетевой интерфейс
	netInterface := database.GetDefaultInterface()
	log.Printf("📡 Определен сетевой интерфейс для NAT: %s", netInterface)

	// Создаем сервер
	server := database.Server{
		ID:           fmt.Sprintf("%d", time.Now().UnixNano()),
		Name:         name,
		Interface:    interfaceName,
		PrivateKey:   privateKey,
		PublicKey:    publicKey,
		Address:      address,
		ListenPort:   port,
		DNS:          dns,
		Enabled:      true, // Запускаем сразу
		CreatedAt:    time.Now(),
		PostUp:       fmt.Sprintf("iptables -I FORWARD 1 -i %%i -j ACCEPT; iptables -I FORWARD 1 -o %%i -j ACCEPT; iptables -t nat -A POSTROUTING -o %s -j MASQUERADE", netInterface),
		PostDown:     fmt.Sprintf("iptables -D FORWARD -i %%i -j ACCEPT; iptables -D FORWARD -o %%i -j ACCEPT; iptables -t nat -D POSTROUTING -o %s -j MASQUERADE", netInterface),
		NextClientIP: 2,
	}

	// Включаем IP forwarding
	log.Println("🔧 Включаю IP forwarding...")
	if err := database.EnableIPForwarding(); err != nil {
		log.Println("⚠️  Предупреждение: не удалось включить IP forwarding:", err)
	} else {
		log.Println("✅ IP forwarding включен")
	}

	// Создаем конфиг файл
	log.Printf("📝 Создаю конфиг %s...", interfaceName)
	configContent := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s
ListenPort = %d
PostUp = %s
PostDown = %s
`, server.PrivateKey, server.Address, server.ListenPort, server.PostUp, server.PostDown)

	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", interfaceName)
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return nil, err
	}

	// Запускаем интерфейс сразу (так как Enabled = true)
	log.Printf("🚀 Запускаю интерфейс %s...", interfaceName)
	cmd := exec.Command("wg-quick", "up", interfaceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ Ошибка запуска: %s", string(output))
		return nil, fmt.Errorf("failed to start interface: %v, output: %s", err, string(output))
	}
	log.Printf("✅ Интерфейс %s запущен", interfaceName)

	return &server, nil
}

// ToggleServer включает/выключает сервер
func ToggleServer(server *database.Server) error {
	if server.Enabled {
		// Выключаем
		cmd := exec.Command("wg-quick", "down", server.Interface)
		if err := cmd.Run(); err != nil {
			return err
		}
		server.Enabled = false
	} else {
		// Включаем IP forwarding перед запуском
		database.EnableIPForwarding()

		// Включаем
		cmd := exec.Command("wg-quick", "up", server.Interface)
		if err := cmd.Run(); err != nil {
			return err
		}
		server.Enabled = true
	}
	return nil
}

// DeleteServer удаляет сервер
func DeleteServer(server *database.Server) error {
	// Останавливаем интерфейс
	if server.Enabled {
		exec.Command("wg-quick", "down", server.Interface).Run()
	}

	// Удаляем конфиг файл
	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", server.Interface)
	return os.Remove(configPath)
}

// UpdateStats обновляет статистику из WireGuard
func UpdateStats(db *database.Database) {
	for _, server := range db.Servers {
		if !server.Enabled {
			continue
		}

		cmd := exec.Command("wg", "show", server.Interface, "dump")
		output, err := cmd.Output()
		if err != nil {
			continue
		}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines[1:] { // Пропускаем первую строку (заголовок)
			if line == "" {
				continue
			}

			fields := strings.Split(line, "\t")
			if len(fields) < 8 {
				continue
			}

			pubKey := fields[0]
			endpoint := fields[2] // IP:Port клиента
			lastHandshake, _ := strconv.ParseInt(fields[4], 10, 64)
			rxBytes, _ := strconv.ParseInt(fields[5], 10, 64) // received
			txBytes, _ := strconv.ParseInt(fields[6], 10, 64) // sent

			// Обновляем статистику клиента
			for i := range db.Clients {
				if db.Clients[i].PublicKey == pubKey && db.Clients[i].ServerID == server.ID {
					db.Clients[i].RxBytes = rxBytes
					db.Clients[i].TxBytes = txBytes
					db.Clients[i].Endpoint = endpoint
					if lastHandshake > 0 {
						db.Clients[i].LastHandshake = time.Unix(lastHandshake, 0)
					}
					break
				}
			}
		}
	}

	database.SaveDatabase(db)
}

// UpdateStatsLoop обновляет статистику каждые 5 секунд
func UpdateStatsLoop(db *database.Database) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		UpdateStats(db)
	}
}
