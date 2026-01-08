package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds all Scout configuration
type Config struct {
	RootPath       string            `json:"root_path"`
	Bookmarks      []string          `json:"bookmarks"`
	ShowHidden     bool              `json:"show_hidden"`
	PreviewEnabled bool              `json:"preview_enabled"`
	Editor         string            `json:"editor"`
	Frecency       map[string]int    `json:"frecency"`
	LastVisited    map[string]string `json:"last_visited"` // path -> timestamp
}

// Load reads config from ~/.config/scout/scout-config.json
func Load() *Config {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".config", "scout")
	configPath := filepath.Join(configDir, "scout-config.json")

	// Create config directory if it doesn't exist
	os.MkdirAll(configDir, 0755)

	// Default config with home directory as first bookmark, but no root restriction
	defaultConfig := &Config{
		RootPath:       "", // Allow full filesystem access
		Bookmarks:      []string{homeDir, "/mnt"},
		ShowHidden:     false,
		PreviewEnabled: true,
		Editor:         "",
		Frecency:       make(map[string]int),
		LastVisited:    make(map[string]string),
	}

	// Try to load existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Save default config and return it
		Save(defaultConfig)
		return defaultConfig
	}

	config := &Config{}
	if err := json.Unmarshal(data, config); err != nil {
		// Return default config if parsing fails
		return defaultConfig
	}

	// Initialize maps if they're nil
	if config.Frecency == nil {
		config.Frecency = make(map[string]int)
	}
	if config.LastVisited == nil {
		config.LastVisited = make(map[string]string)
	}

	// Ensure root path is bookmarked
	if config.RootPath != "" && !contains(config.Bookmarks, config.RootPath) {
		config.Bookmarks = append([]string{config.RootPath}, config.Bookmarks...)
		Save(config)
	}

	return config
}

// Save writes config to ~/.config/scout/scout-config.json
func Save(config *Config) {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".config", "scout")
	configPath := filepath.Join(configDir, "scout-config.json")

	// Create config directory if it doesn't exist
	os.MkdirAll(configDir, 0755)

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(configPath, data, 0644)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
