package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// isVSCodeTerminal checks if running in VS Code integrated terminal
func isVSCodeTerminal() bool {
	return os.Getenv("TERM_PROGRAM") == "vscode"
}

// GetFileIcon returns an emoji icon for a file based on its extension
func GetFileIcon(name string) string {
	ext := strings.ToLower(filepath.Ext(name))

	switch ext {
	case ".go", ".mod", ".sum":
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
		return "🔧"
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
		return "📸"
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
	case ".sh", ".bash", ".zsh", "install":
		return "💻"
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

// hasBinaryContent checks if the first 512 bytes of a file contain binary data
func hasBinaryContent(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false // If we can't open it, assume not binary to avoid hiding files
	}
	defer file.Close()

	// Read first 512 bytes
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		return false // Empty file or read error - assume text
	}

	// Check for null bytes or high ratio of non-printable characters
	buf = buf[:n]
	nullBytes := 0
	nonPrintable := 0

	for _, b := range buf {
		if b == 0 {
			nullBytes++
		}
		// Count non-printable chars (excluding common whitespace: tab, newline, carriage return)
		if b < 32 && b != 9 && b != 10 && b != 13 {
			nonPrintable++
		}
		if b > 126 && b < 128 {
			nonPrintable++
		}
	}

	// If we find null bytes, it's likely binary
	if nullBytes > 0 {
		return true
	}

	// If more than 30% non-printable characters, consider it binary
	if len(buf) > 0 && float64(nonPrintable)/float64(len(buf)) > 0.3 {
		return true
	}

	return false
}

// IsBinaryFile returns true if the file is likely binary based on extension or content
func IsBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	// Known binary extensions - always binary, no need to check content
	binaryExts := []string{
		// Executables
		".exe", ".dll", ".so", ".dylib", ".bin", ".o", ".a", ".lib", ".cat", ".icm", ".inf", ".ini",
		".sys", ".efi", ".elf",
		// Images
		".png", ".jpg", ".jpeg", ".gif", ".ico", ".webp", ".bmp", ".tiff", ".tif", ".psd", ".raw",
		// Video
		".mp4", ".avi", ".mov", ".mkv", ".flv", ".wmv", ".m4v", ".webm",
		// Audio
		".mp3", ".wav", ".flac", ".ogg", ".aac", ".wma", ".m4a", ".aiff", ".alac", ".au", ".aifc", ".aif",
		// Archives
		".zip", ".tar", ".gz", ".rar", ".7z", ".bz2", ".xz", ".tgz", ".tbz2",
		// Documents
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		// Database
		".db", ".sqlite", ".sqlite3", ".mdb", ".accdb", ".bson",
		// Fonts
		".ttf", ".otf", ".woff", ".woff2", ".eot",
		// Other
		".class", ".pyc", ".pyd", ".elc", ".jar", ".war", ".ear", ".state", ".forge", ".h", ".pck", ".tga", ".mui",
	}

	for _, binExt := range binaryExts {
		if ext == binExt {
			return true
		}
	}

	// Greylist - extensions that are usually text but can sometimes be binary
	// Check content for these files
	greylistExts := []string{
		".log", ".dat", ".txt", ".cache", ".tmp", ".bak", ".old",
	}

	for _, greyExt := range greylistExts {
		if ext == greyExt {
			return hasBinaryContent(path)
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
