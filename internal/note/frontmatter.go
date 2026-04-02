package note

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Frontmatter represents the YAML frontmatter of a note.
type Frontmatter struct {
	Tags     []string  `yaml:"tags,omitempty"`
	Created  time.Time `yaml:"created"`
	Modified time.Time `yaml:"modified"`
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
	}

	return n, nil
}
