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
		// Build search text with mode indicator and hint
		searchType := "FILENAME"
		if m.currentSearchType == searchContent {
			searchType = "CONTENT"
		}

		searchMode := "CURRENT DIR"
		if m.recursiveSearch {
			searchMode = "RECURSIVE"
		}

		searchLabel := fmt.Sprintf("🔍 %s [%s]: ", searchType, searchMode)
		hint := " (ctrl+r: toggle recursive | esc: cancel)"

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
		Width(width-4).
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

	// Calculate visible range - reserve space for scroll indicators
	visibleHeight := listHeight
	hasTopIndicator := m.scrollOffset > 0
	hasBottomIndicator := false // Will determine after initial calculation

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

	// Now determine if we need indicators and adjust accordingly
	hasTopIndicator = startIdx > 0
	hasBottomIndicator = endIdx < len(m.filteredFiles)

	// Reduce visible items to make room for indicators
	maxItems := visibleHeight
	if hasTopIndicator {
		maxItems--
	}
	if hasBottomIndicator {
		maxItems--
	}

	// Adjust endIdx to respect maxItems
	if endIdx > startIdx+maxItems {
		endIdx = startIdx + maxItems
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
			icon = utils.GetFileIcon(item.name)
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
		// Total width - checkbox(2) - icon(2) - gitStatus(4) - size(~10) - padding(4)
		maxNameLen := width - 25
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

		// Build left side: checkbox + icon + name + gitStatus
		leftSide := fmt.Sprintf("%s%s %s%s", checkbox, icon, displayName, gitStatus)
		leftWidth := lipgloss.Width(leftSide)

		// Calculate padding to push size to the right
		totalWidth := width - 4 // Account for padding in style
		padding := totalWidth - leftWidth - sizeWidth
		if padding < 1 {
			padding = 1
		}

		// Build the line with size right-aligned
		line := leftSide + strings.Repeat(" ", padding) + sizeStr

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

func (m model) renderPreview(width int) string {
	availableHeight := m.height - 9
	if availableHeight < 3 {
		availableHeight = 3
	}

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(width - 2).
		Height(availableHeight + 2)

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

	// Render sticky header - truncate header lines too
	maxHeaderWidth := width - 6
	var truncatedHeaderLines []string
	for _, line := range headerLines {
		if lipgloss.Width(line) > maxHeaderWidth {
			runes := []rune(line)
			truncated := ""
			for _, r := range runes {
				if lipgloss.Width(truncated+string(r)+"...") > maxHeaderWidth {
					break
				}
				truncated += string(r)
			}
			truncatedHeaderLines = append(truncatedHeaderLines, truncated+"...")
		} else {
			truncatedHeaderLines = append(truncatedHeaderLines, line)
		}
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("99")).
		Bold(true).
		Padding(0, 1)
	header := headerStyle.Render(strings.Join(truncatedHeaderLines, "\n"))

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

	// Build scrollable content with strict line limit
	// Reserve space for scroll indicators if needed
	maxContentLines := scrollableHeight
	hasTopIndicator := startLine > 0
	hasBottomIndicator := endLine < len(contentLines)

	if hasTopIndicator {
		maxContentLines--
	}
	if hasBottomIndicator {
		maxContentLines--
	}

	// Adjust endLine to fit within maxContentLines
	if endLine > startLine+maxContentLines {
		endLine = startLine + maxContentLines
	}

	var content []string
	if hasTopIndicator {
		content = append(content, "▲ w")
	}

	if startLine < len(contentLines) && endLine > startLine {
		content = append(content, contentLines[startLine:endLine]...)
	}

	if hasBottomIndicator && endLine < len(contentLines) {
		content = append(content, "▼ s")
	}

	// Aggressively truncate all lines to prevent ANY wrapping
	// Be conservative with width to handle all edge cases
	maxContentWidth := width - 8 // More conservative: borders(2) + padding(2) + safety margin(4)
	if maxContentWidth < 10 {
		maxContentWidth = 10
	}

	var truncatedContent []string
	for _, line := range content {
		lineWidth := lipgloss.Width(line)
		if lineWidth > maxContentWidth {
			// Truncate long lines
			runes := []rune(line)
			truncated := ""
			for _, r := range runes {
				testWidth := lipgloss.Width(truncated + string(r) + "...")
				if testWidth > maxContentWidth {
					break
				}
				truncated += string(r)
			}
			if truncated == "" && len(runes) > 0 {
				// Even first char doesn't fit - just take it
				truncated = string(runes[0])
			}
			truncatedContent = append(truncatedContent, truncated+"...")
		} else {
			truncatedContent = append(truncatedContent, line)
		}
	}

	// Don't set Width on contentStyle - it can cause unwanted wrapping
	// We've already truncated lines manually above
	contentStyle := lipgloss.NewStyle().
		Padding(0, 1)
	scrollContent := contentStyle.Render(strings.Join(truncatedContent, "\n"))

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

	if len(m.sortedBookmarkPaths) == 0 {
		bookmarkItems = append(bookmarkItems, "No bookmarks yet. Navigate to a directory and press 'B' to bookmark it.")
	} else {
		// Use pre-sorted bookmarks from model
		for i, bookmarkPath := range m.sortedBookmarkPaths {
			icon := "📁"
			name := filepath.Base(bookmarkPath)
			if name == "" || name == "." {
				name = bookmarkPath
			}

			// Show full path relative to root if possible
			displayPath := bookmarkPath
			if m.config.RootPath != "" && strings.HasPrefix(bookmarkPath, m.config.RootPath) {
				rel, err := filepath.Rel(m.config.RootPath, bookmarkPath)
				if err == nil && rel != "." {
					displayPath = "~/" + rel
				} else if rel == "." {
					displayPath = "~"
				}
			}

			// Show frecency score
			frecencyInfo := ""
			score := m.config.Frecency[bookmarkPath]
			if score > 0 {
				frecencyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("141"))
				frecencyInfo = " " + frecencyStyle.Render(fmt.Sprintf("[%d visits]", score))
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
	bgColor := lipgloss.Color("235")
	baseStyle := lipgloss.NewStyle().
		Background(bgColor).
		Foreground(lipgloss.Color("252"))

	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("99")).
		Background(bgColor).
		Bold(true)

	// Build left content as plain text pieces
	var leftParts []string

	if m.mode == modeBookmarks {
		if len(m.sortedBookmarkPaths) > 0 && m.bookmarksCursor < len(m.sortedBookmarkPaths) {
			bookmark := m.sortedBookmarkPaths[m.bookmarksCursor]
			bookmarkName := filepath.Base(bookmark)
			if bookmarkName == "" || bookmarkName == "." {
				bookmarkName = bookmark
			}
			leftParts = append(leftParts, baseStyle.Render(fmt.Sprintf("📁 %s → %s", bookmarkName, bookmark)))
		}
	} else if len(m.filteredFiles) > 0 && m.cursor < len(m.filteredFiles) {
		current := m.filteredFiles[m.cursor]
		if current.isDir {
			leftParts = append(leftParts, baseStyle.Render(fmt.Sprintf("📁 %s", current.name)))
		} else {
			leftParts = append(leftParts, baseStyle.Render(fmt.Sprintf("📄 %s (", current.name)))
			leftParts = append(leftParts, utils.FormatFileSizeColored(current.size))
			leftParts = append(leftParts, baseStyle.Render(")"))
		}
	}

	if m.statusMsg != "" {
		leftParts = append(leftParts, baseStyle.Render("  "+m.statusMsg))
	}

	// Build right side as separate pieces
	rightParts := []string{
		baseStyle.Render("Press "),
		keyStyle.Render("?"),
		baseStyle.Render(" for help"),
	}

	// Calculate total widths
	leftWidth := 0
	for _, part := range leftParts {
		leftWidth += lipgloss.Width(part)
	}

	rightWidth := 0
	for _, part := range rightParts {
		rightWidth += lipgloss.Width(part)
	}

	spacingWidth := m.width - leftWidth - rightWidth
	if spacingWidth < 1 {
		spacingWidth = 1
	}

	// Join everything together
	allParts := leftParts
	allParts = append(allParts, baseStyle.Width(spacingWidth).Render(""))
	allParts = append(allParts, rightParts...)

	return lipgloss.JoinHorizontal(lipgloss.Top, allParts...)
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
