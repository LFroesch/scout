package search

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSubstringMatchNames(t *testing.T) {
	names := []string{
		"file1.txt",
		"file2.txt",
		"document.pdf",
		"readme.md",
		"config.json",
	}

	tests := []struct {
		name          string
		query         string
		expectedCount int
	}{
		{"exact match", "file1.txt", 1},
		{"substring match", "file", 2}, // matches "file1" and "file2"
		{"partial match", "doc", 1},
		{"case insensitive", "FILE", 2}, // should match file1 and file2
		{"no match", "xyz", 0},
		{"empty query", "", 0}, // empty query returns no results
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := SubstringMatchNames(tt.query, names)
			if len(results) != tt.expectedCount {
				t.Errorf("SubstringMatchNames(%s) returned %d results, expected %d", tt.query, len(results), tt.expectedCount)
			}
		})
	}
}

func TestRecursiveSearchFiles(t *testing.T) {
	// Create a temporary directory structure
	tempDir := t.TempDir()

	// Create test files
	os.WriteFile(filepath.Join(tempDir, "test1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tempDir, "test2.go"), []byte("package main"), 0644)

	subdir := filepath.Join(tempDir, "subdir")
	os.Mkdir(subdir, 0755)
	os.WriteFile(filepath.Join(subdir, "nested.txt"), []byte("nested"), 0644)

	shouldIgnore := func(name string) bool {
		return false // Don't ignore anything for testing
	}

	// Create a cancel channel (won't be used in test)
	cancelChan := make(chan struct{})
	defer close(cancelChan)

	// Search for "test"
	results, matches := RecursiveSearchFiles("test", tempDir, false, shouldIgnore, cancelChan, nil, 5000, 5, 100000, []string{})

	if len(results) < 2 {
		t.Errorf("Expected at least 2 results for 'test', got %d", len(results))
	}

	if len(matches) != len(results) {
		t.Errorf("Matches length %d doesn't equal results length %d", len(matches), len(results))
	}

	// Verify results have required fields
	for i, result := range results {
		if result.Path == "" {
			t.Errorf("Result %d has empty Path", i)
		}
		if result.DisplayName == "" {
			t.Errorf("Result %d has empty DisplayName", i)
		}
	}
}

func TestRecursiveSearchFilesHidden(t *testing.T) {
	tempDir := t.TempDir()

	// Create visible and hidden files
	os.WriteFile(filepath.Join(tempDir, "visible.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tempDir, ".hidden.txt"), []byte("test"), 0644)

	shouldIgnore := func(name string) bool {
		return false
	}

	// Create a cancel channel (won't be used in test)
	cancelChan := make(chan struct{})
	defer close(cancelChan)

	// Search without showing hidden
	resultsNoHidden, _ := RecursiveSearchFiles("txt", tempDir, false, shouldIgnore, cancelChan, nil, 5000, 5, 100000, []string{})
	foundHidden := false
	for _, result := range resultsNoHidden {
		if filepath.Base(result.Path) == ".hidden.txt" {
			foundHidden = true
		}
	}
	if foundHidden {
		t.Error("Hidden file found when showHidden=false")
	}

	// Search with showing hidden
	resultsWithHidden, _ := RecursiveSearchFiles("txt", tempDir, true, shouldIgnore, cancelChan, nil, 5000, 5, 100000, []string{})
	foundHiddenNow := false
	for _, result := range resultsWithHidden {
		if filepath.Base(result.Path) == ".hidden.txt" {
			foundHiddenNow = true
		}
	}
	if !foundHiddenNow {
		t.Error("Hidden file not found when showHidden=true")
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
