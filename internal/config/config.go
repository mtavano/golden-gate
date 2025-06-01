package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type ServiceConfig struct {
	BasePrefix string `json:"base_prefix"`
	Target     string `json:"target"`
}

type Config struct {
	Services map[string]ServiceConfig `json:"-"`
}

func LoadConfig(configPath string) (*Config, error) {
	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(file, &config.Services); err != nil {
		return nil, err
	}

	return &config, nil
}

func GetConfigPath() string {
	return filepath.Join("configs", "service.json")
} 