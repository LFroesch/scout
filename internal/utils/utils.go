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

// FileType represents the category of a file for preview handling
type FileType int

const (
	FileTypeText FileType = iota
	FileTypeCode
	FileTypeMedia
	FileTypeDocument
	FileTypeArchive
	FileTypeExecutable
	FileTypeDatabase
	FileTypeFont
	FileTypeUnknown
)

// GetFileType returns the category of file based on extension
func GetFileType(path string) FileType {
	ext := strings.ToLower(filepath.Ext(path))

	// TEXT FILES (config, logs, data)
	textExts := []string{
		".txt", ".log", ".md", ".markdown", ".rst",
		".json", ".yaml", ".yml", ".toml", ".xml",
		".ini", ".conf", ".cfg", ".env", ".properties",
		".csv", ".tsv", ".sql",
		".inf", // Windows driver info files
	}

	// CODE FILES (previewable source code)
	codeExts := []string{
		// Compiled languages
		".go", ".rs", ".c", ".cpp", ".cc", ".cxx", ".h", ".hpp", ".hxx", ".hh",
		".java", ".cs", ".swift", ".kt", ".scala", ".m", ".mm",
		// Scripting
		".js", ".ts", ".jsx", ".tsx", ".py", ".rb", ".php", ".pl", ".lua",
		".sh", ".bash", ".zsh", ".fish", ".ps1", ".bat", ".cmd",
		// Web
		".html", ".htm", ".css", ".scss", ".sass", ".less", ".vue", ".svelte",
		// Functional/Other
		".clj", ".cljs", ".ex", ".exs", ".erl", ".hrl", ".elm", ".ml", ".r",
		".jl", ".dart", ".v", ".vhdl",
	}

	// MEDIA FILES
	imageExts := []string{
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp",
		".bmp", ".tiff", ".tif", ".psd", ".raw", ".tga", ".dds", ".pcx",
	}
	videoExts := []string{
		".mp4", ".avi", ".mov", ".mkv", ".flv", ".wmv", ".m4v", ".webm",
		".mpg", ".mpeg", ".3gp", ".ogv",
	}
	audioExts := []string{
		".mp3", ".wav", ".flac", ".ogg", ".aac", ".wma", ".m4a",
		".aiff", ".alac", ".au", ".aifc", ".aif", ".opus",
	}

	// DOCUMENTS
	docExts := []string{
		".pdf",
		".doc", ".docx", ".odt", ".rtf",
		".xls", ".xlsx", ".ods",
		".ppt", ".pptx", ".odp",
		".epub", ".mobi", ".azw", ".azw3",
	}

	// ARCHIVES
	archiveExts := []string{
		".zip", ".tar", ".gz", ".rar", ".7z", ".bz2", ".xz",
		".tgz", ".tbz2", ".lz", ".lzma", ".z",
		".deb", ".rpm", ".dmg", ".iso", ".img",
	}

	// EXECUTABLES & COMPILED BINARIES
	execExts := []string{
		// Unix/Linux
		".elf", ".so", ".a", ".o", ".out", ".dylib",
		// Windows
		".exe", ".dll", ".sys", ".com", ".msi",
		// Bytecode/Intermediate
		".class", ".pyc", ".pyo", ".pyd", ".jar", ".war", ".ear",
		".elc", // Emacs compiled Lisp
		// Misc
		".bin", ".dat", ".pak", ".pck",
		// System files
		".cat", ".icm", ".mui",
	}

	// DATABASES
	dbExts := []string{
		".db", ".sqlite", ".sqlite3", ".db3",
		".mdb", ".accdb", ".bson",
	}

	// FONTS
	fontExts := []string{
		".ttf", ".otf", ".woff", ".woff2", ".eot",
	}

	// Check categories
	if contains(textExts, ext) {
		return FileTypeText
	}
	if contains(codeExts, ext) {
		return FileTypeCode
	}
	if contains(imageExts, ext) || contains(videoExts, ext) || contains(audioExts, ext) {
		return FileTypeMedia
	}
	if contains(docExts, ext) {
		return FileTypeDocument
	}
	if contains(archiveExts, ext) {
		return FileTypeArchive
	}
	if contains(execExts, ext) {
		return FileTypeExecutable
	}
	if contains(dbExts, ext) {
		return FileTypeDatabase
	}
	if contains(fontExts, ext) {
		return FileTypeFont
	}

	return FileTypeUnknown
}

// IsTextPreviewable returns true if we should attempt text preview
func IsTextPreviewable(path string) bool {
	fileType := GetFileType(path)

	switch fileType {
	case FileTypeText, FileTypeCode:
		return true
	case FileTypeUnknown:
		// For unknown extensions, do content check
		return !hasBinaryContent(path)
	default:
		return false
	}
}

// hasBinaryContent checks if file contains binary data (for unknown extensions)
func hasBinaryContent(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false // Can't open = assume text to not hide files
	}
	defer file.Close()

	// Read first 8KB (more than 512 for better detection)
	buf := make([]byte, 8192)
	n, err := file.Read(buf)
	if n == 0 {
		return false // Empty file = text
	}
	buf = buf[:n]

	// Quick check: null bytes = binary
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return true
		}
	}

	// Check for high ratio of non-printable/non-UTF8 characters
	nonText := 0
	for i := 0; i < n; i++ {
		b := buf[i]
		// Allow: printable ASCII, whitespace (tab, newline, CR), high bytes (UTF-8)
		if b < 32 && b != 9 && b != 10 && b != 13 {
			nonText++
		}
	}

	// If >10% weird characters, probably binary
	return float64(nonText)/float64(n) > 0.10
}

// contains checks if a string slice contains a specific item
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
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
