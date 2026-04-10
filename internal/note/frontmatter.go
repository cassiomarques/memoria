package note

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Frontmatter represents the YAML frontmatter of a note.
type Frontmatter struct {
	Tags      []string  `yaml:"tags,omitempty"`
	Created   time.Time `yaml:"created"`
	Modified  time.Time `yaml:"modified"`
	Todo      bool      `yaml:"todo,omitempty"`
	Done      bool      `yaml:"done,omitempty"`
	Due       *DateOnly `yaml:"due,omitempty"`
	Completed *DateOnly `yaml:"completed,omitempty"`
	Archived  bool      `yaml:"archived,omitempty"`
}

// DateOnly is a date-only time value that serializes as YYYY-MM-DD in YAML.
type DateOnly time.Time

func (d DateOnly) Time() time.Time { return time.Time(d) }

func (d DateOnly) MarshalYAML() (any, error) {
	return time.Time(d).Format(time.DateOnly), nil
}

func (d *DateOnly) UnmarshalYAML(value *yaml.Node) error {
	t, err := time.Parse(time.DateOnly, value.Value)
	if err != nil {
		// Fall back to RFC3339
		t, err = time.Parse(time.RFC3339, value.Value)
		if err != nil {
			return fmt.Errorf("invalid date %q: expected YYYY-MM-DD", value.Value)
		}
	}
	*d = DateOnly(t)
	return nil
}

// ParseFrontmatter parses a raw string that may contain YAML frontmatter delimited
// by "---". Returns the parsed frontmatter (nil if none), the remaining content, and
// any error. If the raw string has no frontmatter, the full content is returned.
func ParseFrontmatter(raw string) (*Frontmatter, string, error) {
	if !strings.HasPrefix(raw, "---\n") {
		return nil, raw, nil
	}

	// Find the closing ---
	rest := raw[4:] // skip opening "---\n"

	// Handle empty frontmatter: "---\n---\n"
	if strings.HasPrefix(rest, "---\n") {
		return &Frontmatter{}, rest[4:], nil
	}
	if rest == "---" {
		return &Frontmatter{}, "", nil
	}

	closingIdx := strings.Index(rest, "\n---\n")
	if closingIdx == -1 {
		// Check if the frontmatter section ends with "\n---" at EOF (no trailing newline)
		if strings.HasSuffix(rest, "\n---") {
			closingIdx = len(rest) - 4
		} else {
			// No closing delimiter found — treat entire content as non-frontmatter
			return nil, raw, nil
		}
	}

	yamlContent := rest[:closingIdx]
	content := rest[closingIdx+5:] // skip "\n---\n"

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return nil, "", fmt.Errorf("invalid frontmatter YAML: %w", err)
	}

	return &fm, content, nil
}

// Serialize produces a "---\nyaml\n---\n" string from the frontmatter.
func (f *Frontmatter) Serialize() (string, error) {
	// Use a map to control output order and format
	data := make(map[string]any)

	if len(f.Tags) > 0 {
		data["tags"] = f.Tags
	}
	data["created"] = f.Created.Format(time.RFC3339)
	data["modified"] = f.Modified.Format(time.RFC3339)

	if f.Todo {
		data["todo"] = true
		data["done"] = f.Done
	}
	if f.Due != nil {
		data["due"] = f.Due.Time().Format(time.DateOnly)
	}
	if f.Completed != nil {
		data["completed"] = f.Completed.Time().Format(time.DateOnly)
	}
	if f.Archived {
		data["archived"] = true
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to serialize frontmatter: %w", err)
	}

	return "---\n" + string(out) + "---\n", nil
}

// HasFrontmatter returns true if the raw content starts with YAML frontmatter delimiters.
func HasFrontmatter(raw string) bool {
	return strings.HasPrefix(raw, "---\n")
}

// ParseNote is a convenience function that parses raw file content into a Note.
// It extracts frontmatter (if present) and builds a Note with the given path.
func ParseNote(path string, raw string) (*Note, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}

	fm, content, err := ParseFrontmatter(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing frontmatter for %q: %w", path, err)
	}

	now := time.Now()
	n := &Note{
		Path:     path,
		Title:    titleFromPath(path),
		Content:  content,
		Folder:   folderFromPath(path),
		Created:  now,
		Modified: now,
	}

	if fm != nil {
		n.Tags = fm.Tags
		if !fm.Created.IsZero() {
			n.Created = fm.Created
		}
		if !fm.Modified.IsZero() {
			n.Modified = fm.Modified
		}
		n.Todo = fm.Todo
		n.Done = fm.Done
		if fm.Due != nil {
			t := fm.Due.Time()
			n.Due = &t
		}
		if fm.Completed != nil {
			t := fm.Completed.Time()
			n.Completed = &t
		}
		n.Archived = fm.Archived
	}

	return n, nil
}
