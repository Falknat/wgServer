package database

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

const pidFile = "/opt/wg_serf/wg_serf.pid"

// CheckAndKillOldProcess проверяет и завершает старый процесс
func CheckAndKillOldProcess() error {
	// Проверяем существует ли PID файл
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		// Файла нет - это первый запуск
		return nil
	}

	// Читаем PID из файла
	data, err := os.ReadFile(pidFile)
	if err != nil {
		log.Println("⚠️  Не удалось прочитать PID файл:", err)
		return nil
	}

	oldPID, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		log.Println("⚠️  Некорректный PID в файле:", err)
		os.Remove(pidFile)
		return nil
	}

	// Проверяем существует ли процесс
	if !processExists(oldPID) {
		log.Println("📝 Старый процесс не найден, очищаю PID файл")
		os.Remove(pidFile)
		return nil
	}

	// Проверяем что это действительно wg-panel
	if !isWGPanelProcess(oldPID) {
		log.Println("⚠️  PID принадлежит другому процессу, очищаю файл")
		os.Remove(pidFile)
		return nil
	}

	// Завершаем старый процесс
	log.Printf("🔄 Обнаружена запущенная версия (PID: %d), завершаю...", oldPID)
	if err := killProcess(oldPID); err != nil {
		return fmt.Errorf("не удалось завершить старый процесс: %v", err)
	}

	log.Println("✅ Старый процесс завершен")
	os.Remove(pidFile)
	return nil
}

// WritePIDFile записывает PID текущего процесса в файл
func WritePIDFile() error {
	pid := os.Getpid()
	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

// RemovePIDFile удаляет PID файл
func RemovePIDFile() {
	os.Remove(pidFile)
}

// processExists проверяет существует ли процесс с данным PID
func processExists(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Отправляем сигнал 0 для проверки существования процесса
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// isWGPanelProcess проверяет что процесс действительно wg-panel
func isWGPanelProcess(pid int) bool {
	// Читаем командную строку процесса
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}

	// Проверяем что в командной строке есть wg-panel
	return strings.Contains(string(cmdline), "wg-panel")
}

// killProcess завершает процесс
func killProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	// Сначала пробуем мягко (SIGTERM)
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	// Ждем немного
	cmd := exec.Command("sleep", "1")
	cmd.Run()

	// Проверяем завершился ли процесс
	if processExists(pid) {
		// Если нет - убиваем принудительно (SIGKILL)
		log.Println("⚠️  Процесс не завершился, использую SIGKILL")
		return process.Signal(syscall.SIGKILL)
	}

	return nil
}
