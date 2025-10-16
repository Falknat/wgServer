package database

import (
	"fmt"
	"strings"
)

// GetNextAvailableNetwork возвращает следующую доступную подсеть
func GetNextAvailableNetwork(db *Database) string {
	// Начинаем с 10.0.0.1/24
	for i := 10; i < 255; i++ {
		network := fmt.Sprintf("%d.0.0.1/24", i)
		if IsNetworkAvailable(db, network) {
			return network
		}
	}
	return "10.0.0.1/24" // fallback
}

// GetNextAvailablePort возвращает следующий доступный порт
func GetNextAvailablePort(db *Database) int {
	// Начинаем с 50000
	for port := 50000; port < 65535; port++ {
		if IsPortAvailableForServer(db, port) {
			return port
		}
	}
	return 50000 // fallback
}

// IsNetworkAvailable проверяет свободна ли подсеть
func IsNetworkAvailable(db *Database, network string) bool {
	// Извлекаем второй октет для сравнения
	// 10.0.0.1/24 -> 10
	// 11.0.0.1/24 -> 11
	parts := strings.Split(network, ".")
	if len(parts) < 2 {
		return false
	}

	networkPrefix := parts[0] + "." + parts[1]

	for _, server := range db.Servers {
		serverParts := strings.Split(server.Address, ".")
		if len(serverParts) < 2 {
			continue
		}
		serverPrefix := serverParts[0] + "." + serverParts[1]

		if networkPrefix == serverPrefix {
			return false
		}
	}

	return true
}

// IsPortAvailableForServer проверяет свободен ли порт для сервера
func IsPortAvailableForServer(db *Database, port int) bool {
	for _, server := range db.Servers {
		if server.ListenPort == port {
			return false
		}
	}
	return true
}

// ValidateServerConfig проверяет корректность конфигурации сервера
func ValidateServerConfig(db *Database, address string, port int) error {
	// Проверяем формат подсети
	if !strings.HasSuffix(address, "/24") {
		return fmt.Errorf("подсеть должна быть /24")
	}

	// Проверяем что подсеть свободна
	if !IsNetworkAvailable(db, address) {
		return fmt.Errorf("подсеть уже используется")
	}

	// Проверяем что порт свободен
	if !IsPortAvailableForServer(db, port) {
		return fmt.Errorf("порт %d уже используется", port)
	}

	return nil
}
