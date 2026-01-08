package search

import (
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sahilm/fuzzy"
)

// Result represents a search result with path and display name
type Result struct {
	Path        string
	DisplayName string
	IsDir       bool
	Size        int64
	ModTime     time.Time
	LineNumber  int // For content searches
}

// SearchFileContent searches for content within files using ripgrep
func SearchFileContent(query, currentDir string, showHidden bool) ([]Result, error) {
	// Try to find ripgrep binary
	rgPath := ""
	for _, path := range []string{"rg", "ripgrep"} {
		if commandExists(path) {
			rgPath = path
			break
		}
	}

	if rgPath == "" {
		return nil, fmt.Errorf("ripgrep not found - install with: sudo apt install ripgrep")
	}

	// Build ripgrep command with appropriate flags
	args := []string{
		"--line-number",
		"--column",
		"--no-heading",
		"--color=never",
	}

	// Respect .gitignore and ignore common directories
	if !showHidden {
		args = append(args, "--no-hidden")
	} else {
		args = append(args, "--hidden")
	}

	args = append(args, query, currentDir)

	// Run ripgrep
	cmd := exec.Command(rgPath, args...)
	output, err := cmd.Output()
	if err != nil {
		// rg returns exit code 1 if no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return []Result{}, nil
		}
		return nil, err
	}

	// Parse results
	var results []Result
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}
		// Format: file:line:column:content
		parts := strings.SplitN(line, ":", 4)
		if len(parts) >= 4 {
			lineNum := 0
			fmt.Sscanf(parts[1], "%d", &lineNum)

			filePath := parts[0]
			content := parts[3]

			// Get relative path for display
			relPath := filePath
			if strings.HasPrefix(filePath, currentDir) {
				rel, err := filepath.Rel(currentDir, filePath)
				if err == nil {
					relPath = rel
				}
			}

			// Create display name: file:line - content preview
			displayName := fmt.Sprintf("%s:%d - %s", relPath, lineNum, strings.TrimSpace(content))
			if len(displayName) > 100 {
				displayName = displayName[:97] + "..."
			}

			results = append(results, Result{
				Path:        filePath,
				DisplayName: displayName,
				IsDir:       false,
				LineNumber:  lineNum,
			})
		}
	}

	return results, nil
}

// MatchResult contains fuzzy match information
type MatchResult struct {
	Index          int
	MatchedIndexes []int
}

// FuzzyMatchNames performs fuzzy matching on a list of names
// Returns the indices of matches and their matched character positions
func FuzzyMatchNames(query string, names []string) []MatchResult {
	matches := fuzzy.Find(query, names)

	var results []MatchResult
	for _, match := range matches {
		results = append(results, MatchResult{
			Index:          match.Index,
			MatchedIndexes: match.MatchedIndexes,
		})
	}

	return results
}

// RecursiveSearchFiles searches for files recursively using fuzzy matching
func RecursiveSearchFiles(query, currentDir string, showHidden bool, shouldIgnoreFn func(string) bool) ([]Result, []MatchResult) {
	// Collect all files recursively
	var allFiles []Result
	var allNames []string

	filepath.WalkDir(currentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden files if not showing them
		if !showHidden && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip ignored patterns
		if shouldIgnoreFn(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path for display
		relPath, _ := filepath.Rel(currentDir, path)
		if relPath == "." {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		allFiles = append(allFiles, Result{
			Path:        path,
			DisplayName: relPath,
			IsDir:       d.IsDir(),
			Size:        info.Size(),
			ModTime:     info.ModTime(),
		})
		allNames = append(allNames, relPath)

		return nil
	})

	// Use fuzzy matching
	matches := FuzzyMatchNames(query, allNames)

	var filteredFiles []Result
	var searchMatches []MatchResult

	for _, match := range matches {
		if match.Index < len(allFiles) {
			filteredFiles = append(filteredFiles, allFiles[match.Index])
			searchMatches = append(searchMatches, match)
		}
	}

	return filteredFiles, searchMatches
}

// commandExists checks if a command is available in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
