package components

import (
	"fmt"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func sampleItems() []NoteItem {
	base := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	items := make([]NoteItem, 10)
	for i := range items {
		items[i] = NoteItem{
			Path:     fmt.Sprintf("folder/note-%d.md", i),
			Title:    fmt.Sprintf("Note %d", i),
			Folder:   "folder",
			Tags:     []string{"go", "test"},
			Modified: base.Add(time.Duration(-i) * time.Hour),
		}
	}
	return items
}

func keyPress(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -2, Text: key}
}

// Build a KeyPressMsg that String() returns the given key.
// For simple printable characters, we set Text. For special keys we rely on the
// Code field, but the bubbletea v2 KeyPressMsg.String() implementation
// combines Code + Text. For single-char keys like "j", "k", "g", "G" the
// simplest approach is to construct them with the character code.
func simpleKeyPress(ch rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: ch}
}

func TestNoteList_EmptyList(t *testing.T) {
	nl := NewNoteList()
	if nl.SelectedItem() != nil {
		t.Error("expected nil for empty list")
	}
	if nl.ItemCount() != 0 {
		t.Errorf("expected 0 items, got %d", nl.ItemCount())
	}
}

func TestNoteList_SetItems(t *testing.T) {
	nl := NewNoteList()
	items := sampleItems()
	nl.SetItems(items)

	if nl.ItemCount() != 10 {
		t.Errorf("expected 10 items, got %d", nl.ItemCount())
	}
	sel := nl.SelectedItem()
	if sel == nil || sel.Title != "Note 0" {
		t.Errorf("expected first item selected, got %v", sel)
	}
}

func TestNoteList_MoveDown(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 20)

	// Move down with 'j'
	nl, _ = nl.Update(simpleKeyPress('j'))
	if nl.Cursor() != 1 {
		t.Errorf("expected cursor at 1, got %d", nl.Cursor())
	}

	// Move down again
	nl, _ = nl.Update(simpleKeyPress('j'))
	if nl.Cursor() != 2 {
		t.Errorf("expected cursor at 2, got %d", nl.Cursor())
	}
}

func TestNoteList_MoveUp(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 20)

	// Move down then up
	nl, _ = nl.Update(simpleKeyPress('j'))
	nl, _ = nl.Update(simpleKeyPress('j'))
	nl, _ = nl.Update(simpleKeyPress('k'))
	if nl.Cursor() != 1 {
		t.Errorf("expected cursor at 1, got %d", nl.Cursor())
	}
}

func TestNoteList_MoveUpAtTop(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 20)

	nl, _ = nl.Update(simpleKeyPress('k'))
	if nl.Cursor() != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", nl.Cursor())
	}
}

func TestNoteList_MoveDownAtBottom(t *testing.T) {
	nl := NewNoteList()
	items := sampleItems()[:2]
	nl.SetItems(items)
	nl.SetSize(80, 20)

	nl, _ = nl.Update(simpleKeyPress('j'))
	nl, _ = nl.Update(simpleKeyPress('j'))
	if nl.Cursor() != 1 {
		t.Errorf("expected cursor to stay at 1, got %d", nl.Cursor())
	}
}

func TestNoteList_GoToBottom(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 20)

	nl, _ = nl.Update(simpleKeyPress('G'))
	if nl.Cursor() != 9 {
		t.Errorf("expected cursor at 9, got %d", nl.Cursor())
	}
}

func TestNoteList_GoToTop(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 20)

	// Go to bottom first
	nl, _ = nl.Update(simpleKeyPress('G'))

	// gg → go to top
	nl, _ = nl.Update(simpleKeyPress('g'))
	nl, _ = nl.Update(simpleKeyPress('g'))
	if nl.Cursor() != 0 {
		t.Errorf("expected cursor at 0, got %d", nl.Cursor())
	}
}

func TestNoteList_ViewNotEmpty(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 20)

	view := nl.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestNoteList_EmptyView(t *testing.T) {
	nl := NewNoteList()
	nl.SetSize(80, 20)

	view := nl.View()
	if view == "" {
		t.Error("expected non-empty view for empty list")
	}
}
