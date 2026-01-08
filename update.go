package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LFroesch/scout/internal/config"
	"github.com/LFroesch/scout/internal/git"
	"github.com/LFroesch/scout/internal/utils"
)

func (m *model) Init() tea.Cmd {
	return tea.SetWindowTitle("🔍 Scout - File Explorer")
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Clear expired status messages
	if m.statusMsg != "" && time.Now().After(m.statusExpiry) {
		m.statusMsg = ""
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Recalculate scroll positions for new height
		if len(m.filteredFiles) > 0 {
			availableHeight := m.height - 9
			if availableHeight < 3 {
				availableHeight = 3
			}
			visibleHeight := availableHeight - 1

			// Ensure cursor is still in valid range
			if m.cursor >= len(m.filteredFiles) {
				m.cursor = len(m.filteredFiles) - 1
			}
			if m.cursor < 0 {
				m.cursor = 0
			}

			// Adjust scroll offset to keep cursor visible
			if m.scrollOffset > m.cursor {
				m.scrollOffset = m.cursor
			}
			maxScroll := len(m.filteredFiles) - visibleHeight
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollOffset > maxScroll {
				m.scrollOffset = maxScroll
			}
			if m.cursor >= m.scrollOffset+visibleHeight {
				m.scrollOffset = m.cursor - visibleHeight + 1
			}
		}

		// Update preview with new width (text needs to reflow)
		m.updatePreview()
		return m, nil

	case previewUpdateMsg:
		// Only update preview if scrolling has actually stopped
		now := time.Now()
		timeSinceLastMove := now.Sub(m.lastCursorMove).Milliseconds()

		// If it's been >200ms since last cursor move, scrolling has stopped
		if timeSinceLastMove > 200 && m.cursor != m.previewCursor {
			m.updatePreview()
		} else if timeSinceLastMove <= 200 {
			// Still scrolling, check again later
			return m, previewUpdateAfterDelay()
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeBookmarks:
			switch msg.String() {
			case "esc", "q":
				m.mode = modeNormal
				return m, nil
			case "j", "down":
				if m.bookmarksCursor < len(m.sortedBookmarkPaths)-1 {
					m.bookmarksCursor++
				}
			case "k", "up":
				if m.bookmarksCursor > 0 {
					m.bookmarksCursor--
				}
			case "enter":
				if len(m.sortedBookmarkPaths) > 0 && m.bookmarksCursor < len(m.sortedBookmarkPaths) {
					targetPath := m.sortedBookmarkPaths[m.bookmarksCursor]
					// Ensure target is within root path
					if m.config.RootPath == "" || strings.HasPrefix(targetPath, m.config.RootPath) {
						m.addToHistory(targetPath)
						m.currentDir = targetPath
						m.cursor = 0
						m.scrollOffset = 0
						m.previewScroll = 0
						m.mode = modeNormal
						m.loadFiles()
						m.gitModified = git.GetModifiedFiles(m.currentDir)
						m.gitBranch = git.GetBranch(m.currentDir)
					}
				}
				return m, nil
			case "o":
				// Open bookmark in VS Code
				if len(m.sortedBookmarkPaths) > 0 && m.bookmarksCursor < len(m.sortedBookmarkPaths) {
					targetPath := m.sortedBookmarkPaths[m.bookmarksCursor]
					// Ensure target is within root path
					if m.config.RootPath == "" || strings.HasPrefix(targetPath, m.config.RootPath) {
						return m, m.openInVSCode(targetPath)
					}
				}
				return m, nil
			case "d":
				// Confirm delete bookmark
				if len(m.sortedBookmarkPaths) > 0 && m.bookmarksCursor < len(m.sortedBookmarkPaths) {
					// Find the actual index in config.Bookmarks
					targetPath := m.sortedBookmarkPaths[m.bookmarksCursor]
					for i, bookmark := range m.config.Bookmarks {
						if bookmark == targetPath {
							m.deleteBookmarkIndex = i
							break
						}
					}
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
					m.sortedBookmarkPaths = m.sortBookmarksByFrecency()
					if m.bookmarksCursor >= len(m.sortedBookmarkPaths) && len(m.sortedBookmarkPaths) > 0 {
						m.bookmarksCursor = len(m.sortedBookmarkPaths) - 1
					}
					config.Save(m.config)
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
				m.helpScroll = 0
				return m, nil
			case "j", "down":
				m.helpScroll++
				return m, nil
			case "k", "up":
				if m.helpScroll > 0 {
					m.helpScroll--
				}
				return m, nil
			case "g":
				m.helpScroll = 0
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
				m.loading = false
				m.updatePreview()
				return m, nil
			case "enter":
				m.mode = modeNormal
				m.loading = false
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
				// Pass all other keys to search input for typing
				m.searchInput, cmd = m.searchInput.Update(msg)

				// For recursive searches or content search, show loading indicator
				if m.recursiveSearch || m.currentSearchType == searchContent {
					m.loading = true
					m.statusMsg = "Searching..."
					m.statusExpiry = time.Now().Add(10 * time.Second)
				}

				m.updateFilter()
				m.loading = false
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
					return m, m.moveCursor(m.cursor + 1)
				}

			case "k", "up":
				if m.cursor > 0 {
					return m, m.moveCursor(m.cursor - 1)
				}

			case "ctrl+d":
				// Half-page down
				pageSize := (m.height - 9) / 2
				if pageSize < 1 {
					pageSize = 5
				}
				newPos := m.cursor + pageSize
				if newPos >= len(m.filteredFiles) {
					newPos = len(m.filteredFiles) - 1
				}
				if newPos < 0 {
					newPos = 0
				}
				return m, m.moveCursor(newPos)

			case "ctrl+u":
				// Half-page up
				pageSize := (m.height - 9) / 2
				if pageSize < 1 {
					pageSize = 5
				}
				newPos := m.cursor - pageSize
				if newPos < 0 {
					newPos = 0
				}
				return m, m.moveCursor(newPos)

			case "ctrl+f":
				// Full-page down
				pageSize := m.height - 9
				if pageSize < 1 {
					pageSize = 10
				}
				newPos := m.cursor + pageSize
				if newPos >= len(m.filteredFiles) {
					newPos = len(m.filteredFiles) - 1
				}
				if newPos < 0 {
					newPos = 0
				}
				return m, m.moveCursor(newPos)

			case "ctrl+b":
				// Full-page up
				pageSize := m.height - 9
				if pageSize < 1 {
					pageSize = 10
				}
				newPos := m.cursor - pageSize
				if newPos < 0 {
					newPos = 0
				}
				return m, m.moveCursor(newPos)

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
				return m, m.moveCursor(0)

			case "G":
				if len(m.filteredFiles) > 0 {
					return m, m.moveCursor(len(m.filteredFiles) - 1)
				}

			case "enter", "l", "right":
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					if selected.isDir {
						// Prevent backing out of WSL home directory
						if selected.name == ".." {
							homeDir, _ := os.UserHomeDir()
							if strings.HasPrefix(m.currentDir, homeDir) && !strings.HasPrefix(selected.path, homeDir) {
								m.statusMsg = "Cannot navigate above home directory (use ` to jump to Windows C:)"
								m.statusExpiry = time.Now().Add(3 * time.Second)
								break
							}
						}
						m.addToHistory(selected.path)
						m.currentDir = selected.path
						m.cursor = 0
						m.scrollOffset = 0
						m.previewScroll = 0
						m.loadFiles()
						m.gitModified = git.GetModifiedFiles(m.currentDir)
						m.gitBranch = git.GetBranch(m.currentDir)
					} else {
						return m, m.openFile(selected.path)
					}
				}

			case "esc", "h", "left":
				parentDir := filepath.Dir(m.currentDir)

				// Prevent backing out of WSL home directory
				homeDir, _ := os.UserHomeDir()
				if strings.HasPrefix(m.currentDir, homeDir) && !strings.HasPrefix(parentDir, homeDir) {
					m.statusMsg = "Cannot navigate above home directory (use ` to jump to Windows C:)"
					m.statusExpiry = time.Now().Add(3 * time.Second)
					break
				}

				// Check if we can go up (respect root path and filesystem root)
				if m.currentDir != "/" && m.currentDir != m.config.RootPath &&
					(m.config.RootPath == "" || strings.HasPrefix(parentDir, m.config.RootPath)) {
					m.addToHistory(parentDir)
					m.currentDir = parentDir
					m.cursor = 0
					m.scrollOffset = 0
					m.previewScroll = 0
					m.loadFiles()
					m.gitModified = git.GetModifiedFiles(m.currentDir)
					m.gitBranch = git.GetBranch(m.currentDir)
				}

			case "/":
				m.mode = modeSearch
				m.currentSearchType = searchFilename
				m.searchInput.SetValue("")
				m.searchInput.Placeholder = "Search filenames..."
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

			case "y":
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					m.copyPath(selected.path)
				}

			case "e":
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					if !selected.isDir {
						// If content search result, open at specific line
						if m.mode == modeNormal && m.currentSearchType == searchContent {
							lineNum := int(selected.size) // Line number stored in size field
							return m, m.editFileAtLine(selected.path, lineNum)
						}
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
				m.gitModified = git.GetModifiedFiles(m.currentDir)
				m.gitBranch = git.GetBranch(m.currentDir)
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

			case "`":
				// Jump to Windows C: drive (WSL /mnt/c)
				windowsC := "/mnt/c"
				if _, err := os.Stat(windowsC); err == nil {
					m.addToHistory(windowsC)
					m.currentDir = windowsC
					m.cursor = 0
					m.scrollOffset = 0
					m.previewScroll = 0
					m.loadFiles()
				} else {
					m.statusMsg = "/mnt/c not available (not in WSL or drive not mounted)"
					m.statusExpiry = time.Now().Add(3 * time.Second)
				}

			case "b":
				m.mode = modeBookmarks
				m.bookmarksCursor = 0
				m.sortedBookmarkPaths = m.sortBookmarksByFrecency()

			case "B":
				// Add highlighted item to bookmarks (only directories)
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					if selected.isDir {
						if !utils.Contains(m.config.Bookmarks, selected.path) {
							m.config.Bookmarks = append(m.config.Bookmarks, selected.path)
							config.Save(m.config)
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

			case "p":
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

			case "?":
				// Show help screen
				m.mode = modeHelp

			case "ctrl+g":
				// Content search (ripgrep)
				m.mode = modeSearch
				m.currentSearchType = searchContent
				m.searchInput.SetValue("")
				m.searchInput.Placeholder = "Search file contents..."
				m.searchInput.Focus()
				return m, textinput.Blink
			}
		}
	}

	return m, nil
}
