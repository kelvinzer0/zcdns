package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Token     string `json:"token"`
	Subdomain string `json:"subdomain"`
}

func GetConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".zvault", "config.json")
}

func LoadConfig() (*Config, error) {
	b, err := os.ReadFile(GetConfigPath())
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func SaveConfig(c *Config) error {
	cfgPath := GetConfigPath()
	os.MkdirAll(filepath.Dir(cfgPath), 0755)
	b, _ := json.Marshal(c)
	return os.WriteFile(cfgPath, b, 0644)
}
