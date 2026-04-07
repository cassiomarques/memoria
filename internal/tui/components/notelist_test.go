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

// --- Fuzzy match tests ---

func TestFuzzyMatch_EmptyPattern(t *testing.T) {
	ok, score := fuzzyMatch("", "anything")
	if !ok {
		t.Error("empty pattern should match anything")
	}
	if score != 0 {
		t.Errorf("expected score 0 for empty pattern, got %d", score)
	}
}

func TestFuzzyMatch_ExactPrefix(t *testing.T) {
	ok, _ := fuzzyMatch("run", "running_azure")
	if !ok {
		t.Error("prefix should match")
	}
}

func TestFuzzyMatch_SubsequenceMatch(t *testing.T) {
	ok, _ := fuzzyMatch("rz", "running_azure")
	if !ok {
		t.Error("subsequence 'rz' should match 'running_azure'")
	}
}

func TestFuzzyMatch_CaseInsensitive(t *testing.T) {
	ok, _ := fuzzyMatch("QA", "qa_report")
	if !ok {
		t.Error("case-insensitive match should work")
	}
}

func TestFuzzyMatch_NoMatch(t *testing.T) {
	ok, _ := fuzzyMatch("xyz", "running_azure")
	if ok {
		t.Error("'xyz' should not match 'running_azure'")
	}
}

func TestFuzzyMatch_ConsecutiveBonusBetterScore(t *testing.T) {
	_, scoreConsec := fuzzyMatch("run", "running_azure")
	_, scoreSpread := fuzzyMatch("rua", "running_azure")
	if scoreConsec >= scoreSpread {
		t.Errorf("consecutive match (%d) should score better (lower) than spread match (%d)",
			scoreConsec, scoreSpread)
	}
}

func TestFuzzyMatch_WordBoundaryBonus(t *testing.T) {
	_, scoreStart := fuzzyMatch("az", "running_azure")
	_, scoreMiddle := fuzzyMatch("zu", "running_azure")
	if scoreStart >= scoreMiddle {
		t.Errorf("word-boundary match (%d) should score better (lower) than mid-word (%d)",
			scoreStart, scoreMiddle)
	}
}

// --- NoteList filter tests ---

func TestNoteList_SetFilter_MatchesByTitle(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	nl.SetFilter("azure")

	// Should find "running_azure" note
	foundAzure := false
	for i := 0; i < len(nl.flatVisible); i++ {
		node := nl.flatVisible[i]
		if !node.isFolder && node.noteItem != nil && node.noteItem.Title == "running_azure" {
			foundAzure = true
			break
		}
	}
	if !foundAzure {
		t.Error("expected to find 'running_azure' note when filtering by 'azure'")
	}
}

func TestNoteList_SetFilter_MatchesByTag(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	nl.SetFilter("monitoring")

	found := false
	for i := 0; i < len(nl.flatVisible); i++ {
		node := nl.flatVisible[i]
		if !node.isFolder && node.noteItem != nil && node.noteItem.Title == "datadog_monitors" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'datadog_monitors' when filtering by tag 'monitoring'")
	}
}

func TestNoteList_SetFilter_ReducesVisibleItems(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	fullCount := visibleNodeCount(&nl)

	nl.SetFilter("qa")
	filteredCount := visibleNodeCount(&nl)

	if filteredCount >= fullCount {
		t.Errorf("filtered count (%d) should be less than full count (%d)", filteredCount, fullCount)
	}
}

func TestNoteList_SetFilter_EmptyRestoresFullList(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	fullCount := visibleNodeCount(&nl)

	nl.SetFilter("qa")
	nl.SetFilter("")

	restoredCount := visibleNodeCount(&nl)
	if restoredCount != fullCount {
		t.Errorf("expected full count (%d) after clearing filter, got %d", fullCount, restoredCount)
	}
}

func TestNoteList_ClearFilter(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	nl.SetFilter("qa")
	if !nl.IsFiltering() {
		t.Error("expected IsFiltering() to be true")
	}

	nl.ClearFilter()
	if nl.IsFiltering() {
		t.Error("expected IsFiltering() to be false after ClearFilter")
	}
}

func TestNoteList_SetFilter_NoMatches(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	nl.SetFilter("zzzznothing")

	noteCount := 0
	for _, node := range nl.flatVisible {
		if !node.isFolder {
			noteCount++
		}
	}
	if noteCount != 0 {
		t.Errorf("expected 0 notes for non-matching filter, got %d", noteCount)
	}
}

func TestNoteList_SetFilter_FuzzySubsequence(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// "dg" should fuzzy-match "datadog_monitors" (d...g)
	nl.SetFilter("dg")

	found := false
	for _, node := range nl.flatVisible {
		if !node.isFolder && node.noteItem != nil && node.noteItem.Title == "datadog_monitors" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected fuzzy match of 'dg' to find 'datadog_monitors'")
	}
}

func TestNoteList_FilterText(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())

	if nl.FilterText() != "" {
		t.Error("expected empty filter text initially")
	}

	nl.SetFilter("test")
	if nl.FilterText() != "test" {
		t.Errorf("expected filter text 'test', got %q", nl.FilterText())
	}

	nl.ClearFilter()
	if nl.FilterText() != "" {
		t.Error("expected empty filter text after clear")
	}
}

func TestNoteList_FilteredCount(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// Unfiltered: all 9 notes
	if nl.FilteredCount() != 9 {
		t.Errorf("expected 9 unfiltered notes, got %d", nl.FilteredCount())
	}

	// "qa" tag matches e2e_stream and qa_report (both in Projects/CodeCoverage)
	nl.SetFilter("qa")
	filtered := nl.FilteredCount()
	if filtered != 2 {
		t.Errorf("expected 2 notes matching 'qa', got %d", filtered)
	}

	// Clear restores all
	nl.ClearFilter()
	if nl.FilteredCount() != 9 {
		t.Errorf("expected 9 notes after clear, got %d", nl.FilteredCount())
	}
}

// --- Search DSL / NoteMatchesFilter tests ---

func TestNoteMatchesFilter_AND_MultipleWords(t *testing.T) {
	item := &NoteItem{Path: "dotcom/running_azure.md", Title: "running_azure", Folder: "Dotcom", Tags: []string{"azure"}}
	// Both "running" and "azure" appear → should match
	ok, _ := NoteMatchesFilter(item, "running azure")
	if !ok {
		t.Error("expected 'running azure' to match 'running_azure' (AND)")
	}
	// "running" matches but "mysql" does not → should NOT match
	ok, _ = NoteMatchesFilter(item, "running mysql")
	if ok {
		t.Error("expected 'running mysql' to NOT match 'running_azure' (AND requires both)")
	}
}

func TestNoteMatchesFilter_ExactPhrase(t *testing.T) {
	item := &NoteItem{Path: "random/mysql_extended.md", Title: "mysql_extended", Folder: "Random", Tags: []string{"db"}}
	// Exact match present
	ok, _ := NoteMatchesFilter(item, `"mysql_extended"`)
	if !ok {
		t.Error(`expected "mysql_extended" exact phrase to match`)
	}
	// Exact match NOT present (wrong order/not substring)
	ok, _ = NoteMatchesFilter(item, `"extended_mysql"`)
	if ok {
		t.Error(`expected "extended_mysql" exact phrase NOT to match`)
	}
}

func TestNoteMatchesFilter_TagFilter(t *testing.T) {
	item := &NoteItem{Path: "dotcom/running_azure.md", Title: "running_azure", Folder: "Dotcom", Tags: []string{"azure", "cloud"}}
	// Tag exists
	ok, _ := NoteMatchesFilter(item, "#azure")
	if !ok {
		t.Error("expected #azure to match note with tag 'azure'")
	}
	// Tag does not exist
	ok, _ = NoteMatchesFilter(item, "#python")
	if ok {
		t.Error("expected #python NOT to match note without tag 'python'")
	}
}

func TestNoteMatchesFilter_TagAndWord(t *testing.T) {
	item := &NoteItem{Path: "dotcom/running_azure.md", Title: "running_azure", Folder: "Dotcom", Tags: []string{"azure"}}
	// Word matches AND tag matches
	ok, _ := NoteMatchesFilter(item, "running #azure")
	if !ok {
		t.Error("expected 'running #azure' to match")
	}
	// Word matches but tag doesn't
	ok, _ = NoteMatchesFilter(item, "running #python")
	if ok {
		t.Error("expected 'running #python' NOT to match (tag mismatch)")
	}
}

func TestNoteMatchesFilter_EmptyQuery(t *testing.T) {
	item := &NoteItem{Path: "daily.md", Title: "daily", Folder: ""}
	ok, _ := NoteMatchesFilter(item, "")
	if !ok {
		t.Error("expected empty query to match everything")
	}
}

func TestNoteMatchesFilter_ExactPhraseInFolder(t *testing.T) {
	item := &NoteItem{Path: "projects/codecoverage/qa_report.md", Title: "qa_report", Folder: "Projects/CodeCoverage", Tags: []string{"qa"}}
	ok, _ := NoteMatchesFilter(item, `"codecoverage"`)
	if !ok {
		t.Error(`expected "codecoverage" to match folder name`)
	}
}

func TestNoteList_SetFilter_AND_MultipleWords(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	// "qa report" should only match qa_report (has both "qa" and "report")
	nl.SetFilter("qa report")

	found := false
	for _, node := range nl.flatVisible {
		if !node.isFolder && node.noteItem != nil && node.noteItem.Title == "qa_report" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'qa report' filter to find 'qa_report'")
	}

	// e2e_stream has tag "qa" but no "report" → should NOT appear
	for _, node := range nl.flatVisible {
		if !node.isFolder && node.noteItem != nil && node.noteItem.Title == "e2e_stream" {
			t.Error("expected 'qa report' filter NOT to find 'e2e_stream' (missing 'report')")
		}
	}
}

func TestNoteList_SetFilter_TagFilter(t *testing.T) {
	nl := NewNoteList()
	nl.SetItems(sampleItems())
	nl.SetSize(80, 40)

	nl.SetFilter("#qa")

	noteCount := 0
	for _, node := range nl.flatVisible {
		if !node.isFolder && node.noteItem != nil {
			noteCount++
			if node.noteItem.Tags == nil || !containsTag(node.noteItem.Tags, "qa") {
				t.Errorf("expected only notes with tag 'qa', found %q", node.noteItem.Title)
			}
		}
	}
	if noteCount != 2 {
		t.Errorf("expected 2 notes with tag 'qa', got %d", noteCount)
	}
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// --- Bookmark/Pin tests ---

func TestBuildTree_PinnedVirtualSection(t *testing.T) {
	items := []NoteItem{
		{Path: "work/meeting.md", Title: "meeting", Folder: "Work"},
		{Path: "daily.md", Title: "daily", Folder: "", Pinned: true},
		{Path: "work/standup.md", Title: "standup", Folder: "Work", Pinned: true},
	}

	nl := NewNoteList()
	nl.SetSize(80, 40)
	nl.SetItems(items)

	nodes := nl.tree
	if len(nodes) == 0 {
		t.Fatal("expected tree nodes")
	}

	// First node should be the virtual "📌 Pinned" folder
	if !nodes[0].isFolder || nodes[0].fullPath != "__pinned__" {
		t.Errorf("expected first node to be virtual pinned folder, got %+v", nodes[0])
	}

	// Virtual folder should contain 2 pinned notes
	if len(nodes[0].children) != 2 {
		t.Errorf("expected 2 pinned children, got %d", len(nodes[0].children))
	}

	// Pinned notes should still exist in their original folders too
	foundInWork := false
	for _, c := range nodes {
		if c.isFolder && c.name == "Work" {
			for _, wc := range c.children {
				if wc.noteItem != nil && wc.noteItem.Path == "work/standup.md" {
					foundInWork = true
				}
			}
		}
	}
	if !foundInWork {
		t.Error("pinned note should still appear in its original folder")
	}
}

func TestNoteItem_PinIcon(t *testing.T) {
	items := []NoteItem{
		{Path: "todo.md", Title: "todo", Folder: "", Pinned: true},
		{Path: "notes.md", Title: "notes", Folder: ""},
	}

	nl := NewNoteList()
	nl.SetSize(80, 10)
	nl.SetItems(items)

	view := nl.View()
	if !containsSubstring(view, "📌") {
		t.Error("expected pin icon in rendered view for pinned note")
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && findSubstring(s, sub)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- Timestamp toggle and formatRelativeTime tests ---

func TestNoteList_ToggleShowModified(t *testing.T) {
	nl := NewNoteList()

	// First call should return true (enabled)
	if got := nl.ToggleShowModified(); !got {
		t.Error("expected ToggleShowModified to return true on first call")
	}
	// Second call should return false (disabled)
	if got := nl.ToggleShowModified(); got {
		t.Error("expected ToggleShowModified to return false on second call")
	}
	// Third call should return true again
	if got := nl.ToggleShowModified(); !got {
		t.Error("expected ToggleShowModified to return true on third call")
	}
}

func TestNoteList_SetShowModified(t *testing.T) {
	nl := NewNoteList()

	nl.SetShowModified(true)
	// Verify by toggling: if currently true, toggle returns false
	if got := nl.ToggleShowModified(); got {
		t.Error("expected false after SetShowModified(true) then toggle")
	}

	nl.SetShowModified(false)
	// Verify by toggling: if currently false, toggle returns true
	if got := nl.ToggleShowModified(); !got {
		t.Error("expected true after SetShowModified(false) then toggle")
	}
}

func TestFormatRelativeTime(t *testing.T) {
	tests := []struct {
		name     string
		offset   time.Duration
		expected string
	}{
		{
			name:     "just now",
			offset:   0,
			expected: "just now",
		},
		{
			name:     "30 seconds ago",
			offset:   30 * time.Second,
			expected: "just now",
		},
		{
			name:     "1 minute ago",
			offset:   time.Minute,
			expected: "1m ago",
		},
		{
			name:     "30 minutes ago",
			offset:   30 * time.Minute,
			expected: "30m ago",
		},
		{
			name:     "1 hour ago",
			offset:   time.Hour,
			expected: "1h ago",
		},
		{
			name:     "5 hours ago",
			offset:   5 * time.Hour,
			expected: "5h ago",
		},
		{
			name:     "yesterday (36 hours ago)",
			offset:   36 * time.Hour,
			expected: "yesterday",
		},
		{
			name:     "3 days ago",
			offset:   3 * 24 * time.Hour,
			expected: "3d ago",
		},
		{
			name:     "6 days ago",
			offset:   6 * 24 * time.Hour,
			expected: "6d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRelativeTime(time.Now().Add(-tt.offset))
			if result != tt.expected {
				t.Errorf("formatRelativeTime(%v ago) = %q, want %q", tt.offset, result, tt.expected)
			}
		})
	}
}

func TestFormatRelativeTime_OlderThanWeek(t *testing.T) {
	// 30 days ago should produce a formatted date like "Dec 07"
	target := time.Now().Add(-30 * 24 * time.Hour)
	result := formatRelativeTime(target)
	expected := target.Format("Jan 02")
	if result != expected {
		t.Errorf("formatRelativeTime(30 days ago) = %q, want %q", result, expected)
	}
}

func TestNoteList_ViewShowsTimestamp(t *testing.T) {
	now := time.Now()
	items := []NoteItem{
		{Path: "recent.md", Title: "recent", Folder: "", Modified: now.Add(-5 * time.Hour)},
	}

	nl := NewNoteList()
	nl.SetSize(80, 20)
	nl.SetShowModified(true)
	nl.SetItems(items)

	view := nl.View()
	// Should contain the time string "5h ago"
	if !containsSubstring(view, "5h ago") {
		t.Errorf("expected view to contain '5h ago' when showModified=true, got:\n%s", view)
	}
}

func TestNoteList_ViewHidesTimestamp(t *testing.T) {
	now := time.Now()
	items := []NoteItem{
		{Path: "recent.md", Title: "recent", Folder: "", Modified: now.Add(-5 * time.Hour)},
	}

	nl := NewNoteList()
	nl.SetSize(80, 20)
	nl.SetShowModified(false) // timestamps OFF
	nl.SetItems(items)

	view := nl.View()
	if containsSubstring(view, "5h ago") {
		t.Errorf("expected view to NOT contain '5h ago' when showModified=false, got:\n%s", view)
	}
}

func TestNoteList_ViewShowsTimestampInFolder(t *testing.T) {
	now := time.Now()
	items := []NoteItem{
		{Path: "work/task.md", Title: "task", Folder: "Work", Modified: now.Add(-2 * time.Hour)},
	}

	nl := NewNoteList()
	nl.SetSize(80, 20)
	nl.SetShowModified(true)
	nl.SetItems(items)

	// Move cursor to the note (skip folder at index 0)
	nl, _ = nl.Update(simpleKeyPress('j'))

	view := nl.View()
	if !containsSubstring(view, "2h ago") {
		t.Errorf("expected view to contain '2h ago' for note in folder, got:\n%s", view)
	}
}

func TestNoteList_ViewShowsJustNow(t *testing.T) {
	items := []NoteItem{
		{Path: "fresh.md", Title: "fresh", Folder: "", Modified: time.Now()},
	}

	nl := NewNoteList()
	nl.SetSize(80, 20)
	nl.SetShowModified(true)
	nl.SetItems(items)

	view := nl.View()
	if !containsSubstring(view, "just now") {
		t.Errorf("expected view to contain 'just now', got:\n%s", view)
	}
}

func TestNoteList_TodoFolderSortsToTop(t *testing.T) {
	items := []NoteItem{
		{Path: "alpha/note1.md", Title: "Note 1", Folder: "Alpha"},
		{Path: "todo/buy-milk.md", Title: "Buy Milk", Folder: "TODO", Todo: true},
		{Path: "beta/note2.md", Title: "Note 2", Folder: "Beta"},
	}

	nl := NewNoteList()
	nl.SetSize(80, 20)
	nl.SetTodoFolder("TODO")
	nl.SetItems(items)

	// Walk the flat visible list; the first folder should be TODO
	var firstFolder string
	for _, node := range nl.flatVisible {
		if node.isFolder && node.fullPath != "__pinned__" {
			firstFolder = node.name
			break
		}
	}

	if firstFolder != "TODO" {
		t.Errorf("expected first folder to be TODO, got %q", firstFolder)
	}
}

func TestNoteList_TodoFolderSortCaseInsensitive(t *testing.T) {
	items := []NoteItem{
		{Path: "alpha/note1.md", Title: "Note 1", Folder: "Alpha"},
		{Path: "todo/buy-milk.md", Title: "Buy Milk", Folder: "todo", Todo: true},
		{Path: "beta/note2.md", Title: "Note 2", Folder: "Beta"},
	}

	nl := NewNoteList()
	nl.SetSize(80, 20)
	nl.SetTodoFolder("TODO") // config says "TODO" but folder is lowercase "todo"
	nl.SetItems(items)

	var firstFolder string
	for _, node := range nl.flatVisible {
		if node.isFolder && node.fullPath != "__pinned__" {
			firstFolder = node.name
			break
		}
	}

	if firstFolder != "todo" {
		t.Errorf("expected first folder to be 'todo', got %q", firstFolder)
	}
}

func TestNoteList_TodoEmojiRendering(t *testing.T) {
	due := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	items := []NoteItem{
		{Path: "TODO/pending-task.md", Title: "Pending Task", Folder: "TODO", Todo: true, Done: false, Due: &due},
		{Path: "TODO/done-task.md", Title: "Done Task", Folder: "TODO", Todo: true, Done: true},
		{Path: "work/regular.md", Title: "Regular Note", Folder: "work"},
	}

	nl := NewNoteList()
	nl.SetSize(80, 20)
	nl.SetItems(items)

	view := nl.View()

	if !containsSubstring(view, "⭕") {
		t.Error("expected view to contain ⭕ for pending todo")
	}
	if !containsSubstring(view, "✅") {
		t.Error("expected view to contain ✅ for done todo")
	}
	// Regular notes should NOT have todo emojis in their line
	if containsSubstring(view, "⭕ Regular Note") || containsSubstring(view, "✅ Regular Note") {
		t.Error("regular note should not have todo emoji prefix")
	}
}

func TestNoteList_TodoDueDateSuffix(t *testing.T) {
	due := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	items := []NoteItem{
		{Path: "TODO/task.md", Title: "Task With Due", Folder: "TODO", Todo: true, Done: false, Due: &due},
		{Path: "TODO/no-due.md", Title: "Task No Due", Folder: "TODO", Todo: true, Done: false},
	}

	nl := NewNoteList()
	nl.SetSize(80, 20)
	nl.SetItems(items)

	view := nl.View()

	if !containsSubstring(view, "@2026-04-15") {
		t.Errorf("expected view to contain due date suffix '@2026-04-15', got:\n%s", view)
	}
}

func TestNoteList_DoneTodoNoDueSuffix(t *testing.T) {
	due := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	items := []NoteItem{
		{Path: "TODO/done.md", Title: "Done Task", Folder: "TODO", Todo: true, Done: true, Due: &due},
	}

	nl := NewNoteList()
	nl.SetSize(80, 20)
	nl.SetItems(items)

	view := nl.View()

	// Done todos should NOT show the due date suffix
	if containsSubstring(view, "@2026-04-15") {
		t.Error("done todo should not show due date suffix")
	}
}

func TestPrepareCursorForDelete_StaysNearby(t *testing.T) {
	nl := NewNoteList()
	nl.SetSize(80, 40)
	nl.SetItems(sampleItems())

	// Move cursor to index 4 (third note: "backporting_deps" under Go)
	// Tree: 0=Dotcom, 1=running_azure, 2=testing_monolith, 3=Go, 4=backporting_deps
	for i := 0; i < 4; i++ {
		nl, _ = nl.Update(simpleKeyPress('j'))
	}
	if nl.Cursor() != 4 {
		t.Fatalf("expected cursor at 4, got %d", nl.Cursor())
	}

	// Simulate deleting the item at cursor 4 — remove backporting_deps
	nl.PrepareCursorForDelete()
	items := sampleItems()
	var remaining []NoteItem
	for _, item := range items {
		if item.Path != "go/backporting_deps.md" {
			remaining = append(remaining, item)
		}
	}
	nl.SetItems(remaining)

	// Cursor should be at 3 (the Go folder, one above the deleted item)
	if nl.Cursor() != 3 {
		t.Errorf("expected cursor at 3 after delete, got %d", nl.Cursor())
	}
}

func TestPrepareCursorForDelete_FirstItem(t *testing.T) {
	nl := NewNoteList()
	nl.SetSize(80, 40)
	nl.SetItems(sampleItems())

	// Cursor at 0 (Dotcom folder). Deleting the first item should keep cursor at 0.
	nl.PrepareCursorForDelete()
	items := sampleItems()[1:] // remove first item
	nl.SetItems(items)

	if nl.Cursor() != 0 {
		t.Errorf("expected cursor at 0 after deleting first item, got %d", nl.Cursor())
	}
}

func TestPrepareCursorForDelete_LastItem(t *testing.T) {
	nl := NewNoteList()
	nl.SetSize(80, 40)
	items := []NoteItem{
		{Path: "a.md", Title: "A", Folder: "", Modified: time.Now()},
		{Path: "b.md", Title: "B", Folder: "", Modified: time.Now()},
	}
	nl.SetItems(items)

	// Move to last item (index 1)
	nl, _ = nl.Update(simpleKeyPress('j'))
	if nl.Cursor() != 1 {
		t.Fatalf("expected cursor at 1, got %d", nl.Cursor())
	}

	// Delete last item — cursor should go to 0
	nl.PrepareCursorForDelete()
	nl.SetItems(items[:1])

	if nl.Cursor() != 0 {
		t.Errorf("expected cursor at 0 after deleting last of 2 items, got %d", nl.Cursor())
	}
}

func TestPinnedFolderPath_ReturnsEmpty(t *testing.T) {
	nl := NewNoteList()
	nl.SetSize(80, 40)
	items := []NoteItem{
		{Path: "a.md", Title: "A", Folder: "", Modified: time.Now(), Pinned: true},
		{Path: "b.md", Title: "B", Folder: "", Modified: time.Now()},
	}
	nl.SetItems(items)

	// Cursor starts at 0 which is the virtual "📌 Pinned" folder
	if !nl.SelectedIsFolder() {
		t.Fatal("expected cursor on folder (Pinned)")
	}

	// SelectedFolderPath must return "" for the virtual pinned folder
	if got := nl.SelectedFolderPath(); got != "" {
		t.Errorf("expected empty path for virtual pinned folder, got %q", got)
	}
}
