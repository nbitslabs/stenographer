package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const configFileName = "config.toml"

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
	Mode           string `toml:"mode"`
	SmartWhitelist *bool  `toml:"smart_whitelist"`
}

// SmartWhitelistEnabled returns whether smart auto-whitelisting is enabled.
// Defaults to true if not explicitly set.
func (f FilterConfig) SmartWhitelistEnabled() bool {
	if f.SmartWhitelist == nil {
		return true
	}
	return *f.SmartWhitelist
}

// DefaultSearchPaths returns the ordered list of directories to search for config.toml.
func DefaultSearchPaths() []string {
	paths := []string{"."}

	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths,
			filepath.Join(home, ".config", "stenographer"),
			filepath.Join(home, ".stenographer"),
		)
	}

	paths = append(paths, filepath.Join("/etc", "stenographer"))

	return paths
}

// FindConfig searches the default locations for a config file and returns
// the path to the first one found. Returns an error if none is found.
func FindConfig() (string, error) {
	for _, dir := range DefaultSearchPaths() {
		p := filepath.Join(dir, configFileName)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("config file not found; searched: %v", DefaultSearchPaths())
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Database: DatabaseConfig{Path: "stenographer.db"},
		Logging:  LoggingConfig{Level: "info"},
		Filter:   FilterConfig{Mode: "default"},
		Telegram: TelegramConfig{SessionFile: "stenographer.session"},
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
