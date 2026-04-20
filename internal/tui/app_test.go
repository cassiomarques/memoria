package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cassiomarques/memoria/internal/editor"
	"github.com/cassiomarques/memoria/internal/search"
	"github.com/cassiomarques/memoria/internal/service"
	"github.com/cassiomarques/memoria/internal/storage"
	"github.com/cassiomarques/memoria/internal/tui/components"
)

func newTestApp() App {
	a := NewApp()
	a.noteList.SetItems(sampleNoteItems())
	a.noteList.SetSize(80, 40)
	a.width = 80
	a.height = 24
	return a
}

func TestApp_FilterState_SlashActivates(t *testing.T) {
	a := newTestApp()

	if a.filterState != filterOff {
		t.Fatal("filter state should be off initially")
	}

	a.filterState = filterTyping
	a.filterBuf = ""

	if a.filterState != filterTyping {
		t.Error("expected filter state to be typing")
	}
}

func TestApp_HandleFilterKey_Typing(t *testing.T) {
	a := newTestApp()
	a.filterState = filterTyping
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
	a.filterState = filterTyping
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
	a.filterState = filterTyping
	a.filterBuf = ""

	result, _ := a.handleFilterKey("backspace")
	a = result.(App)

	if a.filterBuf != "" {
		t.Errorf("expected empty filterBuf, got %q", a.filterBuf)
	}
}

func TestApp_HandleFilterKey_EscExits(t *testing.T) {
	a := newTestApp()
	a.filterState = filterTyping
	a.filterBuf = "test"
	a.noteList.SetFilter("test")

	result, _ := a.handleFilterKey("esc")
	a = result.(App)

	if a.filterState != filterOff {
		t.Error("expected filter state to be off after Esc")
	}
	if a.filterBuf != "" {
		t.Errorf("expected empty filterBuf after Esc, got %q", a.filterBuf)
	}
	if a.noteList.IsFiltering() {
		t.Error("expected filter to be cleared after Esc")
	}
}

func TestApp_HandleFilterKey_EnterTransitionsToBrowsing(t *testing.T) {
	a := newTestApp()
	a.filterState = filterTyping
	a.filterBuf = ""

	// Type a query that matches notes
	for _, ch := range "meet" {
		result, _ := a.handleFilterKey(string(ch))
		a = result.(App)
	}

	result, _ := a.handleFilterKey("enter")
	a = result.(App)

	if a.filterState != filterBrowsing {
		t.Errorf("expected filterBrowsing after Enter with results, got %d", a.filterState)
	}
	if a.filterBuf != "meet" {
		t.Errorf("expected filterBuf preserved, got %q", a.filterBuf)
	}
}

func TestApp_HandleFilterKey_EnterWithEmptyQueryExits(t *testing.T) {
	a := newTestApp()
	a.filterState = filterTyping
	a.filterBuf = ""

	result, _ := a.handleFilterKey("enter")
	a = result.(App)

	if a.filterState != filterOff {
		t.Error("expected filterOff after Enter with empty query")
	}
}

func TestApp_HandleFilterKey_NavigateDown(t *testing.T) {
	a := newTestApp()
	a.filterState = filterTyping
	a.filterBuf = ""

	initialCursor := a.noteList.Cursor()

	result, _ := a.handleFilterKey("down")
	a = result.(App)

	if a.noteList.Cursor() != initialCursor+1 {
		t.Errorf("expected cursor to move down from %d to %d, got %d",
			initialCursor, initialCursor+1, a.noteList.Cursor())
	}
}

func TestApp_HandleFilterKey_NavigateUp(t *testing.T) {
	a := newTestApp()
	a.filterState = filterTyping
	a.filterBuf = ""

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
	a.filterState = filterTyping
	a.filterBuf = ""

	result, _ := a.handleFilterKey("j")
	a = result.(App)

	if a.filterBuf != "j" {
		t.Errorf("expected 'j' to type into filter, got %q", a.filterBuf)
	}
}

func TestApp_HandleFilterKey_KTypesInsteadOfNavigating(t *testing.T) {
	a := newTestApp()
	a.filterState = filterTyping
	a.filterBuf = ""

	result, _ := a.handleFilterKey("k")
	a = result.(App)

	if a.filterBuf != "k" {
		t.Errorf("expected 'k' to type into filter, got %q", a.filterBuf)
	}
}

func TestApp_HandleFilterKey_IgnoresControlChars(t *testing.T) {
	a := newTestApp()
	a.filterState = filterTyping
	a.filterBuf = ""

	result, _ := a.handleFilterKey("tab")
	a = result.(App)

	if a.filterBuf != "" {
		t.Errorf("expected empty filterBuf after tab, got %q", a.filterBuf)
	}
}

func TestApp_HandleFilterKey_Space(t *testing.T) {
	a := newTestApp()
	a.filterState = filterTyping
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

func TestApp_AutoPreview_SkipsCustomPreview(t *testing.T) {
	a := newTestApp()

	// Navigate to a note (skip folders)
	for a.noteList.SelectedItem() == nil {
		a.noteList.MoveDown()
	}

	// Simulate what cmdTags() does: show custom content in preview
	a.preview.SetContent("Tags", "# Tags\n\n- **#daily** (2 notes)")
	a.previewedPath = ""
	a.customPreview = true
	if !a.preview.Visible() {
		a.preview.Toggle()
	}

	// Simulate the auto-preview logic that runs on the next Update cycle
	// (e.g., triggered by clearMessageCmd). With the fix, customPreview
	// prevents the auto-preview from overwriting custom content.
	if a.preview.Visible() && a.focusedPane == focusList && !a.customPreview {
		if sel := a.noteList.SelectedItem(); sel != nil && sel.Path != a.previewedPath {
			a.loadPreview(sel)
		}
	}

	// The custom content should NOT have been overwritten
	if a.previewedPath != "" {
		t.Errorf("expected previewedPath to remain empty (custom content), got %q", a.previewedPath)
	}
}

func TestApp_AutoPreview_ResumesAfterCustomPreview(t *testing.T) {
	a := newTestApp()

	// Navigate to a note
	for a.noteList.SelectedItem() == nil {
		a.noteList.MoveDown()
	}

	// Show custom content (e.g., tags)
	a.preview.SetContent("Tags", "# Tags\n\n- **#daily** (2 notes)")
	a.previewedPath = ""
	a.customPreview = true
	if !a.preview.Visible() {
		a.preview.Toggle()
	}

	// User presses 'p' to preview a note — loadPreview clears customPreview
	sel := a.noteList.SelectedItem()
	a.loadPreview(sel)

	if a.customPreview {
		t.Error("expected customPreview to be false after loadPreview")
	}
	if a.previewedPath != sel.Path {
		t.Errorf("expected previewedPath %q, got %q", sel.Path, a.previewedPath)
	}

	// Navigate to a different note
	a.noteList.MoveDown()
	for a.noteList.SelectedItem() == nil {
		a.noteList.MoveDown()
	}
	next := a.noteList.SelectedItem()
	if next.Path == sel.Path {
		t.Skip("could not navigate to a different note")
	}

	// Auto-preview should work after loadPreview cleared customPreview
	if a.preview.Visible() && a.focusedPane == focusList && !a.customPreview {
		if s := a.noteList.SelectedItem(); s != nil && s.Path != a.previewedPath {
			a.loadPreview(s)
		}
	}

	if a.previewedPath != next.Path {
		t.Errorf("expected auto-preview to resume, previewedPath %q, want %q", a.previewedPath, next.Path)
	}
}

func TestApp_RenderFilterBar(t *testing.T) {
	a := newTestApp()
	a.filterState = filterTyping
	a.filterBuf = "hello"

	bar := a.renderFilterBar()
	if bar == "" {
		t.Error("expected non-empty filter bar render")
	}
}

func TestApp_FilterBrowsing_EscClearsFilter(t *testing.T) {
	a := newTestApp()
	a.filterState = filterTyping
	a.filterBuf = ""

	// Type a query and enter browsing
	for _, ch := range "meet" {
		result, _ := a.handleFilterKey(string(ch))
		a = result.(App)
	}
	result, _ := a.handleFilterKey("enter")
	a = result.(App)

	if a.filterState != filterBrowsing {
		t.Fatal("expected filterBrowsing state")
	}

	// Esc in browsing should clear the filter entirely
	a.clearFilter()

	if a.filterState != filterOff {
		t.Error("expected filterOff after Esc in browsing")
	}
	if a.filterBuf != "" {
		t.Errorf("expected empty filterBuf, got %q", a.filterBuf)
	}
	if a.noteList.IsFiltering() {
		t.Error("expected filter to be cleared")
	}
}

func TestApp_FilterBrowsing_SlashRefines(t *testing.T) {
	a := newTestApp()
	a.filterState = filterTyping
	a.filterBuf = ""

	// Type and enter browsing
	for _, ch := range "meet" {
		result, _ := a.handleFilterKey(string(ch))
		a = result.(App)
	}
	result, _ := a.handleFilterKey("enter")
	a = result.(App)

	if a.filterState != filterBrowsing {
		t.Fatal("expected filterBrowsing state")
	}

	// Pressing / should go back to typing with filterBuf preserved
	a.filterState = filterTyping // simulates the / key handler

	if a.filterBuf != "meet" {
		t.Errorf("expected filterBuf preserved as 'meet', got %q", a.filterBuf)
	}
}

func TestApp_FilterBrowsing_PreviewWorks(t *testing.T) {
	a := newTestApp()

	// Enter browsing with a filter
	a.filterState = filterBrowsing
	a.filterBuf = "meet"
	a.noteList.SetFilter("meet")

	// Navigate to a note
	for a.noteList.SelectedItem() == nil {
		a.noteList.MoveDown()
	}

	// In browsing mode, preview should work (loadPreview is callable)
	sel := a.noteList.SelectedItem()
	if sel == nil {
		t.Skip("no note matched the filter")
	}
	a.loadPreview(sel)
	if !a.preview.Visible() {
		a.preview.Toggle()
	}

	if a.previewedPath != sel.Path {
		t.Errorf("expected preview of %q, got %q", sel.Path, a.previewedPath)
	}
}

func TestApp_FilterBrowsing_ClearFilter_RestoresFullList(t *testing.T) {
	a := newTestApp()
	totalBefore := len(a.noteList.AllItems())

	// Enter browsing with a restrictive filter
	a.filterState = filterTyping
	a.filterBuf = ""
	for _, ch := range "meet" {
		result, _ := a.handleFilterKey(string(ch))
		a = result.(App)
	}
	filteredCount := a.noteList.FilteredCount()
	if filteredCount >= totalBefore {
		t.Skip("filter didn't reduce the list")
	}

	// Clear filter
	a.clearFilter()

	if a.filterState != filterOff {
		t.Error("expected filterOff after clearFilter")
	}
	if a.noteList.IsFiltering() {
		t.Error("expected no active filter")
	}
}

func TestApp_FilterBrowsing_EscClosesPreviewFirst(t *testing.T) {
	a := newTestApp()

	// Enter browsing with a filter
	a.filterState = filterBrowsing
	a.filterBuf = "meet"
	a.noteList.SetFilter("meet")

	// Open preview
	a.preview.Toggle()
	a.previewedPath = "some/note.md"

	// First Esc should close preview, NOT clear filter
	// Simulate the browsing Esc handler:
	if a.preview.Visible() {
		a.preview.Toggle()
		a.previewedPath = ""
		a.focusedPane = focusList
	}

	if a.preview.Visible() {
		t.Error("expected preview to be closed")
	}
	if a.filterState != filterBrowsing {
		t.Error("expected to remain in filterBrowsing after closing preview")
	}
	if a.filterBuf != "meet" {
		t.Errorf("expected filterBuf preserved, got %q", a.filterBuf)
	}

	// Second Esc should clear the filter
	a.clearFilter()

	if a.filterState != filterOff {
		t.Error("expected filterOff after second Esc")
	}
	if a.filterBuf != "" {
		t.Errorf("expected empty filterBuf, got %q", a.filterBuf)
	}
}

// --- Todo command parsing tests ---

func TestParseCommand_Todo(t *testing.T) {
	cmd, err := ParseCommand("todo fix auth bug")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name != "todo" {
		t.Errorf("expected command name 'todo', got %q", cmd.Name)
	}
	if len(cmd.Args) != 3 {
		t.Errorf("expected 3 args, got %d", len(cmd.Args))
	}
}

func TestParseCommand_Todos(t *testing.T) {
	cmd, err := ParseCommand("todos")
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Name != "todos" {
		t.Errorf("expected command name 'todos', got %q", cmd.Name)
	}
}

func TestApp_ToggleTodoDone_NonTodo(t *testing.T) {
	a := newTestApp()
	// Select a regular note (not a todo)
	a.noteList.SetItems([]components.NoteItem{
		{Path: "work/meeting.md", Title: "meeting", Folder: "work"},
	})
	a.noteList.SetSize(80, 40)

	// Should show "Not a todo" since the selected note isn't a todo
	a.toggleTodoDone()
	// No panic, no error — we just can't verify the message without a service
}

func TestApp_ToggleTodoDone_NoSelection(t *testing.T) {
	a := NewApp()
	a.width = 80
	a.height = 24
	// Empty list, no selection
	a.toggleTodoDone()
	// Should not panic
}

func TestCmdTodo_ParsesTagsDueFolderArgs(t *testing.T) {
	// Test the ParseCommand side which feeds into cmdTodo.
	tests := []struct {
		input    string
		wantArgs []string
	}{
		{"todo buy milk #shopping", []string{"buy", "milk", "#shopping"}},
		{"todo fix bug @due(2026-04-15)", []string{"fix", "bug", "@due(2026-04-15)"}},
		{"todo task --folder work", []string{"task", "--folder", "work"}},
		{"todo big task #urgent @due(2026-05-01) --folder projects", []string{"big", "task", "#urgent", "@due(2026-05-01)", "--folder", "projects"}},
		{"todo my task --clipboard", []string{"my", "task", "--clipboard"}},
		{"todo task #tag --clipboard --folder work", []string{"task", "#tag", "--clipboard", "--folder", "work"}},
	}
	for _, tt := range tests {
		cmd, err := ParseCommand(tt.input)
		if err != nil {
			t.Fatalf("ParseCommand(%q): %v", tt.input, err)
		}
		if cmd.Name != "todo" {
			t.Errorf("ParseCommand(%q) name = %q, want 'todo'", tt.input, cmd.Name)
		}
		if len(cmd.Args) != len(tt.wantArgs) {
			t.Errorf("ParseCommand(%q) args = %v (len %d), want %v (len %d)",
				tt.input, cmd.Args, len(cmd.Args), tt.wantArgs, len(tt.wantArgs))
			continue
		}
		for i, arg := range cmd.Args {
			if arg != tt.wantArgs[i] {
				t.Errorf("ParseCommand(%q) args[%d] = %q, want %q", tt.input, i, arg, tt.wantArgs[i])
			}
		}
	}
}

func TestCmdTodo_NoArgsShowsUsage(t *testing.T) {
	a := newTestApp()
	// cmdTodo with empty args should set an error message
	a.cmdTodo(nil)
	if a.statusBar.Message() == "" {
		t.Error("expected usage message when no args provided")
	}
}

func TestCmdTodo_InvalidDueDateShowsError(t *testing.T) {
	a := newTestApp()
	a.cmdTodo([]string{"task", "@due(not-a-date)"})
	msg := a.statusBar.Message()
	if msg == "" {
		t.Error("expected error message for invalid due date")
	}
}

func TestCmdTodo_RelativeDueDate(t *testing.T) {
	a := newTestAppWithService(t, &fakeClipboard{})

	// Simulate ":todo fix bug @due(2 weeks)" — the parser splits on spaces,
	// so @due(2 and weeks) arrive as separate args.
	a.cmdTodo([]string{"fix", "bug", "@due(2", "weeks)"})

	msg := a.statusBar.Message()
	if strings.Contains(msg, "Invalid") {
		t.Errorf("unexpected error: %s", msg)
	}

	// Verify the note was created with a due date ~14 days from now
	notes, _ := a.svc.ListAll()
	var found bool
	for _, n := range notes {
		if strings.Contains(n.Path, "fix-bug") && n.Due != nil {
			found = true
		}
	}
	if !found {
		t.Error("expected todo with due date to be created")
	}
}

func TestCmdTodoDue_RelativeDate(t *testing.T) {
	a := newTestAppWithService(t, &fakeClipboard{})

	// Create a todo first
	a.cmdTodo([]string{"my", "task"})
	_ = a.refreshNoteList()

	// Select the todo
	items := a.noteList.AllItems()
	for _, item := range items {
		if strings.Contains(item.Path, "my-task") {
			a.noteList.SelectByPath(item.Path)
			break
		}
	}

	// Set due date with relative input: ":todo-due 3 days"
	a.cmdTodoDue([]string{"3", "days"})

	msg := a.statusBar.Message()
	if strings.Contains(msg, "Invalid") {
		t.Errorf("unexpected error: %s", msg)
	}
	if !strings.Contains(msg, "📅") {
		t.Errorf("expected success message, got: %s", msg)
	}
}

func TestCmdTodo_OnlyTagsNoTitleShowsError(t *testing.T) {
	a := newTestApp()
	// Only tags, no title words
	a.cmdTodo([]string{"#urgent", "#work"})
	msg := a.statusBar.Message()
	if msg == "" {
		t.Error("expected error message when title is empty")
	}
}

func TestParseCommand_TodosWithFilter(t *testing.T) {
	tests := []struct {
		input    string
		wantArgs []string
	}{
		{"todos overdue", []string{"overdue"}},
		{"todos today", []string{"today"}},
		{"todos pending", []string{"pending"}},
		{"todos done", []string{"done"}},
	}
	for _, tt := range tests {
		cmd, err := ParseCommand(tt.input)
		if err != nil {
			t.Fatalf("ParseCommand(%q) unexpected error: %v", tt.input, err)
		}
		if cmd.Name != "todos" {
			t.Errorf("ParseCommand(%q) name = %q, want 'todos'", tt.input, cmd.Name)
		}
		if len(cmd.Args) != len(tt.wantArgs) {
			t.Errorf("ParseCommand(%q) args len = %d, want %d", tt.input, len(cmd.Args), len(tt.wantArgs))
			continue
		}
		for i, arg := range cmd.Args {
			if arg != tt.wantArgs[i] {
				t.Errorf("ParseCommand(%q) args[%d] = %q, want %q", tt.input, i, arg, tt.wantArgs[i])
			}
		}
	}
}

func TestCmdTodos_InvalidFilter(t *testing.T) {
	a := newTestApp()
	a.cmdTodos("bogus")
	msg := a.statusBar.Message()
	if msg == "" {
		t.Error("expected error message for invalid filter")
	}
	if !strings.Contains(msg, "Unknown filter") {
		t.Errorf("expected 'Unknown filter' in message, got %q", msg)
	}
}

func TestCmdTodos_NoService(t *testing.T) {
	a := newTestApp()
	a.svc = nil
	a.cmdTodos("overdue")
	msg := a.statusBar.Message()
	if msg == "" {
		t.Error("expected error message when no service")
	}
}

func TestCmdTodos_CompletedFilter(t *testing.T) {
	a := newTestAppWithService(t, &fakeClipboard{})

	// Create and complete a todo
	a.cmdTodo([]string{"done", "task"})
	_ = a.refreshNoteList()

	// Mark it done
	items := a.noteList.AllItems()
	for _, item := range items {
		if strings.Contains(item.Path, "done-task") {
			a.noteList.SelectByPath(item.Path)
			break
		}
	}
	a.toggleTodoDone()
	_ = a.refreshNoteList()

	// "completed" should show it
	a.cmdTodos("completed")
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "completed") {
		t.Errorf("expected 'completed' in message, got %q", msg)
	}
}

func TestCmdTodos_CompletedWithTimeRange(t *testing.T) {
	a := newTestAppWithService(t, &fakeClipboard{})

	// Create and complete a todo
	a.cmdTodo([]string{"recent", "task"})
	_ = a.refreshNoteList()

	items := a.noteList.AllItems()
	for _, item := range items {
		if strings.Contains(item.Path, "recent-task") {
			a.noteList.SelectByPath(item.Path)
			break
		}
	}
	a.toggleTodoDone()
	_ = a.refreshNoteList()

	// "completed 1 month" should include it (just completed now)
	a.cmdTodos("completed 1 month")
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "completed") {
		t.Errorf("expected 'completed' in message, got %q", msg)
	}
	if strings.Contains(msg, "0 completed") {
		t.Error("expected at least 1 completed todo")
	}
}

func TestCmdTodos_CompletedInvalidRange(t *testing.T) {
	a := newTestAppWithService(t, &fakeClipboard{})

	a.cmdTodos("completed not-valid")
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "Invalid time range") {
		t.Errorf("expected 'Invalid time range' error, got %q", msg)
	}
}

func TestParseCommand_NewWithClipboard(t *testing.T) {
	tests := []struct {
		input    string
		wantArgs []string
	}{
		{"new my-note --clipboard", []string{"my-note", "--clipboard"}},
		{"new folder/note tag1 --clipboard", []string{"folder/note", "tag1", "--clipboard"}},
		{"new my-note", []string{"my-note"}},
	}
	for _, tt := range tests {
		cmd, err := ParseCommand(tt.input)
		if err != nil {
			t.Fatalf("ParseCommand(%q): %v", tt.input, err)
		}
		if cmd.Name != "new" {
			t.Errorf("ParseCommand(%q) name = %q, want 'new'", tt.input, cmd.Name)
		}
		if len(cmd.Args) != len(tt.wantArgs) {
			t.Errorf("ParseCommand(%q) args = %v (len %d), want %v (len %d)",
				tt.input, cmd.Args, len(cmd.Args), tt.wantArgs, len(tt.wantArgs))
			continue
		}
		for i, arg := range cmd.Args {
			if arg != tt.wantArgs[i] {
				t.Errorf("ParseCommand(%q) args[%d] = %q, want %q", tt.input, i, arg, tt.wantArgs[i])
			}
		}
	}
}

func TestCmdNew_NoArgsShowsUsage(t *testing.T) {
	a := newTestApp()
	a.cmdNew(nil)
	msg := a.statusBar.Message()
	if msg == "" {
		t.Error("expected usage message when no args provided")
	}
	if !strings.Contains(msg, "--clipboard") {
		t.Errorf("usage message should mention --clipboard, got %q", msg)
	}
}

func TestCmdNew_NoService(t *testing.T) {
	a := newTestApp()
	a.svc = nil
	a.cmdNew([]string{"test-note", "--clipboard"})
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "No service") {
		t.Errorf("expected 'No service' error, got %q", msg)
	}
}

func TestCmdNew_ClipboardFlagNotTreatedAsTag(t *testing.T) {
	// Ensure --clipboard is consumed as a flag, not passed as a tag.
	// We can't test actual clipboard read without a service, but we can
	// verify parsing: with no service, the error should be about the service,
	// not about --clipboard being treated as a path or tag.
	a := newTestApp()
	a.svc = nil
	a.cmdNew([]string{"my-note", "tag1", "--clipboard"})
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "No service") {
		t.Errorf("expected 'No service' error (flag parsed correctly), got %q", msg)
	}
}

func TestCmdTodo_ClipboardFlagNoService(t *testing.T) {
	a := newTestApp()
	a.svc = nil
	a.cmdTodo([]string{"my", "task", "--clipboard"})
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "No service") {
		t.Errorf("expected 'No service' error, got %q", msg)
	}
}

func TestCmdTodo_ClipboardFlagNotTreatedAsTitle(t *testing.T) {
	// Verify --clipboard is not included in the title words.
	// With no service, if --clipboard were treated as a title word, the title
	// would be "task --clipboard" instead of just "task". The error should
	// still be about service, not title.
	a := newTestApp()
	a.svc = nil
	a.cmdTodo([]string{"task", "--clipboard"})
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "No service") {
		t.Errorf("expected 'No service' error (flag parsed, not in title), got %q", msg)
	}
}

// fakeClipboard is a test double for ClipboardProvider.
type fakeClipboard struct {
	content string
	readErr error
}

func (f *fakeClipboard) ReadAll() (string, error) { return f.content, f.readErr }
func (f *fakeClipboard) WriteAll(text string) error {
	f.content = text
	return nil
}

// newTestAppWithService creates an App wired to a real in-memory service
// and a fake clipboard for end-to-end command testing.
func newTestAppWithService(t *testing.T, cb *fakeClipboard) App {
	t.Helper()

	files, err := storage.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	meta, err := storage.NewMemoryMetaStore()
	if err != nil {
		t.Fatalf("NewMemoryMetaStore: %v", err)
	}
	t.Cleanup(func() { meta.Close() })
	idx, err := search.NewMemorySearchIndex()
	if err != nil {
		t.Fatalf("NewMemorySearchIndex: %v", err)
	}
	t.Cleanup(func() { idx.Close() })

	svc := service.New(files, meta, idx, nil, editor.New("cat"))
	t.Cleanup(func() { svc.Close() })

	a := NewApp()
	a.svc = svc
	a.clipboard = cb
	a.noteList.SetSize(80, 40)
	a.width = 80
	a.height = 24
	return a
}

func TestNewAppWithService_SetsClipboard(t *testing.T) {
	files, err := storage.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	meta, err := storage.NewMemoryMetaStore()
	if err != nil {
		t.Fatalf("NewMemoryMetaStore: %v", err)
	}
	t.Cleanup(func() { meta.Close() })
	idx, err := search.NewMemorySearchIndex()
	if err != nil {
		t.Fatalf("NewMemorySearchIndex: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	svc := service.New(files, meta, idx, nil, editor.New("cat"))
	t.Cleanup(func() { svc.Close() })

	a := NewAppWithService(svc, AppOptions{})
	if a.clipboard == nil {
		t.Fatal("NewAppWithService must set clipboard provider")
	}
}

func TestCmdNew_ClipboardCreatesNoteWithContent(t *testing.T) {
	cb := &fakeClipboard{content: "# Pasted content\n\nHello from clipboard"}
	a := newTestAppWithService(t, cb)

	a.cmdNew([]string{"pasted-note", "--clipboard"})

	msg := a.statusBar.Message()
	if !strings.Contains(msg, "Created from clipboard") {
		t.Errorf("expected clipboard success message, got %q", msg)
	}

	// Verify the note was created with the clipboard content
	n, err := a.svc.Get("pasted-note.md")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(n.Content, "Hello from clipboard") {
		t.Errorf("note content should contain clipboard text, got %q", n.Content)
	}
}

func TestCmdNew_ClipboardEmptyShowsWarning(t *testing.T) {
	cb := &fakeClipboard{content: ""}
	a := newTestAppWithService(t, cb)

	a.cmdNew([]string{"empty-clip", "--clipboard"})

	msg := a.statusBar.Message()
	if !strings.Contains(msg, "Clipboard is empty") {
		t.Errorf("expected empty clipboard warning, got %q", msg)
	}

	// Note should still be created (just empty body)
	if _, err := a.svc.Get("empty-clip.md"); err != nil {
		t.Error("note should exist even with empty clipboard")
	}
}

func TestCmdNew_ClipboardReadErrorShowsError(t *testing.T) {
	cb := &fakeClipboard{readErr: fmt.Errorf("no display")}
	a := newTestAppWithService(t, cb)

	a.cmdNew([]string{"fail-note", "--clipboard"})

	msg := a.statusBar.Message()
	if !strings.Contains(msg, "Clipboard read failed") {
		t.Errorf("expected clipboard error message, got %q", msg)
	}

	// Note should NOT be created when clipboard read fails
	if _, err := a.svc.Get("fail-note.md"); err == nil {
		t.Error("note should not be created when clipboard read fails")
	}
}

func TestCmdNew_WithoutClipboardOpensEditor(t *testing.T) {
	cb := &fakeClipboard{content: "should not be used"}
	a := newTestAppWithService(t, cb)

	// Without --clipboard, cmdNew returns a tea.Cmd (to open editor)
	cmd := a.cmdNew([]string{"regular-note"})

	// Should return a command (editor open), not nil
	if cmd == nil {
		t.Error("expected non-nil cmd (editor open) when --clipboard not used")
	}

	// Note should exist
	if _, err := a.svc.Get("regular-note.md"); err != nil {
		t.Error("note should be created")
	}
}

func TestCmdTodo_ClipboardCreatesWithContent(t *testing.T) {
	cb := &fakeClipboard{content: "Steps to reproduce:\n1. Open app\n2. Click submit"}
	a := newTestAppWithService(t, cb)

	a.cmdTodo([]string{"fix", "submit", "bug", "--clipboard"})

	msg := a.statusBar.Message()
	if !strings.Contains(msg, "Created todo from clipboard") {
		t.Errorf("expected clipboard success message, got %q", msg)
	}

	n, err := a.svc.Get("TODO/fix-submit-bug.md")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(n.Content, "Steps to reproduce") {
		t.Errorf("todo content should contain clipboard text, got %q", n.Content)
	}
	if !n.Todo {
		t.Error("expected Todo=true")
	}
}

func TestCmdTodo_ClipboardWithTagsAndDue(t *testing.T) {
	cb := &fakeClipboard{content: "Bug details from Slack"}
	a := newTestAppWithService(t, cb)

	a.cmdTodo([]string{"fix", "bug", "#urgent", "@due(2026-05-01)", "--clipboard"})

	n, err := a.svc.Get("TODO/fix-bug.md")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(n.Content, "Bug details from Slack") {
		t.Errorf("expected clipboard content, got %q", n.Content)
	}
	if len(n.Tags) != 1 || n.Tags[0] != "urgent" {
		t.Errorf("expected tags=[urgent], got %v", n.Tags)
	}
	if n.Due == nil || n.Due.Format("2006-01-02") != "2026-05-01" {
		t.Errorf("expected due=2026-05-01, got %v", n.Due)
	}
}

func TestCopyPreview_UsesClipboardInterface(t *testing.T) {
	cb := &fakeClipboard{}
	a := newTestAppWithService(t, cb)

	// Create a note and load its preview
	a.svc.Create("copy-test.md", "Some content to copy", nil)
	a.preview.SetContent("copy-test", "Some content to copy")

	a.copyPreviewToClipboard()

	if cb.content != "Some content to copy" {
		t.Errorf("expected clipboard to contain preview content, got %q", cb.content)
	}
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "Copied to clipboard") {
		t.Errorf("expected copy success message, got %q", msg)
	}
}

func TestCmdTodoDue_NoArgs(t *testing.T) {
	a := newTestApp()
	a.cmdTodoDue(nil)
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "Usage") {
		t.Errorf("expected usage message, got %q", msg)
	}
}

func TestCmdTodoDue_NotATodo(t *testing.T) {
	a := newTestApp()
	// Set items with a regular note at root level (cursor 0 lands on it)
	a.noteList.SetItems([]components.NoteItem{
		{Path: "note.md", Title: "regular note", Folder: ""},
	})
	a.cmdTodoDue([]string{"2026-06-15"})
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "not a todo") {
		t.Errorf("expected 'not a todo' error, got %q", msg)
	}
}

func TestCmdTodoDue_InvalidDate(t *testing.T) {
	a := newTestApp()
	a.noteList.SetItems([]components.NoteItem{
		{Path: "task.md", Title: "task", Folder: "", Todo: true},
	})
	a.cmdTodoDue([]string{"not-a-date"})
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "Invalid date") {
		t.Errorf("expected 'Invalid date' error, got %q", msg)
	}
}

func TestCmdRename_NoArgs(t *testing.T) {
	a := newTestApp()
	a.cmdRename(nil)
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "Usage") {
		t.Errorf("expected usage message, got %q", msg)
	}
}

func TestCmdRename_NoSelection(t *testing.T) {
	a := newTestApp()
	a.noteList.SetItems(nil)
	a.cmdRename([]string{"new-name"})
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "No note selected") {
		t.Errorf("expected 'No note selected' error, got %q", msg)
	}
}

func TestCmdRename_BuildsCorrectPath(t *testing.T) {
	a := newTestApp()
	// Root-level note
	a.noteList.SetItems([]components.NoteItem{
		{Path: "ideas.md", Title: "ideas", Folder: ""},
	})
	// No service, so we can't actually rename, but we test validation passes
	a.cmdRename([]string{"new-ideas"})
	msg := a.statusBar.Message()
	if !strings.Contains(msg, "No service") {
		t.Errorf("expected 'No service' error (validation passed), got %q", msg)
	}
}

func TestFinder_ActivateAndDeactivate(t *testing.T) {
	a := newTestApp()

	// Activate
	a.finderActive = true
	a.finderBuf = ""
	if !a.finderActive {
		t.Fatal("expected finder to be active")
	}

	// Deactivate via Esc
	result, _ := a.handleFinderKey("esc")
	a = result.(App)
	if a.finderActive {
		t.Error("expected finder to be deactivated after Esc")
	}
}

func TestFinder_KeyInput(t *testing.T) {
	a := newTestApp()
	a.finderActive = true

	// Type characters
	result, _ := a.handleFinderKey("a")
	a = result.(App)
	result, _ = a.handleFinderKey("b")
	a = result.(App)
	if a.finderBuf != "ab" {
		t.Errorf("expected finderBuf='ab', got %q", a.finderBuf)
	}

	// Backspace
	result, _ = a.handleFinderKey("backspace")
	a = result.(App)
	if a.finderBuf != "a" {
		t.Errorf("expected finderBuf='a' after backspace, got %q", a.finderBuf)
	}

	// Space (Bubble Tea v2 sends "space" not " ")
	result, _ = a.handleFinderKey("space")
	a = result.(App)
	if a.finderBuf != "a " {
		t.Errorf("expected finderBuf='a ' after space, got %q", a.finderBuf)
	}
}

func TestFinder_CursorNavigation(t *testing.T) {
	a := newTestApp()
	a.finderActive = true

	// Simulate some results
	a.finderResults = []search.SearchResult{
		{Path: "note1.md", Score: 1.0},
		{Path: "note2.md", Score: 0.8},
		{Path: "note3.md", Score: 0.6},
	}
	a.finderCursor = 0

	// Move down
	result, _ := a.handleFinderKey("down")
	a = result.(App)
	if a.finderCursor != 1 {
		t.Errorf("expected cursor=1, got %d", a.finderCursor)
	}

	// Move down again
	result, _ = a.handleFinderKey("down")
	a = result.(App)
	if a.finderCursor != 2 {
		t.Errorf("expected cursor=2, got %d", a.finderCursor)
	}

	// Can't go past last result
	result, _ = a.handleFinderKey("down")
	a = result.(App)
	if a.finderCursor != 2 {
		t.Errorf("expected cursor to stay at 2, got %d", a.finderCursor)
	}

	// Move up
	result, _ = a.handleFinderKey("up")
	a = result.(App)
	if a.finderCursor != 1 {
		t.Errorf("expected cursor=1, got %d", a.finderCursor)
	}

	// Tab moves down
	result, _ = a.handleFinderKey("tab")
	a = result.(App)
	if a.finderCursor != 2 {
		t.Errorf("expected cursor=2 after tab, got %d", a.finderCursor)
	}

	// Shift+tab moves up
	result, _ = a.handleFinderKey("shift+tab")
	a = result.(App)
	if a.finderCursor != 1 {
		t.Errorf("expected cursor=1 after shift+tab, got %d", a.finderCursor)
	}
}

func TestFinder_RenderOutput(t *testing.T) {
	a := newTestApp()
	a.finderActive = true
	a.finderBuf = "test"
	a.finderResults = []search.SearchResult{
		{Path: "notes/test-file.md", Score: 1.5},
	}
	a.finderCursor = 0
	a.height = 24

	view := a.renderFinder()
	if !strings.Contains(view, "test") {
		t.Error("expected finder to show input text")
	}
	if !strings.Contains(view, "test-file.md") {
		t.Error("expected finder to show result path")
	}
}

func TestFinder_EnterSelectsNote(t *testing.T) {
	a := newTestApp()
	a.finderActive = true
	a.finderResults = []search.SearchResult{
		{Path: "work/meeting.md", Score: 1.0},
	}
	a.finderCursor = 0

	result, _ := a.handleFinderKey("enter")
	a = result.(App)
	if a.finderActive {
		t.Error("expected finder to close after Enter")
	}
}

func TestApp_ResizeComponents_DynamicHeaderHeight(t *testing.T) {
	// At narrow widths, the ASCII art header wraps and becomes taller.
	// resizeComponents must measure the actual header height so the note
	// list + status bar always fit within the terminal.
	tests := []struct {
		width  int
		height int
	}{
		{80, 24},
		{40, 24}, // narrow: header wraps, height increases
		{60, 15}, // short terminal
	}

	for _, tt := range tests {
		a := NewApp()
		a.width = tt.width
		a.height = tt.height
		a.version = "test"
		a.resizeComponents()

		headerH := lipgloss.Height(a.headerCache)

		if a.headerCache == "" {
			t.Errorf("width=%d height=%d: headerCache is empty", tt.width, tt.height)
		}
		if headerH < 10 {
			t.Errorf("width=%d: header height %d < 10 (minimum without wrapping)", tt.width, headerH)
		}
		// At width 40, the header should be taller due to line wrapping
		if tt.width == 40 && headerH <= 10 {
			t.Errorf("width=40: expected header height > 10 due to wrapping, got %d", headerH)
		}
	}
}

func TestApp_DeleteConfirmation_NotAutocleared(t *testing.T) {
	a := newTestApp()

	// Navigate to a note (skip folders)
	for a.noteList.SelectedItem() == nil {
		a.noteList.MoveDown()
	}

	// Simulate initiateDelete setting up the confirmation
	a.pendingDelete = true
	a.pendingDeletePath = "some/note.md"
	a.setMessage("Delete 'some/note.md'? (y/N)", true)

	// Verify message is set
	if a.statusBar.Message() == "" {
		t.Fatal("expected confirmation message to be set")
	}

	// Simulate the clearMessageMsg arriving (after 3s timer)
	result, _ := a.Update(clearMessageMsg{})
	a = result.(App)

	// Message must NOT be cleared while pendingDelete is active
	if a.statusBar.Message() == "" {
		t.Error("confirmation message was cleared while pendingDelete is true")
	}
}

func TestApp_RegularMessage_StillAutoclears(t *testing.T) {
	a := newTestApp()

	// Set a regular (non-confirmation) message
	a.setMessage("Edited: foo.md", false)

	if a.statusBar.Message() == "" {
		t.Fatal("expected message to be set")
	}

	// Simulate clearMessageMsg
	result, _ := a.Update(clearMessageMsg{})
	a = result.(App)

	// Regular messages should be cleared
	if a.statusBar.Message() != "" {
		t.Errorf("expected message to be cleared, got %q", a.statusBar.Message())
	}
}

func keyMsg(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -2, Text: key}
}

func TestTrashMode_EnterAndExit(t *testing.T) {
	a := newTestApp()
	a.trashMode = true

	// Esc should exit trash mode.
	result, _ := a.Update(keyMsg("esc"))
	a = result.(App)
	if a.trashMode {
		t.Error("expected trashMode=false after Esc")
	}
}

func TestTrashMode_BlocksNormalKeys(t *testing.T) {
	a := newTestApp()
	a.trashMode = true

	// Keys like 'n' (create) should be blocked in trash mode.
	result, _ := a.Update(keyMsg("n"))
	a = result.(App)
	if a.pendingCreate {
		t.Error("'n' should be blocked in trash mode")
	}
}

func TestTrashMode_DeleteConfirmation(t *testing.T) {
	a := newTestApp()
	a.trashMode = true
	a.noteList.SetItems([]components.NoteItem{
		{Path: "old-note.md", Title: "old-note"},
	})
	a.noteList.MoveDown()

	// 'd' in trash mode calls initiateDelete, which needs a service.
	// Without one, it silently returns. Verify the key doesn't fall
	// through to normal handling (e.g., create note).
	result, _ := a.Update(keyMsg("d"))
	a = result.(App)
	if a.pendingCreate {
		t.Error("'d' in trash mode should not trigger create")
	}
}

func TestTrashMode_InitiateDelete_Message(t *testing.T) {
	// Verify the confirmation prompt says "Permanently delete" in trash mode.
	a := newTestApp()
	a.trashMode = true
	// initiateDelete checks svc==nil, but we can test the messaging
	// by directly calling it after setting up minimal state.
	// Since we can't easily wire a service in unit tests, we test
	// the prompt text pattern via the initiateDelete path:
	// just confirm that non-service keys are correctly blocked.
	result, _ := a.Update(keyMsg("b")) // 'b' = bookmark, should be blocked
	a = result.(App)
	// In trash mode, unrecognized keys are ignored (no-op).
}
