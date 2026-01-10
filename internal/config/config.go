package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LFroesch/scout/internal/logger"
)

// Config holds all Scout configuration
type Config struct {
	SkipDirectories []string          `json:"skip_directories"` // User-configurable directories to skip during search (supports wildcards like "Python*")
	MaxResults      int               `json:"maxResults"`
	MaxDepth        int               `json:"maxDepth"`
	MaxFilesScanned int               `json:"maxFilesScanned"`
	RootPath        string            `json:"root_path"`
	Bookmarks       []string          `json:"bookmarks"`
	ShowHidden      bool              `json:"show_hidden"`
	PreviewEnabled  bool              `json:"preview_enabled"`
	Editor          string            `json:"editor"`
	Frecency        map[string]int    `json:"frecency"`
	LastVisited     map[string]string `json:"last_visited"` // path -> timestamp
}

// Load reads config from ~/.config/scout/scout-config.json
func Load() *Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get home directory: %v", err)
		// Fallback to current directory
		homeDir = "."
	}
	configDir := filepath.Join(homeDir, ".config", "scout")
	configPath := filepath.Join(configDir, "scout-config.json")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		logger.Error("Failed to create config directory %s: %v", configDir, err)
	}

	// Default config with home directory as first bookmark, but no root restriction
	defaultConfig := &Config{
		RootPath:        "", // Allow full filesystem access
		Bookmarks:       []string{homeDir, "/mnt"},
		ShowHidden:      true,
		PreviewEnabled:  true,
		Editor:          "",
		Frecency:        make(map[string]int),
		LastVisited:     make(map[string]string),
		MaxResults:      5000,
		MaxDepth:        5,
		MaxFilesScanned: 100000,
		SkipDirectories: getDefaultSkipDirectories(),
	}

	// Try to load existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Save default config and return it
		if err := Save(defaultConfig); err != nil {
			logger.Warn("Failed to save default config: %v", err)
		}
		return defaultConfig
	}

	config := &Config{}
	if err := json.Unmarshal(data, config); err != nil {
		logger.Warn("Failed to parse config file %s: %v, using defaults", configPath, err)
		return defaultConfig
	}

	// Initialize maps if they're nil
	if config.Frecency == nil {
		config.Frecency = make(map[string]int)
	}
	if config.LastVisited == nil {
		config.LastVisited = make(map[string]string)
	}
	// Initialize skip_directories with defaults if empty or nil
	if config.SkipDirectories == nil || len(config.SkipDirectories) == 0 {
		config.SkipDirectories = getDefaultSkipDirectories()
		// Save to file so users can see and edit the defaults
		if err := Save(config); err != nil {
			logger.Warn("Failed to save config after adding skip_directories: %v", err)
		}
	}

	// Validate and set search parameter defaults/bounds
	if config.MaxResults <= 0 {
		config.MaxResults = defaultConfig.MaxResults
	} else if config.MaxResults < 100 {
		logger.Warn("MaxResults too low (%d), using minimum of 100", config.MaxResults)
		config.MaxResults = 100
	} else if config.MaxResults > 50000 {
		logger.Warn("MaxResults too high (%d), using maximum of 50000", config.MaxResults)
		config.MaxResults = 50000
	}

	if config.MaxDepth <= 0 {
		config.MaxDepth = defaultConfig.MaxDepth
	} else if config.MaxDepth < 1 {
		logger.Warn("MaxDepth too low (%d), using minimum of 1", config.MaxDepth)
		config.MaxDepth = 1
	} else if config.MaxDepth > 20 {
		logger.Warn("MaxDepth too high (%d), using maximum of 20", config.MaxDepth)
		config.MaxDepth = 20
	}

	if config.MaxFilesScanned <= 0 {
		config.MaxFilesScanned = defaultConfig.MaxFilesScanned
	} else if config.MaxFilesScanned < 1000 {
		logger.Warn("MaxFilesScanned too low (%d), using minimum of 1000", config.MaxFilesScanned)
		config.MaxFilesScanned = 1000
	} else if config.MaxFilesScanned > 1000000 {
		logger.Warn("MaxFilesScanned too high (%d), using maximum of 1000000", config.MaxFilesScanned)
		config.MaxFilesScanned = 1000000
	}

	// Ensure root path is bookmarked
	if config.RootPath != "" && !contains(config.Bookmarks, config.RootPath) {
		config.Bookmarks = append([]string{config.RootPath}, config.Bookmarks...)
		if err := Save(config); err != nil {
			logger.Warn("Failed to save config after adding root path bookmark: %v", err)
		}
	}

	return config
}

// Save writes config to ~/.config/scout/scout-config.json
func Save(config *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Error("Failed to get home directory: %v", err)
		return fmt.Errorf("cannot get home directory: %w", err)
	}
	configDir := filepath.Join(homeDir, ".config", "scout")
	configPath := filepath.Join(configDir, "scout-config.json")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		logger.Error("Failed to create config directory %s: %v", configDir, err)
		return fmt.Errorf("cannot create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal config: %v", err)
		return fmt.Errorf("cannot marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		logger.Error("Failed to write config file %s: %v", configPath, err)
		return fmt.Errorf("cannot write config file: %w", err)
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// getDefaultSkipDirectories returns a good default list of directories to skip
// These are user-editable but provide a sensible starting point
func getDefaultSkipDirectories() []string {
	return []string{
		// Python installations (Python27, Python38, Python312, etc.)
		"Python*",
		// Common large game installations not caught by platform folders
		"Call of Duty*",
		"Grand Theft Auto*",
		"Red Dead Redemption*",
		"Cyberpunk*",
		"The Witcher*",
		"Minecraft*",
		"World of Warcraft*",
		// Browser caches and data
		"Google/Chrome/User Data",
		"Mozilla/Firefox/Profiles",
		// Development tools
		"Android/Sdk",
		"node_modules",
		"__pycache__",
		".venv",
		"venv",
		// System/temp directories
		"$Recycle.Bin",
		"System Volume Information",
	}
}

// GetConfigPath returns the path to the config file
func GetConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "scout", "scout-config.json"), nil
}
