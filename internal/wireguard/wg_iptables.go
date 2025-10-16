package wireguard

import (
	"log"
	"os/exec"

	"wg-panel/internal/database"
)

// ApplyWireGuardIPTablesRules применяет правила iptables для WireGuard сервера
func ApplyWireGuardIPTablesRules(server *database.Server) error {
	netInterface := database.GetDefaultInterface()
	iface := server.Interface
	network := getNetworkFromAddress(server.Address)

	log.Printf("    🔧 Применяю правила iptables для %s (сеть: %s, интерфейс: %s)...", iface, network, netInterface)

	// Правила FORWARD
	commands := [][]string{
		{"iptables", "-I", "FORWARD", "1", "-i", iface, "-j", "ACCEPT"},
		{"iptables", "-I", "FORWARD", "1", "-o", iface, "-j", "ACCEPT"},
		{"iptables", "-t", "nat", "-A", "POSTROUTING", "-s", network, "-o", netInterface, "-j", "MASQUERADE"},
	}

	for i, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("    ⚠️  Команда #%d %v: %v (output: %s)", i+1, cmdArgs, err, string(output))
		} else {
			log.Printf("    ✅ Команда #%d выполнена: %v", i+1, cmdArgs)
		}
	}

	log.Printf("    ✅ Правила iptables применены")
	return nil
}

// RemoveWireGuardIPTablesRules удаляет правила iptables для WireGuard сервера
func RemoveWireGuardIPTablesRules(server *database.Server) error {
	netInterface := database.GetDefaultInterface()
	iface := server.Interface

	log.Printf("    🗑️  Удаляю правила iptables для %s...", iface)

	commands := [][]string{
		{"iptables", "-D", "FORWARD", "-i", iface, "-j", "ACCEPT"},
		{"iptables", "-D", "FORWARD", "-o", iface, "-j", "ACCEPT"},
		{"iptables", "-t", "nat", "-D", "POSTROUTING", "-s", getNetworkFromAddress(server.Address), "-o", netInterface, "-j", "MASQUERADE"},
	}

	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Run() // Игнорируем ошибки
	}

	return nil
}

// getNetworkFromAddress извлекает подсеть из адреса типа "10.0.0.1/24" -> "10.0.0.0/24"
func getNetworkFromAddress(address string) string {
	// Простая реализация - заменяем последний октет на 0
	parts := address[:len(address)-1] // убираем последнюю цифру
	// Находим последнюю точку
	lastDot := -1
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == '.' {
			lastDot = i
			break
		}
	}
	if lastDot > 0 {
		return parts[:lastDot+1] + "0" + address[len(address)-3:] // +0 и маска
	}
	return address
}
