package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDefaultsAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("APPDATA", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Hotkey != "Ctrl+Alt+I" {
		t.Errorf("expected Ctrl+Alt+I, got %s", cfg.Hotkey)
	}

	cfg.Theme = "light"
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if reloaded.Theme != "light" {
		t.Errorf("expected theme 'light', got '%s'", reloaded.Theme)
	}

	expectedFile := filepath.Join(tmpDir, "IslandWin", "config.json")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("expected config file at %s, got err: %v", expectedFile, err)
	}
}
