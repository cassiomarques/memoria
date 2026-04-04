package tui

import (
	"testing"
	"time"

	"github.com/cassiomarques/memoria/internal/tui/components"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantCmd *Command
		wantErr bool
	}{
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "unknown command",
			input:   "foobar",
			wantErr: true,
		},
		{
			name:    "simple command no args",
			input:   "sync",
			wantCmd: &Command{Name: "sync", Args: []string{}},
		},
		{
			name:    "command with one arg",
			input:   "open work/my-note",
			wantCmd: &Command{Name: "open", Args: []string{"work/my-note"}},
		},
		{
			name:    "command with multiple args",
			input:   "new work/my-note tag1 tag2",
			wantCmd: &Command{Name: "new", Args: []string{"work/my-note", "tag1", "tag2"}},
		},
		{
			name:    "command with leading/trailing whitespace",
			input:   "  search  hello world  ",
			wantCmd: &Command{Name: "search", Args: []string{"hello", "world"}},
		},
		{
			name:    "case insensitive command",
			input:   "SYNC",
			wantCmd: &Command{Name: "sync", Args: []string{}},
		},
		{
			name:    "quit command",
			input:   "quit",
			wantCmd: &Command{Name: "quit", Args: []string{}},
		},
		{
			name:    "q shorthand",
			input:   "q",
			wantCmd: &Command{Name: "q", Args: []string{}},
		},
		{
			name:    "help command",
			input:   "help",
			wantCmd: &Command{Name: "help", Args: []string{}},
		},
		{
			name:    "tag with path and tags",
			input:   "tag notes/todo.md important urgent",
			wantCmd: &Command{Name: "tag", Args: []string{"notes/todo.md", "important", "urgent"}},
		},
		{
			name:    "mv with two paths",
			input:   "mv old/note new/note",
			wantCmd: &Command{Name: "mv", Args: []string{"old/note", "new/note"}},
		},
		{
			name:    "cd with folder",
			input:   "cd work",
			wantCmd: &Command{Name: "cd", Args: []string{"work"}},
		},
		{
			name:    "ls no args",
			input:   "ls",
			wantCmd: &Command{Name: "ls", Args: []string{}},
		},
		{
			name:    "ls with folder",
			input:   "ls work",
			wantCmd: &Command{Name: "ls", Args: []string{"work"}},
		},
		{
			name:    "rm with path",
			input:   "rm work/old-note",
			wantCmd: &Command{Name: "rm", Args: []string{"work/old-note"}},
		},
		{
			name:    "tags command",
			input:   "tags",
			wantCmd: &Command{Name: "tags", Args: []string{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := ParseCommand(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd.Name != tt.wantCmd.Name {
				t.Errorf("name: got %q, want %q", cmd.Name, tt.wantCmd.Name)
			}
			if len(cmd.Args) != len(tt.wantCmd.Args) {
				t.Fatalf("args len: got %d, want %d", len(cmd.Args), len(tt.wantCmd.Args))
			}
			for i, arg := range cmd.Args {
				if arg != tt.wantCmd.Args[i] {
					t.Errorf("arg[%d]: got %q, want %q", i, arg, tt.wantCmd.Args[i])
				}
			}
		})
	}
}

func sampleNoteItems() []components.NoteItem {
	return []components.NoteItem{
		{Path: "work/meeting.md", Title: "meeting", Folder: "work", Tags: []string{"important"}, Modified: time.Now()},
		{Path: "work/todo.md", Title: "todo", Folder: "work", Tags: []string{"daily"}, Modified: time.Now()},
		{Path: "work/projects/api.md", Title: "api", Folder: "work/projects", Tags: []string{"dev"}, Modified: time.Now()},
		{Path: "work/projects/secret plan/roadmap.md", Title: "roadmap", Folder: "work/projects/secret plan", Tags: nil, Modified: time.Now()},
		{Path: "personal/journal.md", Title: "journal", Folder: "personal", Tags: []string{"daily"}, Modified: time.Now()},
		{Path: "ideas.md", Title: "ideas", Folder: "", Tags: []string{"creative"}, Modified: time.Now()},
	}
}

func sampleTags() []string {
	return []string{"important", "daily", "creative", "urgent"}
}

func TestCompletions_CommandNames(t *testing.T) {
	items := sampleNoteItems()
	tags := sampleTags()

	t.Run("empty input returns all commands", func(t *testing.T) {
		results := Completions("", items, tags)
		if len(results) != len(commandNames) {
			t.Errorf("expected %d commands, got %d", len(commandNames), len(results))
		}
	})

	t.Run("partial command name", func(t *testing.T) {
		results := Completions("se", items, tags)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d: %v", len(results), results)
		}
		if results[0] != "search" {
			t.Errorf("expected 'search', got %q", results[0])
		}
	})

	t.Run("partial command 's' matches search and sync", func(t *testing.T) {
		results := Completions("s", items, tags)
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d: %v", len(results), results)
		}
	})

	t.Run("no match for unknown prefix", func(t *testing.T) {
		results := Completions("xyz", items, tags)
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestCompletions_NotePaths(t *testing.T) {
	items := sampleNoteItems()
	tags := sampleTags()

	t.Run("open with empty prefix shows top-level segments", func(t *testing.T) {
		results := Completions("open ", items, tags)
		// Should show: work/, personal/, ideas.md (hierarchical, not all paths)
		if len(results) != 3 {
			t.Errorf("expected 3 top-level segments, got %d: %v", len(results), results)
		}
	})

	t.Run("open with folder prefix shows children", func(t *testing.T) {
		results := Completions("open work/", items, tags)
		// Should show: work/meeting.md, work/todo.md, work/projects/
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d: %v", len(results), results)
		}
	})

	t.Run("open with nested folder shows deeper children", func(t *testing.T) {
		results := Completions("open work/projects/", items, tags)
		// Should show: work/projects/api.md, work/projects/secret plan/
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d: %v", len(results), results)
		}
	})

	t.Run("open with folder with spaces shows children", func(t *testing.T) {
		results := Completions("open work/projects/secret plan/", items, tags)
		// Should show: work/projects/secret plan/roadmap.md
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d: %v", len(results), results)
		}
		if results[0] != "work/projects/secret plan/roadmap.md" {
			t.Errorf("expected 'work/projects/secret plan/roadmap.md', got %q", results[0])
		}
	})

	t.Run("rm with prefix", func(t *testing.T) {
		results := Completions("rm id", items, tags)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0] != "ideas.md" {
			t.Errorf("expected 'ideas.md', got %q", results[0])
		}
	})
}

func TestCompletions_Folders(t *testing.T) {
	items := sampleNoteItems()
	tags := sampleTags()

	t.Run("cd with empty prefix shows top-level folders", func(t *testing.T) {
		results := Completions("cd ", items, tags)
		// work/, personal/
		if len(results) != 2 {
			t.Fatalf("expected 2 folders, got %d: %v", len(results), results)
		}
	})

	t.Run("cd with prefix filters folders with trailing slash", func(t *testing.T) {
		results := Completions("cd w", items, tags)
		if len(results) != 1 {
			t.Fatalf("expected 1 folder, got %d: %v", len(results), results)
		}
		if results[0] != "work/" {
			t.Errorf("expected 'work/', got %q", results[0])
		}
	})

	t.Run("cd into folder shows subfolders", func(t *testing.T) {
		results := Completions("cd work/", items, tags)
		// work/projects/
		if len(results) != 1 {
			t.Fatalf("expected 1 subfolder, got %d: %v", len(results), results)
		}
		if results[0] != "work/projects/" {
			t.Errorf("expected 'work/projects/', got %q", results[0])
		}
	})

	t.Run("new with prefix suggests folders with trailing slash", func(t *testing.T) {
		results := Completions("new p", items, tags)
		if len(results) != 1 {
			t.Fatalf("expected 1 folder, got %d: %v", len(results), results)
		}
		if results[0] != "personal/" {
			t.Errorf("expected 'personal/', got %q", results[0])
		}
	})
}

func TestCompletions_Tags(t *testing.T) {
	items := sampleNoteItems()
	tags := sampleTags()

	t.Run("tag with note path then space suggests tags", func(t *testing.T) {
		results := Completions("tag work/meeting.md ", items, tags)
		if len(results) != len(tags) {
			t.Errorf("expected %d tags, got %d: %v", len(tags), len(results), results)
		}
	})

	t.Run("tag with note path and partial tag", func(t *testing.T) {
		results := Completions("tag work/meeting.md ur", items, tags)
		if len(results) != 1 {
			t.Fatalf("expected 1 tag, got %d: %v", len(results), results)
		}
		if results[0] != "urgent" {
			t.Errorf("expected 'urgent', got %q", results[0])
		}
	})

	t.Run("tag with no path yet suggests top-level segments", func(t *testing.T) {
		results := Completions("tag ", items, tags)
		// Hierarchical: work/, personal/, ideas.md
		if len(results) != 3 {
			t.Errorf("expected 3 top-level segments, got %d: %v", len(results), results)
		}
	})

	t.Run("untag with partial path", func(t *testing.T) {
		results := Completions("untag work/m", items, tags)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d: %v", len(results), results)
		}
	})
}

func TestCompletions_Search(t *testing.T) {
	items := sampleNoteItems()
	tags := sampleTags()

	t.Run("search returns no suggestions", func(t *testing.T) {
		results := Completions("search ", items, tags)
		if results != nil {
			t.Errorf("expected nil, got %v", results)
		}
	})
}

func TestCompletions_Mv(t *testing.T) {
	items := sampleNoteItems()
	tags := sampleTags()

	t.Run("mv with no args suggests top-level segments", func(t *testing.T) {
		results := Completions("mv ", items, tags)
		// Hierarchical: work/, personal/, ideas.md
		if len(results) != 3 {
			t.Errorf("expected 3 top-level segments, got %d: %v", len(results), results)
		}
	})

	t.Run("mv with one path and space suggests folders", func(t *testing.T) {
		results := Completions("mv work/meeting.md ", items, tags)
		// Folders only: work/, personal/
		if len(results) != 2 {
			t.Fatalf("expected 2 folders, got %d: %v", len(results), results)
		}
	})

	t.Run("mv destination drills into folder", func(t *testing.T) {
		results := Completions("mv work/meeting.md work/", items, tags)
		// Subfolders of work: work/projects/
		if len(results) != 1 {
			t.Fatalf("expected 1 subfolder, got %d: %v", len(results), results)
		}
		if results[0] != "work/projects/" {
			t.Errorf("expected 'work/projects/', got %q", results[0])
		}
	})
}
