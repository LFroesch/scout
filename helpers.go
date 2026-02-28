package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/skratchdot/open-golang/open"

	"github.com/LFroesch/scout/internal/utils"
)

// Helper functions

func (m *model) openFile(path string) tea.Cmd {
	return func() tea.Msg {
		filename := filepath.Base(path)

		// Try to open with default application
		var cmd *exec.Cmd
		var foundEditor bool

		switch {
		case utils.IsCodeFile(path):
			// Try VS Code first, then fall back to other editors
			editors := []string{"code", "subl", "atom", "vim", "nano"}
			for _, editor := range editors {
				if _, err := exec.LookPath(editor); err == nil {
					cmd = exec.Command(editor, path)
					foundEditor = true
					break
				}
			}
			if !foundEditor {
				// No editor found
				return fileOpenResultMsg{
					success: false,
					message: fmt.Sprintf("Can't open %s via scout", filename),
					path:    path,
				}
			}
		default:
			// Use system default opener (handles Linux/macOS/Windows automatically)
			err := open.Run(path)
			if err != nil {
				return fileOpenResultMsg{
					success: false,
					message: fmt.Sprintf("Can't open %s via scout", filename),
					path:    path,
				}
			}
			return fileOpenResultMsg{
				success: true,
				message: fmt.Sprintf("Opened %s", filename),
				path:    path,
			}
		}

		if cmd != nil {
			err := cmd.Start()
			if err != nil {
				return fileOpenResultMsg{
					success: false,
					message: fmt.Sprintf("Can't open %s via scout", filename),
					path:    path,
				}
			}
		}

		return fileOpenResultMsg{
			success: true,
			message: fmt.Sprintf("Opened %s", filename),
			path:    path,
		}
	}
}

func (m *model) editFile(path string) tea.Cmd {
	return m.editFileAtLine(path, 0)
}

func (m *model) editFileAtLine(path string, line int) tea.Cmd {
	return func() tea.Msg {
		// Use configured editor if set, otherwise try defaults
		editors := []string{}
		if m.config.Editor != "" {
			editors = append(editors, m.config.Editor)
		}
		editors = append(editors, "code", "vim", "nano", "vi")

		for _, editor := range editors {
			if _, err := exec.LookPath(editor); err == nil {
				var cmd *exec.Cmd
				if line > 0 {
					// Open at specific line based on editor
					switch editor {
					case "code":
						cmd = exec.Command(editor, "-g", fmt.Sprintf("%s:%d", path, line))
					case "vim", "vi", "nvim":
						cmd = exec.Command(editor, fmt.Sprintf("+%d", line), path)
					case "nano":
						cmd = exec.Command(editor, fmt.Sprintf("+%d", line), path)
					default:
						cmd = exec.Command(editor, path)
					}
				} else {
					cmd = exec.Command(editor, path)
				}
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
			m.statusMsg = fmt.Sprintf("opening %s in vs code", filepath.Base(path))
			m.statusExpiry = time.Now().Add(2 * time.Second)
		} else {
			m.statusMsg = "vs code not found in path"
			m.statusExpiry = time.Now().Add(3 * time.Second)
		}

		return nil
	}
}

func (m *model) openExternalWithFallback(path string, line int) tea.Cmd {
	return func() tea.Msg {
		filename := filepath.Base(path)

		// Try editors first (VS Code, vim, nano)
		editors := []string{"code", "vim", "nano", "vi"}
		for _, editor := range editors {
			if _, err := exec.LookPath(editor); err == nil {
				var cmd *exec.Cmd
				if line > 0 {
					// Open at specific line based on editor
					switch editor {
					case "code":
						cmd = exec.Command(editor, "-g", fmt.Sprintf("%s:%d", path, line))
					case "vim", "vi", "nvim":
						cmd = exec.Command(editor, fmt.Sprintf("+%d", line), path)
					case "nano":
						cmd = exec.Command(editor, fmt.Sprintf("+%d", line), path)
					default:
						cmd = exec.Command(editor, path)
					}
				} else {
					cmd = exec.Command(editor, path)
				}

				err := cmd.Start()
				if err == nil {
					return fileOpenResultMsg{
						success: true,
						message: fmt.Sprintf("Opened %s in %s", filename, editor),
						path:    path,
					}
				}
			}
		}

		// No editor found - fall back to system default opener
		err := open.Run(path)
		if err != nil {
			return fileOpenResultMsg{
				success: false,
				message: fmt.Sprintf("Can't open %s via scout", filename),
				path:    path,
			}
		}
		return fileOpenResultMsg{
			success: true,
			message: fmt.Sprintf("Opened %s", filename),
			path:    path,
		}
	}
}

func (m *model) copyPath(path string) {
	// Use clipboard library for cross-platform support
	err := clipboard.WriteAll(path)
	if err == nil {
		m.statusMsg = fmt.Sprintf("copied: %s", path)
		m.statusExpiry = time.Now().Add(2 * time.Second)
	} else {
		m.statusMsg = fmt.Sprintf("failed to copy: %v", err)
		m.statusExpiry = time.Now().Add(3 * time.Second)
	}
}

// ensureCursorInBounds ensures cursor is within valid range and adjusts scroll to keep it visible
func (m *model) ensureCursorInBounds() {
	// Early return if no files
	if len(m.filteredFiles) == 0 {
		m.cursor = 0
		m.scrollOffset = 0
		return
	}

	// Clamp cursor to valid range - move to last item if out of bounds (user requested behavior)
	if m.cursor >= len(m.filteredFiles) {
		m.cursor = len(m.filteredFiles) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}

	// Calculate visible height
	availableHeight := m.getSafeHeight() - uiOverhead
	if availableHeight < 3 {
		availableHeight = 3
	}
	visibleHeight := availableHeight - 1
	if visibleHeight < 1 {
		visibleHeight = 1
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

// placeOverlay centers fg (dialog box) over bg (background content), merging them line by line.
// Uses ANSI-aware truncation so terminal colors in the background are preserved.
func placeOverlay(bg, fg string) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")

	bgHeight := len(bgLines)
	fgHeight := len(fgLines)

	fgWidth := 0
	for _, line := range fgLines {
		if w := lipgloss.Width(line); w > fgWidth {
			fgWidth = w
		}
	}

	startX := 0
	startY := 0
	// Find the widest bg line to center horizontally
	bgWidth := 0
	for _, line := range bgLines {
		if w := lipgloss.Width(line); w > bgWidth {
			bgWidth = w
		}
	}
	startX = (bgWidth - fgWidth) / 2
	startY = (bgHeight - fgHeight) / 2
	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	result := make([]string, bgHeight)
	copy(result, bgLines)

	for i, fgLine := range fgLines {
		bgIdx := startY + i
		if bgIdx < 0 || bgIdx >= bgHeight {
			continue
		}

		bgLine := bgLines[bgIdx]
		bgLineWidth := lipgloss.Width(bgLine)

		// Left portion of background before the dialog
		left := xansi.Truncate(bgLine, startX, "")
		leftWidth := lipgloss.Width(left)
		// Fill any visual gap (e.g. if bg line is shorter than startX)
		if leftWidth < startX {
			left += strings.Repeat(" ", startX-leftWidth)
		}

		// Right portion of background after the dialog
		right := ""
		rightStart := startX + fgWidth
		if rightStart < bgLineWidth {
			right = xansi.Cut(bgLine, rightStart, bgLineWidth)
		}

		result[bgIdx] = left + fgLine + right
	}

	return strings.Join(result, "\n")
}
