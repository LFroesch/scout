package utils

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// GetFileIcon returns an emoji icon for a file based on its extension
func GetFileIcon(name string) string {
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

// IsCodeFile returns true if the file is a code file based on extension
func IsCodeFile(name string) bool {
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

// IsImageFile returns true if the file is an image based on extension
func IsImageFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	imageExts := []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp", ".bmp"}

	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}

// IsBinaryFile returns true if the file is likely binary based on extension
func IsBinaryFile(path string) bool {
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

	return false
}

// ShouldIgnore returns true if a file/directory should be ignored
func ShouldIgnore(name string) bool {
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

// FormatFileSize formats a file size in bytes to a human-readable string
func FormatFileSize(size int64) string {
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

// FormatFileSizeColored returns a color-styled file size string based on size ranges
func FormatFileSizeColored(size int64) string {
	sizeStr := FormatFileSize(size)

	const (
		KB    = 1024
		MB    = 1024 * KB
		MB100 = 100 * MB
	)

	var style lipgloss.Style
	switch {
	case size < KB:
		// < 1 KB: dim gray for tiny files
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	case size < MB:
		// 1 KB - 1 MB: normal color for typical files
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	case size < MB100:
		// 1 MB - 100 MB: yellow/orange for large files
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	default:
		// > 100 MB: red bold for very large files
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	}

	return style.Render(sizeStr)
}

// CommandExists checks if a command is available in PATH
func CommandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// Contains checks if a string slice contains a specific item
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// HighlightMatches highlights matched characters in a string
func HighlightMatches(text string, matches []int) string {
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

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
