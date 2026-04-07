package note

import (
	"strings"
	"testing"
	"time"
)

func TestNewNote(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		content     string
		tags        []string
		wantTitle   string
		wantFolder  string
		wantTags    []string
		wantErr     bool
		errContains string
	}{
		{
			name:       "simple note",
			path:       "hello.md",
			content:    "Hello world",
			tags:       []string{"greeting"},
			wantTitle:  "hello",
			wantFolder: "",
			wantTags:   []string{"greeting"},
		},
		{
			name:       "note in subfolder",
			path:       "work/meeting.md",
			content:    "Meeting notes",
			tags:       []string{"work", "meeting"},
			wantTitle:  "meeting",
			wantFolder: "work",
			wantTags:   []string{"work", "meeting"},
		},
		{
			name:       "deeply nested path",
			path:       "projects/go/remember/design.md",
			content:    "Design doc",
			tags:       nil,
			wantTitle:  "design",
			wantFolder: "projects/go/remember",
			wantTags:   nil,
		},
		{
			name:       "tags are lowercased and trimmed",
			path:       "test.md",
			content:    "Test",
			tags:       []string{"  Go ", "RUST", " Python "},
			wantTitle:  "test",
			wantFolder: "",
			wantTags:   []string{"go", "rust", "python"},
		},
		{
			name:        "empty path",
			path:        "",
			content:     "content",
			tags:        nil,
			wantErr:     true,
			errContains: "path",
		},
		{
			name:        "path without .md extension",
			path:        "notes.txt",
			content:     "content",
			tags:        nil,
			wantErr:     true,
			errContains: ".md",
		},
		{
			name:       "nil tags become nil slice",
			path:       "empty-tags.md",
			content:    "no tags",
			tags:       nil,
			wantTitle:  "empty-tags",
			wantFolder: "",
			wantTags:   nil,
		},
		{
			name:       "empty tags slice",
			path:       "empty-tags.md",
			content:    "no tags",
			tags:       []string{},
			wantTitle:  "empty-tags",
			wantFolder: "",
			wantTags:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			n, err := NewNote(tt.path, tt.content, tt.tags)
			after := time.Now()

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if n.Path != tt.path {
				t.Errorf("Path = %q, want %q", n.Path, tt.path)
			}
			if n.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", n.Title, tt.wantTitle)
			}
			if n.Content != tt.content {
				t.Errorf("Content = %q, want %q", n.Content, tt.content)
			}
			if n.Folder != tt.wantFolder {
				t.Errorf("Folder = %q, want %q", n.Folder, tt.wantFolder)
			}

			// Check tags
			if tt.wantTags == nil {
				if n.Tags != nil {
					t.Errorf("Tags = %v, want nil", n.Tags)
				}
			} else {
				if len(n.Tags) != len(tt.wantTags) {
					t.Fatalf("Tags length = %d, want %d", len(n.Tags), len(tt.wantTags))
				}
				for i, tag := range n.Tags {
					if tag != tt.wantTags[i] {
						t.Errorf("Tags[%d] = %q, want %q", i, tag, tt.wantTags[i])
					}
				}
			}

			// Check timestamps
			if n.Created.Before(before) || n.Created.After(after) {
				t.Errorf("Created %v not between %v and %v", n.Created, before, after)
			}
			if n.Modified.Before(before) || n.Modified.After(after) {
				t.Errorf("Modified %v not between %v and %v", n.Modified, before, after)
			}
		})
	}
}

func TestAddTag(t *testing.T) {
	tests := []struct {
		name     string
		initial  []string
		add      string
		wantTags []string
	}{
		{
			name:     "add new tag",
			initial:  []string{"go"},
			add:      "rust",
			wantTags: []string{"go", "rust"},
		},
		{
			name:     "duplicate tag not added",
			initial:  []string{"go", "rust"},
			add:      "go",
			wantTags: []string{"go", "rust"},
		},
		{
			name:     "tag is lowercased and trimmed",
			initial:  []string{"go"},
			add:      "  RUST  ",
			wantTags: []string{"go", "rust"},
		},
		{
			name:     "duplicate after normalization",
			initial:  []string{"go"},
			add:      "  GO ",
			wantTags: []string{"go"},
		},
		{
			name:     "add to empty tags",
			initial:  nil,
			add:      "new",
			wantTags: []string{"new"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNote("test.md", "content", tt.initial)
			if err != nil {
				t.Fatalf("NewNote error: %v", err)
			}
			n.AddTag(tt.add)
			if len(n.Tags) != len(tt.wantTags) {
				t.Fatalf("Tags length = %d, want %d; tags = %v", len(n.Tags), len(tt.wantTags), n.Tags)
			}
			for i, tag := range n.Tags {
				if tag != tt.wantTags[i] {
					t.Errorf("Tags[%d] = %q, want %q", i, tag, tt.wantTags[i])
				}
			}
		})
	}
}

func TestRemoveTag(t *testing.T) {
	tests := []struct {
		name     string
		initial  []string
		remove   string
		wantOk   bool
		wantTags []string
	}{
		{
			name:     "remove existing tag",
			initial:  []string{"go", "rust"},
			remove:   "go",
			wantOk:   true,
			wantTags: []string{"rust"},
		},
		{
			name:     "remove nonexistent tag",
			initial:  []string{"go"},
			remove:   "rust",
			wantOk:   false,
			wantTags: []string{"go"},
		},
		{
			name:     "remove from empty tags",
			initial:  nil,
			remove:   "go",
			wantOk:   false,
			wantTags: nil,
		},
		{
			name:     "remove last tag",
			initial:  []string{"go"},
			remove:   "go",
			wantOk:   true,
			wantTags: []string{},
		},
		{
			name:     "remove with normalization",
			initial:  []string{"go", "rust"},
			remove:   "  GO  ",
			wantOk:   true,
			wantTags: []string{"rust"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNote("test.md", "content", tt.initial)
			if err != nil {
				t.Fatalf("NewNote error: %v", err)
			}
			ok := n.RemoveTag(tt.remove)
			if ok != tt.wantOk {
				t.Errorf("RemoveTag() = %v, want %v", ok, tt.wantOk)
			}
			if tt.wantTags == nil {
				if n.Tags != nil {
					t.Errorf("Tags = %v, want nil", n.Tags)
				}
			} else {
				if len(n.Tags) != len(tt.wantTags) {
					t.Fatalf("Tags length = %d, want %d; tags = %v", len(n.Tags), len(tt.wantTags), n.Tags)
				}
				for i, tag := range n.Tags {
					if tag != tt.wantTags[i] {
						t.Errorf("Tags[%d] = %q, want %q", i, tag, tt.wantTags[i])
					}
				}
			}
		})
	}
}

func TestHasTag(t *testing.T) {
	tests := []struct {
		name    string
		initial []string
		check   string
		want    bool
	}{
		{
			name:    "has existing tag",
			initial: []string{"go", "rust"},
			check:   "go",
			want:    true,
		},
		{
			name:    "does not have tag",
			initial: []string{"go"},
			check:   "rust",
			want:    false,
		},
		{
			name:    "check with normalization",
			initial: []string{"go"},
			check:   "  GO  ",
			want:    true,
		},
		{
			name:    "empty tags",
			initial: nil,
			check:   "go",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := NewNote("test.md", "content", tt.initial)
			if err != nil {
				t.Fatalf("NewNote error: %v", err)
			}
			if got := n.HasTag(tt.check); got != tt.want {
				t.Errorf("HasTag(%q) = %v, want %v", tt.check, got, tt.want)
			}
		})
	}
}

func TestFullContent(t *testing.T) {
	n, err := NewNote("test.md", "Hello world", []string{"go", "test"})
	if err != nil {
		t.Fatalf("NewNote error: %v", err)
	}

	full := n.FullContent()

	// Should start with frontmatter delimiters
	if !strings.HasPrefix(full, "---\n") {
		t.Error("FullContent should start with ---")
	}

	// Should contain the content
	if !strings.Contains(full, "Hello world") {
		t.Error("FullContent should contain the note content")
	}

	// Should contain tags
	if !strings.Contains(full, "go") || !strings.Contains(full, "test") {
		t.Error("FullContent should contain tags")
	}

	// Should contain timestamps
	if !strings.Contains(full, "created:") {
		t.Error("FullContent should contain created timestamp")
	}
	if !strings.Contains(full, "modified:") {
		t.Error("FullContent should contain modified timestamp")
	}

	// Content should come after the closing ---
	parts := strings.SplitN(full, "---\n", 3)
	if len(parts) < 3 {
		t.Fatalf("Expected at least 3 parts when splitting on ---, got %d", len(parts))
	}
	if strings.TrimSpace(parts[2]) != "Hello world" {
		t.Errorf("Content after frontmatter = %q, want %q", strings.TrimSpace(parts[2]), "Hello world")
	}
}

// --- Slugify tests ---

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Fix Auth Bug", "fix-auth-bug"},
		{"buy milk", "buy-milk"},
		{"hello", "hello"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
		{"UPPER_CASE_TITLE", "upper-case-title"},
		{"already-slugged", "already-slugged"},
		{"special!@#chars$%^here", "specialcharshere"},
		{"", ""},
		{"123 numbers", "123-numbers"},
		{"multiple---dashes___underscores", "multiple-dashes-underscores"},
	}
	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Todo field tests ---

func TestNote_IsOverdue(t *testing.T) {
	yesterday := time.Now().AddDate(0, 0, -1)
	tomorrow := time.Now().AddDate(0, 0, 1)

	tests := []struct {
		name string
		due  *time.Time
		done bool
		want bool
	}{
		{"nil due", nil, false, false},
		{"yesterday, not done", &yesterday, false, true},
		{"yesterday, done", &yesterday, true, false},
		{"tomorrow, not done", &tomorrow, false, false},
	}
	for _, tt := range tests {
		n := &Note{Todo: true, Done: tt.done, Due: tt.due}
		if got := n.IsOverdue(); got != tt.want {
			t.Errorf("IsOverdue() %s = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestNote_IsDueToday(t *testing.T) {
	today := time.Now()
	yesterday := time.Now().AddDate(0, 0, -1)
	tomorrow := time.Now().AddDate(0, 0, 1)

	tests := []struct {
		name string
		due  *time.Time
		done bool
		want bool
	}{
		{"nil due", nil, false, false},
		{"today, not done", &today, false, true},
		{"today, done", &today, true, false},
		{"yesterday", &yesterday, false, false},
		{"tomorrow", &tomorrow, false, false},
	}
	for _, tt := range tests {
		n := &Note{Todo: true, Done: tt.done, Due: tt.due}
		if got := n.IsDueToday(); got != tt.want {
			t.Errorf("IsDueToday() %s = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestFullContent_TodoFields(t *testing.T) {
	due := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	n := &Note{
		Path:     "TODO/buy-milk.md",
		Title:    "buy-milk",
		Content:  "Get 2% milk\n",
		Tags:     []string{"shopping"},
		Folder:   "TODO",
		Created:  time.Now(),
		Modified: time.Now(),
		Todo:     true,
		Done:     false,
		Due:      &due,
	}

	full := n.FullContent()

	if !strings.Contains(full, "todo: true") {
		t.Error("expected FullContent to contain 'todo: true'")
	}
	if !strings.Contains(full, "done: false") {
		t.Error("expected FullContent to contain 'done: false'")
	}
	if !strings.Contains(full, "due: \"2026-04-15\"") && !strings.Contains(full, "due: 2026-04-15") {
		t.Errorf("expected FullContent to contain due date, got:\n%s", full)
	}
}

func TestParseNote_TodoFields(t *testing.T) {
	raw := `---
tags:
    - work
todo: true
done: false
due: 2026-04-15
created: 2026-01-01T00:00:00Z
modified: 2026-01-01T00:00:00Z
---
Do the thing
`
	n, err := ParseNote("TODO/task.md", raw)
	if err != nil {
		t.Fatalf("ParseNote error: %v", err)
	}
	if !n.Todo {
		t.Error("expected Todo=true")
	}
	if n.Done {
		t.Error("expected Done=false")
	}
	if n.Due == nil {
		t.Fatal("expected Due to be non-nil")
	}
	if n.Due.Format(time.DateOnly) != "2026-04-15" {
		t.Errorf("expected due=2026-04-15, got %s", n.Due.Format(time.DateOnly))
	}
}
