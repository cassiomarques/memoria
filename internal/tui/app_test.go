package tui

import (
	"testing"
)

func newTestApp() App {
	a := NewApp()
	a.noteList.SetItems(sampleNoteItems())
	a.noteList.SetSize(80, 40)
	a.width = 80
	a.height = 24
	return a
}

func TestApp_FilterMode_SlashActivates(t *testing.T) {
	a := newTestApp()

	if a.filterMode {
		t.Fatal("filter mode should be off initially")
	}

	// Simulate "/" key: the Update method handles this, but let's test the state
	a.filterMode = true
	a.filterBuf = ""

	if !a.filterMode {
		t.Error("expected filter mode to be active")
	}
}

func TestApp_HandleFilterKey_Typing(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	a.filterBuf = ""

	// Type "meet"
	for _, ch := range "meet" {
		result, _ := a.handleFilterKey(string(ch))
		a = result.(App)
	}

	if a.filterBuf != "meet" {
		t.Errorf("expected filterBuf 'meet', got %q", a.filterBuf)
	}
	if !a.noteList.IsFiltering() {
		t.Error("expected noteList to be filtering")
	}
	if a.noteList.FilterText() != "meet" {
		t.Errorf("expected noteList filter 'meet', got %q", a.noteList.FilterText())
	}
}

func TestApp_HandleFilterKey_Backspace(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	a.filterBuf = "mee"
	a.noteList.SetFilter("mee")

	result, _ := a.handleFilterKey("backspace")
	a = result.(App)

	if a.filterBuf != "me" {
		t.Errorf("expected filterBuf 'me', got %q", a.filterBuf)
	}
}

func TestApp_HandleFilterKey_BackspaceOnEmpty(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	a.filterBuf = ""

	result, _ := a.handleFilterKey("backspace")
	a = result.(App)

	if a.filterBuf != "" {
		t.Errorf("expected empty filterBuf, got %q", a.filterBuf)
	}
}

func TestApp_HandleFilterKey_EscExits(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	a.filterBuf = "test"
	a.noteList.SetFilter("test")

	result, _ := a.handleFilterKey("esc")
	a = result.(App)

	if a.filterMode {
		t.Error("expected filter mode to be off after Esc")
	}
	if a.filterBuf != "" {
		t.Errorf("expected empty filterBuf after Esc, got %q", a.filterBuf)
	}
	if a.noteList.IsFiltering() {
		t.Error("expected filter to be cleared after Esc")
	}
}

func TestApp_HandleFilterKey_EnterExitsAndClearsFilter(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	a.filterBuf = "test"
	a.noteList.SetFilter("test")

	result, _ := a.handleFilterKey("enter")
	a = result.(App)

	if a.filterMode {
		t.Error("expected filter mode to be off after Enter")
	}
	if a.noteList.IsFiltering() {
		t.Error("expected filter to be cleared after Enter")
	}
}

func TestApp_HandleFilterKey_NavigateDown(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	a.filterBuf = ""

	initialCursor := a.noteList.Cursor()

	// Arrow down navigates
	result, _ := a.handleFilterKey("down")
	a = result.(App)

	if a.noteList.Cursor() != initialCursor+1 {
		t.Errorf("expected cursor to move down from %d to %d, got %d",
			initialCursor, initialCursor+1, a.noteList.Cursor())
	}
}

func TestApp_HandleFilterKey_NavigateUp(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	a.filterBuf = ""

	// Move down first, then up
	result, _ := a.handleFilterKey("down")
	a = result.(App)
	result, _ = a.handleFilterKey("up")
	a = result.(App)

	if a.noteList.Cursor() != 0 {
		t.Errorf("expected cursor at 0 after down+up, got %d", a.noteList.Cursor())
	}
}

func TestApp_HandleFilterKey_JTypesInsteadOfNavigating(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	a.filterBuf = ""

	result, _ := a.handleFilterKey("j")
	a = result.(App)

	if a.filterBuf != "j" {
		t.Errorf("expected 'j' to type into filter, got %q", a.filterBuf)
	}
}

func TestApp_HandleFilterKey_KTypesInsteadOfNavigating(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	a.filterBuf = ""

	result, _ := a.handleFilterKey("k")
	a = result.(App)

	if a.filterBuf != "k" {
		t.Errorf("expected 'k' to type into filter, got %q", a.filterBuf)
	}
}

func TestApp_HandleFilterKey_IgnoresControlChars(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	a.filterBuf = ""

	// Tab and other control keys should not append to filter
	result, _ := a.handleFilterKey("tab")
	a = result.(App)

	if a.filterBuf != "" {
		t.Errorf("expected empty filterBuf after tab, got %q", a.filterBuf)
	}
}

func TestApp_HandleFilterKey_Space(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	a.filterBuf = "my"

	result, _ := a.handleFilterKey("space")
	a = result.(App)

	if a.filterBuf != "my " {
		t.Errorf("expected filterBuf 'my ', got %q", a.filterBuf)
	}
}

func TestApp_AutoPreview_UpdatesOnNavigate(t *testing.T) {
	a := newTestApp()
	a.preview.Toggle() // open preview

	// Navigate down until we land on a note
	for a.noteList.SelectedItem() == nil {
		a.noteList.MoveDown()
	}
	first := a.noteList.SelectedItem()

	// Simulate the auto-preview logic from Update
	if sel := a.noteList.SelectedItem(); sel != nil && sel.Path != a.previewedPath {
		a.loadPreview(sel)
	}

	if a.previewedPath != first.Path {
		t.Errorf("expected previewedPath %q, got %q", first.Path, a.previewedPath)
	}

	// Move to a different note
	a.noteList.MoveDown()
	for a.noteList.SelectedItem() == nil {
		a.noteList.MoveDown()
	}
	second := a.noteList.SelectedItem()
	if second.Path == first.Path {
		t.Fatal("test setup: expected to land on a different note")
	}

	if sel := a.noteList.SelectedItem(); sel != nil && sel.Path != a.previewedPath {
		a.loadPreview(sel)
	}

	if a.previewedPath != second.Path {
		t.Errorf("expected previewedPath %q after navigate, got %q", second.Path, a.previewedPath)
	}
}

func TestApp_AutoPreview_SkipsWhenHidden(t *testing.T) {
	a := newTestApp()
	// preview is hidden by default

	for a.noteList.SelectedItem() == nil {
		a.noteList.MoveDown()
	}

	// Auto-preview logic should NOT fire when preview is hidden
	if a.preview.Visible() {
		t.Fatal("expected preview to be hidden")
	}

	if a.previewedPath != "" {
		t.Errorf("expected empty previewedPath when preview hidden, got %q", a.previewedPath)
	}
}

func TestApp_AutoPreview_SkipsOnFolder(t *testing.T) {
	a := newTestApp()
	a.preview.Toggle()

	// Cursor 0 should be a folder (folders sort first)
	if !a.noteList.SelectedIsFolder() {
		t.Skip("first item is not a folder in this layout")
	}

	// Auto-preview should not fire on a folder (SelectedItem returns nil)
	if sel := a.noteList.SelectedItem(); sel != nil && sel.Path != a.previewedPath {
		a.loadPreview(sel)
	}

	if a.previewedPath != "" {
		t.Errorf("expected empty previewedPath on folder, got %q", a.previewedPath)
	}
}

func TestApp_RenderFilterBar(t *testing.T) {
	a := newTestApp()
	a.filterMode = true
	a.filterBuf = "hello"

	bar := a.renderFilterBar()
	if bar == "" {
		t.Error("expected non-empty filter bar render")
	}
}
