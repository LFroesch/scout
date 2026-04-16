package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LFroesch/scout/internal/config"
)

func testModelForUpdate(t *testing.T, currentDir string) model {
	t.Helper()

	searchInput := textinput.New()
	searchInput.Focus()
	textInput := textinput.New()

	m := model{
		mode:        modeSearch,
		currentDir:  currentDir,
		searchInput: searchInput,
		textInput:   textInput,
		config: &config.Config{
			SkipDirectories: []string{},
			MaxResults:      5000,
			MaxDepth:        5,
			MaxFilesScanned: 100000,
			Bookmarks:       []string{},
			ShowHidden:      true,
			PreviewEnabled:  true,
			Frecency:        map[string]int{},
			LastVisited:     map[string]string{},
		},
		showPreview:          true,
		width:                120,
		height:               40,
		previewCache:         make(map[string]previewCacheEntry),
		visitedDirs:          make(map[string]bool),
		doubleClickThreshold: 400 * time.Millisecond,
	}
	m.config.RootPath = ""
	return m
}

func runeKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestSearchEnterLocksResults(t *testing.T) {
	dir := t.TempDir()
	m := testModelForUpdate(t, dir)
	m.filteredFiles = []fileItem{{name: "file.txt", path: filepath.Join(dir, "file.txt")}}

	gotModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := gotModel.(*model)

	if !got.searchResultsLocked {
		t.Fatalf("expected enter to lock search results")
	}
	if got.mode != modeSearch {
		t.Fatalf("expected modeSearch after locking, got %v", got.mode)
	}
}

func TestSearchTabCyclesModesWhenUnlocked(t *testing.T) {
	dir := t.TempDir()
	m := testModelForUpdate(t, dir)

	gotModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := gotModel.(*model)
	if !got.recursiveSearch || got.currentSearchType != searchFilename {
		t.Fatalf("expected first tab to enable recursive filename search")
	}

	gotModel, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = gotModel.(*model)
	if got.currentSearchType != searchContent || got.recursiveSearch {
		t.Fatalf("expected second tab to switch to content search")
	}

	gotModel, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = gotModel.(*model)
	if got.currentSearchType != searchUltra {
		t.Fatalf("expected third tab to switch to ultra search")
	}

	gotModel, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = gotModel.(*model)
	if got.currentSearchType != searchFilename || got.recursiveSearch {
		t.Fatalf("expected fourth tab to reset to current-directory filename search")
	}
}

func TestSearchTypingUsesKeybindLettersWhenUnlocked(t *testing.T) {
	dir := t.TempDir()
	m := testModelForUpdate(t, dir)

	for _, r := range []rune{'f', 'o', 'w', 's', 'y', 'g'} {
		gotModel, _ := m.Update(runeKey(r))
		m = *gotModel.(*model)
	}

	if got := m.searchInput.Value(); got != "fowsyg" {
		t.Fatalf("expected typed letters to stay in search input, got %q", got)
	}
}

func TestLockedSearchEnterOnFileNavigatesToParent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "nested", "note.txt")

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatal(err)
	}

	m := testModelForUpdate(t, dir)
	m.searchResultsLocked = true
	m.searchInput.SetValue("note")
	m.filteredFiles = []fileItem{{name: "note.txt", path: filePath}}

	gotModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := gotModel.(*model)

	if got.mode != modeNormal {
		t.Fatalf("expected enter on locked file result to exit search")
	}
	if got.currentDir != filepath.Dir(filePath) {
		t.Fatalf("expected currentDir %q, got %q", filepath.Dir(filePath), got.currentDir)
	}
	if got.searchResultsLocked {
		t.Fatalf("expected locked state to clear after navigation")
	}
}

func TestLockedSearchEnterOnParentEntryNavigatesUp(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	m := testModelForUpdate(t, child)
	m.searchResultsLocked = true
	m.searchInput.SetValue(".")
	m.filteredFiles = []fileItem{{name: "..", path: parent, isDir: true}}

	gotModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := gotModel.(*model)

	if got.currentDir != parent {
		t.Fatalf("expected enter on '..' to navigate to parent %q, got %q", parent, got.currentDir)
	}
	if got.mode != modeNormal {
		t.Fatalf("expected enter on '..' to exit search")
	}
}

func TestLockedSearchFOnParentEntryNavigatesUp(t *testing.T) {
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}

	m := testModelForUpdate(t, child)
	m.searchResultsLocked = true
	m.searchInput.SetValue(".")
	m.filteredFiles = []fileItem{{name: "..", path: parent, isDir: true}}

	gotModel, _ := m.Update(runeKey('f'))
	got := gotModel.(*model)

	if got.currentDir != parent {
		t.Fatalf("expected f on '..' to navigate to parent %q, got %q", parent, got.currentDir)
	}
	if got.mode != modeNormal {
		t.Fatalf("expected f on '..' to exit search")
	}
}

func TestLockedSearchBackspaceUnlocksAndEditsQuery(t *testing.T) {
	dir := t.TempDir()
	m := testModelForUpdate(t, dir)
	m.searchResultsLocked = true
	m.searchInput.SetValue("abcd")
	m.filteredFiles = []fileItem{{name: "abcd", path: filepath.Join(dir, "abcd")}}

	gotModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	got := gotModel.(*model)

	if got.searchResultsLocked {
		t.Fatalf("expected backspace to unlock search results")
	}
	if got.searchInput.Value() != "abc" {
		t.Fatalf("expected backspace to edit query to %q, got %q", "abc", got.searchInput.Value())
	}
}

func TestPerformAsyncRecursiveSearchRespectsShowHidden(t *testing.T) {
	dir := t.TempDir()
	visible := filepath.Join(dir, "match-visible.txt")
	hidden := filepath.Join(dir, ".match-hidden.txt")

	if err := os.WriteFile(visible, []byte("visible"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hidden, []byte("hidden"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := testModelForUpdate(t, dir)
	m.currentSearchType = searchRecursive
	m.showHidden = false

	m.performAsyncSearch("match")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if m.searchShared != nil {
			m.searchShared.mu.Lock()
			done := m.searchShared.done
			files := append([]fileItem(nil), m.searchShared.files...)
			m.searchShared.mu.Unlock()

			if done {
				if len(files) == 0 {
					t.Fatalf("expected visible file to be found")
				}
				for _, f := range files {
					if strings.HasPrefix(filepath.Base(f.path), ".") {
						t.Fatalf("expected hidden file to be excluded when showHidden=false, got %q", f.path)
					}
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("timed out waiting for async recursive search to finish")
}
