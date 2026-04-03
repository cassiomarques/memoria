package storage

import (
	"errors"
	"testing"
	"time"

	"github.com/cassiomarques/memoria/internal/note"
)

func newTestStore(t *testing.T) *MetaStore {
	t.Helper()
	store, err := NewMemoryMetaStore()
	if err != nil {
		t.Fatalf("NewMemoryMetaStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func makeNote(t *testing.T, path, content string, tags []string) *note.Note {
	t.Helper()
	n, err := note.NewNote(path, content, tags)
	if err != nil {
		t.Fatalf("NewNote(%q): %v", path, err)
	}
	return n
}

func TestUpsertNote_InsertThenUpdate(t *testing.T) {
	store := newTestStore(t)

	n := makeNote(t, "hello.md", "body", nil)
	originalModified := n.Modified

	if err := store.UpsertNote(n); err != nil {
		t.Fatalf("UpsertNote (insert): %v", err)
	}

	got, err := store.GetNote("hello.md")
	if err != nil {
		t.Fatalf("GetNote after insert: %v", err)
	}
	if got.Title != "hello" {
		t.Errorf("title = %q, want %q", got.Title, "hello")
	}

	// Update: change modified time
	time.Sleep(10 * time.Millisecond)
	n.Modified = time.Now()
	n.Title = "updated"
	if err := store.UpsertNote(n); err != nil {
		t.Fatalf("UpsertNote (update): %v", err)
	}

	got, err = store.GetNote("hello.md")
	if err != nil {
		t.Fatalf("GetNote after update: %v", err)
	}
	if got.Title != "updated" {
		t.Errorf("title after update = %q, want %q", got.Title, "updated")
	}
	if !got.Modified.After(originalModified) {
		t.Errorf("modified should be later than original; got %v, original %v", got.Modified, originalModified)
	}
}

func TestUpsertNote_Tags(t *testing.T) {
	store := newTestStore(t)

	n := makeNote(t, "tagged.md", "body", []string{"go", "sqlite"})
	if err := store.UpsertNote(n); err != nil {
		t.Fatalf("UpsertNote: %v", err)
	}

	tags, err := store.GetTags("tagged.md")
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	wantTags := []string{"go", "sqlite"}
	if len(tags) != len(wantTags) {
		t.Fatalf("tags = %v, want %v", tags, wantTags)
	}
	for i, tag := range tags {
		if tag != wantTags[i] {
			t.Errorf("tag[%d] = %q, want %q", i, tag, wantTags[i])
		}
	}

	// Update with different tags
	n.Tags = []string{"rust", "wasm"}
	if err := store.UpsertNote(n); err != nil {
		t.Fatalf("UpsertNote (update tags): %v", err)
	}

	tags, err = store.GetTags("tagged.md")
	if err != nil {
		t.Fatalf("GetTags after update: %v", err)
	}
	wantTags = []string{"rust", "wasm"}
	if len(tags) != len(wantTags) {
		t.Fatalf("tags after update = %v, want %v", tags, wantTags)
	}
	for i, tag := range tags {
		if tag != wantTags[i] {
			t.Errorf("tag[%d] = %q, want %q", i, tag, wantTags[i])
		}
	}
}

func TestDeleteNote(t *testing.T) {
	store := newTestStore(t)

	n := makeNote(t, "delete-me.md", "body", []string{"temp"})
	if err := store.UpsertNote(n); err != nil {
		t.Fatalf("UpsertNote: %v", err)
	}

	if err := store.DeleteNote("delete-me.md"); err != nil {
		t.Fatalf("DeleteNote: %v", err)
	}

	_, err := store.GetNote("delete-me.md")
	if !errors.Is(err, ErrNoteNotFound) {
		t.Errorf("GetNote after delete: got err=%v, want ErrNoteNotFound", err)
	}

	// Tags should also be gone
	tags, err := store.GetTags("delete-me.md")
	if err != nil {
		t.Fatalf("GetTags after delete: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("tags after delete = %v, want empty", tags)
	}
}

func TestDeleteNote_NonExistent(t *testing.T) {
	store := newTestStore(t)

	if err := store.DeleteNote("no-such-note.md"); err != nil {
		t.Errorf("DeleteNote non-existent should not error, got: %v", err)
	}
}

func TestMoveNote(t *testing.T) {
	store := newTestStore(t)

	n := makeNote(t, "old.md", "body", []string{"move"})
	if err := store.UpsertNote(n); err != nil {
		t.Fatalf("UpsertNote: %v", err)
	}

	if err := store.MoveNote("old.md", "archive/old.md", "archive"); err != nil {
		t.Fatalf("MoveNote: %v", err)
	}

	_, err := store.GetNote("old.md")
	if !errors.Is(err, ErrNoteNotFound) {
		t.Errorf("GetNote old path: got err=%v, want ErrNoteNotFound", err)
	}

	got, err := store.GetNote("archive/old.md")
	if err != nil {
		t.Fatalf("GetNote new path: %v", err)
	}
	if got.Folder != "archive" {
		t.Errorf("folder = %q, want %q", got.Folder, "archive")
	}

	// Tags should follow
	tags, err := store.GetTags("archive/old.md")
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	if len(tags) != 1 || tags[0] != "move" {
		t.Errorf("tags = %v, want [move]", tags)
	}
}

func TestMoveNote_NonExistent(t *testing.T) {
	store := newTestStore(t)

	err := store.MoveNote("no-such.md", "dest.md", "")
	if !errors.Is(err, ErrNoteNotFound) {
		t.Errorf("MoveNote non-existent: got err=%v, want ErrNoteNotFound", err)
	}
}

func TestGetNote_Existing(t *testing.T) {
	store := newTestStore(t)

	n := makeNote(t, "work/meeting.md", "notes", []string{"work", "meeting"})
	if err := store.UpsertNote(n); err != nil {
		t.Fatalf("UpsertNote: %v", err)
	}

	got, err := store.GetNote("work/meeting.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if got.Path != "work/meeting.md" {
		t.Errorf("path = %q, want %q", got.Path, "work/meeting.md")
	}
	if got.Folder != "work" {
		t.Errorf("folder = %q, want %q", got.Folder, "work")
	}
	if len(got.Tags) != 2 {
		t.Errorf("tags = %v, want [meeting work]", got.Tags)
	}
}

func TestGetNote_NonExisting(t *testing.T) {
	store := newTestStore(t)

	_, err := store.GetNote("nope.md")
	if !errors.Is(err, ErrNoteNotFound) {
		t.Errorf("GetNote non-existing: got err=%v, want ErrNoteNotFound", err)
	}
}

func TestListByFolder(t *testing.T) {
	store := newTestStore(t)

	for _, path := range []string{"work/b.md", "work/a.md", "personal/c.md"} {
		n := makeNote(t, path, "body", nil)
		if err := store.UpsertNote(n); err != nil {
			t.Fatalf("UpsertNote(%q): %v", path, err)
		}
	}

	notes, err := store.ListByFolder("work")
	if err != nil {
		t.Fatalf("ListByFolder: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("got %d notes, want 2", len(notes))
	}
	// sorted by title: a, b
	if notes[0].Title != "a" || notes[1].Title != "b" {
		t.Errorf("titles = [%q, %q], want [a, b]", notes[0].Title, notes[1].Title)
	}
}

func TestListByTag(t *testing.T) {
	store := newTestStore(t)

	n1 := makeNote(t, "b.md", "", []string{"go"})
	n2 := makeNote(t, "a.md", "", []string{"go", "sql"})
	n3 := makeNote(t, "c.md", "", []string{"sql"})

	for _, n := range []*note.Note{n1, n2, n3} {
		if err := store.UpsertNote(n); err != nil {
			t.Fatalf("UpsertNote(%q): %v", n.Path, err)
		}
	}

	notes, err := store.ListByTag("go")
	if err != nil {
		t.Fatalf("ListByTag: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("got %d notes, want 2", len(notes))
	}
	// sorted by title: a, b
	if notes[0].Title != "a" || notes[1].Title != "b" {
		t.Errorf("titles = [%q, %q], want [a, b]", notes[0].Title, notes[1].Title)
	}
}

func TestListAllTags(t *testing.T) {
	store := newTestStore(t)

	n1 := makeNote(t, "a.md", "", []string{"go", "sql"})
	n2 := makeNote(t, "b.md", "", []string{"go"})
	for _, n := range []*note.Note{n1, n2} {
		if err := store.UpsertNote(n); err != nil {
			t.Fatalf("UpsertNote: %v", err)
		}
	}

	tags, err := store.ListAllTags()
	if err != nil {
		t.Fatalf("ListAllTags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("got %d tags, want 2", len(tags))
	}
	// sorted by name: go, sql
	if tags[0].Tag != "go" || tags[0].Count != 2 {
		t.Errorf("tags[0] = %+v, want {go 2}", tags[0])
	}
	if tags[1].Tag != "sql" || tags[1].Count != 1 {
		t.Errorf("tags[1] = %+v, want {sql 1}", tags[1])
	}
}

func TestListAll(t *testing.T) {
	store := newTestStore(t)

	for _, path := range []string{"c.md", "a.md", "b.md"} {
		n := makeNote(t, path, "", nil)
		if err := store.UpsertNote(n); err != nil {
			t.Fatalf("UpsertNote(%q): %v", path, err)
		}
	}

	notes, err := store.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(notes) != 3 {
		t.Fatalf("got %d notes, want 3", len(notes))
	}
	// sorted by path: a, b, c
	for i, want := range []string{"a.md", "b.md", "c.md"} {
		if notes[i].Path != want {
			t.Errorf("notes[%d].Path = %q, want %q", i, notes[i].Path, want)
		}
	}
}

func TestGetTags(t *testing.T) {
	store := newTestStore(t)

	n := makeNote(t, "tags.md", "", []string{"zulu", "alpha", "mike"})
	if err := store.UpsertNote(n); err != nil {
		t.Fatalf("UpsertNote: %v", err)
	}

	tags, err := store.GetTags("tags.md")
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	// sorted alphabetically
	want := []string{"alpha", "mike", "zulu"}
	if len(tags) != len(want) {
		t.Fatalf("tags = %v, want %v", tags, want)
	}
	for i, tag := range tags {
		if tag != want[i] {
			t.Errorf("tag[%d] = %q, want %q", i, tag, want[i])
		}
	}
}

func TestGetTags_NoNote(t *testing.T) {
	store := newTestStore(t)

	tags, err := store.GetTags("nope.md")
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("tags = %v, want empty", tags)
	}
}
