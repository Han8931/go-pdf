package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	WatchDir string `json:"watch_dir"`
	MetaDir  string `json:"meta_dir"`
}

func defaultConfigPath() (string, error) {
	cfgHome := os.Getenv("XDG_CONFIG_HOME")
	if cfgHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cfgHome = filepath.Join(home, ".config")
	}
	return filepath.Join(cfgHome, "pdf-tui", "config.json"), nil
}

func LoadOrInit() (*Config, error) {
	path, err := defaultConfigPath()
	if err != nil {
		return nil, err
	}

	// existing config
	if data, err := os.ReadFile(path); err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
		return &cfg, nil
	}

	// first run: ask user
	fmt.Println("No config found. Let's set it up.")
	var watch, meta string

	fmt.Print("Watch folder for papers: ")
	fmt.Scanln(&watch)

	fmt.Print("Metadata directory (for sqlite DB): ")
	fmt.Scanln(&meta)

	cfg := &Config{
		WatchDir: watch,
		MetaDir:  meta,
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.MetaDir, 0o755); err != nil {
		return nil, err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}

	fmt.Println("Config saved to", path)
	return cfg, nil
}
