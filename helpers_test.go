package main

import "testing"

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
