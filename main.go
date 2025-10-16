package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"wg-panel/internal/database"
	"wg-panel/internal/server"
	"wg-panel/internal/wireguard"
)

const version = "1.0.0"

func main() {
	// Обрабатываем аргументы командной строки
	if len(os.Args) > 1 {
		command := os.Args[1]

		// Команды доступные всегда
		switch command {
		case "install":
			installServer()
			return
		case "version", "-v", "--version":
			fmt.Printf("wg_serf version %s\n", version)
			os.Exit(0)
		case "help", "-h", "--help":
			showHelp()
			return
		case "serve":
			// Прямой запуск сервера (используется systemd)
			runServer()
			return
		}

		// Для остальных команд проверяем:
		// 1. Установлена ли служба
		// 2. Запущено через PATH (не ./wg_serf)

		if !isInstalled() {
			fmt.Println("")
			fmt.Println("❌ WG_SERF не установлен!")
			fmt.Println("")
			fmt.Println("Сначала установите:")
			fmt.Println("   sudo wg_serf install")
			fmt.Println("")
			fmt.Println("Или используйте install.sh:")
			fmt.Println("   curl -fsSL https://vserf.ru/download/wgserf/install.sh | sudo bash")
			fmt.Println("")
			os.Exit(1)
		}

		// Проверяем что команда запущена через PATH, а не напрямую
		exePath, _ := os.Executable()
		if strings.HasPrefix(exePath, "./") || strings.HasPrefix(exePath, "/root/") || strings.HasPrefix(exePath, "/home/") {
			// Запущено не через PATH
			if exePath != "/opt/wg_serf/wg_serf" {
				fmt.Println("")
				fmt.Println("⚠️  Команды управления работают только через PATH")
				fmt.Println("")
				fmt.Println("Используйте:")
				fmt.Printf("   wg_serf %s\n", command)
				fmt.Println("")
				fmt.Println("А не:")
				fmt.Printf("   %s %s\n", exePath, command)
				fmt.Println("")
				os.Exit(1)
			}
		}

		// Команды доступные только после установки
		switch command {
		case "start":
			startServer()
		case "stop":
			stopServer()
		case "restart":
			restartServer()
		case "status":
			showStatus()
		case "uninstall", "delete":
			deleteServer()
		default:
			fmt.Printf("Неизвестная команда: %s\n\n", command)
			showHelp()
			os.Exit(1)
		}
		return
	}

	// Без аргументов - интерактивный режим
	handleNoArgs()
}

func handleNoArgs() {
	// Проверяем запущен ли через PATH или напрямую
	exePath, _ := os.Executable()

	// Если не установлено - предлагаем установить
	if !isInstalled() {
		fmt.Println("")
		fmt.Println("╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("║              WG_SERF - WireGuard Server Panel                ║")
		fmt.Println("║                      Version " + version + "                           ║")
		fmt.Println("╚══════════════════════════════════════════════════════════════╝")
		fmt.Println("")
		fmt.Println("❌ WG_SERF не установлен")
		fmt.Println("")
		if askYesNo("📥 Установить в систему? (yes/no): ") {
			installServer()
		} else {
			fmt.Println("")
			fmt.Println("Установка отменена. Установить позже:")
			fmt.Println("   sudo " + exePath + " install")
			fmt.Println("")
		}
		return
	}

	// Если установлено - показываем информацию
	config, _ := database.LoadConfig()
	pid, _ := readPIDFile()
	running := isRunning()

	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              WG_SERF - WireGuard Server Panel                ║")
	if running {
		// Динамическое форматирование для ровной рамки
		line := fmt.Sprintf("Version %s  PID: %d", version, pid)
		padding := 62 - len(line)
		leftPad := padding / 2
		rightPad := padding - leftPad
		fmt.Printf("║%s%s%s║\n", strings.Repeat(" ", leftPad), line, strings.Repeat(" ", rightPad))
	} else {
		line := fmt.Sprintf("Version %s", version)
		padding := 62 - len(line)
		leftPad := padding / 2
		rightPad := padding - leftPad
		fmt.Printf("║%s%s%s║\n", strings.Repeat(" ", leftPad), line, strings.Repeat(" ", rightPad))
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println("")

	if running {
		fmt.Println("✅ Сервис установлен и работает")
		if config != nil {
			serverIP := database.GetLocalIP()
			if serverIP == "127.0.0.1" {
				serverIP = database.GetServerEndpoint()
			}
			fmt.Printf("🌐 Веб-панель: http://%s:%s\n", serverIP, config.Port)
			fmt.Printf("👤 Логин: %s\n", config.Username)
			fmt.Printf("🔒 Пароль: %s\n", config.Password)
		}
	} else {
		fmt.Println("⚠️  Сервис установлен, но не запущен")
		fmt.Println("")
		fmt.Println("Запустить: wg_serf start")
	}

	fmt.Println("")
	fmt.Println("📋 Доступные команды:")
	fmt.Println("   wg_serf status    # Статус")
	fmt.Println("   wg_serf restart   # Перезапустить")
	fmt.Println("   wg_serf stop      # Остановить")
	fmt.Println("   wg_serf delete    # Удалить")
	fmt.Println("")
	fmt.Println("📚 Справка: wg_serf help")
	fmt.Println("")
}

func isInstalled() bool {
	_, err := os.Stat("/etc/systemd/system/wg_serf.service")
	return err == nil
}

// askYesNo запрашивает подтверждение yes/no (толерантно к вводу)
func askYesNo(prompt string) bool {
	fmt.Print(prompt)
	var response string
	fmt.Scanln(&response)

	// Убираем пробелы и переводим в нижний регистр
	response = strings.ToLower(strings.TrimSpace(response))

	// Проверяем содержит ли yes
	if strings.Contains(response, "yes") || strings.Contains(response, "y") {
		return true
	}

	return false
}

func showHelp() {
	fmt.Println(`
╔══════════════════════════════════════════════════════════════╗
║              WG_SERF - WireGuard Server Panel                ║
║                      Version ` + version + `                            ║
╚══════════════════════════════════════════════════════════════╝

📖 ИСПОЛЬЗОВАНИЕ:
   wg_serf <команда>

📋 КОМАНДЫ:
   install    Установить wg_serf как службу (требуется сначала!)
   version    Показать версию
   help       Показать эту справку

📋 КОМАНДЫ ПОСЛЕ УСТАНОВКИ:
   start      Запустить сервер
   stop       Остановить сервер
   restart    Перезапустить сервер
   status     Показать статус сервера
   delete     Удалить wg_serf полностью

🔧 ПРИМЕРЫ:
   sudo wg_serf install    # Сначала установить
   wg_serf status          # Проверить статус
   wg_serf restart         # Перезапустить

📡 ВЕБ-ИНТЕРФЕЙС:
   После запуска откройте в браузере: http://your-server-ip:8080
   Логин по умолчанию: admin / admin

💡 Совет: Для работы требуются root права (sudo)`)
}

func startServer() {
	// Проверяем не запущен ли уже
	if isRunning() {
		fmt.Println("❌ Сервер уже запущен!")
		os.Exit(1)
	}

	fmt.Println("🚀 Запуск WG_SERF...")

	// Пробуем запустить через systemctl (если установлена служба)
	cmd := exec.Command("systemctl", "start", "wg_serf")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Если systemd не доступен или служба не установлена - запускаем напрямую
		if strings.Contains(string(output), "Failed to connect") ||
			strings.Contains(string(output), "not found") ||
			strings.Contains(err.Error(), "executable file not found") {
			fmt.Println("⚠️  Служба не найдена, запускаю напрямую...")
			runServer()
			return
		}
		fmt.Println("❌ Ошибка запуска:", err)
		fmt.Println(string(output))
		os.Exit(1)
	}

	fmt.Println("✅ Сервер запущен как служба")
	fmt.Println("📋 Проверить статус: wg_serf status")
	fmt.Println("📋 Просмотр логов: journalctl -u wg_serf -f")
}

func stopServer() {
	fmt.Println("🛑 Остановка сервера...")

	// Останавливаем через systemctl
	cmd := exec.Command("systemctl", "stop", "wg_serf")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Если systemd не доступен, пробуем через PID файл
		if strings.Contains(string(output), "Failed to connect") || strings.Contains(err.Error(), "executable file not found") {
			stopServerViaPID()
			return
		}
		fmt.Println("❌ Ошибка остановки:", err)
		os.Exit(1)
	}

	fmt.Println("✅ Сервер остановлен")
}

func restartServer() {
	fmt.Println("🔄 Перезапуск сервера...")

	// Перезапускаем через systemctl
	cmd := exec.Command("systemctl", "restart", "wg_serf")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Если systemd не доступен, делаем вручную
		if strings.Contains(string(output), "Failed to connect") || strings.Contains(err.Error(), "executable file not found") {
			stopServerViaPID()
			startServer()
			return
		}
		fmt.Println("❌ Ошибка перезапуска:", err)
		os.Exit(1)
	}

	fmt.Println("✅ Сервер перезапущен")
}

func stopServerViaPID() {
	pid, err := readPIDFile()
	if err != nil {
		fmt.Println("❌ Сервер не запущен или PID файл не найден")
		os.Exit(1)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("❌ Процесс не найден")
		os.Exit(1)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Println("❌ Ошибка остановки сервера:", err)
		os.Exit(1)
	}

	fmt.Println("✅ Сервер остановлен")
}

func showStatus() {
	if isRunning() {
		pid, _ := readPIDFile()
		config, err := database.LoadConfig()
		if err == nil {
			fmt.Printf("✅ Сервер работает (PID: %d)\n", pid)
			fmt.Printf("🌐 Веб-интерфейс: http://%s:%s\n", config.Address, config.Port)
			fmt.Printf("👤 Логин: %s\n", config.Username)
		} else {
			fmt.Printf("✅ Сервер работает (PID: %d)\n", pid)
		}
	} else {
		fmt.Println("❌ Сервер не запущен")
		os.Exit(1)
	}
}

func isRunning() bool {
	pid, err := readPIDFile()
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Проверяем существует ли процесс
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func installServer() {
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              WG_SERF Installer                               ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println("")

	// Проверяем установлена ли уже служба
	if _, err := os.Stat("/etc/systemd/system/wg_serf.service"); err == nil {
		fmt.Println("✅ WG_SERF уже установлен как служба!")
		fmt.Println("")
		fmt.Println("📋 Доступные команды:")
		fmt.Println("   wg_serf status    # Проверить статус")
		fmt.Println("   wg_serf restart   # Перезапустить")
		fmt.Println("   wg_serf stop      # Остановить")
		fmt.Println("   wg_serf delete    # Удалить")
		fmt.Println("")
		return
	}

	// Проверяем и устанавливаем WireGuard
	fmt.Println("🔍 Проверка WireGuard...")
	if !database.CheckWireGuardInstalled() {
		fmt.Println("⚠️  WireGuard не установлен. Устанавливаю...")
		if err := installWireGuard(); err != nil {
			fmt.Println("❌ Не удалось установить WireGuard:", err)
			fmt.Println("")
			fmt.Println("Установите WireGuard вручную:")
			fmt.Println("  apt install wireguard  # Debian/Ubuntu")
			fmt.Println("  dnf install wireguard-tools  # Fedora")
			fmt.Println("  yum install wireguard-tools  # CentOS")
			os.Exit(1)
		}
		fmt.Println("✅ WireGuard установлен")
	} else {
		fmt.Println("✅ WireGuard уже установлен")
	}

	// Проверяем что бинарник в правильном месте
	currentPath, err := os.Executable()
	if err != nil {
		fmt.Println("❌ Ошибка определения пути к бинарнику:", err)
		os.Exit(1)
	}

	targetPath := "/opt/wg_serf/wg_serf"
	if currentPath != targetPath {
		// Создаем директорию
		fmt.Println("📁 Создание /opt/wg_serf/...")
		os.MkdirAll("/opt/wg_serf", 0755)

		// Копируем бинарник
		fmt.Println("📦 Копирование бинарника...")
		input, err := os.ReadFile(currentPath)
		if err != nil {
			fmt.Println("❌ Ошибка чтения:", err)
			os.Exit(1)
		}
		if err := os.WriteFile(targetPath, input, 0755); err != nil {
			fmt.Println("❌ Ошибка записи:", err)
			os.Exit(1)
		}

		// Создаем symlink
		fmt.Println("🔗 Создание symlink...")
		os.Remove("/usr/local/bin/wg_serf")
		os.Symlink(targetPath, "/usr/local/bin/wg_serf")
	}

	// Создаем systemd service
	fmt.Println("⚙️  Создание systemd service...")
	serviceContent := `[Unit]
Description=WG_SERF - WireGuard Server Panel
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/wg_serf
ExecStart=/opt/wg_serf/wg_serf serve
Restart=on-failure
RestartSec=5s
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=5s

# Security
NoNewPrivileges=false
PrivateTmp=false

[Install]
WantedBy=multi-user.target
`
	if err := os.WriteFile("/etc/systemd/system/wg_serf.service", []byte(serviceContent), 0644); err != nil {
		fmt.Println("❌ Ошибка создания service:", err)
		os.Exit(1)
	}

	// Перезагрузка systemd
	fmt.Println("🔄 Перезагрузка systemd...")
	exec.Command("systemctl", "daemon-reload").Run()

	// Включение автозапуска
	fmt.Println("✅ Включение автозапуска...")
	exec.Command("systemctl", "enable", "wg_serf").Run()

	// Запуск
	fmt.Println("🚀 Запуск wg_serf...")
	cmd := exec.Command("systemctl", "start", "wg_serf")
	if err := cmd.Run(); err != nil {
		fmt.Println("❌ Ошибка запуска:", err)
		os.Exit(1)
	}

	fmt.Println("")
	// Получаем IP и конфиг
	serverIP := database.GetLocalIP()
	if serverIP == "127.0.0.1" {
		serverIP = database.GetServerEndpoint()
	}
	config, _ := database.LoadConfig()
	port := "8080"
	if config != nil {
		port = config.Port
	}

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                 ✅ Установка завершена!                      ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	fmt.Printf("🌐 Откройте в браузере: http://%s:%s\n", serverIP, port)
	if config != nil {
		fmt.Printf("👤 Логин: %s\n", config.Username)
		fmt.Printf("🔒 Пароль: %s\n", config.Password)
	} else {
		fmt.Println("👤 Логин: admin")
		fmt.Println("🔒 Пароль: admin")
	}
	fmt.Println("")
	fmt.Println("📋 Команды:")
	fmt.Println("   wg_serf status    # Проверить статус")
	fmt.Println("   wg_serf restart   # Перезапустить")
	fmt.Println("")
}

func deleteServer() {
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║              WG_SERF - Удаление                              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println("")

	// Подтверждение
	if !askYesNo("⚠️  Вы уверены? Все данные будут удалены! (yes/no): ") {
		fmt.Println("❌ Удаление отменено")
		os.Exit(0)
	}

	// Остановка сервиса
	fmt.Println("🛑 Остановка wg_serf...")
	exec.Command("systemctl", "stop", "wg_serf").Run()

	// Отключение автозапуска
	fmt.Println("🔄 Отключение автозапуска...")
	exec.Command("systemctl", "disable", "wg_serf").Run()

	// Удаление service файла
	fmt.Println("🗑️  Удаление systemd service...")
	os.Remove("/etc/systemd/system/wg_serf.service")
	exec.Command("systemctl", "daemon-reload").Run()

	// Удаление symlink
	fmt.Println("🗑️  Удаление symlink...")
	os.Remove("/usr/local/bin/wg_serf")

	// Удаление директории
	fmt.Println("🗑️  Удаление /opt/wg_serf...")
	os.RemoveAll("/opt/wg_serf")

	fmt.Println("")
	fmt.Println("✅ Удаление завершено!")
	fmt.Println("")
}

func installWireGuard() error {
	// Определяем дистрибутив
	osRelease, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return fmt.Errorf("не удалось определить дистрибутив")
	}

	osID := ""
	for _, line := range strings.Split(string(osRelease), "\n") {
		if strings.HasPrefix(line, "ID=") {
			osID = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			break
		}
	}

	var cmd *exec.Cmd
	switch osID {
	case "ubuntu", "debian":
		fmt.Println("📦 Установка для Debian/Ubuntu...")
		exec.Command("apt", "update", "-qq").Run()
		cmd = exec.Command("apt", "install", "-y", "wireguard", "wireguard-tools")
	case "centos", "rhel":
		fmt.Println("📦 Установка для RHEL/CentOS...")
		exec.Command("yum", "install", "-y", "epel-release").Run()
		cmd = exec.Command("yum", "install", "-y", "wireguard-tools")
	case "fedora":
		fmt.Println("📦 Установка для Fedora...")
		cmd = exec.Command("dnf", "install", "-y", "wireguard-tools")
	case "arch", "manjaro":
		fmt.Println("📦 Установка для Arch Linux...")
		cmd = exec.Command("pacman", "-Sy", "--noconfirm", "wireguard-tools")
	default:
		return fmt.Errorf("дистрибутив %s не поддерживается", osID)
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ошибка установки: %v, output: %s", err, string(output))
	}

	// Проверяем что установилось
	if !database.CheckWireGuardInstalled() {
		return fmt.Errorf("WireGuard не найден после установки")
	}

	return nil
}

func readPIDFile() (int, error) {
	data, err := os.ReadFile("/opt/wg_serf/wg_serf.pid")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func runServer() {
	// Проверяем и завершаем старый процесс если запущен
	if err := database.CheckAndKillOldProcess(); err != nil {
		log.Fatal("Ошибка при завершении старого процесса:", err)
	}

	// Записываем PID текущего процесса
	if err := database.WritePIDFile(); err != nil {
		log.Fatal("Ошибка записи PID файла:", err)
	}
	defer database.RemovePIDFile()

	// Проверяем установлен ли WireGuard
	if !database.CheckWireGuardInstalled() {
		log.Fatal("WireGuard не установлен! Установите WireGuard перед запуском.")
	}

	// Загружаем конфигурацию
	config, err := database.LoadConfig()
	if err != nil {
		log.Fatal("Ошибка загрузки конфигурации:", err)
	}
	server.Config = config

	// Загружаем базу данных
	db, err := database.LoadDatabase()
	if err != nil {
		log.Println("Создаю новую базу данных...")
		db = &database.Database{
			Servers: []database.Server{},
			Clients: []database.Client{},
		}
		database.SaveDatabase(db)
	}
	server.DB = db

	// Очищаем iptables (так как сервер только для WireGuard)
	if err := wireguard.CleanIPTables(); err != nil {
		log.Println("Предупреждение: ошибка очистки iptables:", err)
	}

	// Настраиваем базовые правила
	if err := wireguard.SetupBasicIPTables(); err != nil {
		log.Println("Предупреждение: ошибка настройки базовых правил:", err)
	}

	// Включаем IP forwarding
	if err := database.EnableIPForwarding(); err != nil {
		log.Println("Предупреждение: не удалось включить IP forwarding:", err)
	}

	// Синхронизируем WireGuard с базой данных (создаст правила для серверов из БД)
	if err := wireguard.SyncWireGuardWithDatabase(db); err != nil {
		log.Println("Предупреждение: ошибка синхронизации:", err)
	}

	// Настраиваем маршруты
	server.SetupRoutes()

	// Обновляем статистику каждые 5 секунд
	go wireguard.UpdateStatsLoop(db)

	addr := config.Address + ":" + config.Port
	log.Printf("🚀 Сервер запущен на http://%s\n", addr)
	log.Printf("👤 Логин: %s\n", config.Username)
	log.Printf("🔒 Пароль: %s\n", config.Password)
	log.Fatal(http.ListenAndServe(addr, nil))
}
