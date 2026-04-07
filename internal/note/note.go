package note

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Note represents a markdown note with metadata.
type Note struct {
	Path     string   // relative path from notes root (e.g., "work/meeting.md")
	Title    string   // derived from filename without .md extension
	Content  string   // the markdown content (without frontmatter)
	Tags     []string // from frontmatter
	Folder   string   // directory portion of path (e.g., "work")
	Created  time.Time
	Modified time.Time
	Todo     bool       // true if this note is a todo item
	Done     bool       // true if this todo is completed
	Due      *time.Time // optional due date for todos
}

// NewNote creates a new note, auto-setting Title from path, Folder from path,
// and timestamps to now. Tags are normalized to lowercase and trimmed.
func NewNote(path string, content string, tags []string) (*Note, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}

	now := time.Now()
	n := &Note{
		Path:     path,
		Title:    titleFromPath(path),
		Content:  content,
		Tags:     normalizeTags(tags),
		Folder:   folderFromPath(path),
		Created:  now,
		Modified: now,
	}
	return n, nil
}

// AddTag adds a tag if not already present. The tag is normalized.
func (n *Note) AddTag(tag string) {
	tag = normalizeTag(tag)
	if n.HasTag(tag) {
		return
	}
	n.Tags = append(n.Tags, tag)
}

// RemoveTag removes a tag, returns false if not found. The tag is normalized before matching.
func (n *Note) RemoveTag(tag string) bool {
	tag = normalizeTag(tag)
	for i, t := range n.Tags {
		if t == tag {
			n.Tags = append(n.Tags[:i], n.Tags[i+1:]...)
			return true
		}
	}
	return false
}

// HasTag checks whether the note has the given tag. The tag is normalized before matching.
func (n *Note) HasTag(tag string) bool {
	tag = normalizeTag(tag)
	for _, t := range n.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// FullContent returns the frontmatter + content combined as a single string.
func (n *Note) FullContent() string {
	fm := Frontmatter{
		Tags:     n.Tags,
		Created:  n.Created,
		Modified: n.Modified,
		Todo:     n.Todo,
		Done:     n.Done,
	}
	if n.Due != nil {
		d := DateOnly(*n.Due)
		fm.Due = &d
	}
	serialized, err := fm.Serialize()
	if err != nil {
		return n.Content
	}
	return serialized + n.Content
}

func validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path must not be empty")
	}
	if !strings.HasSuffix(path, ".md") {
		return fmt.Errorf("path must end in .md, got %q", path)
	}
	return nil
}

func titleFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".md")
}

func folderFromPath(path string) string {
	dir := filepath.Dir(path)
	if dir == "." {
		return ""
	}
	return dir
}

func normalizeTag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}

func normalizeTags(tags []string) []string {
	if tags == nil {
		return nil
	}
	result := make([]string, len(tags))
	for i, tag := range tags {
		result[i] = normalizeTag(tag)
	}
	return result
}

// Slugify converts a human-readable title into a filename-safe slug.
// "Fix Auth Bug" → "fix-auth-bug"
func Slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))

	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == ' ' || r == '-' || r == '_':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}

	result := b.String()
	return strings.TrimRight(result, "-")
}

// IsOverdue returns true if the todo has a due date in the past (before today).
func (n *Note) IsOverdue() bool {
	if n.Due == nil || n.Done {
		return false
	}
	today := truncateToDate(time.Now())
	due := truncateToDate(*n.Due)
	return due.Before(today)
}

// IsDueToday returns true if the todo is due today.
func (n *Note) IsDueToday() bool {
	if n.Due == nil || n.Done {
		return false
	}
	today := truncateToDate(time.Now())
	due := truncateToDate(*n.Due)
	return due.Equal(today)
}

// truncateToDate strips the time component, keeping only the date in local time.
func truncateToDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}
