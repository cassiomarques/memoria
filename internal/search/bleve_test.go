package search

import (
	"testing"

	"github.com/cassiomarques/memoria/internal/note"
)

func newTestNote(t *testing.T, path, content string, tags []string) *note.Note {
	t.Helper()
	n, err := note.NewNote(path, content, tags)
	if err != nil {
		t.Fatalf("failed to create test note: %v", err)
	}
	return n
}

func newTestIndex(t *testing.T) *SearchIndex {
	t.Helper()
	idx, err := NewMemorySearchIndex()
	if err != nil {
		t.Fatalf("failed to create memory index: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	return idx
}

func TestSearchByContent(t *testing.T) {
	idx := newTestIndex(t)
	n := newTestNote(t, "notes/hello.md", "Go is a statically typed programming language", []string{"golang"})
	if err := idx.Index(n); err != nil {
		t.Fatalf("Index: %v", err)
	}

	results, err := idx.Search("programming language", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != "notes/hello.md" {
		t.Errorf("expected path notes/hello.md, got %s", results[0].Path)
	}
}

func TestSearchByTitle(t *testing.T) {
	idx := newTestIndex(t)
	n := newTestNote(t, "notes/kubernetes-guide.md", "Some content about containers", []string{"devops"})
	if err := idx.Index(n); err != nil {
		t.Fatalf("Index: %v", err)
	}

	results, err := idx.Search("title:kubernetes", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != "notes/kubernetes-guide.md" {
		t.Errorf("expected path notes/kubernetes-guide.md, got %s", results[0].Path)
	}
}

func TestSearchByTag(t *testing.T) {
	idx := newTestIndex(t)
	n := newTestNote(t, "notes/rust.md", "Memory safety without garbage collection", []string{"rust", "systems"})
	if err := idx.Index(n); err != nil {
		t.Fatalf("Index: %v", err)
	}

	results, err := idx.Search("tags:rust", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != "notes/rust.md" {
		t.Errorf("expected path notes/rust.md, got %s", results[0].Path)
	}
}

func TestSearchByFolder(t *testing.T) {
	idx := newTestIndex(t)
	n := newTestNote(t, "work/meeting.md", "Discussed project deadlines", []string{"meeting"})
	if err := idx.Index(n); err != nil {
		t.Fatalf("Index: %v", err)
	}

	results, err := idx.Search("folder:work", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Path != "work/meeting.md" {
		t.Errorf("expected path work/meeting.md, got %s", results[0].Path)
	}
}

func TestSearchSortedByRelevance(t *testing.T) {
	idx := newTestIndex(t)

	// Note with "golang" only in tags
	n1 := newTestNote(t, "notes/other.md", "Some unrelated content about cooking", []string{"golang"})
	// Note with "golang" in both title and content — should score higher
	n2 := newTestNote(t, "notes/golang-tutorial.md", "This is a comprehensive golang tutorial about golang programming", []string{"golang", "tutorial"})

	if err := idx.Index(n1); err != nil {
		t.Fatalf("Index n1: %v", err)
	}
	if err := idx.Index(n2); err != nil {
		t.Fatalf("Index n2: %v", err)
	}

	results, err := idx.Search("golang", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	// The note with golang in more fields should rank first
	if results[0].Path != "notes/golang-tutorial.md" {
		t.Errorf("expected first result to be golang-tutorial.md, got %s", results[0].Path)
	}
	if results[0].Score < results[1].Score {
		t.Errorf("results should be sorted by score descending: %f < %f", results[0].Score, results[1].Score)
	}
}

func TestSearchFuzzyWithTypos(t *testing.T) {
	idx := newTestIndex(t)
	n := newTestNote(t, "notes/golang.md", "The golang programming language is great", []string{"golang"})
	if err := idx.Index(n); err != nil {
		t.Fatalf("Index: %v", err)
	}

	// "golan" is a typo for "golang"
	results, err := idx.SearchFuzzy("golan", 10)
	if err != nil {
		t.Fatalf("SearchFuzzy: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected fuzzy search to find 'golang' when searching for 'golan'")
	}
	if results[0].Path != "notes/golang.md" {
		t.Errorf("expected path notes/golang.md, got %s", results[0].Path)
	}
}

func TestRemoveNote(t *testing.T) {
	idx := newTestIndex(t)
	n := newTestNote(t, "notes/remove-me.md", "This note will be removed", []string{"temp"})
	if err := idx.Index(n); err != nil {
		t.Fatalf("Index: %v", err)
	}

	// Verify it's searchable
	results, err := idx.Search("removed", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result before remove, got %d", len(results))
	}

	// Remove it
	if err := idx.Remove("notes/remove-me.md"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Verify it's gone
	results, err = idx.Search("removed", 10)
	if err != nil {
		t.Fatalf("Search after remove: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results after remove, got %d", len(results))
	}
}

func TestReindex(t *testing.T) {
	idx := newTestIndex(t)

	// Index some initial notes
	n1 := newTestNote(t, "notes/old.md", "Old content", []string{"old"})
	if err := idx.Index(n1); err != nil {
		t.Fatalf("Index: %v", err)
	}

	count, err := idx.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	// Reindex with completely new notes
	newNotes := []*note.Note{
		newTestNote(t, "notes/new1.md", "First new note", []string{"new"}),
		newTestNote(t, "notes/new2.md", "Second new note", []string{"new"}),
	}
	if err := idx.Reindex(newNotes); err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	count, err = idx.Count()
	if err != nil {
		t.Fatalf("Count after reindex: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count 2 after reindex, got %d", count)
	}

	// Old note should be gone
	results, err := idx.Search("old", 10)
	if err != nil {
		t.Fatalf("Search for old: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for old content after reindex, got %d", len(results))
	}

	// New notes should be present
	results, err = idx.Search("new note", 10)
	if err != nil {
		t.Fatalf("Search for new: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for new content, got %d", len(results))
	}
}

func TestCount(t *testing.T) {
	idx := newTestIndex(t)

	count, err := idx.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 on empty index, got %d", count)
	}

	notes := []*note.Note{
		newTestNote(t, "a.md", "alpha", nil),
		newTestNote(t, "b.md", "bravo", nil),
		newTestNote(t, "c.md", "charlie", nil),
	}
	for _, n := range notes {
		if err := idx.Index(n); err != nil {
			t.Fatalf("Index: %v", err)
		}
	}

	count, err = idx.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestSearchNoResults(t *testing.T) {
	idx := newTestIndex(t)
	n := newTestNote(t, "notes/hello.md", "Hello world", nil)
	if err := idx.Index(n); err != nil {
		t.Fatalf("Index: %v", err)
	}

	results, err := idx.Search("xyznonexistent", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if results == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchWithLimit(t *testing.T) {
	idx := newTestIndex(t)

	// Index 5 notes all containing "common"
	for _, path := range []string{"a.md", "b.md", "c.md", "d.md", "e.md"} {
		n := newTestNote(t, path, "common keyword in all notes", nil)
		if err := idx.Index(n); err != nil {
			t.Fatalf("Index %s: %v", path, err)
		}
	}

	tests := []struct {
		name  string
		limit int
		max   int
	}{
		{"limit 2", 2, 2},
		{"limit 3", 3, 3},
		{"limit 0 defaults to 20", 0, 5}, // only 5 docs total
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := idx.Search("common", tt.limit)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if len(results) > tt.max {
				t.Errorf("expected at most %d results, got %d", tt.max, len(results))
			}
			if tt.limit > 0 && len(results) > tt.limit {
				t.Errorf("expected at most %d results (limit), got %d", tt.limit, len(results))
			}
		})
	}
}
