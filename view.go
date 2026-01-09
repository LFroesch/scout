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
		return "Loading..."
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
			// Split view with preview - ensure both panels have same height
			availableHeight := m.height - 9
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
		// Show search in top-right of header
		var searchMode string
		if m.currentSearchType == searchContent {
			searchMode = "CONTENT SEARCH"
		} else if m.recursiveSearch {
			searchMode = "RECURSIVE SEARCH"
		} else {
			searchMode = "FILE SEARCH"
		}

		// Build search parts with different colors
		purpleStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("105"))

		grayStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252"))

		yellowStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("226"))

		searchLabel := fmt.Sprintf("🔍 %s: ", searchMode)
		searchValue := m.searchInput.Value()
		hint := " (Tab: cycle modes | ESC: clear/exit)"

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

		// Render parts: purple label + gray text + yellow cursor + purple hint
		searchLabelRendered := purpleStyle.Render(searchLabel)

		// Split displayValue by cursor placeholder
		parts := strings.Split(displayValue, "{{CURSOR}}")
		var displayRendered string
		if len(parts) == 2 {
			displayRendered = grayStyle.Render(parts[0]) + yellowStyle.Render(cursorChar) + grayStyle.Render(parts[1])
		} else {
			displayRendered = grayStyle.Render(displayValue)
		}

		hintRendered := purpleStyle.Render(hint)

		searchText := searchLabelRendered + displayRendered + hintRendered

		// Calculate widths for split layout
		searchTextLen := lipgloss.Width(searchText)
		titleWidth := m.width - searchTextLen - 2
		if titleWidth < 20 {
			titleWidth = 20
			searchTextLen = m.width - titleWidth - 2
		}

		// Style the title part
		baseStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252"))

		titlePart := baseStyle.Width(titleWidth).Padding(0, 1).Render(title)
		searchPart := searchText

		title = lipgloss.JoinHorizontal(lipgloss.Top, titlePart, searchPart)
	}

	// Return with full width background for search mode
	if m.mode == modeSearch {
		return lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Width(m.width).
			Render(title)
	}
	return titleStyle.Render(title)
}

func (m *model) renderStatusBar() string {
	// Normal status bar
	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("240")).
		Padding(0, 1).
		Width(m.width)

	var statusText string
	var rightSide string

	// Bookmark mode has special status bar
	if m.mode == modeBookmarks {
		if len(m.sortedBookmarkPaths) > 0 && m.bookmarksCursor < len(m.sortedBookmarkPaths) {
			// Show current bookmark path on left
			statusText = m.sortedBookmarkPaths[m.bookmarksCursor]
		} else {
			statusText = "No bookmarks"
		}
		// Show keybinds on right
		rightSide = "enter: open | o: VS Code | d: delete | esc: back"
	} else {
		// File count and position info
		if len(m.filteredFiles) > 0 {
			fileCountInfo := fmt.Sprintf("%d/%d", m.cursor+1, len(m.filteredFiles))
			statusText = fileCountInfo
		}

		// Git info
		if m.gitBranch != "" {
			gitInfo := fmt.Sprintf(" | Branch: %s", m.gitBranch)
			statusText += gitInfo
		}

		// Clipboard info
		if len(m.clipboard) > 0 {
			opStr := "copied"
			if m.clipboardOp == opCut {
				opStr = "cut"
			}
			statusText += fmt.Sprintf(" | %d %s", len(m.clipboard), opStr)
		}

		// Loading indicator
		if m.loading {
			statusText += " | Searching..."
		}

		// Status message (temporary)
		if m.statusMsg != "" {
			statusText += " | " + m.statusMsg
		}

		// Sort mode indicator (on left side)
		sortNames := map[sortMode]string{
			sortByName: "Name",
			sortBySize: "Size",
			sortByDate: "Date",
			sortByType: "Type",
		}
		statusText += fmt.Sprintf(" | Sort: %s (S)", sortNames[m.sortBy])

		// Help hint (on right side)
		rightSide = "? for help"
	}

	totalWidth := m.width - 2 // Account for padding
	padding := totalWidth - lipgloss.Width(statusText) - lipgloss.Width(rightSide) - 3
	if padding < 1 {
		padding = 1
	}
	statusText += strings.Repeat(" ", padding) + rightSide

	return statusStyle.Render(statusText)
}

// renderFileList renders the file list panel with the given width
func (m *model) renderFileList(width int) string {
	// Calculate available height for file list
	availableHeight := m.height - 9 // Account for header, status, borders, padding
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Reserve space for scroll indicators (2 lines)
	contentHeight := availableHeight - 2
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
		if m.currentSearchType == searchContent {
			searchModeIndicator = " [CONTENT]"
		} else if m.recursiveSearch {
			searchModeIndicator = " [RECURSIVE]"
		} else {
			searchModeIndicator = " [CURRENT DIR]"
		}
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Width(width - 4)

	header := headerStyle.Render(fmt.Sprintf("📁 %s%s", dirName, searchModeIndicator))

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
				icon = "⬆️"
			} else {
				icon = "📁"
			}
		} else {
			icon = utils.GetFileIcon(item.name)
		}

		// Git status and symlink indicator (grouped together)
		gitStatus := ""
		if m.gitModified[item.path] {
			modifiedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
			gitStatus = " " + modifiedStyle.Render("[M]")
		}
		if item.isSymlink {
			symlinkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("cyan"))
			gitStatus += " " + symlinkStyle.Render("[→]")
		}

		// Format item with highlighting if in search mode
		name := item.name
		displayName := name

		// Apply search highlighting if we have match positions
		if m.mode == modeSearch && i < len(m.searchMatches) && len(m.searchMatches[i]) > 0 {
			displayName = utils.HighlightMatches(name, m.searchMatches[i])
		}

		// Add file size for files (right-aligned)
		sizeStr := ""
		sizeWidth := 0
		if !item.isDir && item.name != ".." {
			sizeStr = utils.FormatFileSizeColored(item.size)
			sizeWidth = lipgloss.Width(sizeStr)
		}

		// Calculate available width for filename
		// Reserve space: icon(2) + gitStatus(4-8) + size(~10) + padding(8)
		maxNameLen := width - 28
		if maxNameLen < 10 {
			maxNameLen = 10
		}

		// Truncate name if needed
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

		// Build left side: icon + name + gitStatus (which includes symlink indicator)
		leftSide := fmt.Sprintf("%s %s%s", icon, displayName, gitStatus)
		leftWidth := lipgloss.Width(leftSide)

		// Calculate padding to push size to the right
		totalWidth := width - 4 // Account for padding in style
		padding := totalWidth - leftWidth - sizeWidth
		if padding < 1 {
			padding = 1
		}

		// Build the line with size right-aligned
		line := leftSide + strings.Repeat(" ", padding) + sizeStr

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
		items = append([]string{"▲ More files above..."}, items...)
	}
	if hasBottomIndicator {
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

func (m *model) renderPreview(width int) string {
	availableHeight := m.height - 9
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Reserve space for scroll indicators (2 lines)
	contentHeight := availableHeight - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Width(width - 4)

	header := headerStyle.Render("👁 Preview")

	previewStyle := lipgloss.NewStyle().
		Width(width-4).
		Padding(0, 1)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(width - 2).
		Height(availableHeight + 2)

	var content string
	if len(m.previewLines) == 0 {
		content = "No preview available"
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
		lines = append(lines, m.previewLines[startIdx:endIdx]...)
		if hasBottomIndicator {
			lines = append(lines, "▼")
		}

		content = strings.Join(lines, "\n")
	}

	previewContent := previewStyle.Render(content)
	combined := header + "\n" + previewContent
	return borderStyle.Render(combined)
}

func (m model) renderBookmarksView() string {
	availableHeight := m.height - 9
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Reserve space for scroll indicators
	contentHeight := availableHeight - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Width(m.width - 4)

	header := headerStyle.Render("📚 Bookmarks")

	listStyle := lipgloss.NewStyle().
		Padding(0, 1)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(m.width - 2).
		Height(availableHeight + 2)

	var bookmarkItems []string

	if len(m.sortedBookmarkPaths) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Padding(1, 0)
		bookmarkItems = []string{emptyStyle.Render("No bookmarks yet. Press 'B' in normal mode to add current directory.")}
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
			bookmarkItems = append(bookmarkItems, "▲ More bookmarks above...")
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

			// Truncate path if needed
			maxPathLen := m.width - 30
			if maxPathLen < 20 {
				maxPathLen = 20
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

			// Build line without colors: frecency + icon + name + path
			line := fmt.Sprintf("%s%s %s (%s)", frecencyStr, icon, name, displayPath)

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
			bookmarkItems = append(bookmarkItems, "▼ More bookmarks below...")
		}
	}

	content := listStyle.Render(strings.Join(bookmarkItems, "\n"))
	combined := header + "\n" + content
	return borderStyle.Render(combined)
}

func (m model) renderConfirmDeleteView() string {
	dialogWidth := 60
	dialogHeight := 8

	if m.deleteBookmarkIndex < 0 || m.deleteBookmarkIndex >= len(m.config.Bookmarks) {
		return "Error: Invalid bookmark index"
	}

	bookmarkPath := m.config.Bookmarks[m.deleteBookmarkIndex]
	bookmarkName := filepath.Base(bookmarkPath)

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196"))

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(1, 0)

	promptStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(1, 0)

	title := titleStyle.Render("⚠️  Delete Bookmark?")
	content := contentStyle.Render(fmt.Sprintf("Are you sure you want to delete this bookmark?\n\n%s\n(%s)", bookmarkName, bookmarkPath))
	prompt := promptStyle.Render("Press 'y' to confirm, 'n' or ESC to cancel")

	dialog := title + "\n" + content + "\n" + prompt
	rendered := dialogStyle.Render(dialog)

	// Center the dialog
	verticalPadding := (m.height - dialogHeight) / 2
	horizontalPadding := (m.width - dialogWidth) / 2

	centeredStyle := lipgloss.NewStyle().
		Padding(verticalPadding, horizontalPadding)

	return centeredStyle.Render(rendered)
}

func (m model) renderConfirmFileDeleteView() string {
	dialogWidth := 60
	dialogHeight := 8

	if len(m.filteredFiles) == 0 || m.cursor >= len(m.filteredFiles) {
		return "Error: No file selected"
	}

	file := m.filteredFiles[m.cursor]

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196"))

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(1, 0)

	promptStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(1, 0)

	fileType := "file"
	if file.isDir {
		fileType = "directory"
	}

	title := titleStyle.Render(fmt.Sprintf("⚠️  Delete %s?", fileType))
	content := contentStyle.Render(fmt.Sprintf("Are you sure you want to delete:\n\n%s\n\nThis will move it to trash if available.", file.name))
	prompt := promptStyle.Render("Press 'y' to confirm, 'n' or ESC to cancel")

	dialog := title + "\n" + content + "\n" + prompt
	rendered := dialogStyle.Render(dialog)

	// Center the dialog
	verticalPadding := (m.height - dialogHeight) / 2
	horizontalPadding := (m.width - dialogWidth) / 2

	centeredStyle := lipgloss.NewStyle().
		Padding(verticalPadding, horizontalPadding)

	return centeredStyle.Render(rendered)
}

func (m model) renderRenameDialog() string {
	dialogWidth := 60
	dialogHeight := 8

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("105")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105"))

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(1, 0)

	title := titleStyle.Render("✏️  Rename")
	content := contentStyle.Render("Enter new name:")
	inputView := m.textInput.View()

	dialog := title + "\n" + content + "\n" + inputView
	rendered := dialogStyle.Render(dialog)

	// Center the dialog
	verticalPadding := (m.height - dialogHeight) / 2
	horizontalPadding := (m.width - dialogWidth) / 2

	centeredStyle := lipgloss.NewStyle().
		Padding(verticalPadding, horizontalPadding)

	return centeredStyle.Render(rendered)
}

func (m model) renderCreateFileDialog() string {
	dialogWidth := 60
	dialogHeight := 8

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("105")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105"))

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(1, 0)

	title := titleStyle.Render("📄 Create New File")
	content := contentStyle.Render("Enter filename:")
	inputView := m.textInput.View()

	dialog := title + "\n" + content + "\n" + inputView
	rendered := dialogStyle.Render(dialog)

	// Center the dialog
	verticalPadding := (m.height - dialogHeight) / 2
	horizontalPadding := (m.width - dialogWidth) / 2

	centeredStyle := lipgloss.NewStyle().
		Padding(verticalPadding, horizontalPadding)

	return centeredStyle.Render(rendered)
}

func (m model) renderCreateDirDialog() string {
	dialogWidth := 60
	dialogHeight := 8

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("105")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105"))

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(1, 0)

	title := titleStyle.Render("📁 Create New Directory")
	content := contentStyle.Render("Enter directory name:")
	inputView := m.textInput.View()

	dialog := title + "\n" + content + "\n" + inputView
	rendered := dialogStyle.Render(dialog)

	// Center the dialog
	verticalPadding := (m.height - dialogHeight) / 2
	horizontalPadding := (m.width - dialogWidth) / 2

	centeredStyle := lipgloss.NewStyle().
		Padding(verticalPadding, horizontalPadding)

	return centeredStyle.Render(rendered)
}

func (m model) renderHelpView() string {
	availableHeight := m.height - 9
	if availableHeight < 3 {
		availableHeight = 3
	}

	// Reserve space for scroll indicators
	contentHeight := availableHeight - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("105")).
		Width(m.width - 4)

	header := headerStyle.Render("❓ Help")

	listStyle := lipgloss.NewStyle().
		Width(m.width-4).
		Padding(0, 1)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(m.width - 2).
		Height(availableHeight + 2)

	// Build help content
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("105")).
		Bold(true)

	sectionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true)

	var allHelpContent []string

	// Navigation section
	allHelpContent = append(allHelpContent, sectionStyle.Render("Navigation:"))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s           Move down", keyStyle.Render("j / ↓")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s           Move up", keyStyle.Render("k / ↑")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s           Enter directory / Open file", keyStyle.Render("enter / l / →")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s       Go to parent directory", keyStyle.Render("esc / h / ←")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Go to top", keyStyle.Render("g")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Go to bottom", keyStyle.Render("G")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s       Half-page down", keyStyle.Render("ctrl+d")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s       Half-page up", keyStyle.Render("ctrl+u")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s       Full-page down", keyStyle.Render("ctrl+f")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s       Full-page up", keyStyle.Render("ctrl+b")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Jump to home directory", keyStyle.Render("~")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Jump to /mnt/c (WSL)", keyStyle.Render("`")))
	allHelpContent = append(allHelpContent, "")

	// Preview Scrolling section
	allHelpContent = append(allHelpContent, sectionStyle.Render("Preview Scrolling:"))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s       Scroll preview down", keyStyle.Render("s / alt+↓")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s       Scroll preview up", keyStyle.Render("w / alt+↑")))
	allHelpContent = append(allHelpContent, "")

	// File Operations section
	allHelpContent = append(allHelpContent, sectionStyle.Render("File Operations:"))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Open file", keyStyle.Render("o")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Edit file", keyStyle.Render("e")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Rename file/directory", keyStyle.Render("R")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Delete file/directory", keyStyle.Render("D")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Create new file", keyStyle.Render("N")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Create new directory", keyStyle.Render("M")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Refresh current view", keyStyle.Render("r")))
	allHelpContent = append(allHelpContent, "")

	// Clipboard Operations section
	allHelpContent = append(allHelpContent, sectionStyle.Render("Clipboard:"))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s           Copy current file", keyStyle.Render("c")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s           Cut current file", keyStyle.Render("x")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s           Paste files", keyStyle.Render("p")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s           Copy path to clipboard", keyStyle.Render("y")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s           Undo last deletion", keyStyle.Render("u")))
	allHelpContent = append(allHelpContent, "")

	// Search & Filter section
	allHelpContent = append(allHelpContent, sectionStyle.Render("Search & Filter:"))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Start search (File/Recursive/Content)", keyStyle.Render("/")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s           Cycle search modes (while searching)", keyStyle.Render("Tab")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s     Navigate results (while searching)", keyStyle.Render("↑/↓")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Cycle sort mode", keyStyle.Render("S")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Toggle hidden files", keyStyle.Render(".")))
	allHelpContent = append(allHelpContent, "")

	// Bookmarks section
	allHelpContent = append(allHelpContent, sectionStyle.Render("Bookmarks:"))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             View bookmarks", keyStyle.Render("b")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Add bookmark", keyStyle.Render("B")))
	allHelpContent = append(allHelpContent, "")

	// Other section
	allHelpContent = append(allHelpContent, sectionStyle.Render("Other:"))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s             Show this help", keyStyle.Render("?")))
	allHelpContent = append(allHelpContent, fmt.Sprintf("  %s   Quit", keyStyle.Render("q / ctrl+c")))

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
	dialogHeight := 15

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("196")).
		Padding(1, 2).
		Width(dialogWidth).
		Height(dialogHeight)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196"))

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Padding(1, 0)

	promptStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(1, 0)

	title := titleStyle.Render("❌ Error")
	content := contentStyle.Render(fmt.Sprintf("%s\n\nDetails:\n%s", m.errorMsg, m.errorDetails))
	prompt := promptStyle.Render("Press any key to continue")

	dialog := title + "\n" + content + "\n" + prompt
	rendered := dialogStyle.Render(dialog)

	// Center the dialog
	verticalPadding := (m.height - dialogHeight) / 2
	horizontalPadding := (m.width - dialogWidth) / 2

	centeredStyle := lipgloss.NewStyle().
		Padding(verticalPadding, horizontalPadding)

	return centeredStyle.Render(rendered)
}
