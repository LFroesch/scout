package fileops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

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

// Rename renames a file or directory
func Rename(oldPath, newName string) error {
	newPath := filepath.Join(filepath.Dir(oldPath), newName)
	return os.Rename(oldPath, newPath)
}

// CreateFile creates a new empty file
func CreateFile(dir, name string) error {
	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	return file.Close()
}

// CreateDir creates a new directory
func CreateDir(dir, name string) error {
	path := filepath.Join(dir, name)
	return os.Mkdir(path, 0755)
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
