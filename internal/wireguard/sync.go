package wireguard

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"wg-panel/internal/database"
)

// SyncWireGuardWithDatabase синхронизирует WireGuard с базой данных
// База данных - единственный источник истины
func SyncWireGuardWithDatabase(db *database.Database) error {
	log.Println("🔄 Синхронизация WireGuard с базой данных...")
	log.Println("📋 База данных - единственный источник истины")

	// Получаем список всех интерфейсов WireGuard
	cmd := exec.Command("wg", "show", "interfaces")
	output, err := cmd.Output()

	var activeInterfaces []string
	if err == nil && len(output) > 0 {
		activeInterfaces = strings.Fields(strings.TrimSpace(string(output)))
	}

	// Создаем карту интерфейсов из БД
	dbInterfaces := make(map[string]*database.Server)
	for i := range db.Servers {
		dbInterfaces[db.Servers[i].Interface] = &db.Servers[i]
	}

	// Удаляем все интерфейсы которых НЕТ в БД
	for _, iface := range activeInterfaces {
		if _, exists := dbInterfaces[iface]; !exists {
			log.Printf("  ❌ Интерфейс %s не найден в БД - удаление...", iface)
			if err := removeInterface(iface); err != nil {
				log.Printf("    ⚠️  Ошибка удаления интерфейса: %v", err)
			} else {
				log.Printf("    ✅ Интерфейс %s удален", iface)
			}
		}
	}

	// Также удаляем конфиг файлы которых нет в БД (но интерфейс не запущен)
	cmd = exec.Command("sh", "-c", "ls /etc/wireguard/*.conf 2>/dev/null | xargs -n1 basename | sed 's/.conf$//'")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		configInterfaces := strings.Fields(strings.TrimSpace(string(output)))
		for _, iface := range configInterfaces {
			if _, exists := dbInterfaces[iface]; !exists {
				configPath := fmt.Sprintf("/etc/wireguard/%s.conf", iface)
				log.Printf("  🗑️  Удаление конфига %s (не в БД)...", configPath)
				if err := os.Remove(configPath); err != nil {
					log.Printf("    ⚠️  Ошибка удаления конфига: %v", err)
				} else {
					log.Printf("    ✅ Конфиг удален")
				}
			}
		}
	}

	// Синхронизируем каждый сервер из БД
	for i := range db.Servers {
		server := &db.Servers[i]
		log.Printf("  🔧 Синхронизация сервера %s (%s)...", server.Name, server.Interface)

		// Проверяем запущен ли интерфейс
		isRunning := false
		for _, iface := range activeInterfaces {
			if iface == server.Interface {
				isRunning = true
				break
			}
		}

		if server.Enabled {
			// Сервер должен быть запущен
			if isRunning {
				// Если уже запущен - перезапускаем (чтобы применить правила iptables после очистки)
				log.Printf("    🔄 Интерфейс уже запущен, перезапускаю для применения правил...")
				if err := stopInterface(server.Interface); err != nil {
					log.Printf("    ⚠️  Ошибка остановки: %v", err)
				}
			}

			log.Printf("    🚀 Запуск интерфейса %s...", server.Interface)
			if err := startInterface(server, db); err != nil {
				log.Printf("    ❌ Ошибка запуска: %v", err)
				server.Enabled = false
				continue
			}

			// Синхронизируем peers
			log.Printf("    🧹 Очистка всех peers...")
			if err := clearAllPeers(server.Interface); err != nil {
				log.Printf("    ⚠️  Ошибка очистки: %v", err)
			}

			log.Printf("    📤 Загрузка peers из БД...")
			loadedCount := 0
			for j := range db.Clients {
				client := &db.Clients[j]
				if client.ServerID == server.ID && client.Enabled {
					if err := addPeerToWireGuard(server, *client); err != nil {
						log.Printf("    ⚠️  Ошибка добавления %s: %v", client.Name, err)
					} else {
						loadedCount++
						// Применяем пробросы портов
						if len(client.PortForwards) > 0 {
							log.Printf("    🔀 Применяю %d пробросов портов для %s...", len(client.PortForwards), client.Name)
							ApplyAllPortForwards(client)
						}
					}
				}
			}
			log.Printf("    ✅ Загружено %d peers", loadedCount)

		} else {
			// Сервер должен быть остановлен
			if isRunning {
				log.Printf("    🛑 Остановка интерфейса %s...", server.Interface)
				if err := stopInterface(server.Interface); err != nil {
					log.Printf("    ⚠️  Ошибка остановки: %v", err)
				} else {
					log.Printf("    ✅ Интерфейс остановлен")
				}
			}
		}
	}

	database.SaveDatabase(db)
	log.Println("✅ Синхронизация завершена")
	return nil
}

// removeInterface удаляет интерфейс WireGuard
func removeInterface(iface string) error {
	// Останавливаем интерфейс
	cmd := exec.Command("wg-quick", "down", iface)
	cmd.Run() // Игнорируем ошибку если уже остановлен

	// Удаляем конфиг файл
	configPath := fmt.Sprintf("/etc/wireguard/%s.conf", iface)
	return os.Remove(configPath)
}

// stopInterface останавливает интерфейс WireGuard
func stopInterface(iface string) error {
	cmd := exec.Command("wg-quick", "down", iface)
	return cmd.Run()
}

// startInterface запускает интерфейс WireGuard
func startInterface(server *database.Server, db *database.Database) error {
	log.Printf("    📝 Обновляю конфиг файл...")
	// Убеждаемся что конфиг файл существует
	if err := UpdateServerConfig(server, db); err != nil {
		return fmt.Errorf("ошибка создания конфига: %v", err)
	}

	log.Printf("    🔧 Включаю IP forwarding...")
	// Включаем IP forwarding
	if err := database.EnableIPForwarding(); err != nil {
		log.Printf("    ⚠️  Ошибка IP forwarding: %v", err)
	}

	log.Printf("    🚀 Запускаю wg-quick up %s...", server.Interface)
	// Запускаем интерфейс
	cmd := exec.Command("wg-quick", "up", server.Interface)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("    ❌ Вывод wg-quick: %s", string(output))
		return fmt.Errorf("wg-quick up failed: %v, output: %s", err, string(output))
	}
	log.Printf("    ✅ Интерфейс запущен успешно")

	// Применяем правила iptables вручную (не полагаемся на PostUp)
	if err := ApplyWireGuardIPTablesRules(server); err != nil {
		log.Printf("    ⚠️  Ошибка применения правил: %v", err)
	}

	return nil
}

// clearAllPeers очищает все peers из интерфейса WireGuard
func clearAllPeers(iface string) error {
	// Получаем список всех peers
	cmd := exec.Command("wg", "show", iface, "peers")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	peers := strings.Split(strings.TrimSpace(string(output)), "\n")
	removedCount := 0

	for _, peer := range peers {
		peer = strings.TrimSpace(peer)
		if peer == "" {
			continue
		}

		// Удаляем peer
		cmd := exec.Command("wg", "set", iface, "peer", peer, "remove")
		if err := cmd.Run(); err != nil {
			log.Printf("      ⚠️  Не удалось удалить peer %s: %v", peer[:16]+"...", err)
		} else {
			removedCount++
		}
	}

	if removedCount > 0 {
		log.Printf("      🗑️  Удалено %d peers", removedCount)
	}

	return nil
}

// updateNextClientIP обновляет счетчик следующего IP для клиентов
func updateNextClientIP(server *database.Server, db *database.Database) {
	// Парсим адрес сервера
	parts := strings.Split(server.Address, "/")
	if len(parts) != 2 {
		server.NextClientIP = 2
		return
	}

	ipParts := strings.Split(parts[0], ".")
	if len(ipParts) != 4 {
		server.NextClientIP = 2
		return
	}

	// Находим максимальный IP среди клиентов этого сервера
	maxIP := 1
	for _, client := range db.Clients {
		if client.ServerID != server.ID {
			continue
		}

		clientIPParts := strings.Split(client.Address, ".")
		if len(clientIPParts) == 4 {
			lastOctet, err := strconv.Atoi(clientIPParts[3])
			if err == nil && lastOctet > maxIP {
				maxIP = lastOctet
			}
		}
	}

	server.NextClientIP = maxIP + 1
}
