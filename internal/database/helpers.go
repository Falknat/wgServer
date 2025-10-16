package database

import (
	"regexp"
	"strings"
)

// SanitizeFilename очищает имя файла от недопустимых символов
func SanitizeFilename(name string) string {
	// Заменяем пробелы на подчеркивания
	name = strings.ReplaceAll(name, " ", "_")

	// Заменяем тире и другие символы на подчеркивания
	name = strings.ReplaceAll(name, "-", "_")

	// Удаляем все символы кроме букв, цифр и подчеркиваний
	reg := regexp.MustCompile(`[^a-zA-Z0-9_а-яА-ЯёЁ]`)
	name = reg.ReplaceAllString(name, "_")

	// Убираем множественные подчеркивания
	reg = regexp.MustCompile(`_+`)
	name = reg.ReplaceAllString(name, "_")

	// Убираем подчеркивания в начале и конце
	name = strings.Trim(name, "_")

	// Если имя пустое - возвращаем дефолтное
	if name == "" {
		name = "client"
	}

	return name
}
