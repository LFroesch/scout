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
	"github.com/LFroesch/scout/internal/fileops"
	"github.com/LFroesch/scout/internal/git"
	"github.com/LFroesch/scout/internal/utils"
)

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("🔍 Scout - File Explorer"),
		tea.EnableMouseAllMotion,
	)
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Clear expired status messages
	if m.statusMsg != "" && time.Now().After(m.statusExpiry) {
		m.statusMsg = ""
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Skip if dimensions haven't actually changed (debouncing)
		if msg.Width == m.width && msg.Height == m.height {
			return m, nil
		}

		// Enforce minimum dimensions for small terminals
		m.width = msg.Width
		m.height = msg.Height
		if m.width < minTerminalWidth {
			m.width = minTerminalWidth
		}
		if m.height < minTerminalHeight {
			m.height = minTerminalHeight
		}

		// Recalculate scroll positions for new height
		if len(m.filteredFiles) > 0 {
			availableHeight := m.height - uiOverhead
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

	case searchDebounceMsg:
		// Debounce timer expired, start the actual search
		currentQuery := m.searchInput.Value()
		if currentQuery == msg.query && m.mode == modeSearch {
			return m, m.performAsyncSearch(msg.query)
		}
		return m, nil

	case searchResultMsg:
		// Got search results (partial or final)
		if m.searchInProgress {
			m.filteredFiles = msg.files
			m.searchMatches = msg.matches
			// Don't mark as complete yet - might be partial results
			// searchCompleteMsg will mark it complete

			// Show current result count
			resultCount := len(m.filteredFiles)
			if resultCount > 5000 {
				m.statusMsg = fmt.Sprintf("Found %d files so far (search continues...)", resultCount)
			} else {
				m.statusMsg = fmt.Sprintf("Found %d files (searching...)", resultCount)
			}

			// Reset cursor if needed
			if m.cursor >= len(m.filteredFiles) {
				m.cursor = 0
			}
			m.scrollOffset = 0
			m.updatePreview()

			// Keep listening for more results
			if m.searchResultChan != nil {
				return m, waitForSearchMsg(m.searchResultChan)
			}
		}
		return m, nil

	case searchCompleteMsg:
		// Search complete (all drives finished)
		m.searchInProgress = false
		m.loading = false
		m.searchResultChan = nil // Stop listening for more messages

		// Update status with final count
		if len(m.filteredFiles) > 0 {
			if len(m.filteredFiles) > 5000 {
				m.statusMsg = fmt.Sprintf("Search complete: %d files (large result set)", len(m.filteredFiles))
			} else {
				m.statusMsg = fmt.Sprintf("Search complete: %d files", len(m.filteredFiles))
			}
			m.statusExpiry = time.Now().Add(3 * time.Second)
		}
		return m, nil

	case searchErrorMsg:
		// Search error occurred
		m.searchInProgress = false
		m.loading = false
		m.searchResultChan = nil // Stop listening for more messages
		m.statusMsg = fmt.Sprintf("Search error: %v", msg.err)
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil

	case searchPartialMsg:
		// Update progress status with drive info if available
		m.scannedFiles = msg.count
		if msg.drive != "" {
			if msg.count > 0 {
				m.statusMsg = fmt.Sprintf("Searching %s... %d files scanned", msg.drive, msg.count)
			} else {
				m.statusMsg = fmt.Sprintf("Searching %s...", msg.drive)
			}
		} else {
			m.statusMsg = fmt.Sprintf("Searching... %d files scanned", msg.count)
		}
		// Keep listening for more messages
		if m.searchResultChan != nil {
			return m, waitForSearchMsg(m.searchResultChan)
		}
		return m, nil

	case tea.MouseMsg:
		// Handle mouse wheel scroll
		if msg.Action == tea.MouseActionPress && (msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown) {
			switch m.mode {
			case modeNormal:
				// Scroll in file list
				if msg.Button == tea.MouseButtonWheelUp {
					if m.cursor > 0 {
						m.cursor--
						m.updatePreview()
					}
				} else {
					if m.cursor < len(m.filteredFiles)-1 {
						m.cursor++
						m.updatePreview()
					}
				}
				return m, nil

			case modeSearch:
				// Scroll in search results
				if msg.Button == tea.MouseButtonWheelUp {
					if m.cursor > 0 {
						m.cursor--
						m.updatePreview()
					}
				} else {
					if m.cursor < len(m.filteredFiles)-1 {
						m.cursor++
						m.updatePreview()
					}
				}
				return m, nil

			case modeHelp:
				// Scroll in help page
				if msg.Button == tea.MouseButtonWheelUp {
					if m.helpScroll > 0 {
						m.helpScroll--
					}
				} else {
					m.helpScroll++
				}
				return m, nil

			case modeBookmarks:
				// Scroll in bookmarks
				if msg.Button == tea.MouseButtonWheelUp {
					if m.bookmarksCursor > 0 {
						m.bookmarksCursor--
					}
				} else {
					if m.bookmarksCursor < len(m.sortedBookmarkPaths)-1 {
						m.bookmarksCursor++
					}
				}
				return m, nil
			}
		}

		// Handle mouse click
		if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
			switch m.mode {
			case modeSearch:
				// Mouse click in search results
				if len(m.filteredFiles) > 0 {
					clickY := msg.Y - 3 // Adjust for header
					if clickY >= 0 && clickY < len(m.filteredFiles)-m.scrollOffset {
						newCursor := m.scrollOffset + clickY
						if newCursor >= 0 && newCursor < len(m.filteredFiles) {
							// Check for double-click
							now := time.Now()
							isDoubleClick := !m.lastClickTime.IsZero() &&
								now.Sub(m.lastClickTime) <= m.doubleClickThreshold &&
								m.lastClickY == clickY &&
								m.lastClickMode == modeSearch &&
								m.cursor == newCursor

							if isDoubleClick {
								// Double-click: open file or enter directory
								m.lastClickTime = time.Time{} // Reset to prevent triple-click
								selected := m.filteredFiles[m.cursor]
								if selected.isDir {
									if selected.name != ".." && !m.searchResultsLocked {
										// If results are not locked, just select it
										return m, m.moveCursor(newCursor)
									}
									// If locked, navigate into directory
									if m.searchResultsLocked {
										m.addToHistory(selected.path)
										m.currentDir = selected.path
										m.cursor = 0
										m.scrollOffset = 0
										m.previewScroll = 0
										m.mode = modeNormal
										m.searchResultsLocked = false
										m.searchInput.SetValue("")
										m.recursiveSearch = false
										m.currentSearchType = searchFilename
										m.loadFiles()
										m.gitModified = git.GetModifiedFiles(m.currentDir)
										m.gitBranch = git.GetBranch(m.currentDir)
										m.updatePreview()
									}
								} else {
									// Open file
									return m, m.openFile(selected.path)
								}
							} else {
								// Single-click: select
								m.lastClickTime = now
								m.lastClickY = clickY
								m.lastClickMode = modeSearch
								return m, m.moveCursor(newCursor)
							}
						}
					}
				}

			case modeNormal:
				// Mouse click in file list area
				if len(m.filteredFiles) > 0 {
					clickY := msg.Y - 3 // Adjust for header
					if clickY >= 0 && clickY < len(m.filteredFiles)-m.scrollOffset {
						newCursor := m.scrollOffset + clickY
						if newCursor >= 0 && newCursor < len(m.filteredFiles) {
							// Check for double-click
							now := time.Now()
							isDoubleClick := !m.lastClickTime.IsZero() &&
								now.Sub(m.lastClickTime) <= m.doubleClickThreshold &&
								m.lastClickY == clickY &&
								m.lastClickMode == modeNormal &&
								m.cursor == newCursor

							if isDoubleClick {
								// Double-click: enter directory or open file
								m.lastClickTime = time.Time{} // Reset to prevent triple-click
								selected := m.filteredFiles[m.cursor]
								if selected.isDir {
									if selected.name == ".." {
										// Navigate to parent directory
										parentDir := filepath.Dir(m.currentDir)
										if m.config.RootPath == "" || strings.HasPrefix(parentDir, m.config.RootPath) {
											m.addToHistory(parentDir)
											m.currentDir = parentDir
											m.cursor = 0
											m.scrollOffset = 0
											m.previewScroll = 0
											m.loadFiles()
											m.gitModified = git.GetModifiedFiles(m.currentDir)
											m.gitBranch = git.GetBranch(m.currentDir)
											m.updatePreview()
										}
									} else {
										// Enter directory
										if m.config.RootPath == "" || strings.HasPrefix(selected.path, m.config.RootPath) {
											m.addToHistory(selected.path)
											m.currentDir = selected.path
											m.cursor = 0
											m.scrollOffset = 0
											m.previewScroll = 0
											m.loadFiles()
											m.gitModified = git.GetModifiedFiles(m.currentDir)
											m.gitBranch = git.GetBranch(m.currentDir)
											m.updatePreview()
										}
									}
								} else {
									// Open file with default application
									return m, m.openFile(selected.path)
								}
							} else {
								// Single-click: select and preview
								m.lastClickTime = now
								m.lastClickY = clickY
								m.lastClickMode = modeNormal
								return m, m.moveCursor(newCursor)
							}
						}
					}
				}

			case modeBookmarks:
				// Click in bookmarks list with scroll support
				if len(m.sortedBookmarkPaths) > 0 {
					// Calculate scroll offset (same logic as renderBookmarksView)
					availableHeight := m.height - uiOverhead
					if availableHeight < 3 {
						availableHeight = 3
					}
					contentHeight := availableHeight - 2
					if contentHeight < 1 {
						contentHeight = 1
					}

					maxItems := contentHeight
					scrollOffset := 0
					if m.bookmarksCursor >= maxItems {
						scrollOffset = m.bookmarksCursor - maxItems + 1
					}

					hasTopIndicator := scrollOffset > 0
					hasBottomIndicator := scrollOffset+maxItems < len(m.sortedBookmarkPaths)

					// Adjust for indicators
					actualMaxItems := maxItems
					if hasTopIndicator {
						actualMaxItems--
					}
					if hasBottomIndicator {
						actualMaxItems--
					}

					// Recalculate scroll offset
					if m.bookmarksCursor >= scrollOffset+actualMaxItems {
						scrollOffset = m.bookmarksCursor - actualMaxItems + 1
					}
					if m.bookmarksCursor < scrollOffset {
						scrollOffset = m.bookmarksCursor
					}

					hasTopIndicator = scrollOffset > 0

					// Account for: app header (1) + border (1) + bookmarks title (1) = 3
					clickY := msg.Y - 3

					// Adjust for top scroll indicator (shifts content down by 1)
					if hasTopIndicator {
						clickY--
					}

					// Calculate actual bookmark index
					if clickY >= 0 {
						newCursor := scrollOffset + clickY
						if newCursor >= 0 && newCursor < len(m.sortedBookmarkPaths) {
							// Check for double-click
							now := time.Now()
							isDoubleClick := !m.lastClickTime.IsZero() &&
								now.Sub(m.lastClickTime) <= m.doubleClickThreshold &&
								m.lastClickY == clickY &&
								m.lastClickMode == modeBookmarks &&
								m.bookmarksCursor == newCursor

							if isDoubleClick {
								// Double-click: navigate to bookmark
								m.lastClickTime = time.Time{} // Reset to prevent triple-click
								targetPath := m.sortedBookmarkPaths[m.bookmarksCursor]
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
									m.updatePreview()
									// Save config immediately after bookmark navigation to persist frecency
									if err := config.Save(m.config); err != nil {
										m.showError("Config Save Failed", fmt.Sprintf("Failed to save config: %v", err))
									}
									return m, nil
								}
							} else {
								// Single-click: select bookmark
								m.bookmarksCursor = newCursor
								m.lastClickTime = now
								m.lastClickY = clickY
								m.lastClickMode = modeBookmarks
							}
						}
					}
				}
			}
		}

	case tea.KeyMsg:
		switch m.mode {
		case modeErrorDialog:
			// Any key dismisses error dialog
			m.mode = modeNormal
			return m, nil

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
						// Save config immediately after bookmark navigation to persist frecency
						if err := config.Save(m.config); err != nil {
							m.showError("Config Save Failed", fmt.Sprintf("Failed to save config: %v", err))
						}
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
					if err := config.Save(m.config); err != nil {
						m.showError("Config Save Failed", fmt.Sprintf("Failed to save config: %v", err))
					}
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
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					selected := m.filteredFiles[m.cursor]
					if selected.name == ".." {
						m.mode = modeNormal
						return m, nil
					}

					// Add to undo stack before deleting
					undoEntry := undoItem{
						operation: "delete",
						path:      selected.path,
						wasDir:    selected.isDir,
					}

					// Try to move to trash (which we can potentially restore)
					trashPath, err := fileops.DeleteWithUndo(selected.path, selected.isDir)
					if err != nil {
						m.showError("Delete Failed", err.Error())
					} else {
						undoEntry.trashPath = trashPath
						m.addToUndo(undoEntry)
						m.statusMsg = fmt.Sprintf("Deleted: %s (press 'u' to undo)", selected.name)
						m.statusExpiry = time.Now().Add(3 * time.Second)
						m.loadFiles()
					}
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
						m.showError("Rename Failed", err.Error())
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
						m.showError("Create File Failed", err.Error())
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
						m.showError("Create Directory Failed", err.Error())
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

		case modeHelp:
			switch msg.String() {
			case "esc", "?":
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
			case "S":
				// Cycle through sort modes for search results
				m.sortBy = (m.sortBy + 1) % 4
				// Re-sort the filtered files
				m.sortSearchResults()
				sortNames := map[sortMode]string{
					sortByName: "Name",
					sortBySize: "Size",
					sortByDate: "Date",
					sortByType: "Type",
				}
				m.statusMsg = fmt.Sprintf("Sorted by: %s", sortNames[m.sortBy])
				m.statusExpiry = time.Now().Add(2 * time.Second)
				return m, nil

			case "/":
				// Start a fresh search - clear everything
				m.searchResultsLocked = false
				m.searchInput.SetValue("")
				m.filteredFiles = m.files
				m.searchMatches = [][]int{}
				m.cursor = 0
				m.scrollOffset = 0
				m.currentSearchType = searchFilename
				m.recursiveSearch = false
				m.cancelCurrentSearch()
				m.loading = false
				m.statusMsg = ""
				m.searchInput.Focus()
				m.updatePreview()
				return m, textinput.Blink

			case "esc":
				// Cancel any ongoing search
				m.cancelCurrentSearch()
				m.loading = false

				// If results are locked, clear everything and exit search
				if m.searchResultsLocked {
					m.searchResultsLocked = false
					m.searchInput.SetValue("")
					m.filteredFiles = m.files
					m.searchMatches = [][]int{}
					m.cursor = 0
					m.scrollOffset = 0
					m.mode = modeNormal
					m.recursiveSearch = false
					m.currentSearchType = searchFilename
					m.updatePreview()
					return m, nil
				}

				// Progressive clearing: clear query first, then exit search mode
				if m.searchInput.Value() != "" {
					// Clear the search query and restore all files
					m.searchInput.SetValue("")
					m.filteredFiles = m.files
					m.searchMatches = [][]int{}
					m.cursor = 0
					m.scrollOffset = 0
					m.updatePreview()
				} else {
					// Empty search - exit search mode completely
					m.mode = modeNormal
					m.recursiveSearch = false
					m.currentSearchType = searchFilename
					m.loading = false
				}
				return m, nil

			case "q":
				// Only handle 'q' as exit when results are locked
				if m.searchResultsLocked {
					// Cancel any ongoing search
					m.cancelCurrentSearch()
					m.loading = false

					// Clear everything and exit search (same as ESC when locked)
					m.searchResultsLocked = false
					m.searchInput.SetValue("")
					m.filteredFiles = m.files
					m.searchMatches = [][]int{}
					m.cursor = 0
					m.scrollOffset = 0
					m.mode = modeNormal
					m.recursiveSearch = false
					m.currentSearchType = searchFilename
					m.updatePreview()
					return m, nil
				}
				// When not locked, allow typing 'q' into search input
				m.searchInput, cmd = m.searchInput.Update(msg)
				filterCmd := m.updateFilter()
				m.updatePreview()
				if cmd != nil && filterCmd != nil {
					return m, tea.Batch(cmd, filterCmd)
				} else if filterCmd != nil {
					return m, filterCmd
				}
				return m, cmd

			case "enter":
				// If results are already locked, try to enter a directory
				if m.searchResultsLocked {
					if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
						selected := m.filteredFiles[m.cursor]
						if selected.isDir && selected.name != ".." {
							// Navigate into the directory and exit search mode
							m.addToHistory(selected.path)
							m.currentDir = selected.path
							m.cursor = 0
							m.scrollOffset = 0
							m.previewScroll = 0
							m.mode = modeNormal
							m.searchResultsLocked = false
							m.searchInput.SetValue("")
							m.recursiveSearch = false
							m.currentSearchType = searchFilename
							m.loadFiles()
							m.gitModified = git.GetModifiedFiles(m.currentDir)
							m.gitBranch = git.GetBranch(m.currentDir)
							m.updatePreview()
						} else if !selected.isDir {
							// Open file
							return m, m.openFile(selected.path)
						}
					}
					return m, nil
				}

				// Lock in search results - stay in search mode but disable input
				m.searchResultsLocked = true
				m.loading = false
				return m, nil

			case "tab":
				// Don't allow mode cycling when results are locked
				if m.searchResultsLocked {
					return m, nil
				}

				// Cycle through search modes: Current Dir -> Recursive -> Content -> Ultra -> Current Dir
				switch m.currentSearchType {
				case searchFilename:
					if !m.recursiveSearch {
						// Mode 1 -> Mode 2: Current Dir -> Recursive
						m.recursiveSearch = true
						m.statusMsg = "Recursive file search"
					} else {
						// Mode 2 -> Mode 3: Recursive -> Content
						m.currentSearchType = searchContent
						m.recursiveSearch = false
						m.statusMsg = "Content search"
					}
				case searchContent:
					// Mode 3 -> Mode 4: Content -> Ultra
					m.currentSearchType = searchUltra
					m.recursiveSearch = false
					m.statusMsg = "Ultra search (all drives)"
				case searchUltra:
					// Mode 4 -> Mode 1: Ultra -> Current Dir
					m.currentSearchType = searchFilename
					m.recursiveSearch = false
					m.statusMsg = "Current directory file search"
				default:
					// Fallback: reset to current dir
					m.currentSearchType = searchFilename
					m.recursiveSearch = false
					m.statusMsg = "Current directory file search"
				}
				m.statusExpiry = time.Now().Add(2 * time.Second)
				cmd = m.updateFilter()
				m.updatePreview()
				return m, cmd

			case "up", "down":
				// Navigate through filtered results
				if msg.String() == "up" {
					if m.cursor > 0 {
						m.cursor--
						m.updatePreview()
					}
				} else {
					if m.cursor < len(m.filteredFiles)-1 {
						m.cursor++
						m.updatePreview()
					}
				}
				return m, nil

			case "left", "right":
				// Move cursor in text input
				m.searchInput, cmd = m.searchInput.Update(msg)
				return m, cmd

			default:
				// Don't allow typing when results are locked
				if m.searchResultsLocked {
					return m, nil
				}

				// Pass all other keys to search input for typing
				m.searchInput, cmd = m.searchInput.Update(msg)

				// Trigger filter update (async for expensive searches)
				filterCmd := m.updateFilter()
				m.updatePreview()

				// Combine commands if both exist
				if cmd != nil && filterCmd != nil {
					return m, tea.Batch(cmd, filterCmd)
				} else if filterCmd != nil {
					return m, filterCmd
				}
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
			case "ctrl+c", "q":
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
				pageSize := (m.height - uiOverhead) / 2
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
				pageSize := (m.height - uiOverhead) / 2
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
				pageSize := m.height - uiOverhead
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
				pageSize := m.height - uiOverhead
				if pageSize < 1 {
					pageSize = 10
				}
				newPos := m.cursor - pageSize
				if newPos < 0 {
					newPos = 0
				}
				return m, m.moveCursor(newPos)

			case "w", "alt+up":
				// Scroll preview up
				if m.showPreview && m.previewScroll > 0 {
					m.previewScroll--
				}

			case "s", "alt+down":
				// Scroll preview down
				if m.showPreview && len(m.previewLines) > 0 {
					availableHeight := m.height - uiOverhead
					if availableHeight < 3 {
						availableHeight = 3
					}
					contentHeight := availableHeight - 2
					if contentHeight < 1 {
						contentHeight = 1
					}
					if m.previewScroll < len(m.previewLines)-contentHeight {
						m.previewScroll++
					}
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
				m.recursiveSearch = false     // Always start in current directory mode
				m.searchResultsLocked = false // Clear any locked results
				m.searchInput.SetValue("")
				m.searchInput.Placeholder = "Search..."
				m.searchInput.Focus()
				// Clear any previous search results
				m.filteredFiles = m.files
				m.searchMatches = [][]int{}
				m.cursor = 0
				m.scrollOffset = 0
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
							if err := config.Save(m.config); err != nil {
								m.showError("Config Save Failed", fmt.Sprintf("Failed to save config: %v", err))
							}
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
				// Copy current file to clipboard
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					current := m.filteredFiles[m.cursor]
					if current.name != ".." {
						m.clipboard = []string{current.path}
						m.clipboardOp = opCopy
						m.statusMsg = fmt.Sprintf("Copied: %s", current.name)
						m.statusExpiry = time.Now().Add(2 * time.Second)
					}
				}

			case "x":
				// Cut current file to clipboard
				if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
					current := m.filteredFiles[m.cursor]
					if current.name != ".." {
						m.clipboard = []string{current.path}
						m.clipboardOp = opCut
						m.statusMsg = fmt.Sprintf("Cut: %s", current.name)
						m.statusExpiry = time.Now().Add(2 * time.Second)
					}
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
						m.showError("Paste Failed", err.Error())
					} else {
						m.statusMsg = "Pasted successfully"
						m.statusExpiry = time.Now().Add(2 * time.Second)
						m.loadFiles()
					}
				}

			case "u":
				// Undo last deletion
				if len(m.undoStack) > 0 {
					lastUndo := m.undoStack[len(m.undoStack)-1]
					m.undoStack = m.undoStack[:len(m.undoStack)-1]

					if lastUndo.operation == "delete" && lastUndo.trashPath != "" {
						// Try to restore from trash
						if err := fileops.RestoreFromTrash(lastUndo.trashPath, lastUndo.path); err != nil {
							m.showError("Undo Failed", fmt.Sprintf("Could not restore %s: %v", filepath.Base(lastUndo.path), err))
						} else {
							m.statusMsg = fmt.Sprintf("Restored: %s", filepath.Base(lastUndo.path))
							m.statusExpiry = time.Now().Add(2 * time.Second)
							m.loadFiles()
						}
					}
				} else {
					m.statusMsg = "Nothing to undo"
					m.statusExpiry = time.Now().Add(2 * time.Second)
				}

			case "S":
				// Cycle through sort modes: Name → Size → Date → Type → Name...
				m.sortBy = (m.sortBy + 1) % 4
				m.sortFiles()

			case "?":
				// Show help screen
				m.mode = modeHelp

			case ",":
				// Open config file in editor
				configPath, err := config.GetConfigPath()
				if err != nil {
					m.statusMsg = fmt.Sprintf("Error: cannot get config path: %v", err)
					m.statusExpiry = time.Now().Add(3 * time.Second)
					return m, nil
				}
				return m, m.openInVSCode(configPath)
			}
		}
	}

	return m, nil
}
