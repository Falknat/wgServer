package database

import (
	"encoding/json"
	"os"
)

const configFile = "/opt/wg_serf/config.json"

// LoadConfig загружает конфигурацию из config.json
func LoadConfig() (*Config, error) {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Создаем дефолтную конфигурацию
		config := Config{
			Port:     "8080",
			Address:  "0.0.0.0",
			Username: "admin",
			Password: "admin",
		}
		if err := SaveConfig(&config); err != nil {
			return nil, err
		}
		return &config, nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	return &config, err
}

// SaveConfig сохраняет конфигурацию в config.json
func SaveConfig(config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}
