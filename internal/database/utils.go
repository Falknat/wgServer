package database

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strings"
)

// CheckWireGuardInstalled проверяет установлен ли WireGuard
func CheckWireGuardInstalled() bool {
	cmd := exec.Command("which", "wg")
	err := cmd.Run()
	return err == nil
}

// GenerateKeys генерирует пару ключей WireGuard
func GenerateKeys() (privateKey, publicKey string, err error) {
	// Генерируем приватный ключ
	cmd := exec.Command("wg", "genkey")
	privateKeyBytes, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	privateKey = strings.TrimSpace(string(privateKeyBytes))

	// Генерируем публичный ключ из приватного
	cmd = exec.Command("wg", "pubkey")
	cmd.Stdin = strings.NewReader(privateKey)
	publicKeyBytes, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	publicKey = strings.TrimSpace(string(publicKeyBytes))

	return privateKey, publicKey, nil
}

// GetNextClientIP возвращает следующий доступный IP адрес для клиента
func GetNextClientIP(server *Server) string {
	// Парсим адрес сервера (например 10.0.0.1/24)
	parts := strings.Split(server.Address, "/")
	if len(parts) != 2 {
		return ""
	}

	ipParts := strings.Split(parts[0], ".")
	if len(ipParts) != 4 {
		return ""
	}

	// Формируем IP клиента
	ip := fmt.Sprintf("%s.%s.%s.%d", ipParts[0], ipParts[1], ipParts[2], server.NextClientIP)
	server.NextClientIP++
	return ip
}

// GetServerEndpoint получает внешний IP адрес сервера
func GetServerEndpoint() string {
	// Пробуем получить внешний IP
	resp, err := http.Get("https://api.ipify.org")
	if err == nil {
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			return string(body)
		}
	}

	// Если не получилось, возвращаем локальный IP
	return GetLocalIP()
}

// GetLocalIP получает локальный IP адрес
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return "127.0.0.1"
}

// GetDefaultInterface получает основной сетевой интерфейс для интернета
func GetDefaultInterface() string {
	// Пытаемся определить через маршруты
	cmd := exec.Command("sh", "-c", "ip route | grep default | awk '{print $5}' | head -n1")
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		iface := strings.TrimSpace(string(output))
		if iface != "" {
			return iface
		}
	}

	// Пробуем альтернативный способ
	cmd = exec.Command("sh", "-c", "ip -4 route ls | grep default | grep -Po '(?<=dev )\\S+' | head -n1")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		iface := strings.TrimSpace(string(output))
		if iface != "" {
			return iface
		}
	}

	// Возвращаем eth0 по умолчанию
	log.Println("⚠️  Не удалось определить сетевой интерфейс, использую eth0")
	return "eth0"
}

// EnableIPForwarding включает IP forwarding в системе
func EnableIPForwarding() error {
	// Временно включаем
	cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sysctl failed: %v, output: %s", err, string(output))
	}

	// Проверяем что включилось
	cmd = exec.Command("cat", "/proc/sys/net/ipv4/ip_forward")
	output, err = cmd.Output()
	if err == nil {
		value := strings.TrimSpace(string(output))
		if value != "1" {
			return fmt.Errorf("IP forwarding не включился, значение: %s", value)
		}
	}

	// Делаем постоянным
	cmd = exec.Command("sh", "-c", "grep -q 'net.ipv4.ip_forward' /etc/sysctl.conf || echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf")
	return cmd.Run()
}
