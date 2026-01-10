package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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

// Async search messages
type searchDebounceMsg struct{ query string }
type searchResultMsg struct {
	files   []fileItem
	matches [][]int
}
type searchPartialMsg struct {
	count int    // Total files scanned so far
	drive string // Current drive being searched (for ultra search)
}
type searchCompleteMsg struct{}
type searchErrorMsg struct{ err error }

// File open result message
type fileOpenResultMsg struct {
	success bool
	message string
	path    string
}

// Terminal dimension constants
const (
	minTerminalWidth  = 60  // Minimum usable width
	minTerminalHeight = 20  // Minimum usable height
	uiOverhead        = 9   // Header (1) + status (1) + borders (4) + padding (3)
)

// Application behavior constants
const (
	maxPreviewItems      = 20                   // Maximum items to show in directory preview
	maxHistoryEntries    = 100                  // Maximum navigation history entries
	maxUndoStackSize     = 10                   // Maximum undo operations to remember
	previewUpdateDelay   = 250 * time.Millisecond // Delay before updating preview after cursor move
	searchDebounceDelay  = 300 * time.Millisecond // Delay before triggering search after typing
	minSearchChars       = 2                    // Minimum characters before triggering expensive searches
	configSaveInterval   = 10                   // Save config every N directory visits
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
	searchRecursive
	searchContent
	searchUltra // Search across all mounted drives
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
	lastClickTime       time.Time     // Time of last mouse click
	lastClickY          int           // Y position of last click
	lastClickMode       mode          // Mode during last click
	doubleClickThreshold time.Duration // Double-click time threshold (typically 300-500ms)
	searchCancel        chan struct{} // Channel to cancel ongoing search
	searchInProgress    bool          // Whether a search is currently running
	scannedFiles        int           // Number of files scanned in current search
	searchResultsLocked bool          // Whether search results are locked for navigation
	searchResultChan    chan tea.Msg  // Channel for receiving search progress and results
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

// Helper methods for safe dimensions
func (m *model) getSafeWidth() int {
	if m.width < minTerminalWidth {
		return minTerminalWidth
	}
	return m.width
}

func (m *model) getSafeHeight() int {
	if m.height < minTerminalHeight {
		return minTerminalHeight
	}
	return m.height
}

// getContentHeight returns available height for content (total - UI overhead)
func (m *model) getContentHeight() int {
	availableHeight := m.getSafeHeight() - uiOverhead
	if availableHeight < 3 {
		availableHeight = 3
	}
	return availableHeight
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
		doubleClickThreshold: 400 * time.Millisecond,
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

func (m *model) sortSearchResults() {
	// Create a helper struct to keep file and match info together
	type fileMatchPair struct {
		file    fileItem
		matches []int
	}

	// Pair files with their matches
	pairs := make([]fileMatchPair, len(m.filteredFiles))
	for i := range m.filteredFiles {
		pairs[i].file = m.filteredFiles[i]
		if i < len(m.searchMatches) {
			pairs[i].matches = m.searchMatches[i]
		}
	}

	// Sort pairs based on sort mode
	sort.Slice(pairs, func(i, j int) bool {
		// Keep ".." at top always
		if pairs[i].file.name == ".." {
			return true
		}
		if pairs[j].file.name == ".." {
			return false
		}

		// Directories first (except for size sort)
		if m.sortBy != sortBySize && pairs[i].file.isDir != pairs[j].file.isDir {
			return pairs[i].file.isDir
		}

		// Apply sort mode
		switch m.sortBy {
		case sortBySize:
			return pairs[i].file.size > pairs[j].file.size
		case sortByDate:
			return pairs[i].file.modTime.After(pairs[j].file.modTime)
		case sortByType:
			extI := strings.ToLower(filepath.Ext(pairs[i].file.name))
			extJ := strings.ToLower(filepath.Ext(pairs[j].file.name))
			if extI != extJ {
				return extI < extJ
			}
			return strings.ToLower(pairs[i].file.name) < strings.ToLower(pairs[j].file.name)
		default: // sortByName
			return strings.ToLower(pairs[i].file.name) < strings.ToLower(pairs[j].file.name)
		}
	})

	// Unpack sorted pairs back into separate slices
	m.filteredFiles = make([]fileItem, len(pairs))
	m.searchMatches = make([][]int, len(pairs))
	for i, pair := range pairs {
		m.filteredFiles[i] = pair.file
		m.searchMatches[i] = pair.matches
	}
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
	// Keep only last N items
	if len(m.undoStack) > maxUndoStackSize {
		m.undoStack = m.undoStack[1:]
	}
}

func (m *model) searchFileContent(query string) error {
	// Create dummy cancel channel for sync operation
	cancelChan := make(chan struct{})
	defer close(cancelChan)

	results, err := search.SearchFileContent(query, m.currentDir, m.showHidden, cancelChan)
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

func (m *model) updateFilter() tea.Cmd {
	query := m.searchInput.Value()
	if query == "" {
		m.filteredFiles = m.files
		m.searchMatches = [][]int{}
		m.statusMsg = ""
		m.cancelCurrentSearch()
		m.loading = false
		return nil
	}

	// For short queries (< N chars), skip expensive searches
	isExpensiveSearch := m.currentSearchType == searchContent ||
		m.currentSearchType == searchRecursive ||
		m.currentSearchType == searchUltra ||
		m.recursiveSearch
	if len(query) < minSearchChars && isExpensiveSearch {
		m.statusMsg = fmt.Sprintf("Type at least %d characters to search", minSearchChars)
		m.statusExpiry = time.Now().Add(2 * time.Second)
		m.cancelCurrentSearch()
		m.loading = false
		return nil
	}

	// For expensive searches, use async + debounce
	if isExpensiveSearch {
		m.cancelCurrentSearch() // Cancel any ongoing search
		return searchDebounce(query)
	}

	// For simple current directory search, still do it synchronously (it's fast)
	m.searchCurrentDir(query)
	m.statusMsg = fmt.Sprintf("Found %d files", len(m.filteredFiles))
	m.statusExpiry = time.Now().Add(3 * time.Second)

	// Reset cursor if it's out of bounds
	if m.cursor >= len(m.filteredFiles) {
		m.cursor = 0
	}
	return nil
}

func (m *model) searchCurrentDir(query string) {
	// Build list of file names for substring matching
	names := make([]string, len(m.files))
	for i, file := range m.files {
		names[i] = file.name
	}

	// Use search module for substring matching
	matches := search.SubstringMatchNames(query, names)

	m.filteredFiles = []fileItem{}
	m.searchMatches = [][]int{}

	for _, match := range matches {
		m.filteredFiles = append(m.filteredFiles, m.files[match.Index])
		m.searchMatches = append(m.searchMatches, match.MatchedIndexes)
	}
}

func (m *model) recursiveSearchFiles(query string) {
	// Create dummy cancel channel for sync operation
	cancelChan := make(chan struct{})
	defer close(cancelChan)

	results, matches := search.RecursiveSearchFiles(query, m.currentDir, m.showHidden, utils.ShouldIgnore, cancelChan, nil, m.config.MaxResults, m.config.MaxDepth, m.config.MaxFilesScanned, m.config.SkipDirectories)

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

func (m *model) ultraSearchFiles(query string) {
	drives := utils.GetMountedDrives()

	m.filteredFiles = []fileItem{}
	m.searchMatches = [][]int{}

	// Create dummy cancel channel for sync operation
	cancelChan := make(chan struct{})
	defer close(cancelChan)

	// Search across all drives
	for _, drive := range drives {
		results, matches := search.RecursiveSearchFiles(query, drive, m.showHidden, utils.ShouldIgnore, cancelChan, nil, m.config.MaxResults, m.config.MaxDepth, m.config.MaxFilesScanned, m.config.SkipDirectories)

		for i, result := range results {
			// Add drive label prefix to display name for clarity
			driveLabel := utils.GetDriveLabel(drive)
			displayName := fmt.Sprintf("[%s] %s", driveLabel, result.DisplayName)

			m.filteredFiles = append(m.filteredFiles, fileItem{
				path:    result.Path,
				name:    displayName,
				isDir:   result.IsDir,
				size:    result.Size,
				modTime: result.ModTime,
			})
			if i < len(matches) {
				m.searchMatches = append(m.searchMatches, matches[i].MatchedIndexes)
			}
		}
	}
}

// searchDebounce returns a command that waits before triggering search
func searchDebounce(query string) tea.Cmd {
	return tea.Tick(searchDebounceDelay, func(t time.Time) tea.Msg {
		return searchDebounceMsg{query: query}
	})
}

// cancelCurrentSearch cancels any ongoing search
func (m *model) cancelCurrentSearch() {
	if m.searchCancel != nil {
		close(m.searchCancel)
		m.searchCancel = nil
	}
	m.searchInProgress = false
}

// performAsyncSearch runs the appropriate search in a goroutine
func (m *model) performAsyncSearch(query string) tea.Cmd {
	// Cancel any previous search
	m.cancelCurrentSearch()

	// Create new cancellation channel
	m.searchCancel = make(chan struct{})
	m.searchInProgress = true
	m.scannedFiles = 0
	cancelChan := m.searchCancel

	// Capture the necessary state before going async
	searchType := m.currentSearchType
	currentDir := m.currentDir
	showHidden := true // Always search hidden files - if you're searching, you want to find it
	maxResults := m.config.MaxResults
	maxDepth := m.config.MaxDepth
	maxFilesScanned := m.config.MaxFilesScanned
	skipDirectories := m.config.SkipDirectories

	// Clear previous results and show loading
	m.filteredFiles = []fileItem{}
	m.searchMatches = [][]int{}
	m.loading = true

	// Create result channel
	resultChan := make(chan tea.Msg, 10) // Buffered to handle bursts of progress updates
	m.searchResultChan = resultChan

	// Launch search in goroutine
	go func() {
		var files []fileItem
		var matches [][]int
		var err error

		// Progress callback - updates m.scannedFiles (will be polled by ticker)
		var currentDrive string
		onProgress := func(scanned int) {
			// Send progress update
			select {
			case resultChan <- searchPartialMsg{count: scanned, drive: currentDrive}:
			default:
			}
		}

		// Run the appropriate search based on type
		switch searchType {
		case searchContent:
			results, searchErr := search.SearchFileContent(query, currentDir, showHidden, cancelChan)
			if searchErr != nil {
				err = searchErr
			} else {
				// Convert results to fileItems
				for _, result := range results {
					select {
					case <-cancelChan:
						resultChan <- searchCompleteMsg{}
						return
					default:
					}
					files = append(files, fileItem{
						path:  result.Path,
						name:  result.DisplayName,
						isDir: result.IsDir,
						size:  int64(result.LineNumber),
					})
				}
			}

		case searchRecursive:
			results, matchResults := search.RecursiveSearchFiles(query, currentDir, showHidden, utils.ShouldIgnore, cancelChan, onProgress, maxResults, maxDepth, maxFilesScanned, skipDirectories)
			for i, result := range results {
				select {
				case <-cancelChan:
					resultChan <- searchCompleteMsg{}
					return
				default:
				}
				files = append(files, fileItem{
					path:    result.Path,
					name:    result.DisplayName,
					isDir:   result.IsDir,
					size:    result.Size,
					modTime: result.ModTime,
				})
				if i < len(matchResults) {
					matches = append(matches, matchResults[i].MatchedIndexes)
				}
			}

		case searchUltra:
			drives := utils.GetMountedDrives()

			// Channel to collect results from all drive searches
			type driveResult struct {
				files   []fileItem
				matches [][]int
			}
			resultsChan := make(chan driveResult, len(drives))

			// Use WaitGroup to track parallel searches
			var wg sync.WaitGroup

			// Launch parallel search for each drive
			for _, drive := range drives {
				wg.Add(1)
				go func(drivePath string) {
					defer wg.Done()

					select {
					case <-cancelChan:
						return
					default:
					}

					// Notify which drive we're searching
					driveLabel := utils.GetDriveLabel(drivePath)
					resultChan <- searchPartialMsg{count: 0, drive: driveLabel}

					// Progress callback for this drive
					driveProgress := func(scanned int) {
						select {
						case resultChan <- searchPartialMsg{count: scanned, drive: driveLabel}:
						default:
						}
					}

					// Search this drive
					results, matchResults := search.RecursiveSearchFiles(query, drivePath, showHidden, utils.ShouldIgnore, cancelChan, driveProgress, maxResults, maxDepth, maxFilesScanned, skipDirectories)

					// Convert to fileItems with drive label
					var driveFiles []fileItem
					var driveMatches [][]int
					for i, result := range results {
						displayName := fmt.Sprintf("[%s] %s", driveLabel, result.DisplayName)
						prefixLen := len(fmt.Sprintf("[%s] ", driveLabel))

						driveFiles = append(driveFiles, fileItem{
							path:    result.Path,
							name:    displayName,
							isDir:   result.IsDir,
							size:    result.Size,
							modTime: result.ModTime,
						})

						// Adjust match positions to account for drive label prefix
						if i < len(matchResults) {
							adjustedMatches := make([]int, len(matchResults[i].MatchedIndexes))
							for j, pos := range matchResults[i].MatchedIndexes {
								adjustedMatches[j] = pos + prefixLen
							}
							driveMatches = append(driveMatches, adjustedMatches)
						}
					}

					// Send this drive's results
					select {
					case resultsChan <- driveResult{files: driveFiles, matches: driveMatches}:
					case <-cancelChan:
						return
					}
				}(drive)
			}

			// Collector goroutine - waits for all drives and closes channel
			go func() {
				wg.Wait()
				close(resultsChan)
			}()

			// Collect results from all drives as they complete
			for result := range resultsChan {
				files = append(files, result.files...)
				matches = append(matches, result.matches...)
				// Send intermediate results so UI updates immediately
				resultChan <- searchResultMsg{files: files, matches: matches}
			}
			// All drives complete - send final complete message
			resultChan <- searchCompleteMsg{}
			return

		default: // searchFilename
			results, matchResults := search.RecursiveSearchFiles(query, currentDir, showHidden, utils.ShouldIgnore, cancelChan, onProgress, maxResults, maxDepth, maxFilesScanned, skipDirectories)
			for i, result := range results {
				select {
				case <-cancelChan:
					resultChan <- searchCompleteMsg{}
					return
				default:
				}
				files = append(files, fileItem{
					path:    result.Path,
					name:    result.DisplayName,
					isDir:   result.IsDir,
					size:    result.Size,
					modTime: result.ModTime,
				})
				if i < len(matchResults) {
					matches = append(matches, matchResults[i].MatchedIndexes)
				}
			}
		}

		// Send final results
		if err != nil {
			resultChan <- searchErrorMsg{err: err}
		} else {
			resultChan <- searchResultMsg{files: files, matches: matches}
		}
	}()

	// Return command that waits for next message from search
	return waitForSearchMsg(resultChan)
}

// waitForSearchMsg returns a command that waits for the next search message
func waitForSearchMsg(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func previewUpdateAfterDelay() tea.Cmd {
	return tea.Tick(previewUpdateDelay, func(t time.Time) tea.Msg {
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
	return tea.Tick(previewUpdateDelay, func(t time.Time) tea.Msg {
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
	preview.WriteString(fmt.Sprintf("Path: %s\n", path))
	preview.WriteString(fmt.Sprintf("Items: %d\n\n", len(entries)))

	count := 0
	for _, entry := range entries {
		if count >= maxPreviewItems {
			preview.WriteString(fmt.Sprintf("\n... and %d more items", len(entries)-maxPreviewItems))
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
	preview.WriteString(fmt.Sprintf("Path: %s\n", path))
	preview.WriteString(fmt.Sprintf("Size: %s\n", utils.FormatFileSizeColored(info.Size())))
	preview.WriteString(fmt.Sprintf("Modified: %s\n", info.ModTime().Format("Jan 2, 2006 15:04")))
	preview.WriteString(fmt.Sprintf("Permissions: %s\n", info.Mode().String()))

	if m.gitModified[path] {
		preview.WriteString("Git: Modified\n")
	}

	preview.WriteString("\n")

	// Check if binary or large file
	if info.Size() > 1024*1024 || utils.IsBinaryFile(path) {
		preview.WriteString("─── File Metadata ───\n\n")
		preview.WriteString(fmt.Sprintf("Type: %s\n", strings.ToUpper(strings.TrimPrefix(filepath.Ext(path), "."))))
		preview.WriteString(fmt.Sprintf("Full Path: %s\n", path))
		preview.WriteString("\n(Binary file - preview unavailable)")
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

	// Save config periodically (every N visits)
	if m.config.Frecency[dir]%configSaveInterval == 0 {
		if err := config.Save(m.config); err != nil {
			// Silently log error - don't interrupt user experience for config save failures
			// The error will be logged by config.Save itself
		}
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

	// Limit history size to N entries
	if len(m.dirHistory) > maxHistoryEntries {
		m.dirHistory = m.dirHistory[1:]
		m.historyIndex--
	}
}
