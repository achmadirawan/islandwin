// Package config manages reading and persisting IslandWin settings in %APPDATA%\IslandWin\config.json.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type LastPlayed struct {
	Path       string  `json:"path"`
	PositionMs int64   `json:"positionMs"`
	Volume     float64 `json:"volume"`
	Mode       string  `json:"mode"`
}

type Config struct {
	Hotkey          string     `json:"hotkey"`
	Monitor         string     `json:"monitor"`
	HoverOpenMs     int        `json:"hoverOpenMs"`
	HoverCloseMs    int        `json:"hoverCloseMs"`
	MusicFolders    []string   `json:"musicFolders"`
	Theme           string     `json:"theme"`
	Autostart       bool       `json:"autostart"`
	CarryOverTodos  bool       `json:"carryOverTodos"`
	PrayerAlerts    bool       `json:"prayerAlerts"`
	PrayerRemindMin int        `json:"prayerRemindMin"` // minutes before prayer; 0 = off
	PrayerCityID    string     `json:"prayerCityId"`
	PrayerCityName  string     `json:"prayerCityName"`
	LastPlayed      LastPlayed `json:"lastPlayed"`
}

func DefaultConfig() *Config {
	return &Config{
		Hotkey:          "Ctrl+Alt+I",
		Monitor:         "primary",
		HoverOpenMs:     150,
		HoverCloseMs:    600,
		MusicFolders:    []string{},
		Theme:           "dark",
		Autostart:       false,
		CarryOverTodos:  true,
		PrayerAlerts:    true,
		PrayerRemindMin: 15,
		PrayerCityID:    "1301",
		PrayerCityName:  "KOTA JAKARTA",
		LastPlayed: LastPlayed{
			Volume: 0.7,
			Mode:   "sequential",
		},
	}
}

func AppDataDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	dir := filepath.Join(appData, "IslandWin")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func ConfigPath() (string, error) {
	dir, err := AppDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg := DefaultConfig()
		_ = cfg.Save()
		return cfg, nil
	} else if err != nil {
		return DefaultConfig(), err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig(), err
	}
	return cfg, nil
}

func (c *Config) Save() error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write to temp file then rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
