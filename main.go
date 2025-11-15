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
	dirHistory          []string // Navigation history
	historyIndex        int      // Current position in history
	recursiveSearch     bool     // Toggle for recursive vs current dir search
	loading             bool     // Loading indicator
	searchMatches       [][]int  // Character positions that matched in fuzzy search
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

	m := model{
		mode:            modeNormal,
		currentDir:      currentDir,
		files:           []fileItem{},
		cursor:          0,
		previewScroll:   0,
		bookmarksCursor: 0,
		searchInput:     ti,
		width:           0,
		height:          0,
		showHidden:      config.ShowHidden,
		showPreview:     config.PreviewEnabled,
		config:          config,
		gitModified:     getGitModifiedFiles(currentDir),
		gitBranch:       getGitBranch(currentDir),
		dirHistory:      []string{currentDir},
		historyIndex:    0,
		recursiveSearch: false,
		loading:         false,
		searchMatches:   [][]int{},
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

	// Sort: directories first, then by name
	sort.Slice(m.files, func(i, j int) bool {
		if m.files[i].isDir != m.files[j].isDir {
			return m.files[i].isDir
		}
		return strings.ToLower(m.files[i].name) < strings.ToLower(m.files[j].name)
	})

	m.filteredFiles = m.files
	m.updatePreview()

	// Update frecency when visiting a directory
	m.updateFrecency(m.currentDir)
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
			default:
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.updateFilter()
				m.updatePreview()
				return m, cmd
			}

		case modeNormal:
			switch msg.String() {
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
				pageSize := (m.height - 8) / 2
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
				pageSize := (m.height - 8) / 2
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
				pageSize := m.height - 8
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
				pageSize := m.height - 8
				if pageSize < 1 {
					pageSize = 10
				}
				m.cursor -= pageSize
				if m.cursor < 0 {
					m.cursor = 0
				}
				m.updatePreview()

			case "alt+down":
				// Scroll preview down
				if m.showPreview && len(m.previewLines) > 0 {
					availableHeight := m.height - 8
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

			case "alt+up":
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

			case "H":
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
	if m.mode == modeBookmarks {
		// Bookmarks overlay
		mainContent = m.renderBookmarksView()
	} else if m.mode == modeConfirmDelete {
		// Confirmation dialog overlay
		mainContent = m.renderConfirmDeleteView()
	} else if m.showPreview {
		// Split view
		fileList := m.renderFileList(m.width / 2)
		preview := m.renderPreview(m.width / 2)
		mainContent = lipgloss.JoinHorizontal(lipgloss.Top, fileList, preview)
	} else {
		// Full width file list
		mainContent = m.renderFileList(m.width)
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
		// Create search section with matching background
		searchContainerStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("226")).
			Padding(0, 1)

		// Build search text with mode indicator
		searchLabel := "Search: "

		if m.recursiveSearch {
			searchLabel = "Search [RECURSIVE]: "
		}

		// Combine label and input
		searchText := searchLabel + m.searchInput.View()

		// Calculate available width for title
		searchWidth := lipgloss.Width(searchText) + 2 // +2 for padding
		titleWidth := m.width - searchWidth
		if titleWidth < 20 {
			titleWidth = 20
		}

		title = lipgloss.JoinHorizontal(lipgloss.Top,
			titleStyle.Width(titleWidth).Render(title),
			searchContainerStyle.Render(searchText),
		)
	}

	return titleStyle.Render(title)
}

func (m model) renderFileList(width int) string {
	// Use same height calculation as preview for consistency
	availableHeight := m.height - 8
	if availableHeight < 3 {
		availableHeight = 3
	}

	listStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(width-2).
		Height(availableHeight+2).
		Padding(0, 1)

	var items []string

	// Calculate visible range
	visibleHeight := availableHeight
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
			gitStatus = " [M]"
		}

		// Format item with highlighting if in search mode
		name := item.name
		displayName := name

		// Apply search highlighting if we have match positions
		if m.mode == modeSearch && i < len(m.searchMatches) && len(m.searchMatches[i]) > 0 {
			displayName = highlightMatches(name, m.searchMatches[i])
		}

		if len(name) > width-10 {
			displayName = displayName[:min(len(displayName), width-13)] + "..."
		}

		line := fmt.Sprintf("%s %s%s", icon, displayName, gitStatus)

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

	return listStyle.Render(strings.Join(items, "\n"))
}

func (m model) renderPreview(width int) string {
	// Calculate available height more conservatively
	// Header(1) + Status(1) + borders(2) + padding(2) + buffer(2) = 8
	availableHeight := m.height - 8
	if availableHeight < 3 {
		availableHeight = 3 // minimum height
	}

	previewStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(width - 2).
		Height(availableHeight + 2). // +2 for padding
		Padding(1)

	if len(m.previewLines) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true)
		return previewStyle.Render(emptyStyle.Render("No preview available"))
	}

	// Calculate content area (subtract space for scroll indicators)
	contentHeight := availableHeight - 2 // Reserve space for scroll indicators
	if contentHeight < 1 {
		contentHeight = 1
	}

	startLine := m.previewScroll
	endLine := startLine + contentHeight

	if endLine > len(m.previewLines) {
		endLine = len(m.previewLines)
	}

	if startLine >= len(m.previewLines) {
		startLine = 0
		endLine = contentHeight
		if endLine > len(m.previewLines) {
			endLine = len(m.previewLines)
		}
	}

	// Get visible lines
	var visibleLines []string
	if startLine < len(m.previewLines) {
		visibleLines = m.previewLines[startLine:endLine]
	}

	// Add scroll indicators
	var content []string
	if startLine > 0 {
		content = append(content, "▲ Scroll up with Alt+↑")
	}
	content = append(content, visibleLines...)
	if endLine < len(m.previewLines) {
		content = append(content, "▼ Scroll down with Alt+↓")
	}

	return previewStyle.Render(strings.Join(content, "\n"))
}

func (m model) renderBookmarksView() string {
	// Calculate available height
	availableHeight := m.height - 8
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Full width bookmarks panel
	bookmarksStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
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
		Border(lipgloss.RoundedBorder()).
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
	dialogBox := lipgloss.Place(m.width, m.height-6, lipgloss.Center, lipgloss.Center, dialog)

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
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
		help = fmt.Sprintf("%s navigate • %s search • %s open • %s vscode • %s bookmarks • %s add bookmark • %s back • %s preview",
			keyStyle.Render("↑↓:"),
			keyStyle.Render("/:"),
			keyStyle.Render("enter:"),
			keyStyle.Render("o:"),
			keyStyle.Render("b:"),
			keyStyle.Render("B:"),
			keyStyle.Render("esc:"),
			keyStyle.Render("p:"))
		helpPlainText = "↑↓: navigate • /: search • enter: open • o: vscode • b: bookmarks • B: add bookmark • esc: back • p: preview"
	}

	// Combine sections - use fixed widths to avoid overlap
	leftWidth := len(leftInfo)
	rightWidth := len(helpPlainText)
	centerWidth := m.width - leftWidth - rightWidth - 4

	if centerWidth < 0 {
		centerWidth = 0
	}

	status := lipgloss.JoinHorizontal(lipgloss.Top,
		statusStyle.Width(leftWidth+1).Render(leftInfo),
		statusStyle.Width(centerWidth).Align(lipgloss.Center).Render(center),
		statusStyle.Width(rightWidth+1).Align(lipgloss.Right).Render(help),
	)

	return status
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

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
