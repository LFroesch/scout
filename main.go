package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

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
	modeSortMenu
	modeConfirmFileDelete
	modeGitCommit
	modeContentSearch
	modeHelp
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

type fileItem struct {
	path     string
	name     string
	isDir    bool
	size     int64
	modTime  time.Time
	selected bool
}

type Config struct {
	RootPath       string            `json:"root_path"`
	Bookmarks      []string          `json:"bookmarks"`
	ShowHidden     bool              `json:"show_hidden"`
	PreviewEnabled bool              `json:"preview_enabled"`
	Editor         string            `json:"editor"`
	Frecency       map[string]int    `json:"frecency"`
	LastVisited    map[string]string `json:"last_visited"` // path -> timestamp
}

type model struct {
	mode                mode
	currentDir          string
	files               []fileItem
	filteredFiles       []fileItem
	cursor              int
	scrollOffset        int
	previewScroll       int
	bookmarksCursor     int
	deleteBookmarkIndex int // Index of bookmark to delete
	searchInput         textinput.Model
	textInput           textinput.Model // For rename, create, command dialogs
	width               int
	height              int
	showHidden          bool
	showPreview         bool
	previewContent      string
	previewLines        []string
	config              *Config
	gitModified         map[string]bool
	gitBranch           string
	statusMsg           string
	statusExpiry        time.Time
	dirHistory          []string      // Navigation history
	historyIndex        int           // Current position in history
	recursiveSearch     bool          // Toggle for recursive vs current dir search
	loading             bool          // Loading indicator
	searchMatches       [][]int       // Character positions that matched in fuzzy search
	clipboard           []string      // Files in clipboard
	clipboardOp         operationType // Copy or cut
	sortBy              sortMode      // Current sort mode
	sortMenuCursor      int           // Cursor in sort menu
	dualPane            bool          // Dual pane mode enabled
	activePane          int           // 0 = left, 1 = right
	rightDir            string        // Right pane directory
	rightFiles          []fileItem    // Right pane files
	rightCursor         int           // Right pane cursor
	rightScrollOffset   int           // Right pane scroll offset
	permissions         bool          // Show permissions
	contentSearchResults []contentSearchResult // Ripgrep search results
	contentSearchCursor int           // Cursor in content search results
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
	config := loadConfig()

	// Ensure we don't start above root path
	if config.RootPath != "" && !strings.HasPrefix(currentDir, config.RootPath) {
		currentDir = config.RootPath
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
		showHidden:        config.ShowHidden,
		showPreview:       config.PreviewEnabled,
		config:            config,
		gitModified:       getGitModifiedFiles(currentDir),
		gitBranch:         getGitBranch(currentDir),
		dirHistory:        []string{currentDir},
		historyIndex:      0,
		recursiveSearch:   false,
		loading:           false,
		searchMatches:     [][]int{},
		clipboard:         []string{},
		clipboardOp:       opNone,
		sortBy:            sortByName,
		sortMenuCursor:    0,
		dualPane:          false,
		activePane:        0,
		rightDir:          currentDir,
		rightFiles:        []fileItem{},
		rightCursor:       0,
		rightScrollOffset: 0,
		permissions:       false,
	}

	m.loadFiles()
	return m
}

func (m *model) loadFiles() {
	entries, err := os.ReadDir(m.currentDir)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error reading directory: %v", err)
		m.statusExpiry = time.Now().Add(3 * time.Second)
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
		if shouldIgnore(entry.Name()) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		item := fileItem{
			path:    filepath.Join(m.currentDir, entry.Name()),
			name:    entry.Name(),
			isDir:   entry.IsDir(),
			size:    info.Size(),
			modTime: info.ModTime(),
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

func (m *model) deleteSelectedFile() error {
	if len(m.filteredFiles) == 0 || m.cursor >= len(m.filteredFiles) {
		return fmt.Errorf("no file selected")
	}

	selected := m.filteredFiles[m.cursor]
	if selected.name == ".." {
		return fmt.Errorf("cannot delete parent directory")
	}

	// Try to move to trash first, fall back to permanent delete
	if err := m.moveToTrash(selected.path); err != nil {
		// Permanent delete
		if selected.isDir {
			return os.RemoveAll(selected.path)
		}
		return os.Remove(selected.path)
	}
	return nil
}

func (m *model) moveToTrash(path string) error {
	// Try using trash-cli or gio trash on Linux
	if commandExists("gio") {
		cmd := exec.Command("gio", "trash", path)
		return cmd.Run()
	}
	if commandExists("trash-put") {
		cmd := exec.Command("trash-put", path)
		return cmd.Run()
	}
	return fmt.Errorf("trash command not available")
}

func (m *model) renameFile(oldPath, newName string) error {
	newPath := filepath.Join(filepath.Dir(oldPath), newName)
	return os.Rename(oldPath, newPath)
}

func (m *model) createFile(name string) error {
	path := filepath.Join(m.currentDir, name)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	return file.Close()
}

func (m *model) createDir(name string) error {
	path := filepath.Join(m.currentDir, name)
	return os.Mkdir(path, 0755)
}

func (m *model) copyFiles() error {
	for _, srcPath := range m.clipboard {
		destPath := filepath.Join(m.currentDir, filepath.Base(srcPath))
		if err := m.copyFileOrDir(srcPath, destPath); err != nil {
			return err
		}
	}
	return nil
}

func (m *model) cutFiles() error {
	for _, srcPath := range m.clipboard {
		destPath := filepath.Join(m.currentDir, filepath.Base(srcPath))
		if err := os.Rename(srcPath, destPath); err != nil {
			// If rename fails (cross-device), copy then delete
			if err := m.copyFileOrDir(srcPath, destPath); err != nil {
				return err
			}
			if err := os.RemoveAll(srcPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *model) copyFileOrDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return m.copyDir(src, dst)
	}
	return m.copyFile(src, dst)
}

func (m *model) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := os.ReadFile(src); err != nil {
		return err
	}

	srcBytes, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, srcBytes, 0644)
}

func (m *model) copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := m.copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := m.copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *model) toggleSelection() {
	if len(m.filteredFiles) == 0 || m.cursor >= len(m.filteredFiles) {
		return
	}

	selected := &m.filteredFiles[m.cursor]
	selected.selected = !selected.selected

	// Also update in main files list
	for i := range m.files {
		if m.files[i].path == selected.path {
			m.files[i].selected = selected.selected
			break
		}
	}
}

func (m *model) getSelectedFiles() []string {
	var selected []string
	for _, file := range m.files {
		if file.selected && file.name != ".." {
			selected = append(selected, file.path)
		}
	}
	return selected
}

func (m *model) clearSelections() {
	for i := range m.files {
		m.files[i].selected = false
	}
	for i := range m.filteredFiles {
		m.filteredFiles[i].selected = false
	}
}

func (m *model) searchFileContent(query string) error {
	if !commandExists("rg") {
		return fmt.Errorf("ripgrep (rg) not found - install it for content search")
	}

	// Run ripgrep
	cmd := exec.Command("rg", "--line-number", "--column", "--no-heading", "--color=never", query, m.currentDir)
	output, err := cmd.Output()
	if err != nil {
		// rg returns exit code 1 if no matches found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			m.contentSearchResults = []contentSearchResult{}
			return nil
		}
		return err
	}

	// Parse results
	m.contentSearchResults = []contentSearchResult{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Format: file:line:column:content
		parts := strings.SplitN(line, ":", 4)
		if len(parts) >= 4 {
			lineNum := 0
			colNum := 0
			fmt.Sscanf(parts[1], "%d", &lineNum)
			fmt.Sscanf(parts[2], "%d", &colNum)

			result := contentSearchResult{
				file:    parts[0],
				line:    lineNum,
				column:  colNum,
				content: parts[3],
			}
			m.contentSearchResults = append(m.contentSearchResults, result)
		}
	}

	return nil
}

func (m *model) updateFilter() {
	query := m.searchInput.Value()
	if query == "" {
		m.filteredFiles = m.files
		m.searchMatches = [][]int{}
		return
	}

	if m.recursiveSearch {
		// Recursive search across entire project
		m.recursiveSearchFiles(query)
	} else {
		// Search in current directory only
		m.searchCurrentDir(query)
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

	// Use fuzzy library for better matching
	matches := fuzzy.Find(query, names)

	m.filteredFiles = []fileItem{}
	m.searchMatches = [][]int{}

	for _, match := range matches {
		m.filteredFiles = append(m.filteredFiles, m.files[match.Index])
		m.searchMatches = append(m.searchMatches, match.MatchedIndexes)
	}
}

func (m *model) recursiveSearchFiles(query string) {
	// Collect all files recursively
	var allFiles []fileItem
	var allNames []string

	filepath.WalkDir(m.currentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden files if not showing them
		if !m.showHidden && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip ignored patterns
		if shouldIgnore(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path for display
		relPath, _ := filepath.Rel(m.currentDir, path)
		if relPath == "." {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		allFiles = append(allFiles, fileItem{
			path:    path,
			name:    relPath,
			isDir:   d.IsDir(),
			size:    info.Size(),
			modTime: info.ModTime(),
		})
		allNames = append(allNames, relPath)

		return nil
	})

	// Use fuzzy matching
	matches := fuzzy.Find(query, allNames)

	m.filteredFiles = []fileItem{}
	m.searchMatches = [][]int{}

	for _, match := range matches {
		if match.Index < len(allFiles) {
			m.filteredFiles = append(m.filteredFiles, allFiles[match.Index])
			m.searchMatches = append(m.searchMatches, match.MatchedIndexes)
		}
	}
}

func (m *model) updatePreview() {
	if !m.showPreview || len(m.filteredFiles) == 0 || m.cursor >= len(m.filteredFiles) {
		m.previewContent = ""
		m.previewLines = []string{}
		m.previewScroll = 0
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
}

// wrapTextToLines splits text into lines and wraps long lines to fit width
func (m *model) wrapTextToLines(text string, width int) []string {
	if width <= 0 {
		width = 50 // fallback width
	}

	var wrappedLines []string
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= width {
			wrappedLines = append(wrappedLines, line)
		} else {
			// Wrap long lines
			for len(line) > width {
				wrappedLines = append(wrappedLines, line[:width])
				line = line[width:]
			}
			if len(line) > 0 {
				wrappedLines = append(wrappedLines, line)
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
		} else if isImageFile(entry.Name()) {
			icon = "🖼️"
		} else if isCodeFile(entry.Name()) {
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
	icon := getFileIcon(filepath.Base(path))
	preview.WriteString(fmt.Sprintf("%s %s\n", icon, filepath.Base(path)))
	preview.WriteString(fmt.Sprintf("Size: %s\n", formatFileSize(info.Size())))
	preview.WriteString(fmt.Sprintf("Modified: %s\n", info.ModTime().Format("Jan 2, 2006 15:04")))

	if m.gitModified[path] {
		preview.WriteString("Git: Modified\n")
	}

	preview.WriteString("\n")

	// Don't preview binary or large files
	if info.Size() > 1024*1024 || isBinaryFile(path) {
		preview.WriteString("(Binary or large file - no preview)")
		return preview.String()
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		preview.WriteString(fmt.Sprintf("Error reading file: %v", err))
		return preview.String()
	}

	lines := strings.Split(string(content), "\n")
	maxLines := 30
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		preview.WriteString(fmt.Sprintf("%s\n...\n(Showing first %d lines)", strings.Join(lines, "\n"), maxLines))
	} else {
		preview.WriteString(string(content))
	}

	return preview.String()
}

func (m model) Init() tea.Cmd {
	return tea.SetWindowTitle("🔍 Scout - File Explorer")
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Clear expired status messages
	if m.statusMsg != "" && time.Now().After(m.statusExpiry) {
		m.statusMsg = ""
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeBookmarks:
			switch msg.String() {
			case "esc", "q":
				m.mode = modeNormal
				return m, nil
			case "j", "down":
				if m.bookmarksCursor < len(m.config.Bookmarks)-1 {
					m.bookmarksCursor++
				}
			case "k", "up":
				if m.bookmarksCursor > 0 {
					m.bookmarksCursor--
				}
			case "enter":
				if len(m.config.Bookmarks) > 0 && m.bookmarksCursor < len(m.config.Bookmarks) {
					targetPath := m.config.Bookmarks[m.bookmarksCursor]
					// Ensure target is within root path
					if m.config.RootPath == "" || strings.HasPrefix(targetPath, m.config.RootPath) {
						m.addToHistory(targetPath)
						m.currentDir = targetPath
						m.cursor = 0
						m.scrollOffset = 0
						m.previewScroll = 0
						m.mode = modeNormal
						m.loadFiles()
						m.gitModified = getGitModifiedFiles(m.currentDir)
						m.gitBranch = getGitBranch(m.currentDir)
					}
				}
				return m, nil
			case "o":
				// Open bookmark in VS Code
				if len(m.config.Bookmarks) > 0 && m.bookmarksCursor < len(m.config.Bookmarks) {
					targetPath := m.config.Bookmarks[m.bookmarksCursor]
					// Ensure target is within root path
					if m.config.RootPath == "" || strings.HasPrefix(targetPath, m.config.RootPath) {
						return m, m.openInVSCode(targetPath)
					}
				}
				return m, nil
			case "d":
				// Confirm delete bookmark
				if len(m.config.Bookmarks) > 0 && m.bookmarksCursor < len(m.config.Bookmarks) {
					m.deleteBookmarkIndex = m.bookmarksCursor
					m.mode = modeConfirmDelete
				}
			}
			return m, nil

		case modeConfirmDelete:
			switch msg.String() {
			case "y", "Y":
				// Confirmed - delete the bookmark
				if m.deleteBookmarkIndex >= 0 && m.deleteBookmarkIndex < len(m.config.Bookmarks) {
					bookmarkName := filepath.Base(m.config.Bookmarks[m.deleteBookmarkIndex])
					m.config.Bookmarks = append(m.config.Bookmarks[:m.deleteBookmarkIndex], m.config.Bookmarks[m.deleteBookmarkIndex+1:]...)
					if m.bookmarksCursor >= len(m.config.Bookmarks) && len(m.config.Bookmarks) > 0 {
						m.bookmarksCursor = len(m.config.Bookmarks) - 1
					}
					saveConfig(m.config)
					m.statusMsg = fmt.Sprintf("Deleted bookmark: %s", bookmarkName)
					m.statusExpiry = time.Now().Add(2 * time.Second)
				}
				m.mode = modeBookmarks
				return m, nil
			case "n", "N", "esc":
				// Cancelled
				m.mode = modeBookmarks
				return m, nil
			}
			return m, nil

		case modeConfirmFileDelete:
			switch msg.String() {
			case "y", "Y":
				// Confirmed - delete the file
				if err := m.deleteSelectedFile(); err != nil {
					m.statusMsg = fmt.Sprintf("Error deleting: %v", err)
					m.statusExpiry = time.Now().Add(3 * time.Second)
				} else {
					m.statusMsg = "File deleted"
					m.statusExpiry = time.Now().Add(2 * time.Second)
					m.loadFiles()
				}
				m.mode = modeNormal
				return m, nil
			case "n", "N", "esc":
				m.mode = modeNormal
				return m, nil
			}
			return m, nil

		case modeRename:
			switch msg.String() {
			case "esc":
				m.mode = modeNormal
				m.textInput.SetValue("")
				return m, nil
			case "enter":
				newName := m.textInput.Value()
				if newName != "" && len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					if err := m.renameFile(selected.path, newName); err != nil {
						m.statusMsg = fmt.Sprintf("Error renaming: %v", err)
						m.statusExpiry = time.Now().Add(3 * time.Second)
					} else {
						m.statusMsg = fmt.Sprintf("Renamed to: %s", newName)
						m.statusExpiry = time.Now().Add(2 * time.Second)
						m.loadFiles()
					}
				}
				m.mode = modeNormal
				m.textInput.SetValue("")
				return m, nil
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}

		case modeCreateFile:
			switch msg.String() {
			case "esc":
				m.mode = modeNormal
				m.textInput.SetValue("")
				return m, nil
			case "enter":
				name := m.textInput.Value()
				if name != "" {
					if err := m.createFile(name); err != nil {
						m.statusMsg = fmt.Sprintf("Error creating file: %v", err)
						m.statusExpiry = time.Now().Add(3 * time.Second)
					} else {
						m.statusMsg = fmt.Sprintf("Created file: %s", name)
						m.statusExpiry = time.Now().Add(2 * time.Second)
						m.loadFiles()
					}
				}
				m.mode = modeNormal
				m.textInput.SetValue("")
				return m, nil
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}

		case modeCreateDir:
			switch msg.String() {
			case "esc":
				m.mode = modeNormal
				m.textInput.SetValue("")
				return m, nil
			case "enter":
				name := m.textInput.Value()
				if name != "" {
					if err := m.createDir(name); err != nil {
						m.statusMsg = fmt.Sprintf("Error creating directory: %v", err)
						m.statusExpiry = time.Now().Add(3 * time.Second)
					} else {
						m.statusMsg = fmt.Sprintf("Created directory: %s", name)
						m.statusExpiry = time.Now().Add(2 * time.Second)
						m.loadFiles()
					}
				}
				m.mode = modeNormal
				m.textInput.SetValue("")
				return m, nil
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}

		case modeSortMenu:
			switch msg.String() {
			case "esc", "q":
				m.mode = modeNormal
				return m, nil
			case "j", "down":
				if m.sortMenuCursor < 3 {
					m.sortMenuCursor++
				}
			case "k", "up":
				if m.sortMenuCursor > 0 {
					m.sortMenuCursor--
				}
			case "enter":
				m.sortBy = sortMode(m.sortMenuCursor)
				m.sortFiles()
				m.filteredFiles = m.files
				m.mode = modeNormal
				sortNames := []string{"Name", "Size", "Date", "Type"}
				m.statusMsg = fmt.Sprintf("Sorted by: %s", sortNames[m.sortBy])
				m.statusExpiry = time.Now().Add(2 * time.Second)
				return m, nil
			}
			return m, nil

		case modeHelp:
			switch msg.String() {
			case "esc", "q", "?":
				m.mode = modeNormal
				return m, nil
			}
			return m, nil

		case modeContentSearch:
			switch msg.String() {
			case "esc":
				m.mode = modeNormal
				m.textInput.SetValue("")
				m.contentSearchResults = []contentSearchResult{}
				return m, nil
			case "enter":
				query := m.textInput.Value()
				if query != "" {
					if err := m.searchFileContent(query); err != nil {
						m.statusMsg = fmt.Sprintf("Search error: %v", err)
						m.statusExpiry = time.Now().Add(3 * time.Second)
						m.mode = modeNormal
					} else {
						m.statusMsg = fmt.Sprintf("Found %d results", len(m.contentSearchResults))
						m.statusExpiry = time.Now().Add(2 * time.Second)
						m.contentSearchCursor = 0
						// Stay in content search mode to show results
					}
				}
				m.textInput.SetValue("")
				return m, nil
			case "j", "down":
				// Navigate results
				if len(m.contentSearchResults) > 0 && m.contentSearchCursor < len(m.contentSearchResults)-1 {
					m.contentSearchCursor++
				}
			case "k", "up":
				// Navigate results
				if m.contentSearchCursor > 0 {
					m.contentSearchCursor--
				}
			case "o":
				// Open file at result location
				if len(m.contentSearchResults) > 0 && m.contentSearchCursor < len(m.contentSearchResults) {
					result := m.contentSearchResults[m.contentSearchCursor]
					return m, m.editFile(filepath.Join(m.currentDir, result.file))
				}
			default:
				m.textInput, cmd = m.textInput.Update(msg)
				return m, cmd
			}
			return m, nil

		case modeSearch:
			switch msg.String() {
			case "esc":
				m.mode = modeNormal
				m.searchInput.SetValue("")
				m.filteredFiles = m.files
				m.searchMatches = [][]int{}
				m.recursiveSearch = false
				m.updatePreview()
				return m, nil
			case "enter":
				m.mode = modeNormal
				return m, nil
			case "ctrl+r":
				// Toggle recursive search
				m.recursiveSearch = !m.recursiveSearch
				m.updateFilter()
				m.updatePreview()
				if m.recursiveSearch {
					m.statusMsg = "Recursive search enabled"
				} else {
					m.statusMsg = "Current directory search"
				}
				m.statusExpiry = time.Now().Add(2 * time.Second)
				return m, nil
			case "s", "alt+down":
				// Scroll preview down
				if m.showPreview && len(m.previewLines) > 0 {
					availableHeight := m.height - 9
					if availableHeight < 1 {
						availableHeight = 3
					}
					contentHeight := availableHeight
					if m.previewScroll < len(m.previewLines)-contentHeight {
						m.previewScroll++
					}
				}
			case "w", "alt+up":
				// Scroll preview up
				if m.showPreview && m.previewScroll > 0 {
					m.previewScroll--
				}
			default:
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.updateFilter()
				m.updatePreview()
				return m, cmd
			}

		case modeNormal:
			keyStr := msg.String()
			// Debug: show what key was pressed for alt combinations
			if strings.HasPrefix(keyStr, "alt") {
				m.statusMsg = fmt.Sprintf("Key: %q", keyStr)
				m.statusExpiry = time.Now().Add(3 * time.Second)
			}

			switch keyStr {
			case "q", "ctrl+c":
				return m, tea.Quit

			case "j", "down":
				if m.cursor < len(m.filteredFiles)-1 {
					m.cursor++
					m.updatePreview()
				}

			case "k", "up":
				if m.cursor > 0 {
					m.cursor--
					m.updatePreview()
				}

			case "ctrl+d":
				// Half-page down
				pageSize := (m.height - 9) / 2
				if pageSize < 1 {
					pageSize = 5
				}
				m.cursor += pageSize
				if m.cursor >= len(m.filteredFiles) {
					m.cursor = len(m.filteredFiles) - 1
				}
				if m.cursor < 0 {
					m.cursor = 0
				}
				m.updatePreview()

			case "ctrl+u":
				// Half-page up
				pageSize := (m.height - 9) / 2
				if pageSize < 1 {
					pageSize = 5
				}
				m.cursor -= pageSize
				if m.cursor < 0 {
					m.cursor = 0
				}
				m.updatePreview()

			case "ctrl+f":
				// Full-page down
				pageSize := m.height - 9
				if pageSize < 1 {
					pageSize = 10
				}
				m.cursor += pageSize
				if m.cursor >= len(m.filteredFiles) {
					m.cursor = len(m.filteredFiles) - 1
				}
				if m.cursor < 0 {
					m.cursor = 0
				}
				m.updatePreview()

			case "ctrl+b":
				// Full-page up
				pageSize := m.height - 9
				if pageSize < 1 {
					pageSize = 10
				}
				m.cursor -= pageSize
				if m.cursor < 0 {
					m.cursor = 0
				}
				m.updatePreview()

			case "s", "alt+down":
				// Scroll preview down
				if m.showPreview && len(m.previewLines) > 0 {
					availableHeight := m.height - 9
					if availableHeight < 3 {
						availableHeight = 3
					}
					contentHeight := availableHeight - 2 // Reserve space for scroll indicators
					if contentHeight < 1 {
						contentHeight = 1
					}
					if m.previewScroll < len(m.previewLines)-contentHeight {
						m.previewScroll++
					}
				}

			case "w", "alt+up":
				// Scroll preview up
				if m.showPreview && m.previewScroll > 0 {
					m.previewScroll--
				}

			case "g":
				m.cursor = 0
				m.updatePreview()

			case "G":
				if len(m.filteredFiles) > 0 {
					m.cursor = len(m.filteredFiles) - 1
					m.updatePreview()
				}

			case "enter", "l", "right":
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					if selected.isDir {
						m.addToHistory(selected.path)
						m.currentDir = selected.path
						m.cursor = 0
						m.scrollOffset = 0
						m.previewScroll = 0
						m.loadFiles()
						m.gitModified = getGitModifiedFiles(m.currentDir)
						m.gitBranch = getGitBranch(m.currentDir)
					} else {
						return m, m.openFile(selected.path)
					}
				}

			case "esc", "h", "left":
				parentDir := filepath.Dir(m.currentDir)
				// Check if we can go up (respect root path and filesystem root)
				if m.currentDir != "/" && m.currentDir != m.config.RootPath &&
					(m.config.RootPath == "" || strings.HasPrefix(parentDir, m.config.RootPath)) {
					m.addToHistory(parentDir)
					m.currentDir = parentDir
					m.cursor = 0
					m.scrollOffset = 0
					m.previewScroll = 0
					m.loadFiles()
					m.gitModified = getGitModifiedFiles(m.currentDir)
					m.gitBranch = getGitBranch(m.currentDir)
				}

			case "/":
				m.mode = modeSearch
				m.searchInput.Focus()
				return m, textinput.Blink

			case ".":
				m.showHidden = !m.showHidden
				m.loadFiles()
				if m.showHidden {
					m.statusMsg = "Showing hidden files"
				} else {
					m.statusMsg = "Hiding hidden files"
				}
				m.statusExpiry = time.Now().Add(2 * time.Second)

			case "p":
				m.showPreview = !m.showPreview
				if m.showPreview {
					m.statusMsg = "Preview enabled"
					m.updatePreview()
				} else {
					m.statusMsg = "Preview disabled"
					m.previewContent = ""
				}
				m.statusExpiry = time.Now().Add(2 * time.Second)

			case "y":
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					m.copyPath(selected.path)
				}

			case "e":
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					if !selected.isDir {
						return m, m.editFile(selected.path)
					}
				}

			case "o":
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					if selected.isDir && selected.name != ".." {
						return m, m.openInVSCode(selected.path)
					} else if !selected.isDir {
						return m, m.openFile(selected.path)
					}
				}

			case "r":
				m.loadFiles()
				m.gitModified = getGitModifiedFiles(m.currentDir)
				m.gitBranch = getGitBranch(m.currentDir)
				m.statusMsg = "Refreshed"
				m.statusExpiry = time.Now().Add(2 * time.Second)

			case "~":
				home, _ := os.UserHomeDir()
				m.addToHistory(home)
				m.currentDir = home
				m.cursor = 0
				m.scrollOffset = 0
				m.previewScroll = 0
				m.loadFiles()

			case "b":
				m.mode = modeBookmarks
				m.bookmarksCursor = 0

			case "B":
				// Add highlighted item to bookmarks (only directories)
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					if selected.isDir {
						if !contains(m.config.Bookmarks, selected.path) {
							m.config.Bookmarks = append(m.config.Bookmarks, selected.path)
							saveConfig(m.config)
							m.statusMsg = fmt.Sprintf("Bookmark added: %s", selected.name)
							m.statusExpiry = time.Now().Add(2 * time.Second)
						} else {
							m.statusMsg = "Already bookmarked"
							m.statusExpiry = time.Now().Add(2 * time.Second)
						}
					} else {
						m.statusMsg = "Can only bookmark directories"
						m.statusExpiry = time.Now().Add(2 * time.Second)
					}
				}

			// File operations
			case "D":
				// Delete file/directory
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					if selected.name != ".." {
						m.mode = modeConfirmFileDelete
					}
				}

			case "R":
				// Rename file/directory
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					if selected.name != ".." {
						m.mode = modeRename
						m.textInput.SetValue(selected.name)
						m.textInput.Focus()
						return m, textinput.Blink
					}
				}

			case "n":
				// Next key will determine action (nf = new file, nd = new dir)
				// For simplicity, let's make 'n' followed by 'f' or 'd'
				// We'll use a simple approach: N for new file, M for new directory
				return m, nil

			case "N":
				// Create new file
				m.mode = modeCreateFile
				m.textInput.SetValue("")
				m.textInput.Placeholder = "Enter filename..."
				m.textInput.Focus()
				return m, textinput.Blink

			case "M":
				// Create new directory
				m.mode = modeCreateDir
				m.textInput.SetValue("")
				m.textInput.Placeholder = "Enter directory name..."
				m.textInput.Focus()
				return m, textinput.Blink

			case "c":
				// Copy - add selected files to clipboard
				selected := m.getSelectedFiles()
				if len(selected) == 0 && len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					// If nothing selected, copy current file
					current := m.filteredFiles[m.cursor]
					if current.name != ".." {
						selected = []string{current.path}
					}
				}
				if len(selected) > 0 {
					m.clipboard = selected
					m.clipboardOp = opCopy
					m.statusMsg = fmt.Sprintf("Copied %d item(s)", len(selected))
					m.statusExpiry = time.Now().Add(2 * time.Second)
				}

			case "x":
				// Cut - add selected files to clipboard
				selected := m.getSelectedFiles()
				if len(selected) == 0 && len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					// If nothing selected, cut current file
					current := m.filteredFiles[m.cursor]
					if current.name != ".." {
						selected = []string{current.path}
					}
				}
				if len(selected) > 0 {
					m.clipboard = selected
					m.clipboardOp = opCut
					m.statusMsg = fmt.Sprintf("Cut %d item(s)", len(selected))
					m.statusExpiry = time.Now().Add(2 * time.Second)
				}

			case "P":
				// Paste files from clipboard
				if len(m.clipboard) > 0 {
					var err error
					if m.clipboardOp == opCopy {
						err = m.copyFiles()
					} else if m.clipboardOp == opCut {
						err = m.cutFiles()
						if err == nil {
							m.clipboard = []string{}
							m.clipboardOp = opNone
						}
					}
					if err != nil {
						m.statusMsg = fmt.Sprintf("Error pasting: %v", err)
						m.statusExpiry = time.Now().Add(3 * time.Second)
					} else {
						m.statusMsg = "Pasted successfully"
						m.statusExpiry = time.Now().Add(2 * time.Second)
						m.loadFiles()
					}
				}

			case " ":
				// Toggle selection
				m.toggleSelection()
				// Move cursor down
				if m.cursor < len(m.filteredFiles)-1 {
					m.cursor++
					m.updatePreview()
				}

			case "S":
				// Open sort menu
				m.mode = modeSortMenu
				m.sortMenuCursor = int(m.sortBy)

			case "T":
				// Toggle dual pane mode
				m.dualPane = !m.dualPane
				if m.dualPane {
					m.statusMsg = "Dual pane mode enabled"
				} else {
					m.statusMsg = "Dual pane mode disabled"
				}
				m.statusExpiry = time.Now().Add(2 * time.Second)

			case "tab":
				// Switch active pane in dual mode
				if m.dualPane {
					m.activePane = 1 - m.activePane
				}

			case "?":
				// Show help screen
				m.mode = modeHelp

			case "I":
				// Toggle permissions display
				m.permissions = !m.permissions
				if m.permissions {
					m.statusMsg = "Showing permissions"
				} else {
					m.statusMsg = "Hiding permissions"
				}
				m.statusExpiry = time.Now().Add(2 * time.Second)

			case "ctrl+g":
				// Content search (ripgrep)
				m.mode = modeContentSearch
				m.textInput.SetValue("")
				m.textInput.Placeholder = "Search file contents..."
				m.textInput.Focus()
				return m, textinput.Blink
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var content string

	// Header
	header := m.renderHeader()

	// Main content area
	var mainContent string
	switch m.mode {
	case modeBookmarks:
		mainContent = m.renderBookmarksView()
	case modeConfirmDelete:
		mainContent = m.renderConfirmDeleteView()
	case modeConfirmFileDelete:
		mainContent = m.renderConfirmFileDeleteView()
	case modeRename:
		mainContent = m.renderRenameDialog()
	case modeCreateFile:
		mainContent = m.renderCreateFileDialog()
	case modeCreateDir:
		mainContent = m.renderCreateDirDialog()
	case modeSortMenu:
		mainContent = m.renderSortMenu()
	case modeContentSearch:
		mainContent = m.renderContentSearchView()
	case modeHelp:
		mainContent = m.renderHelpView()
	default:
		if m.dualPane {
			// Dual pane mode
			leftPane := m.renderFileList(m.width / 2)
			// For now, right pane shows same directory
			// TODO: Implement independent right pane navigation
			rightPane := m.renderFileList(m.width / 2)
			mainContent = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
		} else if m.showPreview {
			// Split view with preview
			fileList := m.renderFileList(m.width / 2)
			preview := m.renderPreview(m.width / 2)
			mainContent = lipgloss.JoinHorizontal(lipgloss.Top, fileList, preview)
		} else {
			// Full width file list
			mainContent = m.renderFileList(m.width)
		}
	}

	// Status bar
	statusBar := m.renderStatusBar()

	// Combine all sections
	content = lipgloss.JoinVertical(lipgloss.Left,
		header,
		mainContent,
		statusBar,
	)

	return content
}

func (m model) renderHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("99")).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Width(m.width)

	var title string
	if m.mode == modeBookmarks {
		title = "🔍 Scout - Bookmarks (ESC to exit)"
	} else {
		title = fmt.Sprintf("🔍 Scout - %s", m.currentDir)
	}

	if m.mode == modeSearch {
		// Build search text with mode indicator and hint
		searchLabel := "Search: "
		hint := " (ctrl+r)"

		if m.recursiveSearch {
			searchLabel = "Search [RECURSIVE]: "
		}

		// Get the raw search value (not View() which has its own styling)
		searchValue := m.searchInput.Value()

		// Show cursor position with a visible indicator
		cursorPos := m.searchInput.Position()
		displayValue := searchValue
		if len(displayValue) == 0 {
			displayValue = "_" // Show cursor when empty
		} else if cursorPos < len(displayValue) {
			// Insert cursor indicator at position
			displayValue = displayValue[:cursorPos] + "|" + displayValue[cursorPos:]
		} else {
			displayValue = displayValue + "|"
		}

		// Construct the full search text
		searchText := searchLabel + displayValue + hint

		// Calculate available width for title section
		searchTextLen := lipgloss.Width(searchText)
		titleWidth := m.width - searchTextLen - 2
		if titleWidth < 20 {
			titleWidth = 20
			searchTextLen = m.width - titleWidth - 2
		}

		// Apply consistent background to both sections
		baseStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252"))

		searchStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("226"))

		titlePart := baseStyle.Width(titleWidth).Padding(0, 1).Render(title)
		searchPart := searchStyle.Width(searchTextLen).Render(searchText)

		// Join with full background
		title = lipgloss.JoinHorizontal(lipgloss.Top, titlePart, searchPart)
	}

	// Return with or without additional styling
	if m.mode == modeSearch {
		// Already has background applied, return with full width
		return lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Width(m.width).
			Render(title)
	}
	return titleStyle.Render(title)
}

func (m model) renderFileList(width int) string {
	// Use same height calculation as preview for consistency
	availableHeight := m.height - 9
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Sticky header (outside the scrollable area)
	dirName := filepath.Base(m.currentDir)
	if dirName == "" || dirName == "." {
		dirName = m.currentDir
	}
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("99")).
		Bold(true).
		Width(width - 4).
		Padding(0, 1)

	hiddenIndicator := ""
	if m.showHidden {
		hiddenIndicator = "  •  👁️ Show hidden"
	}
	header := headerStyle.Render(fmt.Sprintf("📁 %s  •  Items: %d%s", dirName, len(m.filteredFiles), hiddenIndicator))

	// Scrollable file list (reduced height for header)
	listHeight := availableHeight - 1 // Reserve 1 line for header
	listStyle := lipgloss.NewStyle().
		BorderForeground(lipgloss.Color("240")).
		Width(width-2).
		Padding(0, 1)

	var items []string

	// Calculate visible range
	visibleHeight := listHeight
	startIdx := m.scrollOffset
	endIdx := startIdx + visibleHeight

	if endIdx > len(m.filteredFiles) {
		endIdx = len(m.filteredFiles)
	}

	// Adjust scroll offset if cursor is out of view
	if m.cursor < startIdx {
		m.scrollOffset = m.cursor
		startIdx = m.cursor
		endIdx = startIdx + visibleHeight
		if endIdx > len(m.filteredFiles) {
			endIdx = len(m.filteredFiles)
		}
	} else if m.cursor >= endIdx {
		endIdx = m.cursor + 1
		startIdx = endIdx - visibleHeight
		if startIdx < 0 {
			startIdx = 0
		}
		m.scrollOffset = startIdx
	}

	for i := startIdx; i < endIdx && i < len(m.filteredFiles); i++ {
		item := m.filteredFiles[i]

		// Selection checkbox
		checkbox := "  "
		if item.selected {
			checkbox = "✓ "
		}

		// Icon
		icon := "📄"
		if item.isDir {
			if item.name == ".." {
				icon = "⬆️"
			} else {
				icon = "📁"
			}
		} else {
			icon = getFileIcon(item.name)
		}

		// Git status
		gitStatus := ""
		if m.gitModified[item.path] {
			modifiedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
			gitStatus = " " + modifiedStyle.Render("[M]")
		}

		// Format item with highlighting if in search mode
		name := item.name
		displayName := name

		// Apply search highlighting if we have match positions
		if m.mode == modeSearch && i < len(m.searchMatches) && len(m.searchMatches[i]) > 0 {
			displayName = highlightMatches(name, m.searchMatches[i])
		}

		// Add file size for files
		sizeStr := ""
		if !item.isDir && item.name != ".." {
			sizeStr = " " + formatFileSize(item.size)
		}

		// Truncate name if needed
		maxNameLen := width - 25 // Account for checkbox, icon, size, etc
		if maxNameLen < 10 {
			maxNameLen = 10
		}
		if len(name) > maxNameLen {
			displayName = displayName[:min(len(displayName), maxNameLen-3)] + "..."
		}

		line := fmt.Sprintf("%s%s %s%s%s", checkbox, icon, displayName, sizeStr, gitStatus)

		// Style based on selection
		if i == m.cursor {
			selectedStyle := lipgloss.NewStyle().
				Background(lipgloss.Color("57")).
				Foreground(lipgloss.Color("230")).
				Width(width - 4)
			line = selectedStyle.Render(line)
		} else {
			normalStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Width(width - 4)
			line = normalStyle.Render(line)
		}

		items = append(items, line)
	}

	// Add scroll indicators
	if startIdx > 0 {
		items = append([]string{"▲ More files above..."}, items...)
	}
	if endIdx < len(m.filteredFiles) {
		items = append(items, "▼ More files below...")
	}

	fileList := listStyle.Render(strings.Join(items, "\n"))

	// Combine header and file list with border
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(width - 2).
		Height(availableHeight + 2)

	combined := header + "\n" + fileList
	return borderStyle.Render(combined)
}

func (m model) renderPreview(width int) string {
	availableHeight := m.height - 9
	if availableHeight < 3 {
		availableHeight = 3
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(width-2).
		Height(availableHeight+2)

	if len(m.previewLines) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			Padding(0, 1)
		return borderStyle.Render(emptyStyle.Render("No preview available"))
	}

	// Extract sticky header (first 2-4 lines that contain metadata)
	headerLines := []string{}
	contentStartIdx := 0
	for i, line := range m.previewLines {
		// Header ends at first empty line
		if strings.TrimSpace(line) == "" {
			contentStartIdx = i + 1
			break
		}
		headerLines = append(headerLines, line)
		if i >= 5 { // Max 6 lines for header
			contentStartIdx = i + 1
			break
		}
	}

	// Render sticky header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("99")).
		Bold(true).
		Width(width - 4).
		Padding(0, 1)
	header := headerStyle.Render(strings.Join(headerLines, "\n"))

	// Calculate scrollable content area
	headerLineCount := len(headerLines)
	scrollableHeight := availableHeight - headerLineCount
	if scrollableHeight < 3 {
		scrollableHeight = 3
	}

	// Get scrollable content lines (skip header)
	contentLines := m.previewLines[contentStartIdx:]

	startLine := m.previewScroll
	endLine := startLine + scrollableHeight
	if endLine > len(contentLines) {
		endLine = len(contentLines)
	}
	if startLine >= len(contentLines) {
		startLine = 0
		endLine = scrollableHeight
		if endLine > len(contentLines) {
			endLine = len(contentLines)
		}
	}

	// Build scrollable content
	var content []string
	if startLine > 0 {
		content = append(content, "▲ w")
	}

	if startLine < len(contentLines) {
		content = append(content, contentLines[startLine:endLine]...)
	}

	if endLine < len(contentLines) {
		content = append(content, "▼ s")
	}

	contentStyle := lipgloss.NewStyle().
		Width(width - 4).
		Padding(0, 1)
	scrollContent := contentStyle.Render(strings.Join(content, "\n"))

	// Combine header and scrollable content
	combined := header + "\n" + scrollContent
	return borderStyle.Render(combined)
}

func (m model) renderBookmarksView() string {
	// Calculate available height
	availableHeight := m.height - 9
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Full width bookmarks panel
	bookmarksStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("99")).
		Width(m.width - 2).
		Height(availableHeight + 2).
		Padding(1)

	// Render bookmarks
	var bookmarkItems []string
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	headerText := fmt.Sprintf("📌 Bookmarks - Sorted by Frecency (%s navigate, %s go, %s vscode, %s delete)",
		keyStyle.Render("↑↓:"),
		keyStyle.Render("enter:"),
		keyStyle.Render("o:"),
		keyStyle.Render("d:"))
	bookmarkItems = append(bookmarkItems, headerText)
	bookmarkItems = append(bookmarkItems, "")

	if len(m.config.Bookmarks) == 0 {
		bookmarkItems = append(bookmarkItems, "No bookmarks yet. Navigate to a directory and press 'B' to bookmark it.")
	} else {
		// Create sorted bookmarks by frecency
		type bookmarkScore struct {
			path  string
			score int
		}
		sortedBookmarks := make([]bookmarkScore, len(m.config.Bookmarks))
		for i, bookmark := range m.config.Bookmarks {
			score := m.config.Frecency[bookmark]
			sortedBookmarks[i] = bookmarkScore{path: bookmark, score: score}
		}
		// Sort by score descending
		sort.Slice(sortedBookmarks, func(i, j int) bool {
			return sortedBookmarks[i].score > sortedBookmarks[j].score
		})

		for i, bs := range sortedBookmarks {
			icon := "📁"
			name := filepath.Base(bs.path)
			if name == "" || name == "." {
				name = bs.path
			}

			// Show full path relative to root if possible
			displayPath := bs.path
			if m.config.RootPath != "" && strings.HasPrefix(bs.path, m.config.RootPath) {
				rel, err := filepath.Rel(m.config.RootPath, bs.path)
				if err == nil && rel != "." {
					displayPath = "~/" + rel
				} else if rel == "." {
					displayPath = "~"
				}
			}

			// Show frecency score
			frecencyInfo := ""
			if bs.score > 0 {
				frecencyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
				frecencyInfo = " " + frecencyStyle.Render(fmt.Sprintf("[%d visits]", bs.score))
			}

			line := fmt.Sprintf("%s %s (%s)%s", icon, name, displayPath, frecencyInfo)
			if i == m.bookmarksCursor {
				selectedStyle := lipgloss.NewStyle().
					Background(lipgloss.Color("57")).
					Foreground(lipgloss.Color("230")).
					Width(m.width - 6)
				line = selectedStyle.Render(line)
			}
			bookmarkItems = append(bookmarkItems, line)
		}
	}

	return bookmarksStyle.Render(strings.Join(bookmarkItems, "\n"))
}

func (m model) renderConfirmDeleteView() string {
	// Create a centered dialog box
	dialogWidth := 60
	dialogHeight := 8

	if m.deleteBookmarkIndex < 0 || m.deleteBookmarkIndex >= len(m.config.Bookmarks) {
		return "Error: Invalid bookmark"
	}

	bookmark := m.config.Bookmarks[m.deleteBookmarkIndex]
	bookmarkName := filepath.Base(bookmark)

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("196")). // Red border
		Width(dialogWidth).
		Height(dialogHeight).
		Padding(1).
		Align(lipgloss.Center)

	// Dialog content
	var dialogContent []string
	dialogContent = append(dialogContent, "⚠️  Confirm Delete Bookmark")
	dialogContent = append(dialogContent, "")
	dialogContent = append(dialogContent, fmt.Sprintf("Delete bookmark: %s", bookmarkName))
	dialogContent = append(dialogContent, fmt.Sprintf("Path: %s", bookmark))
	dialogContent = append(dialogContent, "")

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	confirmText := fmt.Sprintf("%s Yes  %s No  %s Cancel",
		keyStyle.Render("y:"),
		keyStyle.Render("n:"),
		keyStyle.Render("esc:"))
	dialogContent = append(dialogContent, confirmText)

	dialog := dialogStyle.Render(strings.Join(dialogContent, "\n"))

	// Center the dialog on screen
	dialogBox := lipgloss.Place(m.width, m.height-7, lipgloss.Center, lipgloss.Center, dialog)

	return dialogBox
}

func (m model) renderStatusBar() string {
	statusStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("252")).
		Width(m.width).
		Padding(0, 1)

	// Left side - file info or bookmark info
	var leftInfo string
	if m.mode == modeBookmarks {
		// Show selected bookmark info
		if len(m.config.Bookmarks) > 0 && m.bookmarksCursor < len(m.config.Bookmarks) {
			bookmark := m.config.Bookmarks[m.bookmarksCursor]
			bookmarkName := filepath.Base(bookmark)
			if bookmarkName == "" || bookmarkName == "." {
				bookmarkName = bookmark
			}
			leftInfo = fmt.Sprintf("📁 %s → %s", bookmarkName, bookmark)
		}
	} else if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
		current := m.filteredFiles[m.cursor]
		if current.isDir {
			leftInfo = fmt.Sprintf("📁 %s", current.name)
		} else {
			leftInfo = fmt.Sprintf("📄 %s (%s)", current.name, formatFileSize(current.size))
		}
	}

	// Center - status message
	center := m.statusMsg

	// Right side - help
	var help string
	var helpPlainText string
	if m.mode == modeBookmarks {
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
		help = fmt.Sprintf("%s navigate • %s go • %s vscode • %s delete • %s exit",
			keyStyle.Render("↑↓:"),
			keyStyle.Render("enter:"),
			keyStyle.Render("o:"),
			keyStyle.Render("d:"),
			keyStyle.Render("esc:"))
		helpPlainText = "↑↓: navigate • enter: go • o: vscode • d: delete • esc: exit"
	} else {
		// Two-row footer with right-aligned hints
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)

		// Row 1 hints
		row1Hints := fmt.Sprintf("%s nav • %s search • %s open • %s edit • %s vscode • %s rename • %s delete • %s new",
			keyStyle.Render("↑↓:"),
			keyStyle.Render("/:"),
			keyStyle.Render("enter:"),
			keyStyle.Render("e:"),
			keyStyle.Render("o:"),
			keyStyle.Render("R:"),
			keyStyle.Render("D:"),
			keyStyle.Render("N/M:"))
		row1HintsPlain := "↑↓: nav • /: search • enter: open • e: edit • o: vscode • R: rename • D: delete • N/M: new"

		// Row 2 hints
		row2Hints := fmt.Sprintf("%s copy/cut • %s paste • %s select • %s preview • %s bookmarks • %s sort • %s help",
			keyStyle.Render("c/x:"),
			keyStyle.Render("P:"),
			keyStyle.Render("space:"),
			keyStyle.Render("p:"),
			keyStyle.Render("b:"),
			keyStyle.Render("S:"),
			keyStyle.Render("?:"))
		row2HintsPlain := "c/x: copy/cut • P: paste • space: select • p: preview • b: bookmarks • S: sort • ?: help"

		// Build row 1: leftInfo + status + right-aligned hints
		leftContent := leftInfo
		if center != "" {
			leftContent += "  " + center
		}
		spacingRow1 := m.width - len(leftContent) - len(row1HintsPlain) - 2
		if spacingRow1 < 2 {
			spacingRow1 = 2
		}
		row1 := leftContent + strings.Repeat(" ", spacingRow1) + row1Hints

		// Build row 2: only right-aligned hints
		spacingRow2 := m.width - len(row2HintsPlain) - 2
		if spacingRow2 < 0 {
			spacingRow2 = 0
		}
		row2 := strings.Repeat(" ", spacingRow2) + row2Hints

		combined := row1 + "\n" + row2
		return statusStyle.Render(combined)
	}

	// Single row for bookmark mode
	padding := m.width - len(leftInfo) - len(helpPlainText) - 2
	if padding < 2 {
		padding = 2
	}
	if center != "" {
		leftInfo += "  " + center
		padding = m.width - len(leftInfo) - len(helpPlainText) - 2
		if padding < 2 {
			padding = 2
		}
	}
	combined := leftInfo + strings.Repeat(" ", padding) + help
	return statusStyle.Render(combined)
}

// Helper functions

func (m *model) openFile(path string) tea.Cmd {
	return func() tea.Msg {
		// Try to open with default application
		var cmd *exec.Cmd
		switch {
		case isCodeFile(path):
			// Try VS Code first, then fall back to other editors
			editors := []string{"code", "subl", "atom", "vim", "nano"}
			for _, editor := range editors {
				if _, err := exec.LookPath(editor); err == nil {
					cmd = exec.Command(editor, path)
					break
				}
			}
		default:
			// Use system default
			cmd = exec.Command("xdg-open", path) // Linux
			// cmd = exec.Command("open", path) // macOS
		}

		if cmd != nil {
			cmd.Start()
		}

		return nil
	}
}

func (m *model) editFile(path string) tea.Cmd {
	return func() tea.Msg {
		// Use configured editor if set, otherwise try defaults
		editors := []string{}
		if m.config.Editor != "" {
			editors = append(editors, m.config.Editor)
		}
		editors = append(editors, "code", "vim", "nano", "vi")

		for _, editor := range editors {
			if _, err := exec.LookPath(editor); err == nil {
				cmd := exec.Command(editor, path)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Run()
				break
			}
		}
		return nil
	}
}

func (m *model) openInVSCode(path string) tea.Cmd {
	return func() tea.Msg {
		// Try to open with VS Code
		if _, err := exec.LookPath("code"); err == nil {
			cmd := exec.Command("code", path)
			cmd.Start()
			m.statusMsg = fmt.Sprintf("Opening %s in VS Code", filepath.Base(path))
			m.statusExpiry = time.Now().Add(2 * time.Second)
		} else {
			m.statusMsg = "VS Code not found in PATH"
			m.statusExpiry = time.Now().Add(3 * time.Second)
		}

		return nil
	}
}

func (m *model) copyPath(path string) {
	// Use clipboard library for cross-platform support
	err := clipboard.WriteAll(path)
	if err == nil {
		m.statusMsg = fmt.Sprintf("Copied: %s", path)
		m.statusExpiry = time.Now().Add(2 * time.Second)
	} else {
		m.statusMsg = fmt.Sprintf("Failed to copy: %v", err)
		m.statusExpiry = time.Now().Add(3 * time.Second)
	}
}

func getFileIcon(name string) string {
	ext := strings.ToLower(filepath.Ext(name))

	switch ext {
	case ".go":
		return "🐹"
	case ".js", ".ts", ".jsx", ".tsx":
		return "📜"
	case ".py":
		return "🐍"
	case ".rb":
		return "💎"
	case ".java":
		return "☕"
	case ".rs":
		return "🦀"
	case ".cpp", ".c", ".h":
		return "⚙️"
	case ".html", ".htm":
		return "🌐"
	case ".css", ".scss", ".sass":
		return "🎨"
	case ".json", ".yaml", ".yml", ".toml":
		return "📋"
	case ".md", ".markdown":
		return "📝"
	case ".txt", ".log":
		return "📄"
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico":
		return "🖼️"
	case ".mp4", ".avi", ".mov", ".mkv":
		return "🎬"
	case ".mp3", ".wav", ".flac", ".ogg":
		return "🎵"
	case ".zip", ".tar", ".gz", ".rar", ".7z":
		return "📦"
	case ".pdf":
		return "📕"
	case ".doc", ".docx":
		return "📘"
	case ".xls", ".xlsx":
		return "📊"
	case ".sh", ".bash", ".zsh":
		return "🖥️"
	case ".git", ".gitignore":
		return "🔀"
	default:
		return "📄"
	}
}

func isCodeFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	codeExts := []string{
		".go", ".js", ".ts", ".jsx", ".tsx", ".py", ".rb", ".java", ".rs",
		".cpp", ".c", ".h", ".cs", ".php", ".swift", ".kt", ".scala",
		".r", ".jl", ".lua", ".dart", ".elm", ".clj", ".ex", ".exs",
	}

	for _, codeExt := range codeExts {
		if ext == codeExt {
			return true
		}
	}
	return false
}

func isImageFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	imageExts := []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp", ".bmp"}

	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}

func isBinaryFile(path string) bool {
	// Check by extension first
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := []string{
		".exe", ".dll", ".so", ".dylib", ".bin", ".dat",
		".png", ".jpg", ".jpeg", ".gif", ".ico", ".webp",
		".mp4", ".avi", ".mov", ".mkv", ".mp3", ".wav",
		".zip", ".tar", ".gz", ".rar", ".7z",
		".pdf", ".doc", ".docx", ".xls", ".xlsx",
	}

	for _, binExt := range binaryExts {
		if ext == binExt {
			return true
		}
	}

	// Could also check file content for binary data
	return false
}

func shouldIgnore(name string) bool {
	ignorePatterns := []string{
		"node_modules",
		".git",
		".svn",
		".hg",
		"__pycache__",
		".pytest_cache",
		".vscode",
		".idea",
		"target",
		"dist",
		"build",
		".DS_Store",
		"Thumbs.db",
	}

	for _, pattern := range ignorePatterns {
		if name == pattern {
			return true
		}
	}
	return false
}

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func getGitModifiedFiles(dir string) map[string]bool {
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

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func loadConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".config", "scout")
	configPath := filepath.Join(configDir, "scout-config.json")

	// Create config directory if it doesn't exist
	os.MkdirAll(configDir, 0755)

	// Default config with root path as first bookmark
	rootPath := filepath.Join(homeDir)
	defaultConfig := &Config{
		RootPath:       rootPath,
		Bookmarks:      []string{rootPath},
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
		saveConfig(defaultConfig)
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
		saveConfig(config)
	}

	return config
}

func saveConfig(config *Config) {
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

// addToHistory adds a directory to navigation history
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

// updateFrecency updates the frecency score for a directory
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
		saveConfig(m.config)
	}
}

// getGitBranch returns the current git branch name
func getGitBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// highlightMatches highlights matched characters in a string
func highlightMatches(text string, matches []int) string {
	if len(matches) == 0 {
		return text
	}

	highlightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)

	runes := []rune(text)
	var result strings.Builder
	matchMap := make(map[int]bool)

	for _, idx := range matches {
		if idx < len(runes) {
			matchMap[idx] = true
		}
	}

	for i, r := range runes {
		if matchMap[i] {
			result.WriteString(highlightStyle.Render(string(r)))
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m model) renderConfirmFileDeleteView() string {
	dialogWidth := 60
	dialogHeight := 8

	if len(m.filteredFiles) == 0 || m.cursor >= len(m.filteredFiles) {
		return "Error: No file selected"
	}

	file := m.filteredFiles[m.cursor]
	fileName := file.name

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("196")).
		Width(dialogWidth).
		Height(dialogHeight).
		Padding(1).
		Align(lipgloss.Center)

	var dialogContent []string
	dialogContent = append(dialogContent, "⚠️  Confirm Delete")
	dialogContent = append(dialogContent, "")

	fileType := "file"
	if file.isDir {
		fileType = "directory"
	}
	dialogContent = append(dialogContent, fmt.Sprintf("Delete %s: %s", fileType, fileName))
	dialogContent = append(dialogContent, "")

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	confirmText := fmt.Sprintf("%s Yes  %s No  %s Cancel",
		keyStyle.Render("y:"),
		keyStyle.Render("n:"),
		keyStyle.Render("esc:"))
	dialogContent = append(dialogContent, confirmText)

	dialog := dialogStyle.Render(strings.Join(dialogContent, "\n"))
	return lipgloss.Place(m.width, m.height-7, lipgloss.Center, lipgloss.Center, dialog)
}

func (m model) renderRenameDialog() string {
	dialogWidth := 60
	dialogHeight := 6

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("99")).
		Width(dialogWidth).
		Height(dialogHeight).
		Padding(1)

	var dialogContent []string
	dialogContent = append(dialogContent, "✏️  Rename")
	dialogContent = append(dialogContent, "")
	dialogContent = append(dialogContent, m.textInput.View())
	dialogContent = append(dialogContent, "")
	dialogContent = append(dialogContent, "Press Enter to confirm, Esc to cancel")

	dialog := dialogStyle.Render(strings.Join(dialogContent, "\n"))
	return lipgloss.Place(m.width, m.height-7, lipgloss.Center, lipgloss.Center, dialog)
}

func (m model) renderCreateFileDialog() string {
	dialogWidth := 60
	dialogHeight := 6

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("99")).
		Width(dialogWidth).
		Height(dialogHeight).
		Padding(1)

	var dialogContent []string
	dialogContent = append(dialogContent, "📄 Create New File")
	dialogContent = append(dialogContent, "")
	dialogContent = append(dialogContent, m.textInput.View())
	dialogContent = append(dialogContent, "")
	dialogContent = append(dialogContent, "Press Enter to confirm, Esc to cancel")

	dialog := dialogStyle.Render(strings.Join(dialogContent, "\n"))
	return lipgloss.Place(m.width, m.height-7, lipgloss.Center, lipgloss.Center, dialog)
}

func (m model) renderCreateDirDialog() string {
	dialogWidth := 60
	dialogHeight := 6

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("99")).
		Width(dialogWidth).
		Height(dialogHeight).
		Padding(1)

	var dialogContent []string
	dialogContent = append(dialogContent, "📁 Create New Directory")
	dialogContent = append(dialogContent, "")
	dialogContent = append(dialogContent, m.textInput.View())
	dialogContent = append(dialogContent, "")
	dialogContent = append(dialogContent, "Press Enter to confirm, Esc to cancel")

	dialog := dialogStyle.Render(strings.Join(dialogContent, "\n"))
	return lipgloss.Place(m.width, m.height-7, lipgloss.Center, lipgloss.Center, dialog)
}

func (m model) renderSortMenu() string {
	dialogWidth := 40
	dialogHeight := 10

	menuStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("99")).
		Width(dialogWidth).
		Height(dialogHeight).
		Padding(1)

	var menuItems []string
	menuItems = append(menuItems, "📊 Sort By")
	menuItems = append(menuItems, "")

	sortOptions := []string{"Name", "Size", "Date", "Type"}
	for i, option := range sortOptions {
		prefix := "  "
		if i == m.sortMenuCursor {
			prefix = "▶ "
		}
		current := ""
		if i == int(m.sortBy) {
			current = " ✓"
		}
		menuItems = append(menuItems, fmt.Sprintf("%s%s%s", prefix, option, current))
	}

	menuItems = append(menuItems, "")
	menuItems = append(menuItems, "Enter: Select • Esc: Cancel")

	menu := menuStyle.Render(strings.Join(menuItems, "\n"))
	return lipgloss.Place(m.width, m.height-7, lipgloss.Center, lipgloss.Center, menu)
}

func (m model) renderHelpView() string {
	availableHeight := m.height - 9
	if availableHeight < 3 {
		availableHeight = 3
	}

	helpStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("99")).
		Width(m.width - 4).
		Height(availableHeight + 2).
		Padding(1)

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

	var helpContent []string
	helpContent = append(helpContent, sectionStyle.Render("🔍 Scout Keyboard Shortcuts"))
	helpContent = append(helpContent, "")

	// Navigation section
	helpContent = append(helpContent, sectionStyle.Render("Navigation:"))
	helpContent = append(helpContent, fmt.Sprintf("  %s          Move up/down", keyStyle.Render("↑/↓ or j/k")))
	helpContent = append(helpContent, fmt.Sprintf("  %s         Scroll preview pane up/down", keyStyle.Render("w/s")))
	helpContent = append(helpContent, fmt.Sprintf("  %s          Jump to top/bottom", keyStyle.Render("g/G")))
	helpContent = append(helpContent, fmt.Sprintf("  %s       Half-page up/down", keyStyle.Render("ctrl+u/d")))
	helpContent = append(helpContent, fmt.Sprintf("  %s       Full-page up/down", keyStyle.Render("ctrl+b/f")))
	helpContent = append(helpContent, fmt.Sprintf("  %s         Go to home directory", keyStyle.Render("~")))
	helpContent = append(helpContent, fmt.Sprintf("  %s         Go back", keyStyle.Render("esc")))
	helpContent = append(helpContent, "")

	// File Operations section
	helpContent = append(helpContent, sectionStyle.Render("File Operations:"))
	helpContent = append(helpContent, fmt.Sprintf("  %s           Open file/directory", keyStyle.Render("enter")))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Open in VS Code", keyStyle.Render("o")))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Edit file", keyStyle.Render("e")))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Rename file/directory", keyStyle.Render("R")))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Delete file/directory", keyStyle.Render("D")))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Create new file", keyStyle.Render("N")))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Create new directory", keyStyle.Render("M")))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Refresh current view", keyStyle.Render("r")))
	helpContent = append(helpContent, "")

	// Clipboard Operations section
	helpContent = append(helpContent, sectionStyle.Render("Clipboard:"))
	helpContent = append(helpContent, fmt.Sprintf("  %s           Copy selected files", keyStyle.Render("c")))
	helpContent = append(helpContent, fmt.Sprintf("  %s           Cut selected files", keyStyle.Render("x")))
	helpContent = append(helpContent, fmt.Sprintf("  %s           Paste files", keyStyle.Render("P")))
	helpContent = append(helpContent, fmt.Sprintf("  %s           Copy path to clipboard", keyStyle.Render("y")))
	helpContent = append(helpContent, fmt.Sprintf("  %s       Toggle selection (for bulk ops)", keyStyle.Render("space")))
	helpContent = append(helpContent, "")

	// Search & Filter section
	helpContent = append(helpContent, sectionStyle.Render("Search & Filter:"))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Search by filename", keyStyle.Render("/")))
	helpContent = append(helpContent, fmt.Sprintf("  %s       Search file content (ripgrep)", keyStyle.Render("ctrl+g")))
	helpContent = append(helpContent, fmt.Sprintf("  %s       Toggle recursive search", keyStyle.Render("ctrl+r")))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Open sort menu", keyStyle.Render("S")))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Toggle hidden files", keyStyle.Render(".")))
	helpContent = append(helpContent, "")

	// View Options section
	helpContent = append(helpContent, sectionStyle.Render("View Options:"))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Toggle preview pane", keyStyle.Render("p")))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Toggle dual pane mode", keyStyle.Render("T")))
	helpContent = append(helpContent, fmt.Sprintf("  %s           Switch active pane", keyStyle.Render("tab")))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Toggle permissions display", keyStyle.Render("I")))
	helpContent = append(helpContent, "")

	// Bookmarks section
	helpContent = append(helpContent, sectionStyle.Render("Bookmarks:"))
	helpContent = append(helpContent, fmt.Sprintf("  %s             View bookmarks", keyStyle.Render("b")))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Add bookmark", keyStyle.Render("B")))
	helpContent = append(helpContent, "")

	// Other section
	helpContent = append(helpContent, sectionStyle.Render("Other:"))
	helpContent = append(helpContent, fmt.Sprintf("  %s             Show this help screen", keyStyle.Render("?")))
	helpContent = append(helpContent, fmt.Sprintf("  %s       Quit Scout", keyStyle.Render("q / ctrl+c")))
	helpContent = append(helpContent, "")
	helpContent = append(helpContent, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Press esc, q, or ? to close this help screen"))

	help := helpStyle.Render(strings.Join(helpContent, "\n"))
	return lipgloss.Place(m.width, m.height-7, lipgloss.Center, lipgloss.Center, help)
}

func (m model) renderContentSearchView() string {
	availableHeight := m.height - 9
	if availableHeight < 3 {
		availableHeight = 3
	}

	searchStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("99")).
		Width(m.width - 2).
		Height(availableHeight + 2).
		Padding(1)

	var content []string

	// Show search input if no results yet
	if len(m.contentSearchResults) == 0 {
		content = append(content, "🔍 Content Search (ripgrep)")
		content = append(content, "")
		content = append(content, m.textInput.View())
		content = append(content, "")
		content = append(content, "Type your search query and press Enter")
		content = append(content, "")
		content = append(content, "Esc: Cancel")
	} else {
		// Show results
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
		header := fmt.Sprintf("🔍 Content Search Results (%d found) - %s navigate • %s open • %s close",
			len(m.contentSearchResults),
			keyStyle.Render("↑↓:"),
			keyStyle.Render("o:"),
			keyStyle.Render("esc:"))
		content = append(content, header)
		content = append(content, "")

		// Show results with cursor
		visibleResults := availableHeight - 4
		if visibleResults < 1 {
			visibleResults = 1
		}

		startIdx := m.contentSearchCursor
		if startIdx > len(m.contentSearchResults)-visibleResults {
			startIdx = len(m.contentSearchResults) - visibleResults
		}
		if startIdx < 0 {
			startIdx = 0
		}

		endIdx := startIdx + visibleResults
		if endIdx > len(m.contentSearchResults) {
			endIdx = len(m.contentSearchResults)
		}

		for i := startIdx; i < endIdx; i++ {
			result := m.contentSearchResults[i]
			cursor := "  "
			if i == m.contentSearchCursor {
				cursor = "▶ "
			}

			line := fmt.Sprintf("%s%s:%d:%d - %s",
				cursor,
				filepath.Base(result.file),
				result.line,
				result.column,
				strings.TrimSpace(result.content))

			// Truncate if too long
			maxLen := m.width - 8
			if len(line) > maxLen {
				line = line[:maxLen-3] + "..."
			}

			if i == m.contentSearchCursor {
				selectedStyle := lipgloss.NewStyle().
					Background(lipgloss.Color("57")).
					Foreground(lipgloss.Color("230")).
					Width(m.width - 6)
				line = selectedStyle.Render(line)
			}

			content = append(content, line)
		}
	}

	return searchStyle.Render(strings.Join(content, "\n"))
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
