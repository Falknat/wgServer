package wireguard

import (
	"log"
	"os/exec"
)

// CleanIPTables очищает все правила iptables
func CleanIPTables() error {
	log.Println("🧹 Очистка iptables...")

	commands := [][]string{
		// Устанавливаем политики по умолчанию в ACCEPT
		{"iptables", "-P", "INPUT", "ACCEPT"},
		{"iptables", "-P", "FORWARD", "ACCEPT"},
		{"iptables", "-P", "OUTPUT", "ACCEPT"},

		// Очищаем все цепочки
		{"iptables", "-t", "nat", "-F"},
		{"iptables", "-t", "mangle", "-F"},
		{"iptables", "-t", "filter", "-F"},
		{"iptables", "-t", "raw", "-F"},

		// Удаляем пользовательские цепочки
		{"iptables", "-t", "nat", "-X"},
		{"iptables", "-t", "mangle", "-X"},
		{"iptables", "-t", "filter", "-X"},
		{"iptables", "-t", "raw", "-X"},
	}

	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("  ⚠️  Команда %v: %v (output: %s)", cmdArgs, err, string(output))
			// Продолжаем даже при ошибках
		}
	}

	log.Println("  ✅ iptables очищен")
	return nil
}

// SetupBasicIPTables настраивает базовые правила iptables
func SetupBasicIPTables() error {
	log.Println("🔧 Настройка базовых правил iptables...")

	commands := [][]string{
		// Разрешаем loopback
		{"iptables", "-A", "INPUT", "-i", "lo", "-j", "ACCEPT"},
		{"iptables", "-A", "OUTPUT", "-o", "lo", "-j", "ACCEPT"},

		// Разрешаем established и related соединения
		{"iptables", "-A", "INPUT", "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
		{"iptables", "-A", "OUTPUT", "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
		{"iptables", "-A", "FORWARD", "-m", "state", "--state", "ESTABLISHED,RELATED", "-j", "ACCEPT"},

		// Разрешаем SSH (чтобы не потерять доступ)
		{"iptables", "-A", "INPUT", "-p", "tcp", "--dport", "22", "-j", "ACCEPT"},
	}

	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("  ⚠️  Команда %v: %v (output: %s)", cmdArgs, err, string(output))
		}
	}

	log.Println("  ✅ Базовые правила настроены")
	return nil
}
