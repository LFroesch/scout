package search

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LFroesch/scout/internal/logger"
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

// shouldSkipDir checks if a directory should be skipped based on the skip list.
// Supports three pattern types:
//   - Absolute paths: "/usr/bin" matches the exact full path
//   - Wildcards: "Python*" matches any name starting with "Python"
//   - Exact names: "node_modules" matches the directory name
func shouldSkipDir(path, name string, skipDirs []string) bool {
	for _, pattern := range skipDirs {
		if strings.HasPrefix(pattern, "/") {
			// Absolute path — match against full path
			if path == pattern {
				return true
			}
		} else if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
			// Contains match: *Font Driver* → name contains "Font Driver"
			inner := pattern[1 : len(pattern)-1]
			if strings.Contains(name, inner) {
				return true
			}
		} else if strings.HasSuffix(pattern, "*") {
			// Prefix wildcard: "Python*" → name starts with "Python"
			prefix := pattern[:len(pattern)-1]
			if strings.HasPrefix(name, prefix) {
				return true
			}
		} else {
			// Exact name match
			if name == pattern {
				return true
			}
		}
	}
	return false
}

// SearchFileContent searches for content within files using ripgrep
// Limits configurable via maxResults, maxDepth parameters
// onResult is called for each result as it's found (may be nil)
func SearchFileContent(query, currentDir string, showHidden bool, cancelChan <-chan struct{}, onResult func(Result), maxResults, maxDepth int, customSkipDirs []string) ([]Result, error) {
	logger.Info("Starting content search in %s for query '%s'", currentDir, query)
	startTime := time.Now()
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
		fmt.Sprintf("--max-depth=%d", maxDepth),
		fmt.Sprintf("--max-count=%d", maxResults),
		"--max-filesize=1M",  // Skip files larger than 1MB (performance)
	}

	// Build ripgrep exclusions from the config skip list
	for _, pattern := range customSkipDirs {
		args = append(args, "--glob", "!"+pattern)
	}

	// Respect .gitignore and ignore common directories
	if !showHidden {
		args = append(args, "--no-hidden")
	} else {
		args = append(args, "--hidden")
	}

	args = append(args, query, currentDir)

	// Log the command for debugging
	logger.Debug("Running ripgrep: %s %v", rgPath, strings.Join(args, " "))

	// Check cancellation before starting expensive operation
	select {
	case <-cancelChan:
		logger.Info("Content search cancelled before execution")
		return []Result{}, nil
	default:
	}

	// Run ripgrep with streaming output
	cmd := exec.Command(rgPath, args...)

	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	stdout, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		return nil, pipeErr
	}

	// Set up timeout AND cancellation monitoring
	done := make(chan struct{})
	go func() {
		select {
		case <-time.After(30 * time.Second):
			if cmd.Process != nil {
				cmd.Process.Kill()
				logger.Warn("Content search timed out after 30s")
			}
		case <-cancelChan:
			if cmd.Process != nil {
				cmd.Process.Kill()
				logger.Info("Content search cancelled during execution")
			}
		case <-done:
		}
	}()

	if err := cmd.Start(); err != nil {
		close(done)
		return nil, err
	}

	// Parse results line-by-line as ripgrep outputs them
	var results []Result
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
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

			relPath := filePath
			if strings.HasPrefix(filePath, currentDir) {
				if rel, err := filepath.Rel(currentDir, filePath); err == nil {
					relPath = rel
				}
			}

			displayName := fmt.Sprintf("%s:%d - %s", relPath, lineNum, strings.TrimSpace(content))
			if len(displayName) > 100 {
				displayName = displayName[:97] + "..."
			}

			result := Result{
				Path:        filePath,
				DisplayName: displayName,
				IsDir:       false,
				LineNumber:  lineNum,
			}
			results = append(results, result)
			if onResult != nil {
				onResult(result)
			}
		}
	}

	waitErr := cmd.Wait()
	close(done)

	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			switch exitErr.ExitCode() {
			case 1:
				// No matches - normal
				logger.Info("Content search complete: no matches in %v", time.Since(startTime))
				return results, nil
			case 2:
				stderr := stderrBuf.String()
				if !strings.Contains(stderr, "Permission denied") {
					logger.Error("Content search invalid args (exit 2): %s", stderr)
					return results, fmt.Errorf("ripgrep error: %s", stderr)
				}
				logger.Warn("Content search had permission errors, returning partial results")
			}
		}
		if strings.Contains(waitErr.Error(), "killed") {
			// Cancelled - return whatever we collected
			return results, nil
		}
		logger.Warn("Content search wait error: %v", waitErr)
	}

	logger.Info("Content search complete: %d results in %v", len(results), time.Since(startTime))
	return results, nil
}

// MatchResult contains fuzzy match information
type MatchResult struct {
	Index          int
	MatchedIndexes []int
}

// SubstringMatchNames performs case-insensitive substring matching on a list of names
// Returns the indices of matches and their matched character positions
func SubstringMatchNames(query string, names []string) []MatchResult {
	if query == "" {
		return nil
	}

	lowerQuery := strings.ToLower(query)
	var results []MatchResult

	for i, name := range names {
		lowerName := strings.ToLower(name)
		if idx := strings.Index(lowerName, lowerQuery); idx != -1 {
			// Found a match - calculate matched character positions
			matchedIndexes := make([]int, len(query))
			for j := 0; j < len(query); j++ {
				matchedIndexes[j] = idx + j
			}
			results = append(results, MatchResult{
				Index:          i,
				MatchedIndexes: matchedIndexes,
			})
		}
	}

	return results
}

// RecursiveSearchFiles searches for files recursively using substring matching with streaming results
// Limits configurable via maxResults, maxDepth, maxFilesScanned parameters
// onResult is called for each matching file as it's found (may be nil)
// customSkipDirs are user-configurable directories to skip (merged with hardcoded essentials)
func RecursiveSearchFiles(query, currentDir string, showHidden bool, shouldIgnoreFn func(string) bool, cancelChan <-chan struct{}, onProgress func(scanned int), onResult func(Result, MatchResult), maxResults, maxDepth, maxFilesScanned int, customSkipDirs []string, nameOnly bool) ([]Result, []MatchResult) {

	logger.Info("Starting recursive search in %s for query '%s'", currentDir, query)
	startTime := time.Now()

	lowerQuery := strings.ToLower(query)
	var filteredFiles []Result
	var searchMatches []MatchResult
	scannedCount := 0
	skippedDirs := 0
	permissionErrors := 0

	// Precompute prefix for fast relative path calculation (avoid filepath.Rel per file)
	dirPrefix := currentDir
	if !strings.HasSuffix(dirPrefix, string(filepath.Separator)) {
		dirPrefix += string(filepath.Separator)
	}

	filepath.WalkDir(currentDir, func(path string, d fs.DirEntry, err error) error {
		// Check cancellation first (critical for responsiveness)
		select {
		case <-cancelChan:
			logger.Info("Recursive search cancelled after scanning %d files in %v", scannedCount, time.Since(startTime))
			return filepath.SkipAll
		default:
		}

		if err != nil {
			// Don't spam logs with permission errors - just count them
			if strings.Contains(err.Error(), "permission denied") {
				permissionErrors++
				return nil
			}
			// Log other errors normally
			logger.Error("WalkDir error at %s: %v", path, err)
			return nil // Skip errors
		}

		// Hard stop if we've scanned too many files
		scannedCount++
		if scannedCount%1000 == 0 && onProgress != nil {
			onProgress(scannedCount) // Report progress every 1000 files
		}

		if scannedCount > maxFilesScanned {
			logger.Warn("Hit max files scanned limit (%d)", maxFilesScanned)
			return filepath.SkipAll
		}

		// Skip directories from config skip list (supports exact, wildcard, and absolute paths)
		if d.IsDir() && shouldSkipDir(path, d.Name(), customSkipDirs) {
			skippedDirs++
			return filepath.SkipDir
		}

		// Fast relative path via prefix trim (avoids filepath.Rel syscall overhead)
		relPath := strings.TrimPrefix(path, dirPrefix)
		if relPath == path {
			// path == currentDir itself, skip the root entry
			return nil
		}

		// Check depth limit (prevents deep recursion on large drives)
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > maxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
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

		// Inline substring matching - match as we walk, no second pass needed
		// d.Info() is deferred to only matched files (avoids stat syscall on every file)
		var matchTarget string
		var matchOffset int // offset to translate match position back to relPath for highlighting
		if nameOnly {
			matchTarget = strings.ToLower(d.Name())
			// basename starts at last separator + 1 in relPath
			if lastSep := strings.LastIndex(relPath, string(filepath.Separator)); lastSep != -1 {
				matchOffset = lastSep + 1
			}
		} else {
			matchTarget = strings.ToLower(relPath)
		}
		if idx := strings.Index(matchTarget, lowerQuery); idx != -1 {
			matchedIndexes := make([]int, len(query))
			for j := range query {
				matchedIndexes[j] = idx + j + matchOffset
			}

			// Only stat matched files (d.Info() triggers a syscall)
			var size int64
			var modTime time.Time
			if info, err := d.Info(); err == nil {
				size = info.Size()
				modTime = info.ModTime()
			}

			result := Result{
				Path:        path,
				DisplayName: relPath,
				IsDir:       d.IsDir(),
				Size:        size,
				ModTime:     modTime,
			}
			mr := MatchResult{Index: len(filteredFiles), MatchedIndexes: matchedIndexes}
			filteredFiles = append(filteredFiles, result)
			searchMatches = append(searchMatches, mr)
			if onResult != nil {
				onResult(result, mr)
			}
			if len(filteredFiles) >= maxResults {
				logger.Warn("Hit max results limit (%d)", maxResults)
				return filepath.SkipAll
			}
		}

		return nil
	})

	// Log summary with permission error count if any occurred
	if permissionErrors > 0 {
		logger.Warn("Scan complete: %d files scanned, %d dirs skipped, %d permission errors in %v", scannedCount, skippedDirs, permissionErrors, time.Since(startTime))
	} else {
		logger.Info("Scan complete: %d files scanned, %d dirs skipped in %v", scannedCount, skippedDirs, time.Since(startTime))
	}

	logger.Info("Recursive search complete: returned %d results", len(filteredFiles))
	return filteredFiles, searchMatches
}

// commandExists checks if a command is available in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
