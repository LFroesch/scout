package git

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// GetModifiedFiles returns a map of modified files in a git repository
func GetModifiedFiles(dir string) map[string]bool {
	modified := make(map[string]bool)

	// Check if we're in a git repository
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return modified
	}

	// Get modified files
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return modified
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) > 3 {
			// Status is in first two characters, filename starts at position 3
			filename := strings.TrimSpace(line[3:])
			if filename != "" {
				fullPath := filepath.Join(dir, filename)
				modified[fullPath] = true
			}
		}
	}

	return modified
}

// GetBranch returns the current git branch name
func GetBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
