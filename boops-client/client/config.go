package client

import (
	"encoding/json"
	"os"
)

type Config struct {
	ID string `json:"id"`
}

var configPath = "/etc/boops/config.json"

func SaveConfig(id string) error {
	cfg := Config{ID: id}
	data, _ := json.Marshal(cfg)
	os.MkdirAll("/etc/boops", 0755)
	return os.WriteFile(configPath, data, 0644)
}

func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(data, &cfg)
	return &cfg, err
}