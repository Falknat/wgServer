package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"wg-panel/internal/database"
	"wg-panel/internal/wireguard"
)

var (
	DB     *database.Database
	Config *database.Config
)

// authMiddleware проверяет авторизацию
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Проверяем cookie
		cookie, err := r.Cookie("auth")
		if err != nil || cookie.Value != "authenticated" {
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// handleLogin обрабатывает страницу входа
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tmplData, err := TemplatesFS.ReadFile("templates/login.html")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl, err := template.New("login").Parse(string(tmplData))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
		return
	}

	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")

		if username == Config.Username && password == Config.Password {
			http.SetCookie(w, &http.Cookie{
				Name:     "auth",
				Value:    "authenticated",
				Path:     "/",
				MaxAge:   86400, // 24 часа
				HttpOnly: true,
			})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/login?error=1", http.StatusSeeOther)
	}
}

// handleLogout обрабатывает выход
func HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// handleIndex обрабатывает главную страницу
func HandleIndex(w http.ResponseWriter, r *http.Request) {
	tmplData, err := TemplatesFS.ReadFile("templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl, err := template.New("index").Parse(string(tmplData))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// === ОБРАБОТЧИКИ СЕРВЕРОВ ===

// HandleServers возвращает список серверов
func HandleServers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if DB.Servers == nil {
		json.NewEncoder(w).Encode([]database.Server{})
	} else {
		json.NewEncoder(w).Encode(DB.Servers)
	}
}

// HandleCreateServer создает новый WireGuard сервер
func HandleCreateServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	address := r.FormValue("address")
	portStr := r.FormValue("port")
	dns := r.FormValue("dns")

	if name == "" || address == "" || portStr == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	// Добавляем /24 если не указано
	if !strings.Contains(address, "/") {
		address = address + "/24"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		http.Error(w, "Invalid port", http.StatusBadRequest)
		return
	}

	// Валидируем конфигурацию
	if err := database.ValidateServerConfig(DB, address, port); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	server, err := wireguard.CreateServer(DB, name, address, port, dns)
	if err != nil {
		http.Error(w, "Failed to create server: "+err.Error(), http.StatusInternalServerError)
		return
	}

	DB.Servers = append(DB.Servers, *server)
	database.SaveDatabase(DB)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(server)
}

// HandleUpdateServer обновляет настройки сервера
func HandleUpdateServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.FormValue("id")
	name := r.FormValue("name")
	portStr := r.FormValue("port")
	dns := r.FormValue("dns")

	for i, server := range DB.Servers {
		if server.ID == id {
			if name != "" {
				DB.Servers[i].Name = name
			}
			if portStr != "" {
				port, err := strconv.Atoi(portStr)
				if err == nil {
					DB.Servers[i].ListenPort = port
				}
			}
			if dns != "" {
				DB.Servers[i].DNS = dns
			}

			// Обновляем конфиг файл
			wireguard.UpdateServerConfig(&DB.Servers[i], DB)

			database.SaveDatabase(DB)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DB.Servers[i])
			return
		}
	}

	http.Error(w, "Server not found", http.StatusNotFound)
}

// HandleDeleteServer удаляет сервер
func HandleDeleteServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.FormValue("id")

	for i, server := range DB.Servers {
		if server.ID == id {
			// Удаляем сервер
			if err := wireguard.DeleteServer(&server); err != nil {
				http.Error(w, "Failed to delete server", http.StatusInternalServerError)
				return
			}

			// Удаляем всех клиентов этого сервера
			var newClients []database.Client
			for _, client := range DB.Clients {
				if client.ServerID != id {
					newClients = append(newClients, client)
				}
			}
			DB.Clients = newClients

			// Удаляем сервер из базы
			DB.Servers = append(DB.Servers[:i], DB.Servers[i+1:]...)
			database.SaveDatabase(DB)

			w.WriteHeader(http.StatusOK)
			return
		}
	}

	http.Error(w, "Server not found", http.StatusNotFound)
}

// HandleToggleServer включает/выключает сервер
func HandleToggleServer(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.FormValue("id")

	for i, server := range DB.Servers {
		if server.ID == id {
			if err := wireguard.ToggleServer(&DB.Servers[i]); err != nil {
				http.Error(w, "Failed to toggle server: "+err.Error(), http.StatusInternalServerError)
				return
			}

			database.SaveDatabase(DB)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DB.Servers[i])
			return
		}
	}

	http.Error(w, "Server not found", http.StatusNotFound)
}

// === ОБРАБОТЧИКИ КЛИЕНТОВ ===

// HandleClients возвращает список клиентов
func HandleClients(w http.ResponseWriter, r *http.Request) {
	serverID := r.URL.Query().Get("server_id")

	var clients []database.Client
	for _, client := range DB.Clients {
		if serverID == "" || client.ServerID == serverID {
			clients = append(clients, client)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if clients == nil {
		json.NewEncoder(w).Encode([]database.Client{})
	} else {
		json.NewEncoder(w).Encode(clients)
	}
}

// HandleCreateClient создает новый конфиг клиента
func HandleCreateClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serverID := r.FormValue("server_id")
	name := r.FormValue("name")
	comment := r.FormValue("comment")

	if serverID == "" || name == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	client, err := wireguard.CreateClient(DB, serverID, name, comment)
	if err != nil {
		http.Error(w, "Failed to create client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	DB.Clients = append(DB.Clients, *client)
	database.SaveDatabase(DB)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(client)
}

// HandleDeleteClient удаляет клиента
func HandleDeleteClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.FormValue("id")

	for i, client := range DB.Clients {
		if client.ID == id {
			// Удаляем клиента
			if err := wireguard.DeleteClient(DB, &client); err != nil {
				http.Error(w, "Failed to delete client", http.StatusInternalServerError)
				return
			}

			// Удаляем из базы
			DB.Clients = append(DB.Clients[:i], DB.Clients[i+1:]...)
			database.SaveDatabase(DB)

			w.WriteHeader(http.StatusOK)
			return
		}
	}

	http.Error(w, "Client not found", http.StatusNotFound)
}

// HandleToggleClient включает/выключает клиента
func HandleToggleClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.FormValue("id")

	for i, client := range DB.Clients {
		if client.ID == id {
			if err := wireguard.ToggleClient(DB, &DB.Clients[i]); err != nil {
				http.Error(w, "Failed to toggle client", http.StatusInternalServerError)
				return
			}

			database.SaveDatabase(DB)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DB.Clients[i])
			return
		}
	}

	http.Error(w, "Client not found", http.StatusNotFound)
}

// HandleUpdateClient обновляет имя и комментарий клиента
func HandleUpdateClient(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.FormValue("id")
	name := r.FormValue("name")
	comment := r.FormValue("comment")

	for i, client := range DB.Clients {
		if client.ID == id {
			if name != "" {
				DB.Clients[i].Name = name
			}
			DB.Clients[i].Comment = comment
			database.SaveDatabase(DB)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DB.Clients[i])
			return
		}
	}

	http.Error(w, "Client not found", http.StatusNotFound)
}

// HandleAddPortForward добавляет проброс порта
func HandleAddPortForward(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientID := r.FormValue("client_id")
	portStr := r.FormValue("port")
	protocol := r.FormValue("protocol")
	description := r.FormValue("description")

	port, err := strconv.Atoi(portStr)
	if err != nil {
		http.Error(w, "Invalid port", http.StatusBadRequest)
		return
	}

	if protocol != "tcp" && protocol != "udp" && protocol != "both" {
		http.Error(w, "Protocol must be tcp, udp or both", http.StatusBadRequest)
		return
	}

	for i, client := range DB.Clients {
		if client.ID == clientID {
			if err := wireguard.AddPortForward(DB, &DB.Clients[i], port, protocol, description); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			database.SaveDatabase(DB)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DB.Clients[i])
			return
		}
	}

	http.Error(w, "Client not found", http.StatusNotFound)
}

// HandleRemovePortForward удаляет проброс порта
func HandleRemovePortForward(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	clientID := r.FormValue("client_id")
	portStr := r.FormValue("port")
	protocol := r.FormValue("protocol")

	port, err := strconv.Atoi(portStr)
	if err != nil {
		http.Error(w, "Invalid port", http.StatusBadRequest)
		return
	}

	for i, client := range DB.Clients {
		if client.ID == clientID {
			if err := wireguard.RemovePortForward(&DB.Clients[i], port, protocol); err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}

			database.SaveDatabase(DB)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(DB.Clients[i])
			return
		}
	}

	http.Error(w, "Client not found", http.StatusNotFound)
}

// HandleDownloadConfig скачивает конфиг клиента
func HandleDownloadConfig(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

	for _, client := range DB.Clients {
		if client.ID == id {
			// Находим сервер
			var server *database.Server
			for i := range DB.Servers {
				if DB.Servers[i].ID == client.ServerID {
					server = &DB.Servers[i]
					break
				}
			}

			if server == nil {
				http.Error(w, "Server not found", http.StatusNotFound)
				return
			}

			config := wireguard.GenerateClientConfig(client, server)

			// Создаем безопасное имя файла (без пробелов и спецсимволов)
			safeName := database.SanitizeFilename(client.Name)

			w.Header().Set("Content-Type", "application/x-wireguard-profile")
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.conf", safeName))
			w.Write([]byte(config))
			return
		}
	}

	http.Error(w, "Client not found", http.StatusNotFound)
}

// HandleQRCode генерирует QR код для конфига
func HandleQRCode(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

	for _, client := range DB.Clients {
		if client.ID == id {
			// Находим сервер
			var server *database.Server
			for i := range DB.Servers {
				if DB.Servers[i].ID == client.ServerID {
					server = &DB.Servers[i]
					break
				}
			}

			if server == nil {
				http.Error(w, "Server not found", http.StatusNotFound)
				return
			}

			config := wireguard.GenerateClientConfig(client, server)

			// Генерируем QR код
			png, err := wireguard.GenerateQRCode(config)
			if err != nil {
				http.Error(w, "Failed to generate QR code", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "image/png")
			w.Write(png)
			return
		}
	}

	http.Error(w, "Client not found", http.StatusNotFound)
}

// HandleStats возвращает статистику
func HandleStats(w http.ResponseWriter, r *http.Request) {
	wireguard.UpdateStats(DB)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DB.Clients)
}
