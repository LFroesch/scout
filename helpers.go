package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/skratchdot/open-golang/open"

	"github.com/LFroesch/scout/internal/utils"
)

// Helper functions

func (m *model) openFile(path string) tea.Cmd {
	return func() tea.Msg {
		// Try to open with default application
		var cmd *exec.Cmd
		switch {
		case utils.IsCodeFile(path):
			// Try VS Code first, then fall back to other editors
			editors := []string{"code", "subl", "atom", "vim", "nano"}
			for _, editor := range editors {
				if _, err := exec.LookPath(editor); err == nil {
					cmd = exec.Command(editor, path)
					break
				}
			}
		default:
			// Use system default opener (handles Linux/macOS/Windows automatically)
			open.Run(path)
			return nil
		}

		if cmd != nil {
			cmd.Start()
		}

		return nil
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
