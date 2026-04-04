package components

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func sampleItems() []NoteItem {
	base := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	return []NoteItem{
		{Path: "dotcom/running_azure.md", Title: "running_azure", Folder: "Dotcom", Tags: []string{"azure"}, Modified: base},
		{Path: "dotcom/testing_monolith.md", Title: "testing_monolith", Folder: "Dotcom", Tags: []string{"test"}, Modified: base.Add(-time.Hour)},
		{Path: "go/backporting_deps.md", Title: "backporting_deps", Folder: "Go", Tags: []string{"go"}, Modified: base.Add(-2 * time.Hour)},
		{Path: "projects/codecoverage/e2e_stream.md", Title: "e2e_stream", Folder: "Projects/CodeCoverage", Tags: []string{"qa"}, Modified: base.Add(-3 * time.Hour)},
		{Path: "projects/codecoverage/qa_report.md", Title: "qa_report", Folder: "Projects/CodeCoverage", Tags: []string{"qa"}, Modified: base.Add(-4 * time.Hour)},
		{Path: "random/datadog_monitors.md", Title: "datadog_monitors", Folder: "Random", Tags: []string{"monitoring"}, Modified: base.Add(-5 * time.Hour)},
		{Path: "random/mysql_extended.md", Title: "mysql_extended", Folder: "Random", Tags: []string{"db"}, Modified: base.Add(-6 * time.Hour)},
		{Path: "daily.md", Title: "daily", Folder: "", Tags: nil, Modified: base.Add(-7 * time.Hour)},
		{Path: "scratch.md", Title: "scratch_notes", Folder: "", Tags: nil, Modified: base.Add(-8 * time.Hour)},
	}
}

func keyPress(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -2, Text: key}
}

// simpleKeyPress builds a KeyPressMsg for a single printable character.
func simpleKeyPress(ch rune) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: ch}
}

// visibleNodeCount returns the number of nodes in the flatVisible slice.
func visibleNodeCount(nl *NoteList) int {
	return len(nl.flatVisible)
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

	if nl.ItemCount() != 9 {
		t.Errorf("expected 9 notes, got %d", nl.ItemCount())
	}
	// Cursor starts at 0 which is the first folder (Dotcom)
	sel := nl.SelectedItem()
	if sel != nil {
		t.Errorf("expected nil for folder at cursor 0, got %v", sel)
	}
}

func TestNoteList_TreeStructure(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// All folders expanded (default expandAll=true).
	// Dotcom(folder), Running Azure, Testing Monolith,
	// Go(folder), Backporting Deps,
	// Projects(folder), CodeCoverage(folder), E2e Stream, Qa Report,
	// Random(folder), Datadog Monitors, Mysql Extended,
	// Daily, Scratch Notes
	// = 14 visible
	expected := 14
	got := visibleNodeCount(&nl)
	if got != expected {
		t.Errorf("expected %d visible nodes, got %d", expected, got)
	}
}

func TestNoteList_FolderSelectedItemNil(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Cursor at 0 = Dotcom folder
	if nl.SelectedItem() != nil {
		t.Error("expected nil for folder selection")
	}
}

func TestNoteList_NoteSelectedItem(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Move to first note (cursor 1 = Running Azure under Dotcom)
	nl, _ = nl.Update(simpleKeyPress('j'))
	sel := nl.SelectedItem()
	if sel == nil {
		t.Fatal("expected non-nil for note selection")
	}
	if sel.Title != "running_azure" {
		t.Errorf("expected running_azure, got %s", sel.Title)
	}
}

func TestNoteList_FolderCollapse(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	beforeCount := visibleNodeCount(&nl) // 14

	// Cursor at 0 = Dotcom folder, press Enter to collapse
	nl, _ = nl.Update(keyPress("enter"))

	afterCount := visibleNodeCount(&nl)
	// Dotcom had 2 notes, now hidden
	if afterCount != beforeCount-2 {
		t.Errorf("expected %d visible nodes after collapse, got %d", beforeCount-2, afterCount)
	}
}

func TestNoteList_FolderExpandAfterCollapse(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	original := visibleNodeCount(&nl)

	// Collapse then expand Dotcom
	nl, _ = nl.Update(keyPress("enter"))
	nl, _ = nl.Update(keyPress("enter"))

	if visibleNodeCount(&nl) != original {
		t.Errorf("expected %d visible nodes after re-expand, got %d", original, visibleNodeCount(&nl))
	}

	// Move to first note, should still work
	nl, _ = nl.Update(simpleKeyPress('j'))
	sel := nl.SelectedItem()
	if sel == nil {
		t.Fatal("expected note after expanding folder")
	}
}

func TestNoteList_NavigationSkipsCollapsedChildren(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Collapse Dotcom (cursor at 0)
	nl, _ = nl.Update(keyPress("enter"))

	// Move down: should go to Go folder (cursor 1), skipping collapsed children
	nl, _ = nl.Update(simpleKeyPress('j'))
	if nl.SelectedItem() != nil {
		t.Error("expected nil for Go folder")
	}
	if nl.Cursor() != 1 {
		t.Errorf("expected cursor at 1, got %d", nl.Cursor())
	}

	// Move down to Go's note
	nl, _ = nl.Update(simpleKeyPress('j'))
	sel := nl.SelectedItem()
	if sel == nil {
		t.Fatal("expected note")
	}
	if sel.Title != "backporting_deps" {
		t.Errorf("expected backporting_deps, got %s", sel.Title)
	}
}

func TestNoteList_EnterOnNoteDoesNothing(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Move to a note
	nl, _ = nl.Update(simpleKeyPress('j'))
	beforeCount := visibleNodeCount(&nl)

	// Press Enter on a note — should not change tree
	nl, _ = nl.Update(keyPress("enter"))

	if visibleNodeCount(&nl) != beforeCount {
		t.Error("Enter on a note should not change visible count")
	}
}

func TestNoteList_RootNotesAtBottom(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Go to bottom
	nl, _ = nl.Update(simpleKeyPress('G'))
	sel := nl.SelectedItem()
	if sel == nil {
		t.Fatal("expected root note at bottom")
	}
	// Last item should be "scratch_notes" (Scratch Notes > Daily alphabetically)
	if sel.Title != "scratch_notes" {
		t.Errorf("expected scratch_notes, got %s", sel.Title)
	}

	// Move up one to get Daily
	nl, _ = nl.Update(simpleKeyPress('k'))
	sel = nl.SelectedItem()
	if sel == nil {
		t.Fatal("expected root note")
	}
	if sel.Title != "daily" {
		t.Errorf("expected daily, got %s", sel.Title)
	}
}

func TestNoteList_MoveDown(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	nl, _ = nl.Update(simpleKeyPress('j'))
	if nl.Cursor() != 1 {
		t.Errorf("expected cursor at 1, got %d", nl.Cursor())
	}

	nl, _ = nl.Update(simpleKeyPress('j'))
	if nl.Cursor() != 2 {
		t.Errorf("expected cursor at 2, got %d", nl.Cursor())
	}
}

func TestNoteList_MoveUp(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

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
	nl.SetSize(80, 40)

	nl, _ = nl.Update(simpleKeyPress('k'))
	if nl.Cursor() != 0 {
		t.Errorf("expected cursor to stay at 0, got %d", nl.Cursor())
	}
}

func TestNoteList_MoveDownAtBottom(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Go to bottom then try to go further
	nl, _ = nl.Update(simpleKeyPress('G'))
	last := nl.Cursor()
	nl, _ = nl.Update(simpleKeyPress('j'))
	if nl.Cursor() != last {
		t.Errorf("expected cursor to stay at %d, got %d", last, nl.Cursor())
	}
}

func TestNoteList_GoToBottom(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	nl, _ = nl.Update(simpleKeyPress('G'))
	expected := visibleNodeCount(&nl) - 1
	if nl.Cursor() != expected {
		t.Errorf("expected cursor at %d, got %d", expected, nl.Cursor())
	}
}

func TestNoteList_GoToTop(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Go to bottom first
	nl, _ = nl.Update(simpleKeyPress('G'))

	// gg → go to top
	nl, _ = nl.Update(simpleKeyPress('g'))
	nl, _ = nl.Update(simpleKeyPress('g'))
	if nl.Cursor() != 0 {
		t.Errorf("expected cursor at 0, got %d", nl.Cursor())
	}
}

func TestNoteList_CtrlD_PageDown(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 10) // 10 lines visible with linesPerItem=1

	nl, _ = nl.Update(keyPress("ctrl+d"))
	// Page down moves by visibleCount/2 = 10/2 = 5
	if nl.Cursor() != 5 {
		t.Errorf("expected cursor at 5, got %d", nl.Cursor())
	}
}

func TestNoteList_CtrlU_PageUp(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 10)

	// Go to bottom first
	nl, _ = nl.Update(simpleKeyPress('G'))
	last := nl.Cursor()

	nl, _ = nl.Update(keyPress("ctrl+u"))
	expected := last - 5
	if nl.Cursor() != expected {
		t.Errorf("expected cursor at %d, got %d", expected, nl.Cursor())
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

func TestNoteList_ItemAt(t *testing.T) {
	nl := NewNoteList()
	items := sampleItems()
	nl.SetItems(items)

	// ItemAt indexes into the flat note list (original items)
	for i, item := range items {
		got := nl.ItemAt(i)
		if got == nil {
			t.Fatalf("ItemAt(%d) returned nil", i)
		}
		if got.Title != item.Title {
			t.Errorf("ItemAt(%d): expected %s, got %s", i, item.Title, got.Title)
		}
	}

	// Out of range
	if nl.ItemAt(-1) != nil {
		t.Error("expected nil for negative index")
	}
	if nl.ItemAt(len(items)) != nil {
		t.Error("expected nil for out-of-range index")
	}
}

func TestNoteList_NestedFolderCollapse(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Navigate to Projects folder (index 5: Dotcom, note, note, Go, note, Projects)
	for i := 0; i < 5; i++ {
		nl, _ = nl.Update(simpleKeyPress('j'))
	}

	// Navigate to CodeCoverage folder (next after Projects)
	nl, _ = nl.Update(simpleKeyPress('j'))

	beforeCount := visibleNodeCount(&nl)

	// CodeCoverage starts expanded (default) — collapse it first
	nl, _ = nl.Update(keyPress("enter"))
	collapsedCount := visibleNodeCount(&nl)
	if collapsedCount != beforeCount-2 {
		t.Errorf("expected %d visible after collapsing CodeCoverage, got %d", beforeCount-2, collapsedCount)
	}

	// Now expand CodeCoverage again
	nl, _ = nl.Update(keyPress("enter"))
	afterCount := visibleNodeCount(&nl)
	if afterCount != beforeCount {
		t.Errorf("expected %d visible after re-expanding CodeCoverage, got %d", beforeCount, afterCount)
	}
}

func TestNoteList_CollapseParentHidesNestedChildren(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	beforeCount := visibleNodeCount(&nl)

	// Navigate to Projects folder (index 5)
	for i := 0; i < 5; i++ {
		nl, _ = nl.Update(simpleKeyPress('j'))
	}

	// Collapse Projects — hides CodeCoverage (subfolder) + its 2 notes = 3 nodes hidden
	nl, _ = nl.Update(keyPress("enter"))

	afterCount := visibleNodeCount(&nl)
	if afterCount != beforeCount-3 {
		t.Errorf("expected %d visible after collapsing Projects, got %d", beforeCount-3, afterCount)
	}
}

func TestNoteList_OnlyRootNotes(t *testing.T) {
	nl := NewNoteList()
	items := []NoteItem{
		{Path: "daily.md", Title: "daily", Folder: ""},
		{Path: "scratch.md", Title: "scratch_notes", Folder: ""},
	}
	nl.SetItems(items)
	nl.SetSize(80, 20)

	// No folders, just 2 root notes
	if visibleNodeCount(&nl) != 2 {
		t.Errorf("expected 2 visible nodes, got %d", visibleNodeCount(&nl))
	}

	// First item should be a note (Daily comes before Scratch Notes)
	sel := nl.SelectedItem()
	if sel == nil {
		t.Fatal("expected note for root-level item")
	}
	if sel.Title != "daily" {
		t.Errorf("expected daily, got %s", sel.Title)
	}
}

func TestNoteList_SingleFolder(t *testing.T) {
	nl := NewNoteList()
	base := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	items := []NoteItem{
		{Path: "work/note_a.md", Title: "note_a", Folder: "Work", Modified: base},
		{Path: "work/note_b.md", Title: "note_b", Folder: "Work", Modified: base},
	}
	nl.SetItems(items)
	nl.SetSize(80, 20)

	// 1 folder + 2 notes = 3 visible
	if visibleNodeCount(&nl) != 3 {
		t.Errorf("expected 3 visible nodes, got %d", visibleNodeCount(&nl))
	}

	// Cursor at folder
	if nl.SelectedItem() != nil {
		t.Error("expected nil for folder")
	}

	// Move to first note
	nl, _ = nl.Update(simpleKeyPress('j'))
	sel := nl.SelectedItem()
	if sel == nil || sel.Title != "note_a" {
		t.Errorf("expected note_a, got %v", sel)
	}
}

func TestNoteList_SelectedIsFolder(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Cursor at 0 = Dotcom folder
	if !nl.SelectedIsFolder() {
		t.Error("expected true for folder at cursor 0")
	}

	// Move to first note (Running Azure)
	nl, _ = nl.Update(simpleKeyPress('j'))
	if nl.SelectedIsFolder() {
		t.Error("expected false for note")
	}

	// Empty list
	empty := NewNoteList()
	if empty.SelectedIsFolder() {
		t.Error("expected false for empty list")
	}
}

func TestNoteList_SelectedFolderPath(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Cursor at 0 = Dotcom folder
	if got := nl.SelectedFolderPath(); got != "Dotcom" {
		t.Errorf("expected Dotcom, got %q", got)
	}

	// Navigate to CodeCoverage folder (cursor 6)
	// 0:Dotcom 1:Running Azure 2:Testing Monolith 3:Go 4:Backporting Deps 5:Projects 6:CodeCoverage
	for i := 0; i < 6; i++ {
		nl, _ = nl.Update(simpleKeyPress('j'))
	}
	if got := nl.SelectedFolderPath(); got != "Projects/CodeCoverage" {
		t.Errorf("expected Projects/CodeCoverage, got %q", got)
	}

	// Move to a note (Random folder then its first note)
	nl, _ = nl.Update(simpleKeyPress('j')) // Random folder
	nl, _ = nl.Update(simpleKeyPress('j')) // Datadog Monitors note
	if got := nl.SelectedFolderPath(); got != "" {
		t.Errorf("expected empty string for note, got %q", got)
	}
}

func TestNoteList_SelectedFolderNoteCount(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Cursor at 0 = Dotcom folder → 2 notes
	if got := nl.SelectedFolderNoteCount(); got != 2 {
		t.Errorf("expected 2 notes in Dotcom, got %d", got)
	}

	// Navigate to Projects folder (cursor 5)
	for i := 0; i < 5; i++ {
		nl, _ = nl.Update(simpleKeyPress('j'))
	}
	// Projects contains CodeCoverage (subfolder) with 2 notes → recursive count = 2
	if got := nl.SelectedFolderNoteCount(); got != 2 {
		t.Errorf("expected 2 notes in Projects (recursive), got %d", got)
	}

	// Move to a note
	nl, _ = nl.Update(simpleKeyPress('j')) // CodeCoverage folder
	nl, _ = nl.Update(simpleKeyPress('j')) // Random folder
	nl, _ = nl.Update(simpleKeyPress('j')) // Datadog Monitors note
	if got := nl.SelectedFolderNoteCount(); got != 0 {
		t.Errorf("expected 0 for note, got %d", got)
	}
}

func TestNoteList_SelectedIsExpanded(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Cursor at 0 = Dotcom (top-level, starts expanded)
	if !nl.SelectedIsExpanded() {
		t.Error("expected top-level folder Dotcom to be expanded")
	}

	// Navigate to CodeCoverage (cursor 6, nested, starts expanded with expandAll=true)
	for i := 0; i < 6; i++ {
		nl, _ = nl.Update(simpleKeyPress('j'))
	}
	if !nl.SelectedIsExpanded() {
		t.Error("expected nested folder CodeCoverage to start expanded (expandAll default)")
	}

	// Navigate to a note inside CodeCoverage
	nl, _ = nl.Update(simpleKeyPress('j')) // E2e Stream note
	if nl.SelectedIsExpanded() {
		t.Error("expected false for note")
	}
}

func TestNoteList_CollapseSelectedFromNote(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	beforeCount := visibleNodeCount(&nl) // 12

	// Navigate to Running Azure (cursor 1, note under Dotcom)
	nl, _ = nl.Update(simpleKeyPress('j'))
	if nl.SelectedItem() == nil || nl.SelectedItem().Title != "running_azure" {
		t.Fatal("expected to be on Running Azure note")
	}

	nl.CollapseSelected()

	// Dotcom should now be collapsed, cursor moved to Dotcom folder
	if !nl.SelectedIsFolder() {
		t.Error("expected cursor to be on Dotcom folder after collapse")
	}
	if got := nl.SelectedFolderPath(); got != "Dotcom" {
		t.Errorf("expected cursor on Dotcom, got %q", got)
	}

	// Visible count decreased by 2 (Running Azure + Testing Monolith hidden)
	afterCount := visibleNodeCount(&nl)
	if afterCount != beforeCount-2 {
		t.Errorf("expected %d visible nodes after collapse, got %d", beforeCount-2, afterCount)
	}
}

func TestNoteList_ExpandSelected(t *testing.T) {
	nl := NewNoteList()
	nl.SetExpandAll(false) // nested folders start collapsed
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	beforeCount := visibleNodeCount(&nl) // 12 (nested collapsed)

	// Navigate to CodeCoverage (cursor 6, collapsed)
	for i := 0; i < 6; i++ {
		nl, _ = nl.Update(simpleKeyPress('j'))
	}
	if got := nl.SelectedFolderPath(); got != "Projects/CodeCoverage" {
		t.Fatalf("expected to be on CodeCoverage, got %q", got)
	}

	nl.ExpandSelected()

	// Visible count increased by 2 (E2e Stream + Qa Report now visible)
	afterCount := visibleNodeCount(&nl)
	if afterCount != beforeCount+2 {
		t.Errorf("expected %d visible nodes after expand, got %d", beforeCount+2, afterCount)
	}

	// Cursor stays on CodeCoverage folder
	if got := nl.SelectedFolderPath(); got != "Projects/CodeCoverage" {
		t.Errorf("expected cursor to stay on CodeCoverage, got %q", got)
	}
}

func TestNoteList_NestedFoldersStartCollapsed(t *testing.T) {
	nl := NewNoteList()
	nl.SetExpandAll(false) // explicitly test collapse behavior
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// CodeCoverage's notes (E2e Stream, Qa Report) should NOT be in flatVisible
	for i := 0; i < visibleNodeCount(&nl); i++ {
		node := nl.flatVisible[i]
		if !node.isFolder && node.noteItem != nil {
			if node.noteItem.Folder == "Projects/CodeCoverage" {
				t.Errorf("CodeCoverage note %q should not be visible when collapsed", node.noteItem.Title)
			}
		}
	}

	// Navigate to CodeCoverage and expand it
	for i := 0; i < 6; i++ {
		nl, _ = nl.Update(simpleKeyPress('j'))
	}
	nl.ExpandSelected()

	// Now the notes should appear
	found := 0
	for i := 0; i < visibleNodeCount(&nl); i++ {
		node := nl.flatVisible[i]
		if !node.isFolder && node.noteItem != nil && node.noteItem.Folder == "Projects/CodeCoverage" {
			found++
		}
	}
	if found != 2 {
		t.Errorf("expected 2 CodeCoverage notes visible after expand, got %d", found)
	}
}

func TestNoteList_CollapseAlreadyCollapsed(t *testing.T) {
	nl := NewNoteList()
	nl.SetExpandAll(false) // nested folders start collapsed
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Navigate to CodeCoverage (cursor 6, already collapsed)
	for i := 0; i < 6; i++ {
		nl, _ = nl.Update(simpleKeyPress('j'))
	}

	beforeCount := visibleNodeCount(&nl)
	beforeCursor := nl.Cursor()

	// Collapse an already-collapsed folder — should be a no-op
	nl.CollapseSelected()

	if visibleNodeCount(&nl) != beforeCount {
		t.Errorf("expected no change in visible count, got %d (was %d)", visibleNodeCount(&nl), beforeCount)
	}
	if nl.Cursor() != beforeCursor {
		t.Errorf("expected cursor to stay at %d, got %d", beforeCursor, nl.Cursor())
	}
}

func TestNoteList_ExpandAlreadyExpanded(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Cursor at 0 = Dotcom (already expanded)
	beforeCount := visibleNodeCount(&nl)
	beforeCursor := nl.Cursor()

	nl.ExpandSelected()

	if visibleNodeCount(&nl) != beforeCount {
		t.Errorf("expected no change in visible count, got %d (was %d)", visibleNodeCount(&nl), beforeCount)
	}
	if nl.Cursor() != beforeCursor {
		t.Errorf("expected cursor to stay at %d, got %d", beforeCursor, nl.Cursor())
	}
}

func TestNoteList_ExpandAll(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	nl.CollapseAll()
	collapsedCount := visibleNodeCount(&nl)

	nl.ExpandAll()
	expandedCount := visibleNodeCount(&nl)

	if expandedCount <= collapsedCount {
		t.Errorf("expected more visible nodes after ExpandAll (%d) than after CollapseAll (%d)",
			expandedCount, collapsedCount)
	}

	found := false
	for i := 0; i < nl.ItemCount(); i++ {
		item := nl.ItemAt(i)
		if item != nil && item.Folder == "Projects/CodeCoverage" {
			found = true
			break
		}
	}
	if !found {
		t.Error("nested folder items should be visible after ExpandAll")
	}
}

func TestNoteList_CollapseAll(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	beforeCount := visibleNodeCount(&nl)

	nl.CollapseAll()
	afterCount := visibleNodeCount(&nl)

	if afterCount >= beforeCount {
		t.Errorf("expected fewer visible nodes after CollapseAll (%d) than before (%d)",
			afterCount, beforeCount)
	}

	// Only top-level folders and root-level notes should be visible
	// Folders: Dotcom, Go, Projects, Random = 4, Root notes: daily.md, scratch.md = 2
	expected := 6
	if afterCount != expected {
		t.Errorf("expected %d visible nodes after CollapseAll, got %d", expected, afterCount)
	}
}

func TestNoteList_CollapseAllThenExpandAll_Roundtrip(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	nl.ExpandAll()
	fullyExpanded := visibleNodeCount(&nl)

	nl.CollapseAll()
	nl.ExpandAll()
	roundtrip := visibleNodeCount(&nl)

	if roundtrip != fullyExpanded {
		t.Errorf("expected same count after roundtrip (%d), got %d", fullyExpanded, roundtrip)
	}
}
