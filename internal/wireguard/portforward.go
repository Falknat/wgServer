package wireguard

import (
	"fmt"
	"log"
	"os/exec"

	"wg-panel/internal/database"
)

// isPortAvailable проверяет доступен ли порт
func isPortAvailable(db *database.Database, port int, protocol string) bool {
	protocols := []string{protocol}
	if protocol == "both" {
		protocols = []string{"tcp", "udp", "both"}
	}

	for _, client := range db.Clients {
		for _, pf := range client.PortForwards {
			if pf.Port == port {
				// Проверяем конфликт протоколов
				if pf.Protocol == "both" || protocol == "both" {
					return false
				}
				for _, p := range protocols {
					if pf.Protocol == p {
						return false
					}
				}
			}
		}
	}
	return true
}

// AddPortForward добавляет проброс порта для клиента
func AddPortForward(db *database.Database, client *database.Client, port int, protocol, description string) error {
	// Проверяем что порт свободен
	if !isPortAvailable(db, port, protocol) {
		return fmt.Errorf("порт %d/%s уже используется", port, protocol)
	}

	// Добавляем в список
	portForward := database.PortForward{
		Port:        port,
		Protocol:    protocol,
		Description: description,
	}
	client.PortForwards = append(client.PortForwards, portForward)

	// Применяем правила iptables если клиент активен
	if client.Enabled {
		if err := applyPortForwardRules(client, portForward); err != nil {
			return err
		}
	}

	return nil
}

// updatePortForward обновляет проброс порта
func updatePortForward(client *database.Client, port int, protocol, newDescription string) error {
	for i, pf := range client.PortForwards {
		if pf.Port == port && pf.Protocol == protocol {
			client.PortForwards[i].Description = newDescription
			return nil
		}
	}
	return fmt.Errorf("проброс порта не найден")
}

// RemovePortForward удаляет проброс порта
func RemovePortForward(client *database.Client, port int, protocol string) error {
	// Находим и удаляем проброс
	for i, pf := range client.PortForwards {
		if pf.Port == port && pf.Protocol == protocol {
			// Удаляем правила iptables
			if client.Enabled {
				removePortForwardRules(client, pf)
			}

			// Удаляем из списка
			client.PortForwards = append(client.PortForwards[:i], client.PortForwards[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("проброс порта не найден")
}

// applyPortForwardRules применяет правила iptables для проброса порта
func applyPortForwardRules(client *database.Client, pf database.PortForward) error {
	log.Printf("    🔀 Применяю проброс порта %d (%s)", pf.Port, pf.Protocol)

	netInterface := database.GetDefaultInterface()

	protocols := []string{pf.Protocol}
	if pf.Protocol == "both" {
		protocols = []string{"tcp", "udp"}
	}

	for _, proto := range protocols {
		commands := [][]string{
			// DNAT только для пакетов приходящих с внешнего интерфейса
			{"iptables", "-t", "nat", "-A", "PREROUTING", "-i", netInterface, "-p", proto,
				"--dport", fmt.Sprintf("%d", pf.Port), "-j", "DNAT",
				"--to-destination", fmt.Sprintf("%s:%d", client.Address, pf.Port)},

			// Разрешаем FORWARD для этого порта
			{"iptables", "-I", "FORWARD", "1", "-p", proto, "-d", client.Address,
				"--dport", fmt.Sprintf("%d", pf.Port), "-j", "ACCEPT"},
		}

		for _, cmdArgs := range commands {
			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("    ⚠️  Ошибка: %v (output: %s)", err, string(output))
				return err
			}
		}
	}

	log.Printf("    ✅ Проброс порта настроен")
	return nil
}

// removePortForwardRules удаляет правила iptables для проброса порта
func removePortForwardRules(client *database.Client, pf database.PortForward) error {
	netInterface := database.GetDefaultInterface()

	protocols := []string{pf.Protocol}
	if pf.Protocol == "both" {
		protocols = []string{"tcp", "udp"}
	}

	for _, proto := range protocols {
		commands := [][]string{
			{"iptables", "-t", "nat", "-D", "PREROUTING", "-i", netInterface, "-p", proto,
				"--dport", fmt.Sprintf("%d", pf.Port), "-j", "DNAT",
				"--to-destination", fmt.Sprintf("%s:%d", client.Address, pf.Port)},

			{"iptables", "-D", "FORWARD", "-p", proto, "-d", client.Address,
				"--dport", fmt.Sprintf("%d", pf.Port), "-j", "ACCEPT"},
		}

		for _, cmdArgs := range commands {
			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			cmd.Run() // Игнорируем ошибки при удалении
		}
	}

	return nil
}

// ApplyAllPortForwards применяет все пробросы портов для клиента
func ApplyAllPortForwards(client *database.Client) error {
	if !client.Enabled {
		return nil
	}

	for _, pf := range client.PortForwards {
		if err := applyPortForwardRules(client, pf); err != nil {
			log.Printf("    ⚠️  Ошибка проброса порта %d: %v", pf.Port, err)
		}
	}

	return nil
}

// removeAllPortForwards удаляет все пробросы портов для клиента
func removeAllPortForwards(client *database.Client) error {
	for _, pf := range client.PortForwards {
		removePortForwardRules(client, pf)
	}
	return nil
}
