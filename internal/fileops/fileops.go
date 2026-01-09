package fileops

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// Permission type for validation
type Permission int

const (
	PermRead Permission = iota
	PermWrite
	PermExecute
)

// CheckPermissions validates if the current user has the required permissions
func CheckPermissions(path string, perm Permission) error {
	info, err := os.Stat(path)
	if err != nil {
		return FormatError(err, path, "check permissions")
	}

	mode := info.Mode()

	// Get file permissions
	var hasPermission bool
	switch perm {
	case PermRead:
		hasPermission = mode&0400 != 0 // Owner read
	case PermWrite:
		hasPermission = mode&0200 != 0 // Owner write
	case PermExecute:
		hasPermission = mode&0100 != 0 // Owner execute
	}

	if !hasPermission {
		permName := map[Permission]string{
			PermRead:    "read",
			PermWrite:   "write",
			PermExecute: "execute",
		}[perm]

		return fmt.Errorf("permission denied: no %s access to %s. Try: chmod u+%s \"%s\"",
			permName, filepath.Base(path), string(permName[0]), path)
	}

	// Check parent directory write permission for operations that modify directory
	if perm == PermWrite {
		parentDir := filepath.Dir(path)
		parentInfo, err := os.Stat(parentDir)
		if err != nil {
			return FormatError(err, parentDir, "check parent directory")
		}

		if parentInfo.Mode()&0200 == 0 {
			return fmt.Errorf("permission denied: parent directory %s is not writable. Try: chmod u+w \"%s\"",
				filepath.Base(parentDir), parentDir)
		}
	}

	return nil
}

// FormatError formats an error with helpful context and suggestions
func FormatError(err error, path string, operation string) error {
	if err == nil {
		return nil
	}

	basename := filepath.Base(path)

	// Check for common error types and provide helpful messages
	if os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s does not exist", basename)
	}

	if os.IsPermission(err) {
		return fmt.Errorf("permission denied: cannot %s %s. Try: sudo chmod u+rw \"%s\"",
			operation, basename, path)
	}

	// Check if it's a syscall error for more specific messages
	if pathErr, ok := err.(*os.PathError); ok {
		if errno, ok := pathErr.Err.(syscall.Errno); ok {
			switch errno {
			case syscall.EACCES:
				return fmt.Errorf("permission denied: cannot %s %s. Check file permissions with: ls -l \"%s\"",
					operation, basename, path)
			case syscall.ENOSPC:
				return fmt.Errorf("no space left on device: cannot %s %s. Free up disk space and try again",
					operation, basename)
			case syscall.EROFS:
				return fmt.Errorf("read-only filesystem: cannot %s %s. The filesystem is mounted as read-only",
					operation, basename)
			case syscall.ENOTDIR:
				return fmt.Errorf("not a directory: %s is not a directory", basename)
			case syscall.EISDIR:
				return fmt.Errorf("is a directory: %s is a directory (use recursive delete for directories)",
					basename)
			case syscall.ENOTEMPTY:
				return fmt.Errorf("directory not empty: %s contains files (delete contents first or use recursive delete)",
					basename)
			}
		}
	}

	// Check for cross-device errors
	if strings.Contains(err.Error(), "cross-device") || strings.Contains(err.Error(), "invalid cross-device link") {
		return fmt.Errorf("cannot move across filesystems: %s is on a different device (try copy instead)",
			basename)
	}

	// Default error with operation context
	return fmt.Errorf("failed to %s %s: %v", operation, basename, err)
}

// MoveToTrash moves a file or directory to the system trash/recycle bin
func MoveToTrash(path string) error {
	switch runtime.GOOS {
	case "darwin": // macOS
		// Use AppleScript to move to trash
		script := fmt.Sprintf(`tell application "Finder" to delete POSIX file "%s"`, path)
		cmd := exec.Command("osascript", "-e", script)
		return cmd.Run()

	case "windows":
		// Use PowerShell to move to Recycle Bin
		cmd := exec.Command("powershell", "-Command", fmt.Sprintf(`Add-Type -AssemblyName Microsoft.VisualBasic; [Microsoft.VisualBasic.FileIO.FileSystem]::DeleteFile('%s', 'OnlyErrorDialogs', 'SendToRecycleBin')`, path))
		return cmd.Run()

	default: // Linux and others
		// Try using trash-cli or gio trash on Linux
		if commandExists("gio") {
			cmd := exec.Command("gio", "trash", path)
			return cmd.Run()
		}
		if commandExists("trash-put") {
			cmd := exec.Command("trash-put", path)
			return cmd.Run()
		}
		return fmt.Errorf("trash command not available (install trash-cli or gvfs)")
	}
}

// Delete deletes a file or directory (tries trash first, then permanent delete)
func Delete(path string, isDir bool) error {
	// Try to move to trash first
	if err := MoveToTrash(path); err != nil {
		// Fall back to permanent delete
		if isDir {
			return os.RemoveAll(path)
		}
		return os.Remove(path)
	}
	return nil
}

// undoInfo stores information needed to undo a deletion
type undoInfo struct {
	OriginalPath string    `json:"original_path"`
	TrashPath    string    `json:"trash_path"`
	IsDir        bool      `json:"is_dir"`
	DeletedAt    time.Time `json:"deleted_at"`
}

// getUndoDir returns the directory where undo metadata is stored
func getUndoDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	undoDir := filepath.Join(home, ".config", "scout", "undo")
	if err := os.MkdirAll(undoDir, 0755); err != nil {
		return "", err
	}
	return undoDir, nil
}

// DeleteWithUndo deletes a file or directory and returns the trash path for undo
// Returns (trashPath, error)
func DeleteWithUndo(path string, isDir bool) (string, error) {
	// Check permissions first
	if err := CheckPermissions(path, PermWrite); err != nil {
		return "", err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	// Create a unique identifier for this deletion
	timestamp := time.Now().Format("20060102_150405")
	basename := filepath.Base(absPath)
	undoID := fmt.Sprintf("%s_%s", timestamp, basename)

	// For Linux with trash-cli, files go to ~/.local/share/Trash
	// For simple undo, we'll track the metadata
	undoDir, err := getUndoDir()
	if err != nil {
		return "", err
	}

	// Store undo metadata
	info := undoInfo{
		OriginalPath: absPath,
		TrashPath:    "", // Will be set if we can determine it
		IsDir:        isDir,
		DeletedAt:    time.Now(),
	}

	// Try to determine trash location based on OS
	switch runtime.GOOS {
	case "linux":
		// Linux trash location
		home, _ := os.UserHomeDir()
		if home != "" {
			info.TrashPath = filepath.Join(home, ".local", "share", "Trash", "files", basename)
		}
	case "darwin":
		// macOS trash location
		home, _ := os.UserHomeDir()
		if home != "" {
			info.TrashPath = filepath.Join(home, ".Trash", basename)
		}
	}

	// Save undo info before deletion
	infoPath := filepath.Join(undoDir, undoID+".json")
	data, err := json.Marshal(info)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(infoPath, data, 0644); err != nil {
		return "", err
	}

	// Perform the deletion
	if err := Delete(absPath, isDir); err != nil {
		// Clean up undo info if deletion failed
		os.Remove(infoPath)
		return "", err
	}

	return infoPath, nil
}

// RestoreFromTrash attempts to restore a file from trash
func RestoreFromTrash(undoInfoPath, originalPath string) error {
	if undoInfoPath == "" {
		return fmt.Errorf("undo not available for this deletion")
	}

	// Read undo info
	data, err := os.ReadFile(undoInfoPath)
	if err != nil {
		return fmt.Errorf("undo information not found: %v", err)
	}

	var info undoInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("corrupted undo information: %v", err)
	}

	// Check if trash path exists
	if info.TrashPath == "" {
		return fmt.Errorf("undo not available (trash location unknown)")
	}

	if _, err := os.Stat(info.TrashPath); os.IsNotExist(err) {
		return fmt.Errorf("file no longer in trash (may have been permanently deleted)")
	}

	// Check if target location is available
	if _, err := os.Stat(info.OriginalPath); err == nil {
		return fmt.Errorf("cannot restore: a file already exists at %s", info.OriginalPath)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(info.OriginalPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("cannot create parent directory: %v", err)
	}

	// Move file back from trash
	if err := os.Rename(info.TrashPath, info.OriginalPath); err != nil {
		return fmt.Errorf("failed to restore file: %v", err)
	}

	// Clean up undo info
	os.Remove(undoInfoPath)

	return nil
}


// Rename renames a file or directory
func Rename(oldPath, newName string) error {
	// Check permissions first
	if err := CheckPermissions(oldPath, PermWrite); err != nil {
		return err
	}

	newPath := filepath.Join(filepath.Dir(oldPath), newName)

	// Check if target already exists
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("file already exists: %s already exists in this directory", newName)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return FormatError(err, oldPath, "rename")
	}

	return nil
}

// CreateFile creates a new empty file
func CreateFile(dir, name string) error {
	// Check parent directory permissions
	if err := CheckPermissions(dir, PermWrite); err != nil {
		return err
	}

	path := filepath.Join(dir, name)

	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("file already exists: %s already exists in this directory", name)
	}

	file, err := os.Create(path)
	if err != nil {
		return FormatError(err, path, "create file")
	}
	return file.Close()
}

// CreateDir creates a new directory
func CreateDir(dir, name string) error {
	// Check parent directory permissions
	if err := CheckPermissions(dir, PermWrite); err != nil {
		return err
	}

	path := filepath.Join(dir, name)

	// Check if directory already exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("directory already exists: %s already exists in this directory", name)
	}

	if err := os.Mkdir(path, 0755); err != nil {
		return FormatError(err, path, "create directory")
	}
	return nil
}

// CopyFileOrDir copies a file or directory from src to dst
func CopyFileOrDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcBytes, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, srcBytes, 0644)
}

// copyDir copies a directory recursively
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// CopyMultiple copies multiple files/directories to a destination directory
func CopyMultiple(sources []string, destDir string) error {
	for _, srcPath := range sources {
		destPath := filepath.Join(destDir, filepath.Base(srcPath))
		if err := CopyFileOrDir(srcPath, destPath); err != nil {
			return err
		}
	}
	return nil
}

// MoveMultiple moves multiple files/directories to a destination directory
func MoveMultiple(sources []string, destDir string) error {
	for _, srcPath := range sources {
		destPath := filepath.Join(destDir, filepath.Base(srcPath))
		if err := os.Rename(srcPath, destPath); err != nil {
			// If rename fails (cross-device), copy then delete
			if err := CopyFileOrDir(srcPath, destPath); err != nil {
				return err
			}
			if err := os.RemoveAll(srcPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// commandExists checks if a command is available in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
