package database

import (
	"encoding/json"
	"os"
)

const dbFile = "/opt/wg_serf/db.json"

// LoadDatabase загружает базу данных из db.json
func LoadDatabase() (*Database, error) {
	var db Database
	data, err := os.ReadFile(dbFile)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &db)
	return &db, err
}

// SaveDatabase сохраняет базу данных в db.json
func SaveDatabase(db *Database) error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dbFile, data, 0644)
}
