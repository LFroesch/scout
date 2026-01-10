package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// GetMountedDrives returns a list of all mounted drives/volumes
func GetMountedDrives() []string {
	var drives []string

	switch runtime.GOOS {
	case "windows":
		// Windows: Check drive letters A-Z
		for letter := 'A'; letter <= 'Z'; letter++ {
			drive := string(letter) + ":\\"
			if _, err := os.Stat(drive); err == nil {
				drives = append(drives, drive)
			}
		}

	case "darwin":
		// macOS: Check /Volumes
		volumesDir := "/Volumes"
		if entries, err := os.ReadDir(volumesDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					volumePath := filepath.Join(volumesDir, entry.Name())
					drives = append(drives, volumePath)
				}
			}
		}
		// Also add root
		drives = append(drives, "/")

	default:
		// Linux/Unix: Add root
		drives = append(drives, "/")

		// Check /mnt for mounted drives (common on WSL and Linux)
		mntDir := "/mnt"
		if entries, err := os.ReadDir(mntDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					mntPath := filepath.Join(mntDir, entry.Name())
					// Verify it's actually accessible
					if _, err := os.Stat(mntPath); err == nil {
						drives = append(drives, mntPath)
					}
				}
			}
		}

		// Check /media for mounted drives (common on Linux)
		mediaDir := "/media"
		if entries, err := os.ReadDir(mediaDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					// /media often has user subdirectories
					userDir := filepath.Join(mediaDir, entry.Name())
					if userEntries, err := os.ReadDir(userDir); err == nil {
						for _, userEntry := range userEntries {
							if userEntry.IsDir() {
								mediaPath := filepath.Join(userDir, userEntry.Name())
								if _, err := os.Stat(mediaPath); err == nil {
									drives = append(drives, mediaPath)
								}
							}
						}
					}
				}
			}
		}
	}

	// Remove duplicates
	return removeDuplicates(drives)
}

// GetDriveLabel returns a human-readable label for a drive path
func GetDriveLabel(path string) string {
	switch runtime.GOOS {
	case "windows":
		return strings.ToUpper(string(path[0])) + ":"
	case "darwin":
		if path == "/" {
			return "Root"
		}
		return filepath.Base(path)
	default:
		if path == "/" {
			return "Root"
		}
		if strings.HasPrefix(path, "/mnt/") {
			// WSL drives like /mnt/c -> "C:"
			driveLetter := strings.TrimPrefix(path, "/mnt/")
			if len(driveLetter) == 1 {
				return strings.ToUpper(driveLetter) + ":"
			}
			return filepath.Base(path)
		}
		if strings.HasPrefix(path, "/media/") {
			return filepath.Base(path)
		}
		return filepath.Base(path)
	}
}

// NormalizeToWSLPath normalizes Windows-style paths to WSL format
// Examples: "L:" -> "/mnt/l", "L:\\" -> "/mnt/l", "L:/foo" -> "/mnt/l/foo"
// Leaves WSL paths unchanged: "/mnt/l" -> "/mnt/l"
func NormalizeToWSLPath(path string) string {
	// Already a WSL/Unix path
	if strings.HasPrefix(path, "/") {
		return path
	}

	// Handle Windows drive letter format (e.g., "C:", "C:\\", "C:/")
	if len(path) >= 2 && path[1] == ':' {
		driveLetter := strings.ToLower(string(path[0]))

		// Just the drive letter (e.g., "C:")
		if len(path) == 2 {
			return "/mnt/" + driveLetter
		}

		// Drive with path (e.g., "C:\\Users" or "C:/Users")
		if len(path) > 2 && (path[2] == '\\' || path[2] == '/') {
			// Convert backslashes to forward slashes and strip leading separator
			remainingPath := strings.ReplaceAll(path[3:], "\\", "/")
			if remainingPath == "" {
				return "/mnt/" + driveLetter
			}
			return "/mnt/" + driveLetter + "/" + remainingPath
		}
	}

	return path
}

// removeDuplicates removes duplicate entries from a slice of paths
// Now normalizes paths to WSL format before deduplication
func removeDuplicates(paths []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, path := range paths {
		// Normalize to WSL format first
		normalized := NormalizeToWSLPath(path)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	return result
}
