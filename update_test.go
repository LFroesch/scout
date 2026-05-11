package main

import "testing"

func TestSearchQuestionMarkStaysInInput(t *testing.T) {
	dir := t.TempDir()
	m := testModelForUpdate(t, dir)

	gotModel, _ := m.Update(runeKey('?'))
	got := gotModel.(*model)

	if got.mode != modeSearch {
		t.Fatalf("expected modeSearch, got %v", got.mode)
	}
	if got.searchInput.Value() != "?" {
		t.Fatalf("search input = %q, want ?", got.searchInput.Value())
	}
}
