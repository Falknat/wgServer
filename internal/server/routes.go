package server

import (
	"net/http"
)

// SetupRoutes настраивает маршруты HTTP сервера
func SetupRoutes() {
	// Главная страница
	http.HandleFunc("/", authMiddleware(HandleIndex))

	// API для серверов
	http.HandleFunc("/api/servers", authMiddleware(HandleServers))
	http.HandleFunc("/api/server/create", authMiddleware(HandleCreateServer))
	http.HandleFunc("/api/server/update", authMiddleware(HandleUpdateServer))
	http.HandleFunc("/api/server/delete", authMiddleware(HandleDeleteServer))
	http.HandleFunc("/api/server/toggle", authMiddleware(HandleToggleServer))

	// API для клиентов
	http.HandleFunc("/api/clients", authMiddleware(HandleClients))
	http.HandleFunc("/api/client/create", authMiddleware(HandleCreateClient))
	http.HandleFunc("/api/client/delete", authMiddleware(HandleDeleteClient))
	http.HandleFunc("/api/client/toggle", authMiddleware(HandleToggleClient))
	http.HandleFunc("/api/client/update", authMiddleware(HandleUpdateClient))
	http.HandleFunc("/api/client/download", authMiddleware(HandleDownloadConfig))
	http.HandleFunc("/api/client/qr", authMiddleware(HandleQRCode))
	http.HandleFunc("/api/client/portforward/add", authMiddleware(HandleAddPortForward))
	http.HandleFunc("/api/client/portforward/remove", authMiddleware(HandleRemovePortForward))
	http.HandleFunc("/api/stats", authMiddleware(HandleStats))

	// Авторизация
	http.HandleFunc("/login", HandleLogin)
	http.HandleFunc("/logout", HandleLogout)
}
