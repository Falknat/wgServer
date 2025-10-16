package database

import "time"

// Config структура для конфигурации приложения
type Config struct {
	Port     string `json:"port"`
	Address  string `json:"address"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Server структура для WireGuard сервера
type Server struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Interface    string    `json:"interface"`
	PrivateKey   string    `json:"private_key"`
	PublicKey    string    `json:"public_key"`
	Address      string    `json:"address"`
	ListenPort   int       `json:"listen_port"`
	DNS          string    `json:"dns"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	PostUp       string    `json:"post_up"`
	PostDown     string    `json:"post_down"`
	NextClientIP int       `json:"next_client_ip"`
}

// PortForward структура для проброса порта
type PortForward struct {
	Port        int    `json:"port"`        // Порт (одинаковый внешний и внутренний)
	Protocol    string `json:"protocol"`    // tcp, udp или both
	Description string `json:"description"` // Описание
}

// Client структура для клиента WireGuard
type Client struct {
	ID            string        `json:"id"`
	ServerID      string        `json:"server_id"`
	Name          string        `json:"name"`
	PublicKey     string        `json:"public_key"`
	PrivateKey    string        `json:"private_key"`
	Address       string        `json:"address"`
	Enabled       bool          `json:"enabled"`
	Comment       string        `json:"comment"`
	CreatedAt     time.Time     `json:"created_at"`
	RxBytes       int64         `json:"rx_bytes"`
	TxBytes       int64         `json:"tx_bytes"`
	LastHandshake time.Time     `json:"last_handshake"`
	Endpoint      string        `json:"endpoint"` // IP:Port клиента
	PortForwards  []PortForward `json:"port_forwards"`
}

// Database структура для хранения данных
type Database struct {
	Servers []Server `json:"servers"`
	Clients []Client `json:"clients"`
}
