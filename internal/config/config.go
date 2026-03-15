package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Telegram TelegramConfig `toml:"telegram"`
	Database DatabaseConfig `toml:"database"`
	Logging  LoggingConfig  `toml:"logging"`
	Filter   FilterConfig   `toml:"filter"`
}

type TelegramConfig struct {
	AppID       int    `toml:"app_id"`
	AppHash     string `toml:"app_hash"`
	Phone       string `toml:"phone"`
	SessionFile string `toml:"session_file"`
}

type DatabaseConfig struct {
	Path string `toml:"path"`
}

type LoggingConfig struct {
	Level string `toml:"level"`
}

type FilterConfig struct {
	Mode string `toml:"mode"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Database: DatabaseConfig{Path: "stenographer.db"},
		Logging:  LoggingConfig{Level: "info"},
		Filter:   FilterConfig{Mode: "blacklist"},
		Telegram: TelegramConfig{SessionFile: "stenographer.session"},
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
