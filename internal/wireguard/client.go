package wireguard

import (
	"fmt"
	"log"
	"os/exec"
	"time"

	"wg-panel/internal/database"

	"github.com/skip2/go-qrcode"
)

// CreateClient создает нового клиента
func CreateClient(db *database.Database, serverID, name, comment string) (*database.Client, error) {
	// Находим сервер
	var server *database.Server
	for i := range db.Servers {
		if db.Servers[i].ID == serverID {
			server = &db.Servers[i]
			break
		}
	}

	if server == nil {
		return nil, fmt.Errorf("server not found")
	}

	// Генерируем ключи
	privateKey, publicKey, err := database.GenerateKeys()
	if err != nil {
		return nil, err
	}

	// Создаем клиента
	client := database.Client{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		ServerID:   serverID,
		Name:       name,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		Address:    database.GetNextClientIP(server),
		Enabled:    true,
		Comment:    comment,
		CreatedAt:  time.Now(),
	}

	// Добавляем peer в WireGuard если сервер запущен
	if server.Enabled {
		log.Printf("➕ Добавляю peer %s в WireGuard...", client.Name)
		if err := addPeerToWireGuard(server, client); err != nil {
			log.Printf("⚠️  Ошибка добавления peer: %v", err)
		} else {
			log.Printf("✅ Peer добавлен")
		}
	}

	// Обновляем конфиг файл
	UpdateServerConfig(server, db)

	return &client, nil
}

// DeleteClient удаляет клиента
func DeleteClient(db *database.Database, client *database.Client) error {
	// Находим сервер
	var server *database.Server
	for j := range db.Servers {
		if db.Servers[j].ID == client.ServerID {
			server = &db.Servers[j]
			break
		}
	}

	// Удаляем peer из WireGuard
	if server != nil && server.Enabled {
		removePeerFromWireGuard(server, *client)
	}

	// Обновляем конфиг файл
	if server != nil {
		UpdateServerConfig(server, db)
	}

	return nil
}

// ToggleClient включает/выключает клиента
func ToggleClient(db *database.Database, client *database.Client) error {
	client.Enabled = !client.Enabled

	// Находим сервер
	var server *database.Server
	for j := range db.Servers {
		if db.Servers[j].ID == client.ServerID {
			server = &db.Servers[j]
			break
		}
	}

	if server != nil && server.Enabled {
		if client.Enabled {
			addPeerToWireGuard(server, *client)
		} else {
			removePeerFromWireGuard(server, *client)
		}
	}

	// Обновляем конфиг файл
	if server != nil {
		UpdateServerConfig(server, db)
	}

	return nil
}

// GenerateClientConfig генерирует конфиг для клиента
func GenerateClientConfig(client database.Client, server *database.Server) string {
	// Получаем endpoint сервера
	endpoint := database.GetServerEndpoint()

	config := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/32
DNS = %s

[Peer]
PublicKey = %s
Endpoint = %s:%d
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 10
`, client.PrivateKey, client.Address, server.DNS, server.PublicKey, endpoint, server.ListenPort)

	return config
}

// GenerateQRCode генерирует QR код для конфига
func GenerateQRCode(config string) ([]byte, error) {
	return qrcode.Encode(config, qrcode.Medium, 256)
}

// addPeerToWireGuard добавляет peer в WireGuard
func addPeerToWireGuard(server *database.Server, client database.Client) error {
	cmd := exec.Command("wg", "set", server.Interface, "peer", client.PublicKey,
		"allowed-ips", client.Address+"/32")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Ошибка добавления peer: %s, %v", string(output), err)
		return err
	}
	return nil
}

// removePeerFromWireGuard удаляет peer из WireGuard
func removePeerFromWireGuard(server *database.Server, client database.Client) error {
	cmd := exec.Command("wg", "set", server.Interface, "peer", client.PublicKey, "remove")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Ошибка удаления peer: %s, %v", string(output), err)
		return err
	}
	return nil
}
