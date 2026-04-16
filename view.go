package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/LFroesch/scout/internal/utils"
)

func (m *model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}

	// Show helpful message for very small terminals
	if m.width < minTerminalWidth || m.height < minTerminalHeight {
		warningStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true).
			Padding(1)
		return warningStyle.Render(fmt.Sprintf(
			"terminal too small: %dx%d\nminimum: %dx%d\n\nplease resize terminal or zoom out",
			m.width, m.height, minTerminalWidth, minTerminalHeight,
		))
	}

	var content string

	// Header
	header := m.renderHeader()

	// Main content area
	var mainContent string
	switch m.mode {
	case modeErrorDialog:
		mainContent = m.renderErrorDialog()
	case modeBookmarks:
		mainContent = m.renderBookmarksView()
	case modeHelp:
		mainContent = m.renderHelpView()
	default:
		if m.dualPane {
			// Dual pane mode
			leftPane := m.renderFileList(m.width / 2)
			// For now, right pane shows same directory
			rightPane := m.renderFileList(m.width / 2)
			mainContent = lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
		} else if m.showPreview {
			// Split view with preview - ensure both panels have same height
			availableHeight := m.height - uiOverhead
			if availableHeight < 3 {
				availableHeight = 3
			}
			panelHeight := availableHeight + 2

			fileList := m.renderFileList(m.width / 2)
			preview := m.renderPreview(m.width / 2)

			// Force both panels to exact same height
			fileListStyled := lipgloss.NewStyle().Height(panelHeight).Render(fileList)
			previewStyled := lipgloss.NewStyle().Height(panelHeight).Render(preview)

			mainContent = lipgloss.JoinHorizontal(lipgloss.Top, fileListStyled, previewStyled)
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

	// Overlay dialogs on top of the background content
	switch m.mode {
	case modeConfirmDelete:
		content = placeOverlay(content, m.renderConfirmDeleteView())
	case modeConfirmFileDelete:
		content = placeOverlay(content, m.renderConfirmFileDeleteView())
	case modeRename:
		content = placeOverlay(content, m.renderRenameDialog())
	case modeCreateFile:
		content = placeOverlay(content, m.renderCreateFileDialog())
	case modeCreateDir:
		content = placeOverlay(content, m.renderCreateDirDialog())
	}

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
		title = "🔍 scout - bookmarks (esc to exit)"
	} else {
		title = fmt.Sprintf("🔍 scout - %s", m.currentDir)
	}

	// Show search query only when in search mode
	if m.mode == modeSearch {
		// Build search parts with different colors
		purpleStyle := lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("214"))

		grayStyle := lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252"))

		yellowStyle := lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("226"))

		// Pick search mode label based on available width
		var searchMode string
		switch m.currentSearchType {
		case searchUltra:
			if m.width < 80 {
				searchMode = "ULTRA"
			} else {
				searchMode = "ULTRA SEARCH"
			}
		case searchContent:
			if m.width < 80 {
				searchMode = "CONTENT"
			} else {
				searchMode = "CONTENT SEARCH"
			}
		case searchRecursive:
			if m.width < 80 {
				searchMode = "RECURSIVE"
			} else {
				searchMode = "RECURSIVE SEARCH"
			}
		case searchFilename:
			if m.recursiveSearch {
				if m.width < 80 {
					searchMode = "RECURSIVE"
				} else {
					searchMode = "RECURSIVE SEARCH"
				}
			} else {
				if m.width < 80 {
					searchMode = "DIR"
				} else {
					searchMode = "CURRENT DIR SEARCH"
				}
			}
		default:
			searchMode = "SEARCH"
		}

		// Add name/path indicator for recursive modes
		if m.searchNameOnly && (m.recursiveSearch || m.currentSearchType == searchUltra) {
			searchMode += " (name)"
		}

		searchLabel := fmt.Sprintf("🔍 %s: ", searchMode)
		searchValue := m.searchInput.Value()

		// Show cursor in search with yellow color
		cursorPos := m.searchInput.Position()
		displayValue := searchValue
		var cursorChar string
		if len(displayValue) == 0 {
			cursorChar = "_"
		} else if cursorPos < len(displayValue) {
			cursorChar = "|"
			displayValue = displayValue[:cursorPos] + "{{CURSOR}}" + displayValue[cursorPos:]
		} else {
			cursorChar = "|"
			displayValue = displayValue + "{{CURSOR}}"
		}

		// Render cursor-bearing text
		searchLabelRendered := purpleStyle.Render(searchLabel)
		parts := strings.Split(displayValue, "{{CURSOR}}")
		var displayRendered string
		if len(parts) == 2 {
			displayRendered = grayStyle.Render(parts[0]) + yellowStyle.Render(cursorChar) + grayStyle.Render(parts[1])
		} else {
			displayRendered = grayStyle.Render(displayValue)
		}

		// Pick hint based on width
		var hint string
		if m.searchResultsLocked {
			if m.width >= 120 {
				hint = " [🔒] (enter: go to | esc: exit | /: new search | ctrl+p: preview)"
			} else if m.width >= 100 {
				hint = " [🔒] (enter: go to | esc: exit | /: new search)"
			} else if m.width >= 75 {
				hint = " [🔒]"
			}
		} else {
			if m.width >= 140 {
				hint = " (tab: modes | ctrl+n: name/path | ctrl+p: preview | esc: clear/exit)"
			} else if m.width >= 120 {
				hint = " (tab: modes | ctrl+n: name/path | esc: clear/exit)"
			} else if m.width >= 100 {
				hint = " (tab: cycle modes | esc: clear/exit | enter: lock results)"
			} else if m.width >= 75 {
				hint = " (tab: modes)"
			}
		}
		hintRendered := purpleStyle.Render(hint)

		// Build the search bar as a single line
		searchText := searchLabelRendered + displayRendered + hintRendered
		searchTextLen := lipgloss.Width(searchText)

		// Decide layout: side-by-side title+search, or search-only
		if searchTextLen+22 <= m.width {
			// Enough room: title on left, search on right
			titleWidth := m.width - searchTextLen - 2
			// Truncate the title path to fit
			titleRunes := []rune(title)
			if lipgloss.Width(title) > titleWidth-2 {
				// Right-truncate the path
				truncated := ""
				for _, r := range titleRunes {
					if lipgloss.Width(truncated+string(r)+"...") > titleWidth-2 {
						break
					}
					truncated += string(r)
				}
				title = truncated + "..."
			}

			baseStyle := lipgloss.NewStyle().
				Bold(true).
				Background(lipgloss.Color("235")).
				Foreground(lipgloss.Color("99"))

			titlePart := baseStyle.Width(titleWidth).Padding(0, 1).Render(title)
			title = lipgloss.JoinHorizontal(lipgloss.Top, titlePart, searchText)
		} else {
			// Not enough room — search bar is the entire header
			title = searchText
		}

		return lipgloss.NewStyle().
			Bold(true).
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("99")).
			Width(m.width).
			MaxWidth(m.width).
			MaxHeight(1).
			Render(title)
	}
	return titleStyle.Render(title)
}

func (m *model) renderStatusBar() string {
	// Normal status bar
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Width(m.width)

	// Style for purple numbers and keybinds (inline to avoid vertical stacking)
	purpleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("99")).
		Background(lipgloss.Color("235")).
		Bold(true).
		Inline(true)

	// Style for white text values
	whiteStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("235")).
		Inline(true)

	var statusText string
	var rightSide string

	// Bookmark mode has special status bar
	if m.mode == modeBookmarks {
		if len(m.sortedBookmarkPaths) > 0 && m.bookmarksCursor < len(m.sortedBookmarkPaths) {
			// Show current bookmark path on left
			statusText = whiteStyle.Render(m.sortedBookmarkPaths[m.bookmarksCursor])
		} else {
			statusText = whiteStyle.Render("no bookmarks")
		}
		// Show keybinds on right
		rightSide = purpleStyle.Render("enter") + whiteStyle.Render(": open | ") + purpleStyle.Render("o") + whiteStyle.Render(": vs code | ") + purpleStyle.Render("d") + whiteStyle.Render(": delete | ") + purpleStyle.Render("esc") + whiteStyle.Render(": back")
	} else if m.mode == modeHelp {
		statusText = whiteStyle.Render("help")
		rightSide = purpleStyle.Render("j/k") + whiteStyle.Render(": scroll | ") + purpleStyle.Render("g/G") + whiteStyle.Render(": top/bottom | ") + purpleStyle.Render("q/esc") + whiteStyle.Render(": close")
	} else {
		// File count and position info
		if len(m.filteredFiles) > 0 {
			fileCountInfo := purpleStyle.Render(fmt.Sprintf("%d", m.cursor+1)) + whiteStyle.Render("/") + purpleStyle.Render(fmt.Sprintf("%d", len(m.filteredFiles)))
			statusText = fileCountInfo
		} else {
			fileCountInfo := purpleStyle.Render("0") + whiteStyle.Render("/") + purpleStyle.Render("0")
			statusText = fileCountInfo
		}

		// Git info
		if m.gitBranch != "" {
			statusText += whiteStyle.Render(" | ") + purpleStyle.Render("branch:") + whiteStyle.Render(" "+m.gitBranch)
		}

		// Clipboard info
		if len(m.clipboard) > 0 {
			opStr := "copy"
			if m.clipboardOp == opCut {
				opStr = "cut"
			}
			firstName := filepath.Base(m.clipboard[0])
			if len(firstName) > 18 {
				firstName = firstName[:15] + "..."
			}
			clipInfo := opStr + ": " + firstName
			if len(m.clipboard) > 1 {
				clipInfo += fmt.Sprintf(" +%d", len(m.clipboard)-1)
			}
			statusText += whiteStyle.Render(" | ") + purpleStyle.Render(clipInfo)
		}

		// Status message (shows drive info during loading or other temporary messages)
		if m.statusMsg != "" {
			statusText += whiteStyle.Render(" | " + m.statusMsg)
		} else if m.loading {
			// Fallback loading indicator if statusMsg is not set
			orangeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Background(lipgloss.Color("235")).Bold(true).Inline(true)
			if m.scannedFiles > 0 {
				statusText += whiteStyle.Render(" | ") + orangeStyle.Render("searching... ") + whiteStyle.Render("(") + purpleStyle.Render(fmt.Sprintf("%d", m.scannedFiles)) + whiteStyle.Render(" files scanned)")
			} else {
				statusText += whiteStyle.Render(" | ") + orangeStyle.Render("searching...")
			}
		}

		// Sort mode indicator (on left side)
		sortNames := map[sortMode]string{
			sortByName: "name",
			sortBySize: "size",
			sortByDate: "date",
			sortByType: "type",
		}
		statusText += whiteStyle.Render(" | ") + purpleStyle.Render("sort:") + whiteStyle.Render(" ") + whiteStyle.Render(sortNames[m.sortBy]) + whiteStyle.Render(" (") + purpleStyle.Render("S") + whiteStyle.Render(")")

		// Dynamic hints based on selected item (on right side)
		if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
			selected := m.filteredFiles[m.cursor]
			if m.width < 90 {
				rightSide = purpleStyle.Render("?") + whiteStyle.Render(" help")
			} else if m.mode == modeSearch && m.searchResultsLocked {
				// Search mode (locked): enter navigates for both dirs and files
				if selected.isDir {
					rightSide = purpleStyle.Render("enter") + whiteStyle.Render(": go to | ") + purpleStyle.Render("o") + whiteStyle.Render(": editor | ") + purpleStyle.Render("?") + whiteStyle.Render(" for help")
				} else {
					rightSide = purpleStyle.Render("enter") + whiteStyle.Render(": go to | ") + purpleStyle.Render("o") + whiteStyle.Render(": open | ") + purpleStyle.Render("?") + whiteStyle.Render(" for help")
				}
			} else if selected.name == ".." {
				rightSide = purpleStyle.Render("enter") + whiteStyle.Render(": back | ") + purpleStyle.Render("?") + whiteStyle.Render(" for help")
			} else if selected.isDir {
				rightSide = purpleStyle.Render("enter") + whiteStyle.Render(": open | ") + purpleStyle.Render("o") + whiteStyle.Render(": vs code | ") + purpleStyle.Render("?") + whiteStyle.Render(" for help")
			} else {
				rightSide = purpleStyle.Render("enter") + whiteStyle.Render(": open | ") + purpleStyle.Render("o") + whiteStyle.Render(": editor | ") + purpleStyle.Render("f") + whiteStyle.Render(": parent dir | ") + purpleStyle.Render("?") + whiteStyle.Render(" for help")
			}
		} else {
			rightSide = purpleStyle.Render("?") + whiteStyle.Render(" for help")
		}
	}

	totalWidth := m.width - 2 // Account for padding
	leftWidth := lipgloss.Width(statusText)
	rightWidth := lipgloss.Width(rightSide)
	padding := totalWidth - leftWidth - rightWidth - 3

	// If not enough room for right side, truncate or hide it
	if padding < 1 {
		// Try showing just "? help" as minimal right side
		minRight := purpleStyle.Render("?") + whiteStyle.Render(" help")
		minRightWidth := lipgloss.Width(minRight)
		minPadding := totalWidth - leftWidth - minRightWidth - 3
		if minPadding >= 1 {
			rightSide = minRight
			padding = minPadding
		} else {
			// No room at all — drop right side entirely
			rightSide = ""
			padding = totalWidth - leftWidth - 3
			if padding < 0 {
				padding = 0
			}
		}
	}
	if rightSide != "" {
		statusText += strings.Repeat(whiteStyle.Render(" "), padding) + rightSide
	}

	return statusStyle.Render(statusText)
}

// renderFileList renders the file list panel with the given width
func (m *model) renderFileList(width int) string {
	// Calculate available height for file list
	availableHeight := m.height - uiOverhead // Account for header, status, borders, padding
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Reserve space for internal header (1 line)
	contentHeight := availableHeight - 1
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Create header with current directory info
	dirName := filepath.Base(m.currentDir)
	if m.currentDir == "/" {
		dirName = "/"
	}

	var searchModeIndicator string
	if m.mode == modeSearch {
		switch m.currentSearchType {
		case searchUltra:
			searchModeIndicator = " [ULTRA SEARCH]"
		case searchContent:
			searchModeIndicator = " [CONTENT SEARCH]"
		case searchRecursive:
			searchModeIndicator = " [RECURSIVE SEARCH]"
		case searchFilename:
			if m.recursiveSearch {
				searchModeIndicator = " [RECURSIVE SEARCH]"
			} else {
				searchModeIndicator = " [CURRENT DIR SEARCH]"
			}
		default:
			searchModeIndicator = " [CURRENT DIR SEARCH]"
		}
	}

	headerBaseStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105"))

	var header string
	if searchModeIndicator != "" {
		searchIndicatorStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214"))
		header = headerBaseStyle.Render(fmt.Sprintf("📁 %s", dirName)) + searchIndicatorStyle.Render(searchModeIndicator)
	} else {
		header = headerBaseStyle.Render(fmt.Sprintf("📁 %s", dirName))
	}

	// Calculate how many items we can show (reserve space for potential scroll indicators)
	maxItems := contentHeight
	if maxItems < 1 {
		maxItems = 1
	}

	// Check if we need scroll indicators FIRST (before adjusting cursor)
	hasTopIndicator := m.scrollOffset > 0
	hasBottomIndicator := m.scrollOffset+maxItems < len(m.filteredFiles)

	// Reduce maxItems if we need scroll indicators
	actualMaxItems := maxItems
	if hasTopIndicator {
		actualMaxItems--
	}
	if hasBottomIndicator {
		actualMaxItems--
	}
	if actualMaxItems < 1 {
		actualMaxItems = 1
	}

	// NOW adjust scroll offset to keep cursor visible (using actualMaxItems)
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+actualMaxItems {
		m.scrollOffset = m.cursor - actualMaxItems + 1
	}

	// Recalculate scroll indicators after adjusting offset
	hasTopIndicator = m.scrollOffset > 0
	hasBottomIndicator = m.scrollOffset+actualMaxItems < len(m.filteredFiles)

	listStyle := lipgloss.NewStyle().
		Padding(0, 1)

	var items []string

	// Calculate visible range using actualMaxItems
	startIdx := m.scrollOffset
	endIdx := m.scrollOffset + actualMaxItems
	if endIdx > len(m.filteredFiles) {
		endIdx = len(m.filteredFiles)
	}

	for i := startIdx; i < endIdx && i < len(m.filteredFiles); i++ {
		item := m.filteredFiles[i]

		// Icon
		icon := "📄"
		if item.isDir {
			if item.name == ".." {
				icon = "⤴"
			} else {
				icon = "📁"
			}
		} else {
			icon = utils.GetFileIcon(item.name)
		}

		// Git status and symlink indicator (grouped together)
		// Build with selection-aware styling so background color is consistent
		isSelected := i == m.cursor
		gitStatus := ""
		if m.gitModified[item.path] {
			modifiedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
			if isSelected {
				modifiedStyle = modifiedStyle.Background(lipgloss.Color("57"))
			}
			gitStatus = " " + modifiedStyle.Render("[M]")
		}
		if item.isSymlink {
			symlinkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("cyan"))
			if isSelected {
				symlinkStyle = symlinkStyle.Background(lipgloss.Color("57"))
			}
			gitStatus += " " + symlinkStyle.Render("[→]")
		}

		// Format item with highlighting if in search mode
		name := item.name
		displayName := name

		// Apply search highlighting if we have match positions
		if m.mode == modeSearch && i < len(m.searchMatches) && len(m.searchMatches[i]) > 0 {
			displayName = utils.HighlightMatches(name, m.searchMatches[i])
		}

		// Calculate total available width (accounting for borders and padding)
		totalWidth := width - 4
		if totalWidth < 20 {
			totalWidth = 20 // Minimum usable width
		}

		// Add file size for files (right-aligned), but only if we have space
		sizeStr := ""
		sizeWidth := 0
		const fixedSizeWidth = 8
		minSpaceNeeded := 30 // icon(2) + name(10) + gitStatus(8) + padding(10)
		if !item.isDir && item.name != ".." && totalWidth > minSpaceNeeded {
			sizeStr = utils.FormatFileSizeColored(item.size)
			actualSizeW := lipgloss.Width(sizeStr)
			if actualSizeW < fixedSizeWidth {
				sizeStr = strings.Repeat(" ", fixedSizeWidth-actualSizeW) + sizeStr
			}
			sizeWidth = fixedSizeWidth
		}
		if isSelected && sizeStr != "" {
			sizeStr = lipgloss.NewStyle().Background(lipgloss.Color("57")).Inline(true).Render(sizeStr)
		}

		// Add modification date when there's enough space
		dateStr := ""
		dateWidth := 0
		if !item.modTime.IsZero() && totalWidth > 72 {
			timeStr := item.modTime.Format("3:04PM")
			dateStr = item.modTime.Format("01/02/06 ") + fmt.Sprintf("%7s", timeStr)
			dateWidth = 16 // always fixed
		}

		// Calculate available width for filename
		// Reserve: icon(2) + space(1) + gitStatus(~8) + date + size(sizeWidth) + separators + padding(~10)
		rightColWidth := sizeWidth
		if dateWidth > 0 {
			rightColWidth += dateWidth + 2 // "  " separator
		}
		reservedSpace := 2 + 1 + 8 + rightColWidth + 10
		maxNameLen := totalWidth - reservedSpace

		// Progressively drop columns if name would be too short
		if maxNameLen < 8 && dateWidth > 0 {
			// Drop date column first
			dateStr = ""
			dateWidth = 0
			rightColWidth = sizeWidth
			reservedSpace = 2 + 1 + 8 + rightColWidth + 10
			maxNameLen = totalWidth - reservedSpace
		}
		if maxNameLen < 8 && sizeWidth > 0 {
			// Drop size column too
			sizeStr = ""
			sizeWidth = 0
			rightColWidth = 0
			maxNameLen = totalWidth - 15 // Just icon + gitStatus + padding
		}
		if maxNameLen < 8 {
			maxNameLen = 8
		}

		// Truncate name if needed
		if lipgloss.Width(name) > maxNameLen {
			if m.mode == modeSearch && strings.Contains(name, string(filepath.Separator)) {
				// Search results: left-truncate to preserve filename
				runes := []rune(name)
				truncated := ""
				for i := len(runes) - 1; i >= 0; i-- {
					test := string(runes[i]) + truncated
					if lipgloss.Width("..."+test) > maxNameLen {
						break
					}
					truncated = test
				}
				if truncated == "" && len(runes) > 0 {
					truncated = string(runes[len(runes)-1])
				}
				displayName = "..." + truncated
			} else {
				// Normal mode: right-truncate
				runes := []rune(name)
				truncated := ""
				for _, r := range runes {
					if lipgloss.Width(truncated+string(r)+"...") > maxNameLen {
						break
					}
					truncated += string(r)
				}
				if truncated == "" && len(runes) > 0 {
					truncated = string(runes[0])
				}
				displayName = truncated + "..."
			}
		}

		// Build left side: icon + name + gitStatus (which includes symlink indicator)
		leftSide := fmt.Sprintf("%s %s%s", icon, displayName, gitStatus)
		leftWidth := lipgloss.Width(leftSide)

		// Build right side: size + date
		rightStr := ""
		totalRightWidth := 0
		if dateStr != "" && sizeStr != "" {
			var dateStyled string
			if isSelected {
				dateStyled = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Background(lipgloss.Color("57")).Inline(true).Render(dateStr)
			} else {
				dateStyled = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Inline(true).Render(dateStr)
			}
			sep := "  "
			if isSelected {
				sep = lipgloss.NewStyle().Background(lipgloss.Color("57")).Render("  ")
			}
			rightStr = sizeStr + sep + dateStyled
			totalRightWidth = sizeWidth + 2 + dateWidth
		} else if dateStr != "" {
			if isSelected {
				rightStr = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Background(lipgloss.Color("57")).Inline(true).Render(dateStr)
			} else {
				rightStr = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Inline(true).Render(dateStr)
			}
			totalRightWidth = dateWidth
		} else {
			rightStr = sizeStr
			totalRightWidth = sizeWidth
		}

		// Calculate padding to push right side to the right edge
		padding := totalWidth - leftWidth - totalRightWidth
		if padding < 1 {
			padding = 1
		}
		if padding > totalWidth {
			// Overflow protection - just use minimum padding
			padding = 1
			rightStr = sizeStr // Fall back to just size if overflowing
			totalRightWidth = sizeWidth
		}

		// Build the line with right side right-aligned
		var paddingStr string
		if isSelected {
			paddingStr = lipgloss.NewStyle().Background(lipgloss.Color("57")).Render(strings.Repeat(" ", padding))
		} else {
			paddingStr = strings.Repeat(" ", padding)
		}
		line := leftSide + paddingStr + rightStr

		// Style based on selection (don't set Width here - we already calculated exact spacing)
		if i == m.cursor {
			selectedStyle := lipgloss.NewStyle().
				Background(lipgloss.Color("57")).
				Foreground(lipgloss.Color("230"))
			line = selectedStyle.Render(line)
		} else {
			normalStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))
			line = normalStyle.Render(line)
		}

		items = append(items, line)
	}

	// Add scroll indicators (already accounted for in height)
	if hasTopIndicator {
		items = append([]string{"▲ more files above..."}, items...)
	}
	if hasBottomIndicator {
		items = append(items, "▼ more files below...")
	}

	fileList := listStyle.Render(strings.Join(items, "\n"))

	// Combine header and file list with border
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(width - 2).
		Height(availableHeight + 1)

	combined := header + "\n" + fileList
	return borderStyle.Render(combined)
}

func (m *model) renderPreview(width int) string {
	availableHeight := m.height - uiOverhead
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Reserve space for internal header (1 line)
	contentHeight := availableHeight - 1
	if contentHeight < 1 {
		contentHeight = 1
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Width(width - 4)

	header := headerStyle.Render("👁 preview")

	previewStyle := lipgloss.NewStyle().
		Width(width-4).
		Padding(0, 1)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(width - 2).
		Height(availableHeight + 1)

	var content string
	if len(m.previewLines) == 0 {
		content = "no preview available"
	} else {
		// Calculate visible range
		startIdx := m.previewScroll
		endIdx := m.previewScroll + contentHeight

		// Check if we need scroll indicators
		hasTopIndicator := startIdx > 0
		hasBottomIndicator := endIdx < len(m.previewLines)

		// Adjust for indicators
		if hasTopIndicator {
			contentHeight--
		}
		if hasBottomIndicator {
			contentHeight--
		}
		if contentHeight < 1 {
			contentHeight = 1
		}

		endIdx = startIdx + contentHeight
		if endIdx > len(m.previewLines) {
			endIdx = len(m.previewLines)
		}

		var lines []string
		if hasTopIndicator {
			lines = append(lines, "▲")
		}

		// Word-wrap long lines to fit panel width without losing content
		maxLineWidth := width - 6 // Account for borders and padding
		for _, line := range m.previewLines[startIdx:endIdx] {
			runes := []rune(line)
			for len(runes) > maxLineWidth {
				// Find last space within maxLineWidth for word break
				breakAt := maxLineWidth
				for i := maxLineWidth - 1; i > 0; i-- {
					if runes[i] == ' ' || runes[i] == '\t' {
						breakAt = i + 1
						break
					}
				}
				lines = append(lines, string(runes[:breakAt]))
				runes = runes[breakAt:]
			}
			lines = append(lines, string(runes))
		}

		if hasBottomIndicator {
			lines = append(lines, "▼")
		}

		// STRICT HEIGHT ENFORCEMENT: Ensure we never exceed contentHeight
		// Reserve space for header + separator (2 lines total)
		maxLines := availableHeight - 1
		if len(lines) > maxLines {
			lines = lines[:maxLines]
		}

		content = strings.Join(lines, "\n")
	}

	previewContent := previewStyle.Render(content)
	combined := header + "\n" + previewContent
	return borderStyle.Render(combined)
}

func (m model) renderBookmarksView() string {
	availableHeight := m.height - uiOverhead
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Reserve space for scroll indicators
	contentHeight := availableHeight - 1
	if contentHeight < 1 {
		contentHeight = 1
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Width(m.width - 4)

	header := headerStyle.Render("📚 bookmarks")

	listStyle := lipgloss.NewStyle().
		Padding(0, 1)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(m.width - 2).
		Height(availableHeight + 1)

	var bookmarkItems []string

	if len(m.sortedBookmarkPaths) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Padding(1, 0)
		bookmarkItems = []string{emptyStyle.Render("no bookmarks yet. press 'B' in normal mode to add current directory.")}
	} else {
		// Calculate scroll position
		maxItems := contentHeight
		if maxItems < 1 {
			maxItems = 1
		}

		// Check if we need scroll indicators
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
		if actualMaxItems < 1 {
			actualMaxItems = 1
		}

		// Recalculate scroll offset with adjusted max
		if m.bookmarksCursor >= scrollOffset+actualMaxItems {
			scrollOffset = m.bookmarksCursor - actualMaxItems + 1
		}
		if m.bookmarksCursor < scrollOffset {
			scrollOffset = m.bookmarksCursor
		}

		// Recalculate indicators
		hasTopIndicator = scrollOffset > 0
		hasBottomIndicator = scrollOffset+actualMaxItems < len(m.sortedBookmarkPaths)

		// Add top indicator
		if hasTopIndicator {
			bookmarkItems = append(bookmarkItems, "▲ more bookmarks above...")
		}

		// Calculate visible range
		startIdx := scrollOffset
		endIdx := scrollOffset + actualMaxItems
		if endIdx > len(m.sortedBookmarkPaths) {
			endIdx = len(m.sortedBookmarkPaths)
		}

		// Render visible bookmarks
		for i := startIdx; i < endIdx; i++ {
			path := m.sortedBookmarkPaths[i]
			name := filepath.Base(path)
			if name == "" || name == "." {
				name = path
			}

			icon := "📁"

			// Frecency score (left side, before icon)
			frecencyStr := ""
			if score, ok := m.config.Frecency[path]; ok && score > 0 {
				frecencyStr = fmt.Sprintf("×%-3d ", score)
			} else {
				frecencyStr = "     " // 5 spaces to align with "×99 "
			}

			// Calculate available width for path based on name length
			nameWidth := lipgloss.Width(name)
			frecencyWidth := lipgloss.Width(frecencyStr)
			// icon(2) + spaces(3) + parens(2) + frecency + name + some padding
			usedWidth := frecencyWidth + 2 + 1 + nameWidth + 3 + 4
			maxPathLen := (m.width - 4) - usedWidth
			if maxPathLen < 10 {
				maxPathLen = 10
			}
			displayPath := path
			if lipgloss.Width(path) > maxPathLen {
				runes := []rune(path)
				truncated := "..."
				for j := len(runes) - 1; j >= 0; j-- {
					if lipgloss.Width(truncated) >= maxPathLen {
						break
					}
					truncated = string(runes[j]) + truncated
				}
				displayPath = "..." + truncated[3:]
			}

			// Truncate name too if terminal is very narrow
			maxNameLen := (m.width - 4) - frecencyWidth - 10
			if maxNameLen < 8 {
				maxNameLen = 8
			}
			displayName := name
			if lipgloss.Width(name) > maxNameLen {
				runes := []rune(name)
				truncated := ""
				for _, r := range runes {
					if lipgloss.Width(truncated+string(r)+"...") > maxNameLen {
						break
					}
					truncated += string(r)
				}
				displayName = truncated + "..."
			}

			// Build line without colors: frecency + icon + name + path
			line := fmt.Sprintf("%s%s %s (%s)", frecencyStr, icon, displayName, displayPath)

			// Apply selection style with full width OR normal style with gray path
			if i == m.bookmarksCursor {
				selectedStyle := lipgloss.NewStyle().
					Background(lipgloss.Color("57")).
					Foreground(lipgloss.Color("230")).
					Width(m.width - 4)
				line = selectedStyle.Render(line)
			} else {
				// For normal (non-selected) lines, style frecency and path separately
				normalFrecencyStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("244"))
				pathStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("244"))

				styledFrecency := normalFrecencyStyle.Render(frecencyStr)
				styledPath := pathStyle.Render(fmt.Sprintf("(%s)", displayPath))
				line = fmt.Sprintf("%s%s %s %s", styledFrecency, icon, name, styledPath)
			}

			bookmarkItems = append(bookmarkItems, line)
		}

		// Add bottom indicator
		if hasBottomIndicator {
			bookmarkItems = append(bookmarkItems, "▼ more bookmarks below...")
		}
	}

	content := listStyle.Render(strings.Join(bookmarkItems, "\n"))
	combined := header + "\n" + content
	return borderStyle.Render(combined)
}

func (m model) renderConfirmDeleteView() string {
	dialogWidth := 60
	if m.width-4 < dialogWidth {
		dialogWidth = m.width - 4
	}
	dialogHeight := 8

	if m.deleteBookmarkIndex < 0 || m.deleteBookmarkIndex >= len(m.config.Bookmarks) {
		return "Error: Invalid bookmark index"
	}

	bookmarkPath := m.config.Bookmarks[m.deleteBookmarkIndex]
	bookmarkName := filepath.Base(bookmarkPath)

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Background(lipgloss.Color("232")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		Background(lipgloss.Color("232"))

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(1, 0).
		Background(lipgloss.Color("232"))

	promptStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(1, 0).
		Background(lipgloss.Color("232"))

	title := titleStyle.Render("DELETE BOOKMARK?")
	content := contentStyle.Render(fmt.Sprintf("are you sure you want to delete this bookmark?\n\n%s\n(%s)", bookmarkName, bookmarkPath))
	prompt := promptStyle.Render("press 'y' to confirm, 'n' or esc to cancel")

	dialog := title + "\n" + content + "\n" + prompt
	return dialogStyle.Render(dialog)
}

func (m model) renderConfirmFileDeleteView() string {
	dialogWidth := 60
	if m.width-4 < dialogWidth {
		dialogWidth = m.width - 4
	}
	dialogHeight := 8

	if len(m.filteredFiles) == 0 || m.cursor >= len(m.filteredFiles) {
		return "Error: No file selected"
	}

	file := m.filteredFiles[m.cursor]

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Background(lipgloss.Color("232")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		Background(lipgloss.Color("232"))

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(1, 0).
		Background(lipgloss.Color("232"))

	promptStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(1, 0).
		Background(lipgloss.Color("232"))

	fileType := "file"
	if file.isDir {
		fileType = "directory"
	}

	title := titleStyle.Render(fmt.Sprintf("DELETE %s?", strings.ToUpper(fileType)))
	content := contentStyle.Render(fmt.Sprintf("are you sure you want to delete:\n\n%s\n\nthis will move it to trash if available.", file.name))
	prompt := promptStyle.Render("press 'y' to confirm, 'n' or esc to cancel")

	dialog := title + "\n" + content + "\n" + prompt
	return dialogStyle.Render(dialog)
}

func (m model) renderRenameDialog() string {
	dialogWidth := 60
	if m.width-4 < dialogWidth {
		dialogWidth = m.width - 4
	}
	dialogHeight := 8

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("105")).
		Background(lipgloss.Color("232")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Background(lipgloss.Color("232"))

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(1, 0).
		Background(lipgloss.Color("232"))

	title := titleStyle.Render("✏️  RENAME")
	content := contentStyle.Render("enter new name:")
	inputView := m.textInput.View()

	dialog := title + "\n" + content + "\n" + inputView
	return dialogStyle.Render(dialog)
}

func (m model) renderCreateFileDialog() string {
	dialogWidth := 60
	if m.width-4 < dialogWidth {
		dialogWidth = m.width - 4
	}
	dialogHeight := 8

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("105")).
		Background(lipgloss.Color("232")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Background(lipgloss.Color("232"))

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(1, 0).
		Background(lipgloss.Color("232"))

	title := titleStyle.Render("📄 CREATE NEW FILE")
	content := contentStyle.Render("enter filename:")
	inputView := m.textInput.View()

	dialog := title + "\n" + content + "\n" + inputView
	return dialogStyle.Render(dialog)
}

func (m model) renderCreateDirDialog() string {
	dialogWidth := 60
	if m.width-4 < dialogWidth {
		dialogWidth = m.width - 4
	}
	dialogHeight := 8

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("105")).
		Background(lipgloss.Color("232")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Background(lipgloss.Color("232"))

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(1, 0).
		Background(lipgloss.Color("232"))

	title := titleStyle.Render("📁 CREATE NEW DIRECTORY")
	content := contentStyle.Render("enter directory name:")
	inputView := m.textInput.View()

	dialog := title + "\n" + content + "\n" + inputView
	return dialogStyle.Render(dialog)
}

func (m *model) renderHelpView() string {
	availableHeight := m.height - uiOverhead
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Reserve space for scroll indicators
	contentHeight := availableHeight - 1
	if contentHeight < 1 {
		contentHeight = 1
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Width(m.width - 4)

	header := headerStyle.Render("❓ HELP")

	listStyle := lipgloss.NewStyle().
		Width(m.width-4).
		Padding(0, 1)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(m.width - 2).
		Height(availableHeight + 1)

	// Build help content
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("105")).
		Bold(true)

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true)

	var allHelpContent []string

	// helpLine pads the key to a fixed column width so descriptions align
	const keyColWidth = 14
	helpLine := func(key, desc string) string {
		pad := keyColWidth - lipgloss.Width(key)
		if pad < 1 {
			pad = 1
		}
		return "  " + keyStyle.Render(key) + strings.Repeat(" ", pad) + desc
	}

	// Navigation section
	allHelpContent = append(allHelpContent, sectionStyle.Render("NAVIGATION:"))
	allHelpContent = append(allHelpContent, helpLine("j / ↓", "move down"))
	allHelpContent = append(allHelpContent, helpLine("k / ↑", "move up"))
	allHelpContent = append(allHelpContent, helpLine("enter / l / →", "enter directory / open file"))
	allHelpContent = append(allHelpContent, helpLine("esc / h / ←", "go to parent directory"))
	allHelpContent = append(allHelpContent, helpLine("f", "navigate to dir/parent (exit search)"))
	allHelpContent = append(allHelpContent, helpLine("middle-click", "navigate to clicked dir/parent"))
	allHelpContent = append(allHelpContent, helpLine("g", "go to top"))
	allHelpContent = append(allHelpContent, helpLine("G", "go to bottom"))
	allHelpContent = append(allHelpContent, helpLine("ctrl+d", "half-page down"))
	allHelpContent = append(allHelpContent, helpLine("ctrl+u", "half-page up"))
	allHelpContent = append(allHelpContent, helpLine("ctrl+f", "full-page down"))
	allHelpContent = append(allHelpContent, helpLine("ctrl+b", "full-page up"))
	allHelpContent = append(allHelpContent, helpLine("~", "jump to home directory"))
	allHelpContent = append(allHelpContent, helpLine("`", "jump to /mnt/c (wsl)"))
	allHelpContent = append(allHelpContent, "")

	// Preview Scrolling section
	allHelpContent = append(allHelpContent, sectionStyle.Render("PREVIEW SCROLLING:"))
	allHelpContent = append(allHelpContent, helpLine("s / alt+↓", "scroll preview down"))
	allHelpContent = append(allHelpContent, helpLine("w / alt+↑", "scroll preview up"))
	allHelpContent = append(allHelpContent, "")

	// File Operations section
	allHelpContent = append(allHelpContent, sectionStyle.Render("FILE OPERATIONS:"))
	allHelpContent = append(allHelpContent, helpLine("o", "open selected in editor/vs code"))
	allHelpContent = append(allHelpContent, helpLine("O", "open current directory in vs code"))
	allHelpContent = append(allHelpContent, helpLine("R", "rename file/directory"))
	allHelpContent = append(allHelpContent, helpLine("D", "delete file/directory"))
	allHelpContent = append(allHelpContent, helpLine("N", "create new file"))
	allHelpContent = append(allHelpContent, helpLine("M", "create new directory"))
	allHelpContent = append(allHelpContent, helpLine("r", "refresh current view"))
	allHelpContent = append(allHelpContent, "")

	// Clipboard Operations section
	allHelpContent = append(allHelpContent, sectionStyle.Render("CLIPBOARD:"))
	allHelpContent = append(allHelpContent, helpLine("c", "copy file (replaces clipboard)"))
	allHelpContent = append(allHelpContent, helpLine("x", "cut file (replaces clipboard)"))
	allHelpContent = append(allHelpContent, helpLine("C", "append to copy clipboard (multi-file)"))
	allHelpContent = append(allHelpContent, helpLine("X", "append to cut clipboard (multi-file)"))
	allHelpContent = append(allHelpContent, helpLine("p", "paste files"))
	allHelpContent = append(allHelpContent, helpLine("y", "copy path to clipboard"))
	allHelpContent = append(allHelpContent, helpLine("u", "undo last deletion"))
	allHelpContent = append(allHelpContent, "")

	// Search & Filter section
	allHelpContent = append(allHelpContent, sectionStyle.Render("SEARCH & FILTER:"))
	allHelpContent = append(allHelpContent, helpLine("/", "start search (file/recursive/content)"))
	allHelpContent = append(allHelpContent, helpLine("tab", "cycle search modes (while searching)"))
	allHelpContent = append(allHelpContent, helpLine("↑/↓", "navigate results (while searching)"))
	allHelpContent = append(allHelpContent, helpLine("enter", "lock results / go to file or dir"))
	allHelpContent = append(allHelpContent, helpLine("ctrl+p", "toggle preview panel (while searching)"))
	allHelpContent = append(allHelpContent, helpLine("ctrl+n", "toggle name-only / full-path search"))
	allHelpContent = append(allHelpContent, helpLine("s", "cycle sort mode"))
	allHelpContent = append(allHelpContent, helpLine(".", "toggle hidden files"))
	allHelpContent = append(allHelpContent, "")

	// Bookmarks section
	allHelpContent = append(allHelpContent, sectionStyle.Render("BOOKMARKS:"))
	allHelpContent = append(allHelpContent, helpLine("b", "view bookmarks"))
	allHelpContent = append(allHelpContent, helpLine("B", "add bookmark"))
	allHelpContent = append(allHelpContent, "")

	// Other section
	allHelpContent = append(allHelpContent, sectionStyle.Render("OTHER:"))
	allHelpContent = append(allHelpContent, helpLine("?", "show this help"))
	allHelpContent = append(allHelpContent, helpLine(",", "open config file"))
	allHelpContent = append(allHelpContent, helpLine("q / ctrl+c", "quit"))
	allHelpContent = append(allHelpContent, helpLine("ctrl+g", "quit and cd to selected/current dir"))
	allHelpContent = append(allHelpContent, "")
	allHelpContent = append(allHelpContent, sectionStyle.Render("SHELL CD INTEGRATION (ctrl+g):"))
	allHelpContent = append(allHelpContent, "  Add this to ~/.zshrc or ~/.bashrc:")
	allHelpContent = append(allHelpContent, "")
	allHelpContent = append(allHelpContent, `  function scout() {`)
	allHelpContent = append(allHelpContent, `    command scout "$@"`)
	allHelpContent = append(allHelpContent, `    local f="$HOME/.config/scout/last_dir"`)
	allHelpContent = append(allHelpContent, `    [ -f "$f" ] && cd "$(cat "$f")" && rm -f "$f"`)
	allHelpContent = append(allHelpContent, `  }`)

	// Calculate visible range
	startIdx := m.helpScroll
	endIdx := m.helpScroll + contentHeight

	// Check if we need scroll indicators
	hasTopIndicator := startIdx > 0
	hasBottomIndicator := endIdx < len(allHelpContent)

	// Adjust for indicators
	if hasTopIndicator {
		contentHeight--
	}
	if hasBottomIndicator {
		contentHeight--
	}
	if contentHeight < 1 {
		contentHeight = 1
	}

	endIdx = startIdx + contentHeight
	if endIdx > len(allHelpContent) {
		endIdx = len(allHelpContent)
	}

	// Adjust scroll bounds
	if m.helpScroll > len(allHelpContent)-contentHeight {
		m.helpScroll = len(allHelpContent) - contentHeight
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
	}

	var displayLines []string
	if hasTopIndicator {
		displayLines = append(displayLines, "▲")
	}
	if startIdx < len(allHelpContent) {
		displayLines = append(displayLines, allHelpContent[startIdx:endIdx]...)
	}
	if hasBottomIndicator {
		displayLines = append(displayLines, "▼")
	}

	content := strings.Join(displayLines, "\n")
	listContent := listStyle.Render(content)

	combined := header + "\n" + listContent
	return borderStyle.Render(combined)
}

func (m model) renderErrorDialog() string {
	dialogWidth := 70
	if m.width-4 < dialogWidth {
		dialogWidth = m.width - 4
	}
	dialogHeight := 15
	// Clamp dialog height to terminal height minus some breathing room
	maxDialogHeight := m.height - 4
	if maxDialogHeight < 8 {
		maxDialogHeight = 8
	}
	if dialogHeight > maxDialogHeight {
		dialogHeight = maxDialogHeight
	}

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Background(lipgloss.Color("232")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight).
		MaxHeight(dialogHeight + 2) // +2 for border

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")).
		Background(lipgloss.Color("232"))

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(1, 0).
		Background(lipgloss.Color("232"))

	promptStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(1, 0).
		Background(lipgloss.Color("232"))

	// Truncate error details if they'd overflow the dialog
	details := m.errorDetails
	maxDetailLines := dialogHeight - 8 // title + padding + prompt take ~8 lines
	if maxDetailLines < 1 {
		maxDetailLines = 1
	}
	detailLines := strings.Split(details, "\n")
	if len(detailLines) > maxDetailLines {
		detailLines = detailLines[:maxDetailLines]
		detailLines = append(detailLines, "... (truncated)")
	}
	details = strings.Join(detailLines, "\n")

	title := titleStyle.Render("❌ ERROR")
	content := contentStyle.Render(fmt.Sprintf("%s\n\ndetails:\n%s", m.errorMsg, details))
	prompt := promptStyle.Render("press any key to continue")

	dialog := title + "\n" + content + "\n" + prompt
	rendered := dialogStyle.Render(dialog)

	// Center the dialog
	verticalPadding := (m.height - dialogHeight) / 2
	if verticalPadding < 0 {
		verticalPadding = 0
	}
	horizontalPadding := (m.width - dialogWidth) / 2
	if horizontalPadding < 0 {
		horizontalPadding = 0
	}

	centeredStyle := lipgloss.NewStyle().
		Padding(verticalPadding, horizontalPadding)

	return centeredStyle.Render(rendered)
}
