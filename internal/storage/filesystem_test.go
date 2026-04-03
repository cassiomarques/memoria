package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cassiomarques/memoria/internal/note"
)

func mustNewNote(t *testing.T, path, content string, tags []string) *note.Note {
	t.Helper()
	n, err := note.NewNote(path, content, tags)
	if err != nil {
		t.Fatalf("NewNote(%q) error: %v", path, err)
	}
	return n
}

func mustNewStore(t *testing.T) *FileStore {
	t.Helper()
	root := t.TempDir()
	fs, err := NewFileStore(root)
	if err != nil {
		t.Fatalf("NewFileStore error: %v", err)
	}
	return fs
}

func TestFS_NewFileStore(t *testing.T) {
	t.Run("creates root directory", func(t *testing.T) {
		root := filepath.Join(t.TempDir(), "notes")
		fs, err := NewFileStore(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		info, err := os.Stat(fs.Root())
		if err != nil {
			t.Fatalf("root dir does not exist: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("root is not a directory")
		}
	})

	t.Run("works with existing directory", func(t *testing.T) {
		root := t.TempDir()
		fs, err := NewFileStore(root)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if fs.Root() != root {
			t.Errorf("Root() = %q, want %q", fs.Root(), root)
		}
	})
}

func TestFS_SaveAndLoad(t *testing.T) {
	t.Run("round-trip preserves content and frontmatter", func(t *testing.T) {
		fs := mustNewStore(t)
		n := mustNewNote(t, "hello.md", "Hello world\n", []string{"greeting", "test"})

		if err := fs.Save(n); err != nil {
			t.Fatalf("Save error: %v", err)
		}

		loaded, err := fs.Load("hello.md")
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}

		if loaded.Path != n.Path {
			t.Errorf("Path = %q, want %q", loaded.Path, n.Path)
		}
		if loaded.Title != n.Title {
			t.Errorf("Title = %q, want %q", loaded.Title, n.Title)
		}
		if loaded.Content != n.Content {
			t.Errorf("Content = %q, want %q", loaded.Content, n.Content)
		}
		if loaded.Folder != n.Folder {
			t.Errorf("Folder = %q, want %q", loaded.Folder, n.Folder)
		}
		if len(loaded.Tags) != len(n.Tags) {
			t.Fatalf("Tags length = %d, want %d", len(loaded.Tags), len(n.Tags))
		}
		for i, tag := range loaded.Tags {
			if tag != n.Tags[i] {
				t.Errorf("Tags[%d] = %q, want %q", i, tag, n.Tags[i])
			}
		}
		// Timestamps are serialized as RFC3339 (second precision), so compare truncated
		if !loaded.Created.Truncate(time.Second).Equal(n.Created.Truncate(time.Second)) {
			t.Errorf("Created = %v, want %v", loaded.Created, n.Created)
		}
		if !loaded.Modified.Truncate(time.Second).Equal(n.Modified.Truncate(time.Second)) {
			t.Errorf("Modified = %v, want %v", loaded.Modified, n.Modified)
		}
	})

	t.Run("save creates parent directories", func(t *testing.T) {
		fs := mustNewStore(t)
		n := mustNewNote(t, "deep/nested/dir/note.md", "Deep note", nil)

		if err := fs.Save(n); err != nil {
			t.Fatalf("Save error: %v", err)
		}

		if !fs.Exists("deep/nested/dir/note.md") {
			t.Error("note file does not exist after Save")
		}

		parentDir := filepath.Join(fs.Root(), "deep", "nested", "dir")
		info, err := os.Stat(parentDir)
		if err != nil {
			t.Fatalf("parent dir does not exist: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("parent path is not a directory")
		}
	})

	t.Run("save updates Modified timestamp", func(t *testing.T) {
		fs := mustNewStore(t)
		n := mustNewNote(t, "ts.md", "content", nil)
		originalModified := n.Modified

		if err := fs.Save(n); err != nil {
			t.Fatalf("Save error: %v", err)
		}

		if n.Modified.Before(originalModified) {
			t.Error("Modified was not updated")
		}
	})

	t.Run("load non-existent file returns error", func(t *testing.T) {
		fs := mustNewStore(t)
		_, err := fs.Load("does-not-exist.md")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestFS_Delete(t *testing.T) {
	t.Run("removes file", func(t *testing.T) {
		fs := mustNewStore(t)
		n := mustNewNote(t, "delete-me.md", "bye", nil)
		if err := fs.Save(n); err != nil {
			t.Fatalf("Save error: %v", err)
		}

		if err := fs.Delete("delete-me.md"); err != nil {
			t.Fatalf("Delete error: %v", err)
		}

		if fs.Exists("delete-me.md") {
			t.Error("note still exists after Delete")
		}
	})

	t.Run("cleans empty parent directories", func(t *testing.T) {
		fs := mustNewStore(t)
		n := mustNewNote(t, "a/b/c/note.md", "deep", nil)
		if err := fs.Save(n); err != nil {
			t.Fatalf("Save error: %v", err)
		}

		if err := fs.Delete("a/b/c/note.md"); err != nil {
			t.Fatalf("Delete error: %v", err)
		}

		// All empty parent dirs should be removed
		for _, dir := range []string{"a/b/c", "a/b", "a"} {
			absDir := filepath.Join(fs.Root(), dir)
			if _, err := os.Stat(absDir); !os.IsNotExist(err) {
				t.Errorf("directory %q should have been removed", dir)
			}
		}

		// Root should still exist
		if _, err := os.Stat(fs.Root()); err != nil {
			t.Errorf("root directory should still exist: %v", err)
		}
	})

	t.Run("does not remove non-empty parent dirs", func(t *testing.T) {
		fs := mustNewStore(t)
		n1 := mustNewNote(t, "folder/keep.md", "stay", nil)
		n2 := mustNewNote(t, "folder/remove.md", "go", nil)
		if err := fs.Save(n1); err != nil {
			t.Fatalf("Save error: %v", err)
		}
		if err := fs.Save(n2); err != nil {
			t.Fatalf("Save error: %v", err)
		}

		if err := fs.Delete("folder/remove.md"); err != nil {
			t.Fatalf("Delete error: %v", err)
		}

		folderDir := filepath.Join(fs.Root(), "folder")
		if _, err := os.Stat(folderDir); os.IsNotExist(err) {
			t.Error("folder should still exist because keep.md is there")
		}
	})

	t.Run("non-existent file returns os.ErrNotExist", func(t *testing.T) {
		fs := mustNewStore(t)
		err := fs.Delete("ghost.md")
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("expected os.ErrNotExist, got %v", err)
		}
	})
}

func TestFS_Move(t *testing.T) {
	t.Run("renames file correctly", func(t *testing.T) {
		fs := mustNewStore(t)
		n := mustNewNote(t, "old.md", "content", []string{"tag"})
		if err := fs.Save(n); err != nil {
			t.Fatalf("Save error: %v", err)
		}

		if err := fs.Move("old.md", "new.md"); err != nil {
			t.Fatalf("Move error: %v", err)
		}

		if fs.Exists("old.md") {
			t.Error("old file still exists")
		}
		if !fs.Exists("new.md") {
			t.Error("new file does not exist")
		}

		loaded, err := fs.Load("new.md")
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if loaded.Content != "content" {
			t.Errorf("Content = %q, want %q", loaded.Content, "content")
		}
	})

	t.Run("creates destination parent dirs", func(t *testing.T) {
		fs := mustNewStore(t)
		n := mustNewNote(t, "src.md", "content", nil)
		if err := fs.Save(n); err != nil {
			t.Fatalf("Save error: %v", err)
		}

		if err := fs.Move("src.md", "new/dir/dest.md"); err != nil {
			t.Fatalf("Move error: %v", err)
		}

		if !fs.Exists("new/dir/dest.md") {
			t.Error("destination file does not exist")
		}
	})

	t.Run("cleans empty source parent dirs", func(t *testing.T) {
		fs := mustNewStore(t)
		n := mustNewNote(t, "a/b/note.md", "content", nil)
		if err := fs.Save(n); err != nil {
			t.Fatalf("Save error: %v", err)
		}

		if err := fs.Move("a/b/note.md", "moved.md"); err != nil {
			t.Fatalf("Move error: %v", err)
		}

		for _, dir := range []string{"a/b", "a"} {
			absDir := filepath.Join(fs.Root(), dir)
			if _, err := os.Stat(absDir); !os.IsNotExist(err) {
				t.Errorf("directory %q should have been removed", dir)
			}
		}
	})

	t.Run("overwrite existing destination", func(t *testing.T) {
		fs := mustNewStore(t)
		n1 := mustNewNote(t, "src.md", "new content", nil)
		n2 := mustNewNote(t, "dest.md", "old content", nil)
		if err := fs.Save(n1); err != nil {
			t.Fatalf("Save error: %v", err)
		}
		if err := fs.Save(n2); err != nil {
			t.Fatalf("Save error: %v", err)
		}

		if err := fs.Move("src.md", "dest.md"); err != nil {
			t.Fatalf("Move error: %v", err)
		}

		loaded, err := fs.Load("dest.md")
		if err != nil {
			t.Fatalf("Load error: %v", err)
		}
		if loaded.Content != "new content" {
			t.Errorf("Content = %q, want %q (should have been overwritten)", loaded.Content, "new content")
		}
	})

	t.Run("move non-existent returns error", func(t *testing.T) {
		fs := mustNewStore(t)
		err := fs.Move("ghost.md", "dest.md")
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("expected os.ErrNotExist, got %v", err)
		}
	})
}

func TestFS_List(t *testing.T) {
	t.Run("lists notes in root", func(t *testing.T) {
		fs := mustNewStore(t)
		for _, name := range []string{"bravo.md", "alpha.md", "charlie.md"} {
			n := mustNewNote(t, name, "content of "+name, nil)
			if err := fs.Save(n); err != nil {
				t.Fatalf("Save error: %v", err)
			}
		}

		notes, err := fs.List("")
		if err != nil {
			t.Fatalf("List error: %v", err)
		}

		wantPaths := []string{"alpha.md", "bravo.md", "charlie.md"}
		if len(notes) != len(wantPaths) {
			t.Fatalf("got %d notes, want %d", len(notes), len(wantPaths))
		}
		for i, n := range notes {
			if n.Path != wantPaths[i] {
				t.Errorf("notes[%d].Path = %q, want %q", i, n.Path, wantPaths[i])
			}
		}
	})

	t.Run("lists notes in specific folder", func(t *testing.T) {
		fs := mustNewStore(t)
		for _, path := range []string{"work/b.md", "work/a.md", "personal/x.md"} {
			n := mustNewNote(t, path, "content", nil)
			if err := fs.Save(n); err != nil {
				t.Fatalf("Save error: %v", err)
			}
		}

		notes, err := fs.List("work")
		if err != nil {
			t.Fatalf("List error: %v", err)
		}

		wantPaths := []string{"work/a.md", "work/b.md"}
		if len(notes) != len(wantPaths) {
			t.Fatalf("got %d notes, want %d", len(notes), len(wantPaths))
		}
		for i, n := range notes {
			if n.Path != wantPaths[i] {
				t.Errorf("notes[%d].Path = %q, want %q", i, n.Path, wantPaths[i])
			}
		}
	})

	t.Run("non-recursive — does not include nested", func(t *testing.T) {
		fs := mustNewStore(t)
		for _, path := range []string{"top.md", "sub/nested.md"} {
			n := mustNewNote(t, path, "content", nil)
			if err := fs.Save(n); err != nil {
				t.Fatalf("Save error: %v", err)
			}
		}

		notes, err := fs.List("")
		if err != nil {
			t.Fatalf("List error: %v", err)
		}

		if len(notes) != 1 {
			t.Fatalf("got %d notes, want 1 (non-recursive)", len(notes))
		}
		if notes[0].Path != "top.md" {
			t.Errorf("Path = %q, want %q", notes[0].Path, "top.md")
		}
	})

	t.Run("empty folder returns nil", func(t *testing.T) {
		fs := mustNewStore(t)
		notes, err := fs.List("")
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if notes != nil {
			t.Errorf("expected nil, got %v", notes)
		}
	})

	t.Run("non-existent folder returns nil", func(t *testing.T) {
		fs := mustNewStore(t)
		notes, err := fs.List("nope")
		if err != nil {
			t.Fatalf("List error: %v", err)
		}
		if notes != nil {
			t.Errorf("expected nil, got %v", notes)
		}
	})
}

func TestFS_ListAll(t *testing.T) {
	t.Run("finds nested notes sorted by path", func(t *testing.T) {
		fs := mustNewStore(t)
		paths := []string{
			"z.md",
			"a/b.md",
			"a/a.md",
			"m/n/o.md",
		}
		for _, p := range paths {
			n := mustNewNote(t, p, "content of "+p, nil)
			if err := fs.Save(n); err != nil {
				t.Fatalf("Save error: %v", err)
			}
		}

		notes, err := fs.ListAll()
		if err != nil {
			t.Fatalf("ListAll error: %v", err)
		}

		wantPaths := []string{"a/a.md", "a/b.md", "m/n/o.md", "z.md"}
		if len(notes) != len(wantPaths) {
			t.Fatalf("got %d notes, want %d", len(notes), len(wantPaths))
		}
		for i, n := range notes {
			if n.Path != wantPaths[i] {
				t.Errorf("notes[%d].Path = %q, want %q", i, n.Path, wantPaths[i])
			}
		}
	})

	t.Run("empty store returns nil", func(t *testing.T) {
		fs := mustNewStore(t)
		notes, err := fs.ListAll()
		if err != nil {
			t.Fatalf("ListAll error: %v", err)
		}
		if notes != nil {
			t.Errorf("expected nil, got %v", notes)
		}
	})
}

func TestFS_Exists(t *testing.T) {
	tests := []struct {
		name   string
		create bool
		path   string
		want   bool
	}{
		{
			name:   "existing note",
			create: true,
			path:   "exists.md",
			want:   true,
		},
		{
			name:   "non-existing note",
			create: false,
			path:   "nope.md",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := mustNewStore(t)
			if tt.create {
				n := mustNewNote(t, tt.path, "content", nil)
				if err := fs.Save(n); err != nil {
					t.Fatalf("Save error: %v", err)
				}
			}
			if got := fs.Exists(tt.path); got != tt.want {
				t.Errorf("Exists(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFS_AbsPath(t *testing.T) {
	fs := mustNewStore(t)
	got := fs.AbsPath("work/meeting.md")
	want := filepath.Join(fs.Root(), "work", "meeting.md")
	if got != want {
		t.Errorf("AbsPath = %q, want %q", got, want)
	}
}
