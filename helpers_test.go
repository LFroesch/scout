package main

import (
	"strings"
	"testing"
)

func TestCtrlGTargetDir(t *testing.T) {
	m := model{currentDir: "/tmp/project"}

	tests := []struct {
		name     string
		selected fileItem
		want     string
	}{
		{
			name:     "back entry keeps current dir",
			selected: fileItem{name: "..", path: "/tmp"},
			want:     "/tmp/project",
		},
		{
			name:     "directory uses selected dir",
			selected: fileItem{name: "subdir", path: "/tmp/project/subdir", isDir: true},
			want:     "/tmp/project/subdir",
		},
		{
			name:     "file uses containing dir",
			selected: fileItem{name: "main.go", path: "/tmp/project/main.go"},
			want:     "/tmp/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.ctrlGTargetDir(tt.selected); got != tt.want {
				t.Fatalf("ctrlGTargetDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSortMode(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want sortMode
	}{
		{name: "default empty", in: "", want: sortByName},
		{name: "name", in: "name", want: sortByName},
		{name: "size", in: "size", want: sortBySize},
		{name: "date", in: "date", want: sortByDate},
		{name: "type", in: "type", want: sortByType},
		{name: "case insensitive", in: "DATE", want: sortByDate},
		{name: "unknown falls back", in: "weird", want: sortByName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSortMode(tt.in); got != tt.want {
				t.Fatalf("parseSortMode(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNextSearchMode(t *testing.T) {
	tests := []struct {
		name       string
		current    searchType
		recursive  bool
		hasRipgrep bool
		wantType   searchType
		wantRec    bool
		wantMsg    string
	}{
		{"filename to recursive", searchFilename, false, true, searchFilename, true, "recursive file search"},
		{"recursive to content", searchFilename, true, true, searchContent, false, "content search"},
		{"recursive skips missing ripgrep", searchFilename, true, false, searchUltra, false, "ripgrep missing; ultra search (all drives)"},
		{"content to ultra", searchContent, false, true, searchUltra, false, "ultra search (all drives)"},
		{"ultra to current", searchUltra, false, true, searchFilename, false, "current directory file search"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotRec, gotMsg := nextSearchMode(tt.current, tt.recursive, tt.hasRipgrep)
			if gotType != tt.wantType || gotRec != tt.wantRec || gotMsg != tt.wantMsg {
				t.Fatalf("nextSearchMode(...) = (%v, %v, %q), want (%v, %v, %q)", gotType, gotRec, gotMsg, tt.wantType, tt.wantRec, tt.wantMsg)
			}
		})
	}
}

func TestFileDeleteMessage(t *testing.T) {
	withTrash := fileDeleteMessage("demo.txt", true)
	if withTrash == "" || !containsText(withTrash, "move it to trash") {
		t.Fatalf("expected trash-aware delete message, got %q", withTrash)
	}

	withoutTrash := fileDeleteMessage("demo.txt", false)
	if withoutTrash == "" || !containsText(withoutTrash, "permanently deleted") {
		t.Fatalf("expected permanent-delete warning, got %q", withoutTrash)
	}
}

func containsText(s, want string) bool {
	return strings.Contains(s, want)
}
