package fileops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateFile(t *testing.T) {
	tempDir := t.TempDir()

	// Test successful file creation
	err := CreateFile(tempDir, "testfile.txt")
	if err != nil {
		t.Fatalf("CreateFile failed: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(tempDir, "testfile.txt")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("File was not created")
	}

	// Test creating file that already exists
	err = CreateFile(tempDir, "testfile.txt")
	if err == nil {
		t.Error("Expected error when creating existing file")
	}
}

func TestCreateDir(t *testing.T) {
	tempDir := t.TempDir()

	// Test successful directory creation
	err := CreateDir(tempDir, "testdir")
	if err != nil {
		t.Fatalf("CreateDir failed: %v", err)
	}

	// Verify directory exists
	dirPath := filepath.Join(tempDir, "testdir")
	info, err := os.Stat(dirPath)
	if os.IsNotExist(err) {
		t.Error("Directory was not created")
	}
	if err == nil && !info.IsDir() {
		t.Error("Created path is not a directory")
	}

	// Test creating directory that already exists
	err = CreateDir(tempDir, "testdir")
	if err == nil {
		t.Error("Expected error when creating existing directory")
	}
}

func TestRename(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test file
	oldPath := filepath.Join(tempDir, "oldname.txt")
	os.WriteFile(oldPath, []byte("test content"), 0644)

	// Test successful rename
	err := Rename(oldPath, "newname.txt")
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}

	// Verify new file exists
	newPath := filepath.Join(tempDir, "newname.txt")
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("Renamed file does not exist")
	}

	// Verify old file does not exist
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("Old file still exists after rename")
	}

	// Test renaming to existing file
	anotherFile := filepath.Join(tempDir, "another.txt")
	os.WriteFile(anotherFile, []byte("another"), 0644)
	err = Rename(newPath, "another.txt")
	if err == nil {
		t.Error("Expected error when renaming to existing file")
	}
}

func TestCopyFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tempDir, "source.txt")
	content := []byte("test content")
	os.WriteFile(srcPath, content, 0644)

	// Copy file
	dstPath := filepath.Join(tempDir, "dest.txt")
	err := copyFile(srcPath, dstPath)
	if err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify destination exists
	if _, err := os.Stat(dstPath); os.IsNotExist(err) {
		t.Error("Destination file was not created")
	}

	// Verify content matches
	dstContent, _ := os.ReadFile(dstPath)
	if string(dstContent) != string(content) {
		t.Error("Copied file content doesn't match original")
	}
}

func TestCopyDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory structure
	srcDir := filepath.Join(tempDir, "srcdir")
	os.Mkdir(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)

	subdir := filepath.Join(srcDir, "subdir")
	os.Mkdir(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "file2.txt"), []byte("content2"), 0644)

	// Copy directory
	dstDir := filepath.Join(tempDir, "dstdir")
	err := copyDir(srcDir, dstDir)
	if err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify destination directory exists
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		t.Error("Destination directory was not created")
	}

	// Verify files were copied
	if _, err := os.Stat(filepath.Join(dstDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("file1.txt was not copied")
	}

	if _, err := os.Stat(filepath.Join(dstDir, "subdir", "file2.txt")); os.IsNotExist(err) {
		t.Error("subdir/file2.txt was not copied")
	}
}

func TestCopyMultiple(t *testing.T) {
	tempDir := t.TempDir()

	// Create source files
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")
	os.WriteFile(file1, []byte("content1"), 0644)
	os.WriteFile(file2, []byte("content2"), 0644)

	// Create destination directory
	dstDir := filepath.Join(tempDir, "dest")
	os.Mkdir(dstDir, 0755)

	// Copy multiple files
	sources := []string{file1, file2}
	err := CopyMultiple(sources, dstDir)
	if err != nil {
		t.Fatalf("CopyMultiple failed: %v", err)
	}

	// Verify files were copied
	if _, err := os.Stat(filepath.Join(dstDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("file1.txt was not copied to destination")
	}

	if _, err := os.Stat(filepath.Join(dstDir, "file2.txt")); os.IsNotExist(err) {
		t.Error("file2.txt was not copied to destination")
	}
}

func TestFormatError(t *testing.T) {
	// Test with nil error
	err := FormatError(nil, "/test/path", "test operation")
	if err != nil {
		t.Error("FormatError should return nil for nil input")
	}

	// Test with generic error
	genericErr := os.ErrNotExist
	err = FormatError(genericErr, "/test/file.txt", "read")
	if err == nil {
		t.Error("FormatError should return error for non-nil input")
	}
	if err != nil && err.Error() == "" {
		t.Error("FormatError should return non-empty error message")
	}
}

func TestCommandExists(t *testing.T) {
	// Test with a command that should exist
	if !commandExists("ls") {
		t.Error("'ls' command should exist")
	}

	// Test with a command that shouldn't exist
	if commandExists("nonexistentcommandxyz123") {
		t.Error("Nonexistent command should return false")
	}
}
