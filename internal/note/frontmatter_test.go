package note

import (
	"strings"
	"testing"
	"time"
)

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantFM      bool
		wantTags    []string
		wantContent string
		wantErr     bool
	}{
		{
			name: "valid frontmatter with tags",
			raw: `---
tags:
  - go
  - rust
created: 2024-01-15T10:30:00Z
modified: 2024-01-15T10:30:00Z
---
Hello world`,
			wantFM:      true,
			wantTags:    []string{"go", "rust"},
			wantContent: "Hello world",
		},
		{
			name:        "no frontmatter",
			raw:         "Just plain content\nwith multiple lines",
			wantFM:      false,
			wantContent: "Just plain content\nwith multiple lines",
		},
		{
			name: "empty frontmatter",
			raw: `---
---
Content after empty frontmatter`,
			wantFM:      true,
			wantTags:    nil,
			wantContent: "Content after empty frontmatter",
		},
		{
			name: "frontmatter with no tags",
			raw: `---
created: 2024-01-15T10:30:00Z
modified: 2024-01-15T10:30:00Z
---
Content here`,
			wantFM:      true,
			wantTags:    nil,
			wantContent: "Content here",
		},
		{
			name:        "empty string",
			raw:         "",
			wantFM:      false,
			wantContent: "",
		},
		{
			name: "frontmatter with empty tags list",
			raw: `---
tags: []
created: 2024-01-15T10:30:00Z
modified: 2024-01-15T10:30:00Z
---
Body`,
			wantFM:      true,
			wantTags:    []string{},
			wantContent: "Body",
		},
		{
			name: "malformed YAML in frontmatter",
			raw: `---
tags: [invalid yaml
  - broken
created: not-a-date
---
content`,
			wantErr: true,
		},
		{
			name:        "only opening delimiter, no closing",
			raw:         "---\ntags:\n  - go\nno closing delimiter",
			wantFM:      false,
			wantContent: "---\ntags:\n  - go\nno closing delimiter",
		},
		{
			name: "frontmatter with trailing newline in content",
			raw: `---
tags:
  - test
created: 2024-01-15T10:30:00Z
modified: 2024-01-15T10:30:00Z
---
Line 1
Line 2
`,
			wantFM:      true,
			wantTags:    []string{"test"},
			wantContent: "Line 1\nLine 2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, content, err := ParseFrontmatter(tt.raw)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantFM {
				if fm == nil {
					t.Fatal("expected frontmatter, got nil")
				}
				if tt.wantTags == nil {
					if fm.Tags != nil {
						t.Errorf("Tags = %v, want nil", fm.Tags)
					}
				} else {
					if len(fm.Tags) != len(tt.wantTags) {
						t.Fatalf("Tags length = %d, want %d; tags = %v", len(fm.Tags), len(tt.wantTags), fm.Tags)
					}
					for i, tag := range fm.Tags {
						if tag != tt.wantTags[i] {
							t.Errorf("Tags[%d] = %q, want %q", i, tag, tt.wantTags[i])
						}
					}
				}
			} else if fm != nil {
				t.Errorf("expected nil frontmatter, got %+v", fm)
			}

			if content != tt.wantContent {
				t.Errorf("content = %q, want %q", content, tt.wantContent)
			}
		})
	}
}

func TestSerializeFrontmatter(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		fm       Frontmatter
		wantTags bool
		wantErr  bool
	}{
		{
			name: "with tags",
			fm: Frontmatter{
				Tags:     []string{"go", "rust"},
				Created:  now,
				Modified: now,
			},
			wantTags: true,
		},
		{
			name: "without tags",
			fm: Frontmatter{
				Tags:     nil,
				Created:  now,
				Modified: now,
			},
			wantTags: false,
		},
		{
			name: "empty tags slice",
			fm: Frontmatter{
				Tags:     []string{},
				Created:  now,
				Modified: now,
			},
			wantTags: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.fm.Serialize()

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Should be wrapped in ---
			if !strings.HasPrefix(result, "---\n") {
				t.Error("should start with ---")
			}
			if !strings.HasSuffix(result, "---\n") {
				t.Errorf("should end with ---\\n, got suffix: %q", result[len(result)-10:])
			}

			// Check for tags presence
			if tt.wantTags {
				if !strings.Contains(result, "tags:") {
					t.Error("should contain tags")
				}
			}

			// Check timestamps are RFC3339
			if !strings.Contains(result, "created:") {
				t.Error("should contain created timestamp")
			}
			if !strings.Contains(result, "modified:") {
				t.Error("should contain modified timestamp")
			}
		})
	}
}

func TestSerializeParseRoundTrip(t *testing.T) {
	now := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	original := Frontmatter{
		Tags:     []string{"go", "testing", "tdd"},
		Created:  now,
		Modified: now,
	}

	serialized, err := original.Serialize()
	if err != nil {
		t.Fatalf("Serialize error: %v", err)
	}

	raw := serialized + "Some content here"
	fm, content, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter error: %v", err)
	}

	if fm == nil {
		t.Fatal("expected frontmatter, got nil")
	}

	if len(fm.Tags) != len(original.Tags) {
		t.Fatalf("Tags length = %d, want %d", len(fm.Tags), len(original.Tags))
	}
	for i, tag := range fm.Tags {
		if tag != original.Tags[i] {
			t.Errorf("Tags[%d] = %q, want %q", i, tag, original.Tags[i])
		}
	}

	if !fm.Created.Equal(original.Created) {
		t.Errorf("Created = %v, want %v", fm.Created, original.Created)
	}
	if !fm.Modified.Equal(original.Modified) {
		t.Errorf("Modified = %v, want %v", fm.Modified, original.Modified)
	}
	if content != "Some content here" {
		t.Errorf("content = %q, want %q", content, "Some content here")
	}
}

func TestParseNote(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		raw         string
		wantTitle   string
		wantFolder  string
		wantTags    []string
		wantContent string
		wantErr     bool
		errContains string
	}{
		{
			name: "full note with frontmatter",
			path: "work/meeting.md",
			raw: `---
tags:
  - work
  - meeting
created: 2024-01-15T10:30:00Z
modified: 2024-01-15T10:30:00Z
---
Meeting notes go here`,
			wantTitle:   "meeting",
			wantFolder:  "work",
			wantTags:    []string{"work", "meeting"},
			wantContent: "Meeting notes go here",
		},
		{
			name:        "note without frontmatter",
			path:        "quick.md",
			raw:         "Just a quick note",
			wantTitle:   "quick",
			wantFolder:  "",
			wantTags:    nil,
			wantContent: "Just a quick note",
		},
		{
			name:        "invalid path",
			path:        "invalid.txt",
			raw:         "content",
			wantErr:     true,
			errContains: ".md",
		},
		{
			name: "note preserves timestamps from frontmatter",
			path: "dated.md",
			raw: `---
tags:
  - old
created: 2020-06-01T08:00:00Z
modified: 2023-12-25T12:00:00Z
---
Old note`,
			wantTitle:   "dated",
			wantFolder:  "",
			wantTags:    []string{"old"},
			wantContent: "Old note",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, err := ParseNote(tt.path, tt.raw)

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

			if n.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", n.Title, tt.wantTitle)
			}
			if n.Folder != tt.wantFolder {
				t.Errorf("Folder = %q, want %q", n.Folder, tt.wantFolder)
			}
			if n.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", n.Content, tt.wantContent)
			}

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
		})
	}
}

func TestHasFrontmatter(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"with frontmatter", "---\ntags:\n  - go\n---\nContent", true},
		{"without frontmatter", "# Just a title\nSome content", false},
		{"empty string", "", false},
		{"dashes but no newline", "---content", false},
		{"empty frontmatter", "---\n---\nContent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasFrontmatter(tt.raw)
			if got != tt.want {
				t.Errorf("HasFrontmatter(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseNotePreservesTimestamps(t *testing.T) {
	raw := `---
tags:
  - old
created: 2020-06-01T08:00:00Z
modified: 2023-12-25T12:00:00Z
---
Old note`

	n, err := ParseNote("dated.md", raw)
	if err != nil {
		t.Fatalf("ParseNote error: %v", err)
	}

	wantCreated := time.Date(2020, 6, 1, 8, 0, 0, 0, time.UTC)
	wantModified := time.Date(2023, 12, 25, 12, 0, 0, 0, time.UTC)

	if !n.Created.Equal(wantCreated) {
		t.Errorf("Created = %v, want %v", n.Created, wantCreated)
	}
	if !n.Modified.Equal(wantModified) {
		t.Errorf("Modified = %v, want %v", n.Modified, wantModified)
	}
}
