package search

import (
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

// Large directories to skip during recursive searches
var skipDirs = map[string]bool{
	// Version control
	".git": true, ".svn": true, ".hg": true,
	// Dependencies
	"node_modules": true, "vendor": true, ".npm": true, ".yarn": true,
	// Build outputs
	"dist": true, "build": true, "target": true, ".next": true, ".nuxt": true,
	// Caches
	".cache": true, "__pycache__": true, ".pytest_cache": true,
	// Python environments
	".venv": true, "venv": true, "env": true, "virtualenv": true,
	"site-packages": true, ".tox": true, ".mypy_cache": true,
	// Language toolchains
	".cargo": true, ".rustup": true, ".go": true, ".gradle": true,
	".m2": true, ".ivy2": true, ".pub-cache": true,
	// Game directories
	"steamapps": true, "Steam": true, "SteamLibrary": true,
	"Epic Games": true, "EpicGamesLauncher": true,
	"XboxGames": true, "WindowsApps": true, "ModifiableWindowsApps": true,
	"GOG Galaxy": true, "Origin Games": true, "Riot Games": true,
	// Windows system dirs
	"$Recycle.Bin": true, "$RECYCLE.BIN": true, "System Volume Information": true, "Recovery": true,
	"Windows": true, "Program Files": true, "Program Files (x86)": true,
	"ProgramData": true, "AppData": true,
	"Config.Msi": true, "PerfLogs": true, "AMD": true,
	// Linux system dirs (critical for WSL/native Linux)
	"proc": true, "sys": true, "dev": true, "run": true, "lost+found": true,
	"tmp": true, "var": true, "boot": true, "snap": true,
	// WSL-specific
	"wslg": true,
	// macOS system
	"Library": true, "System": true, ".Trash": true,
	// IDE/Editor
	".idea": true, ".vscode": true, ".vs": true,
}

// Skip directory patterns (for dynamic matching like TEMP*, wsl*, etc.)
var skipDirPatterns = []string{
	"TEMP",           // TEMP.* directories
	"UMFD-",          // UMFD-* font driver directories
	"wsl",            // wsl* temporary directories
	"found.",         // found.* system directories
	"AMD",            // AMD* installer directories
	".Font Driver",   // Font driver temp directories
	"Unreal",         // Unreal Engine projects (UnrealEngine, Unreal Projects, etc.)
	"Unity",          // Unity projects and cache
}

// Absolute paths to skip (for Linux/WSL system directories)
// These are only checked for absolute paths to avoid filtering project folders
var skipAbsolutePaths = []string{
	"/usr",     // System binaries, libraries, documentation
	"/bin",     // Essential command binaries
	"/sbin",    // System binaries
	"/lib",     // System libraries
	"/lib32",   // 32-bit libraries
	"/lib64",   // 64-bit libraries
	"/libx32",  // x32 ABI libraries
	"/etc",     // System configuration
	"/opt",     // Optional software packages
	"/mnt",     // Mounted drives (searched separately in ultrasearch to avoid duplicates)
	"/media",   // Mounted media (searched separately in ultrasearch to avoid duplicates)
}

// SearchFileContent searches for content within files using ripgrep
// Limits: Max depth 5, max 2000 results, 30 second timeout
func SearchFileContent(query, currentDir string, showHidden bool, cancelChan <-chan struct{}) ([]Result, error) {
	logger.Warn("Starting content search in %s for query '%s'", currentDir, query)
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
		"--max-depth=5",      // Limit search depth
		"--max-count=2000",   // Max total results
		"--max-filesize=1M",  // Skip files larger than 1MB (performance)
	}

	// Exclude large/system directories (same as recursive search filters)
	// Note: ripgrep glob patterns use different syntax than filepath patterns
	excludeDirs := []string{
		// Version control
		".git", ".svn", ".hg",
		// Dependencies
		"node_modules", "vendor", ".npm", ".yarn",
		// Build outputs
		"dist", "build", "target", ".next", ".nuxt",
		// Caches
		".cache", "__pycache__", ".pytest_cache",
		// Python environments
		".venv", "venv", "env", "virtualenv", "site-packages", ".tox", ".mypy_cache",
		// Language toolchains
		".cargo", ".rustup", ".go", ".gradle", ".m2", ".ivy2", ".pub-cache",
		// Game directories
		"steamapps", "Steam", "SteamLibrary", "Epic Games", "EpicGamesLauncher",
		"XboxGames", "WindowsApps", "ModifiableWindowsApps",
		"GOG Galaxy", "Origin Games", "Riot Games",
		// Windows system
		"$Recycle.Bin", "$RECYCLE.BIN", "System Volume Information", "Recovery",
		"Windows", "Program Files", "Program Files (x86)", "ProgramData", "AppData",
		"Config.Msi", "PerfLogs",
		// Linux system
		"proc", "sys", "dev", "run", "lost+found", "tmp", "var", "boot", "snap", "wslg",
		// macOS system
		"Library", "System", ".Trash",
		// IDE/Editor
		".idea", ".vscode", ".vs",
	}
	// Add pattern-based exclusions separately (ripgrep glob syntax)
	patternDirs := []string{
		"TEMP*",         // TEMP, TEMP.*, etc.
		"UMFD-*",        // UMFD-0, UMFD-1, etc.
		"wsl*",          // wsl temporary dirs
		"found.*",       // found.000, etc.
		"AMD*",          // AMD installer dirs
		"*Font Driver*", // Font driver directories
		"Unreal*",       // Unreal Engine projects
		"Unity*",        // Unity projects and cache
	}
	// Add absolute path exclusions for Linux/WSL system directories
	// Using --glob with full paths for ripgrep
	absoluteExcludes := []string{
		"/usr", "/bin", "/sbin", "/lib", "/lib32", "/lib64", "/libx32", "/etc", "/opt",
	}

	for _, dir := range excludeDirs {
		args = append(args, "--glob", "!"+dir)
	}
	for _, pattern := range patternDirs {
		args = append(args, "--glob", "!"+pattern)
	}
	for _, absPath := range absoluteExcludes {
		args = append(args, "--glob", "!"+absPath)
	}

	// Respect .gitignore and ignore common directories
	if !showHidden {
		args = append(args, "--no-hidden")
	} else {
		args = append(args, "--hidden")
	}

	args = append(args, query, currentDir)

	// Log the command for debugging
	logger.Warn("Running ripgrep: %s %v", rgPath, strings.Join(args, " "))

	// Check cancellation before starting expensive operation
	select {
	case <-cancelChan:
		logger.Warn("Content search cancelled before execution")
		return []Result{}, nil
	default:
	}

	// Run ripgrep with 30 second timeout and cancellation support
	cmd := exec.Command(rgPath, args...)

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
				logger.Warn("Content search cancelled during execution")
			}
		case <-done:
			// Command completed normally
		}
	}()

	output, err := cmd.Output()
	close(done) // Signal completion

	if err != nil {
		// rg returns exit code 1 if no matches found
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				logger.Warn("Content search complete: no matches in %v", time.Since(startTime))
				return []Result{}, nil
			}
			// Exit code 2 can mean invalid args OR permission errors
			// Permission errors are normal - just parse what we got
			if exitErr.ExitCode() == 2 {
				stderr := string(exitErr.Stderr)
				// If stderr only contains permission errors, treat as success with partial results
				if strings.Contains(stderr, "Permission denied") {
					logger.Warn("Content search had permission errors, continuing with partial results")
					// Fall through to parse results
				} else {
					// Actual invalid args
					logger.Error("Content search invalid args (exit 2): %s", stderr)
					return nil, fmt.Errorf("ripgrep error: %s", stderr)
				}
			}
		}
		// Check if killed (by timeout or cancellation)
		if strings.Contains(err.Error(), "killed") {
			return nil, fmt.Errorf("search cancelled or timed out")
		}
		// For other errors, log but try to parse results anyway
		logger.Warn("Content search had errors: %v", err)
	}

	logger.Warn("Content search got results, parsing output (%d bytes)", len(output))

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

	logger.Warn("Content search complete: %d results in %v", len(results), time.Since(startTime))
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
// Results are sent via callback as they're found, allowing progressive display
// customSkipDirs are user-configurable directories to skip (merged with hardcoded essentials)
func RecursiveSearchFiles(query, currentDir string, showHidden bool, shouldIgnoreFn func(string) bool, cancelChan <-chan struct{}, onProgress func(scanned int), maxResults, maxDepth, maxFilesScanned int, customSkipDirs []string) ([]Result, []MatchResult) {

	logger.Warn("Starting recursive search in %s for query '%s'", currentDir, query)
	startTime := time.Now()

	var allFiles []Result
	var allNames []string
	scannedCount := 0
	skippedDirs := 0
	permissionErrors := 0

	filepath.WalkDir(currentDir, func(path string, d fs.DirEntry, err error) error {
		// Check cancellation first (critical for responsiveness)
		select {
		case <-cancelChan:
			logger.Warn("Recursive search cancelled after scanning %d files in %v", scannedCount, time.Since(startTime))
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

		// Skip large/system directories early (BEFORE depth check)
		if d.IsDir() {
			// Check absolute system paths first (only for root-level directories)
			for _, absPath := range skipAbsolutePaths {
				if path == absPath {
					skippedDirs++
					return filepath.SkipDir
				}
			}

			// Check hardcoded essential directories (exact matches)
			if skipDirs[d.Name()] {
				skippedDirs++
				return filepath.SkipDir
			}

			// Check hardcoded pattern matches (for TEMP*, UMFD-*, wsl*, etc.)
			for _, pattern := range skipDirPatterns {
				if strings.HasPrefix(d.Name(), pattern) {
					skippedDirs++
					return filepath.SkipDir
				}
			}

			// Check user-configurable skip directories
			for _, customPattern := range customSkipDirs {
				// Support wildcards with * (e.g., "Python*", "Call of Duty*")
				if strings.HasSuffix(customPattern, "*") {
					prefix := strings.TrimSuffix(customPattern, "*")
					if strings.HasPrefix(d.Name(), prefix) {
						skippedDirs++
						return filepath.SkipDir
					}
				} else if d.Name() == customPattern {
					// Exact match
					skippedDirs++
					return filepath.SkipDir
				}
			}
		}

		// Check depth limit (prevents deep recursion on large drives)
		relPath, _ := filepath.Rel(currentDir, path)
		if relPath != "." {
			depth := strings.Count(relPath, string(filepath.Separator))
			if depth > maxDepth {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
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

	// Log summary with permission error count if any occurred
	if permissionErrors > 0 {
		logger.Warn("Scan complete: %d files scanned, %d dirs skipped, %d permission errors in %v", scannedCount, skippedDirs, permissionErrors, time.Since(startTime))
	} else {
		logger.Warn("Scan complete: %d files scanned, %d dirs skipped in %v", scannedCount, skippedDirs, time.Since(startTime))
	}

	// Check cancellation before fuzzy matching
	select {
	case <-cancelChan:
		logger.Warn("Search cancelled before fuzzy matching")
		return []Result{}, []MatchResult{}
	default:
	}

	// Use substring matching (more predictable than fuzzy)
	matchStart := time.Now()
	matches := SubstringMatchNames(query, allNames)
	logger.Warn("Substring matching took %v, found %d matches", time.Since(matchStart), len(matches))

	var filteredFiles []Result
	var searchMatches []MatchResult

	for _, match := range matches {
		// Check cancellation during result processing
		select {
		case <-cancelChan:
			logger.Warn("Search cancelled during result processing")
			return filteredFiles, searchMatches
		default:
		}

		if len(filteredFiles) >= maxResults {
			logger.Warn("Hit max results limit (%d)", maxResults)
			break
		}
		if match.Index < len(allFiles) {
			filteredFiles = append(filteredFiles, allFiles[match.Index])
			searchMatches = append(searchMatches, match)
		}
	}

	logger.Warn("Recursive search complete: returned %d results", len(filteredFiles))
	return filteredFiles, searchMatches
}

// commandExists checks if a command is available in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
