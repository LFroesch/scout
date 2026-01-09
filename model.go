package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LFroesch/scout/internal/config"
	"github.com/LFroesch/scout/internal/fileops"
	"github.com/LFroesch/scout/internal/git"
	"github.com/LFroesch/scout/internal/search"
	"github.com/LFroesch/scout/internal/utils"
)

type previewUpdateMsg struct{}

type mode int

const (
	modeNormal mode = iota
	modeSearch
	modePreview
	modeBookmarks
	modeConfirmDelete
	modeRename
	modeCreateFile
	modeCreateDir
	modeCommand
	modeConfirmFileDelete
	modeGitCommit
	modeHelp
	modeErrorDialog
)

type sortMode int

const (
	sortByName sortMode = iota
	sortBySize
	sortByDate
	sortByType
)

type operationType int

const (
	opNone operationType = iota
	opCopy
	opCut
)

type searchType int

const (
	searchFilename searchType = iota
	searchContent
)

type fileItem struct {
	path      string
	name      string
	isDir     bool
	size      int64
	modTime   time.Time
	isSymlink bool
	linkTarget string
}

// Config type is now in internal/config package

type model struct {
	mode                mode
	currentDir          string
	files               []fileItem
	filteredFiles       []fileItem
	cursor              int
	scrollOffset        int
	previewScroll       int
	bookmarksCursor      int
	sortedBookmarkPaths  []string // Bookmarks sorted by frecency for display
	deleteBookmarkIndex  int      // Index of bookmark to delete
	searchInput          textinput.Model
	textInput           textinput.Model // For rename, create, command dialogs
	width               int
	height              int
	showHidden          bool
	showPreview         bool
	previewContent      string
	previewLines        []string
	config              *config.Config
	gitModified         map[string]bool
	gitBranch           string
	statusMsg           string
	statusExpiry        time.Time
	dirHistory          []string      // Navigation history
	historyIndex        int           // Current position in history
	recursiveSearch     bool          // Toggle for recursive vs current dir search
	currentSearchType   searchType    // Filename or content search
	loading             bool          // Loading indicator
	searchMatches       [][]int       // Character positions that matched in fuzzy search
	clipboard           []string      // Files in clipboard
	clipboardOp         operationType // Copy or cut
	sortBy              sortMode      // Current sort mode
	dualPane            bool          // Dual pane mode enabled
	activePane          int           // 0 = left, 1 = right
	rightDir            string        // Right pane directory
	rightFiles          []fileItem    // Right pane files
	rightCursor         int           // Right pane cursor
	rightScrollOffset   int           // Right pane scroll offset
	contentSearchResults []contentSearchResult // Ripgrep search results
	contentSearchCursor int           // Cursor in content search results
	previewPending      bool          // Preview update pending
	previewCursor       int           // Cursor position preview is showing
	lastCursorMove      time.Time     // Last time cursor moved
	scrollingFast       bool          // Currently in fast scroll mode
	helpScroll          int           // Help screen scroll position
	errorMsg            string        // Error dialog message
	errorDetails        string        // Detailed error info
	undoStack           []undoItem    // Undo history
	visitedDirs         map[string]bool // Track visited dirs for symlink loop detection
}

type undoItem struct {
	operation   string // "delete"
	path        string // Original path
	wasDir      bool   // Was it a directory?
	trashPath   string // Path in trash (if applicable)
}

type contentSearchResult struct {
	file    string
	line    int
	column  int
	content string
}

func initialModel() model {
	currentDir, _ := os.Getwd()

	// Load config
	cfg := config.Load()

	// Ensure we don't start above root path
	if cfg.RootPath != "" && !strings.HasPrefix(currentDir, cfg.RootPath) {
		currentDir = cfg.RootPath
	}

	ti := textinput.New()
	ti.Placeholder = "Type to search..."
	ti.CharLimit = 256
	ti.Width = 50

	textIn := textinput.New()
	textIn.CharLimit = 256
	textIn.Width = 50

	m := model{
		mode:              modeNormal,
		currentDir:        currentDir,
		files:             []fileItem{},
		cursor:            0,
		previewScroll:     0,
		bookmarksCursor:   0,
		searchInput:       ti,
		textInput:         textIn,
		width:             0,
		height:            0,
		showHidden:        cfg.ShowHidden,
		showPreview:       cfg.PreviewEnabled,
		config:            cfg,
		gitModified:       git.GetModifiedFiles(currentDir),
		gitBranch:         git.GetBranch(currentDir),
		dirHistory:        []string{currentDir},
		historyIndex:      0,
		recursiveSearch:   false,
		currentSearchType: searchFilename,
		loading:           false,
		searchMatches:     [][]int{},
		clipboard:         []string{},
		clipboardOp:       opNone,
		sortBy:            sortByName,
		dualPane:          false,
		activePane:        0,
		rightDir:          currentDir,
		rightFiles:        []fileItem{},
		rightCursor:       0,
		rightScrollOffset: 0,
		visitedDirs:       make(map[string]bool),
	}

	m.loadFiles()
	return m
}

func (m *model) loadFiles() {
	entries, err := os.ReadDir(m.currentDir)
	if err != nil {
		m.showError("Cannot Read Directory", fmt.Sprintf("Failed to read %s: %v", filepath.Base(m.currentDir), err))
		return
	}

	m.files = []fileItem{}

	// Add parent directory (only if we can go up)
	parentDir := filepath.Dir(m.currentDir)
	if m.currentDir != "/" && m.currentDir != m.config.RootPath &&
		(m.config.RootPath == "" || strings.HasPrefix(parentDir, m.config.RootPath)) {
		m.files = append(m.files, fileItem{
			path:  parentDir,
			name:  "..",
			isDir: true,
		})
	}

	for _, entry := range entries {
		if !m.showHidden && strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		// Skip common ignore patterns
		if utils.ShouldIgnore(entry.Name()) {
			continue
		}

		itemPath := filepath.Join(m.currentDir, entry.Name())

		// Use Lstat to get symlink info without following it
		linfo, err := os.Lstat(itemPath)
		if err != nil {
			continue
		}

		// Check if it's a symlink
		var isSymlink bool
		var linkTarget string
		var actualIsDir bool
		var actualSize int64
		var actualModTime time.Time

		if linfo.Mode()&os.ModeSymlink != 0 {
			isSymlink = true
			if target, err := os.Readlink(itemPath); err == nil {
				// Make absolute if relative
				if !filepath.IsAbs(target) {
					linkTarget = filepath.Join(m.currentDir, target)
				} else {
					linkTarget = target
				}
			}
			// Try to stat the target to get actual properties
			if targetInfo, err := os.Stat(itemPath); err == nil {
				actualIsDir = targetInfo.IsDir()
				actualSize = targetInfo.Size()
				actualModTime = targetInfo.ModTime()
			} else {
				// Broken symlink - use link info
				actualIsDir = false
				actualSize = linfo.Size()
				actualModTime = linfo.ModTime()
			}
		} else {
			actualIsDir = linfo.IsDir()
			actualSize = linfo.Size()
			actualModTime = linfo.ModTime()
		}

		item := fileItem{
			path:       itemPath,
			name:       entry.Name(),
			isDir:      actualIsDir,
			size:       actualSize,
			modTime:    actualModTime,
			isSymlink:  isSymlink,
			linkTarget: linkTarget,
		}

		m.files = append(m.files, item)
	}

	// Sort based on current sort mode
	m.sortFiles()

	m.filteredFiles = m.files
	m.updatePreview()

	// Update frecency when visiting a directory
	m.updateFrecency(m.currentDir)
}

func (m *model) sortFiles() {
	sort.Slice(m.files, func(i, j int) bool {
		// Keep ".." at top always
		if m.files[i].name == ".." {
			return true
		}
		if m.files[j].name == ".." {
			return false
		}

		// Directories first (except for size sort)
		if m.sortBy != sortBySize && m.files[i].isDir != m.files[j].isDir {
			return m.files[i].isDir
		}

		// Apply sort mode
		switch m.sortBy {
		case sortBySize:
			return m.files[i].size > m.files[j].size
		case sortByDate:
			return m.files[i].modTime.After(m.files[j].modTime)
		case sortByType:
			extI := strings.ToLower(filepath.Ext(m.files[i].name))
			extJ := strings.ToLower(filepath.Ext(m.files[j].name))
			if extI != extJ {
				return extI < extJ
			}
			return strings.ToLower(m.files[i].name) < strings.ToLower(m.files[j].name)
		default: // sortByName
			return strings.ToLower(m.files[i].name) < strings.ToLower(m.files[j].name)
		}
	})
}

func (m *model) renameFile(oldPath, newName string) error {
	return fileops.Rename(oldPath, newName)
}

func (m *model) createFile(name string) error {
	return fileops.CreateFile(m.currentDir, name)
}

func (m *model) createDir(name string) error {
	return fileops.CreateDir(m.currentDir, name)
}

func (m *model) copyFiles() error {
	return fileops.CopyMultiple(m.clipboard, m.currentDir)
}

func (m *model) cutFiles() error {
	return fileops.MoveMultiple(m.clipboard, m.currentDir)
}

func (m *model) showError(title string, details string) {
	m.errorMsg = title
	m.errorDetails = details
	m.mode = modeErrorDialog
}

func (m *model) addToUndo(item undoItem) {
	m.undoStack = append(m.undoStack, item)
	// Keep only last 10 items
	if len(m.undoStack) > 10 {
		m.undoStack = m.undoStack[1:]
	}
}

func (m *model) searchFileContent(query string) error {
	results, err := search.SearchFileContent(query, m.currentDir, m.showHidden)
	if err != nil {
		return err
	}

	// Convert search results to fileItems
	m.filteredFiles = []fileItem{}
	m.searchMatches = [][]int{}

	for _, result := range results {
		m.filteredFiles = append(m.filteredFiles, fileItem{
			path:  result.Path,
			name:  result.DisplayName,
			isDir: result.IsDir,
			size:  int64(result.LineNumber), // Store line number in size field
		})
	}

	return nil
}

func (m *model) updateFilter() {
	query := m.searchInput.Value()
	if query == "" {
		m.filteredFiles = m.files
		m.searchMatches = [][]int{}
		m.statusMsg = ""
		return
	}

	// For short queries (< 2 chars), skip expensive searches
	if len(query) < 2 && (m.recursiveSearch || m.currentSearchType == searchContent) {
		m.statusMsg = "Type at least 2 characters to search"
		m.statusExpiry = time.Now().Add(2 * time.Second)
		return
	}

	if m.currentSearchType == searchContent {
		// Content search using ripgrep
		if err := m.searchFileContent(query); err != nil {
			m.statusMsg = fmt.Sprintf("Search error: %v", err)
			m.statusExpiry = time.Now().Add(3 * time.Second)
			m.filteredFiles = []fileItem{}
		} else {
			m.statusMsg = fmt.Sprintf("Found %d matches", len(m.filteredFiles))
			m.statusExpiry = time.Now().Add(3 * time.Second)
		}
	} else {
		// Filename search
		if m.recursiveSearch {
			// Recursive search across entire project
			m.recursiveSearchFiles(query)
		} else {
			// Search in current directory only
			m.searchCurrentDir(query)
		}
		m.statusMsg = fmt.Sprintf("Found %d files", len(m.filteredFiles))
		m.statusExpiry = time.Now().Add(3 * time.Second)
	}

	// Reset cursor if it's out of bounds
	if m.cursor >= len(m.filteredFiles) {
		m.cursor = 0
	}
}

func (m *model) searchCurrentDir(query string) {
	// Build list of file names for fuzzy matching
	names := make([]string, len(m.files))
	for i, file := range m.files {
		names[i] = file.name
	}

	// Use search module for fuzzy matching
	matches := search.FuzzyMatchNames(query, names)

	m.filteredFiles = []fileItem{}
	m.searchMatches = [][]int{}

	for _, match := range matches {
		m.filteredFiles = append(m.filteredFiles, m.files[match.Index])
		m.searchMatches = append(m.searchMatches, match.MatchedIndexes)
	}
}

func (m *model) recursiveSearchFiles(query string) {
	results, matches := search.RecursiveSearchFiles(query, m.currentDir, m.showHidden, utils.ShouldIgnore)

	// Convert search results to fileItems
	m.filteredFiles = []fileItem{}
	m.searchMatches = [][]int{}

	for i, result := range results {
		m.filteredFiles = append(m.filteredFiles, fileItem{
			path:    result.Path,
			name:    result.DisplayName,
			isDir:   result.IsDir,
			size:    result.Size,
			modTime: result.ModTime,
		})
		if i < len(matches) {
			m.searchMatches = append(m.searchMatches, matches[i].MatchedIndexes)
		}
	}
}

func previewUpdateAfterDelay() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return previewUpdateMsg{}
	})
}

// Helper to handle cursor movement with fast-scroll detection
func (m *model) moveCursor(newPos int) tea.Cmd {
	now := time.Now()

	// Detect fast scrolling: if last move was less than 100ms ago
	if !m.lastCursorMove.IsZero() && now.Sub(m.lastCursorMove).Milliseconds() < 100 {
		m.scrollingFast = true
	} else {
		m.scrollingFast = false
	}

	m.lastCursorMove = now
	m.cursor = newPos

	// Only schedule preview update if NOT scrolling fast
	if !m.scrollingFast {
		m.previewPending = true
		return previewUpdateAfterDelay()
	}

	// If scrolling fast, schedule a delayed check to see if scrolling stopped
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
		return previewUpdateMsg{}
	})
}

func (m *model) updatePreview() {
	if !m.showPreview || len(m.filteredFiles) == 0 || m.cursor >= len(m.filteredFiles) {
		m.previewContent = ""
		m.previewLines = []string{}
		m.previewScroll = 0
		m.previewCursor = m.cursor
		m.previewPending = false
		return
	}

	selected := m.filteredFiles[m.cursor]
	if selected.isDir {
		m.previewContent = m.previewDirectory(selected.path)
	} else {
		m.previewContent = m.previewFile(selected.path)
	}

	// Split content into lines and handle wrapping
	previewWidth := (m.width / 2) - 4 // Account for borders and padding when in split view

	m.previewLines = m.wrapTextToLines(m.previewContent, previewWidth)
	m.previewScroll = 0
	m.previewCursor = m.cursor
	m.previewPending = false
}

// wrapTextToLines splits text into lines and wraps long lines to fit width
func (m *model) wrapTextToLines(text string, width int) []string {
	if width <= 0 {
		width = 50 // fallback width
	}

	var wrappedLines []string
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		// Use lipgloss.Width for proper unicode and ANSI handling
		if lipgloss.Width(line) <= width {
			wrappedLines = append(wrappedLines, line)
		} else {
			// Wrap long lines character by character to handle multi-byte chars
			runes := []rune(line)
			currentLine := ""
			for _, r := range runes {
				testLine := currentLine + string(r)
				if lipgloss.Width(testLine) > width {
					if currentLine != "" {
						wrappedLines = append(wrappedLines, currentLine)
					}
					currentLine = string(r)
				} else {
					currentLine = testLine
				}
			}
			if currentLine != "" {
				wrappedLines = append(wrappedLines, currentLine)
			}
		}
	}

	return wrappedLines
}

func (m *model) previewDirectory(path string) string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Sprintf("Error reading directory: %v", err)
	}

	var preview strings.Builder
	preview.WriteString(fmt.Sprintf("📁 Directory: %s\n", filepath.Base(path)))
	preview.WriteString(fmt.Sprintf("Items: %d\n\n", len(entries)))

	count := 0
	for _, entry := range entries {
		if count >= 20 {
			preview.WriteString(fmt.Sprintf("\n... and %d more items", len(entries)-20))
			break
		}

		icon := "📄"
		if entry.IsDir() {
			icon = "📁"
		} else if utils.IsImageFile(entry.Name()) {
			icon = "🖼️"
		} else if utils.IsCodeFile(entry.Name()) {
			icon = "📝"
		}

		preview.WriteString(fmt.Sprintf("%s %s\n", icon, entry.Name()))
		count++
	}

	return preview.String()
}

func (m *model) previewFile(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	var preview strings.Builder

	// File info header
	icon := utils.GetFileIcon(filepath.Base(path))
	preview.WriteString(fmt.Sprintf("%s %s\n", icon, filepath.Base(path)))
	preview.WriteString(fmt.Sprintf("Size: %s\n", utils.FormatFileSizeColored(info.Size())))
	preview.WriteString(fmt.Sprintf("Modified: %s\n", info.ModTime().Format("Jan 2, 2006 15:04")))

	if m.gitModified[path] {
		preview.WriteString("Git: Modified\n")
	}

	preview.WriteString("\n")

	// Don't preview binary or large files
	if info.Size() > 1024*1024 || utils.IsBinaryFile(path) {
		preview.WriteString("(Binary or large file - no preview)")
		return preview.String()
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		preview.WriteString(fmt.Sprintf("Error reading file: %v", err))
		return preview.String()
	}

	preview.WriteString(string(content))

	return preview.String()
}

func (m *model) updateFrecency(dir string) {
	if m.config.Frecency == nil {
		m.config.Frecency = make(map[string]int)
	}
	if m.config.LastVisited == nil {
		m.config.LastVisited = make(map[string]string)
	}

	// Increment visit count
	m.config.Frecency[dir]++

	// Update last visited timestamp
	m.config.LastVisited[dir] = time.Now().Format(time.RFC3339)

	// Save config periodically (every 10 visits)
	if m.config.Frecency[dir]%10 == 0 {
		config.Save(m.config)
	}
}

func (m *model) sortBookmarksByFrecency() []string {
	if len(m.config.Bookmarks) == 0 {
		return []string{}
	}

	type bookmarkScore struct {
		path  string
		score int
	}

	sorted := make([]bookmarkScore, len(m.config.Bookmarks))
	for i, bookmark := range m.config.Bookmarks {
		score := m.config.Frecency[bookmark]
		sorted[i] = bookmarkScore{path: bookmark, score: score}
	}

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})

	paths := make([]string, len(sorted))
	for i, bs := range sorted {
		paths[i] = bs.path
	}

	return paths
}

func (m *model) addToHistory(dir string) {
	// Don't add if it's the same as current position
	if m.historyIndex < len(m.dirHistory) && m.dirHistory[m.historyIndex] == dir {
		return
	}

	// Truncate forward history if we're not at the end
	if m.historyIndex < len(m.dirHistory)-1 {
		m.dirHistory = m.dirHistory[:m.historyIndex+1]
	}

	// Add new directory
	m.dirHistory = append(m.dirHistory, dir)
	m.historyIndex = len(m.dirHistory) - 1

	// Limit history size to 100 entries
	if len(m.dirHistory) > 100 {
		m.dirHistory = m.dirHistory[1:]
		m.historyIndex--
	}
}
