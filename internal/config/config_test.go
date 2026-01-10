package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaultConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	os.Setenv("HOME", homeDir)
	defer os.Unsetenv("HOME")

	cfg := Load()

	if cfg == nil {
		t.Fatal("Load() returned nil")
	}

	if cfg.Frecency == nil {
		t.Error("Frecency map not initialized")
	}

	if cfg.LastVisited == nil {
		t.Error("LastVisited map not initialized")
	}

	if len(cfg.Bookmarks) == 0 {
		t.Error("Default bookmarks not set")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	homeDir := filepath.Join(tempDir, "home")
	os.Setenv("HOME", homeDir)
	defer os.Unsetenv("HOME")

	// Create config
	cfg := &Config{
		RootPath:       "/test",
		Bookmarks:      []string{"/test/path1", "/test/path2"},
		ShowHidden:     true,
		PreviewEnabled: false,
		Editor:         "vim",
		Frecency:       map[string]int{"/test/path1": 5},
		LastVisited:    map[string]string{"/test/path1": "2026-01-09T12:00:00Z"},
	}

	// Save config
	err := Save(cfg)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load config
	loadedCfg := Load()

	// Verify loaded config matches saved config
	if loadedCfg.RootPath != cfg.RootPath {
		t.Errorf("RootPath mismatch: got %s, want %s", loadedCfg.RootPath, cfg.RootPath)
	}

	if loadedCfg.ShowHidden != cfg.ShowHidden {
		t.Errorf("ShowHidden mismatch: got %v, want %v", loadedCfg.ShowHidden, cfg.ShowHidden)
	}

	if loadedCfg.PreviewEnabled != cfg.PreviewEnabled {
		t.Errorf("PreviewEnabled mismatch: got %v, want %v", loadedCfg.PreviewEnabled, cfg.PreviewEnabled)
	}

	if loadedCfg.Editor != cfg.Editor {
		t.Errorf("Editor mismatch: got %s, want %s", loadedCfg.Editor, cfg.Editor)
	}

	// Note: bookmarks might have root path added, so check for minimum length
	if len(loadedCfg.Bookmarks) < len(cfg.Bookmarks) {
		t.Errorf("Bookmarks length too small: got %d, want at least %d", len(loadedCfg.Bookmarks), len(cfg.Bookmarks))
	}

	// Verify original bookmarks are present
	for _, bookmark := range cfg.Bookmarks {
		found := false
		for _, loadedBookmark := range loadedCfg.Bookmarks {
			if loadedBookmark == bookmark {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Bookmark %s not found in loaded config", bookmark)
		}
	}

	if loadedCfg.Frecency["/test/path1"] != 5 {
		t.Errorf("Frecency mismatch: got %d, want 5", loadedCfg.Frecency["/test/path1"])
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"nil slice", nil, "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			if result != tt.expected {
				t.Errorf("contains(%v, %s) = %v, want %v", tt.slice, tt.item, result, tt.expected)
			}
		})
	}
}
