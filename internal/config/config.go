package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	WatchDir            string `json:"watch_dir"`
	MetaDir             string `json:"meta_dir"`
	RecentlyAddedDir    string `json:"recent_dir,omitempty"` // keep legacy key for compatibility
	RecentlyAddedDays   int    `json:"recent_days,omitempty"`
	RecentlyOpenedDir   string `json:"recently_opened_dir,omitempty"`
	RecentlyOpenedLimit int    `json:"recently_opened_limit,omitempty"`
	Editor              string `json:"editor,omitempty"`
}

const (
	defaultRecentDays           = 30
	defaultRecentlyOpenedLimit  = 20
	legacyRecentDirName         = "_recent"
	defaultRecentlyAddedDirName = "_recently_added"
	defaultRecentlyOpenedName   = "_recently_opened"
)

func defaultConfigPath() (string, error) {
	cfgHome := os.Getenv("XDG_CONFIG_HOME")
	if cfgHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cfgHome = filepath.Join(home, ".config")
	}
	return filepath.Join(cfgHome, "gorae", "config.json"), nil
}

// Path returns the full path to the config file, using the same rules as LoadOrInit.
func Path() (string, error) {
	return defaultConfigPath()
}

func defaultWatchDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("GORAE_WATCH_DIR")); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv("GOPAPYRUS_WATCH_DIR")); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv("PDF_TUI_WATCH_DIR")); v != "" {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Documents", "Papers"), nil
}

func defaultMetaDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv("GORAE_META_DIR")); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv("GOPAPYRUS_META_DIR")); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(os.Getenv("PDF_TUI_META_DIR")); v != "" {
		return v, nil
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, "gorae"), nil
}

func defaultEditor() string {
	if v := strings.TrimSpace(os.Getenv("EDITOR")); v != "" {
		return v
	}
	return "vi"
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
		changed, err := cfg.ensureDefaults()
		if err != nil {
			return nil, err
		}
		if changed {
			if err := writeConfig(path, &cfg); err != nil {
				return nil, err
			}
		}
		return &cfg, nil
	}

	// first run: create config from defaults so the app starts immediately.
	fmt.Println("No config found. Creating one with defaults.")
	watch, err := defaultWatchDir()
	if err != nil {
		return nil, err
	}
	meta, err := defaultMetaDir()
	if err != nil {
		return nil, err
	}
	recentAdded := filepath.Join(watch, defaultRecentlyAddedDirName)
	recentOpened := filepath.Join(watch, defaultRecentlyOpenedName)
	cfg := &Config{
		WatchDir:            watch,
		MetaDir:             meta,
		RecentlyAddedDir:    recentAdded,
		RecentlyAddedDays:   defaultRecentDays,
		RecentlyOpenedDir:   recentOpened,
		RecentlyOpenedLimit: defaultRecentlyOpenedLimit,
		Editor:              defaultEditor(),
	}
	fmt.Printf("  watch_dir: %s\n", cfg.WatchDir)
	fmt.Printf("  meta_dir : %s\n", cfg.MetaDir)
	fmt.Printf("Edit %s to change these paths.\n", path)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.MetaDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.RecentlyAddedDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.RecentlyOpenedDir, 0o755); err != nil {
		return nil, err
	}

	if err := writeConfig(path, cfg); err != nil {
		return nil, err
	}

	fmt.Println("Config saved to", path)
	if _, err := cfg.ensureDefaults(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func writeConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c *Config) ensureDefaults() (bool, error) {
	changed := false
	if c == nil {
		return changed, fmt.Errorf("config is nil")
	}
	if strings.TrimSpace(c.WatchDir) == "" {
		w, err := defaultWatchDir()
		if err != nil {
			return changed, err
		}
		c.WatchDir = w
		changed = true
	}
	if strings.TrimSpace(c.MetaDir) == "" {
		m, err := defaultMetaDir()
		if err != nil {
			return changed, err
		}
		c.MetaDir = m
		changed = true
	}
	if strings.TrimSpace(c.RecentlyAddedDir) == "" {
		c.RecentlyAddedDir = filepath.Join(c.WatchDir, defaultRecentlyAddedDirName)
		changed = true
	} else if isLegacyRecentPath(c.RecentlyAddedDir, c.WatchDir) {
		c.RecentlyAddedDir = upgradeLegacyRecentPath(c.RecentlyAddedDir)
		changed = true
	}
	if c.RecentlyAddedDays <= 0 {
		c.RecentlyAddedDays = defaultRecentDays
		changed = true
	}
	if strings.TrimSpace(c.RecentlyOpenedDir) == "" {
		c.RecentlyOpenedDir = filepath.Join(c.WatchDir, defaultRecentlyOpenedName)
		changed = true
	}
	if c.RecentlyOpenedLimit <= 0 {
		c.RecentlyOpenedLimit = defaultRecentlyOpenedLimit
		changed = true
	}
	if strings.TrimSpace(c.Editor) == "" {
		c.Editor = defaultEditor()
		changed = true
	}
	return changed, nil
}

func isLegacyRecentPath(path, watch string) bool {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" {
		return false
	}
	legacy := filepath.Clean(filepath.Join(watch, legacyRecentDirName))
	return clean == legacy || filepath.Base(clean) == legacyRecentDirName
}

func upgradeLegacyRecentPath(path string) string {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" {
		return filepath.Join(".", defaultRecentlyAddedDirName)
	}
	dir := filepath.Dir(clean)
	return filepath.Join(dir, defaultRecentlyAddedDirName)
}
