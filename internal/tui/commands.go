package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cassiomarques/memoria/internal/tui/components"
)

// Command represents a parsed user command.
type Command struct {
	Name string
	Args []string
}

// commandNames is the list of all recognized command names.
var commandNames = []string{
	"new", "open", "search", "recent", "all", "tag", "untag", "ls", "cd", "mv", "rm", "tags", "sync", "remote", "fixfm", "help", "quit", "q",
}

// ParseCommand parses raw command-bar input into a Command.
// Returns an error for empty input or unknown commands.
func ParseCommand(input string) (*Command, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty command")
	}

	parts := strings.Fields(input)
	name := strings.ToLower(parts[0])

	known := false
	for _, cn := range commandNames {
		if cn == name {
			known = true
			break
		}
	}
	if !known {
		return nil, fmt.Errorf("unknown command: %s", name)
	}

	return &Command{
		Name: name,
		Args: parts[1:],
	}, nil
}

// Completions returns tab-completion suggestions for the current input.
func Completions(input string, noteItems []components.NoteItem, tagList []string) []string {
	if input == "" {
		return commandNames
	}

	// If no space yet, complete the command name
	if !strings.Contains(input, " ") {
		prefix := strings.ToLower(input)
		var matches []string
		for _, cn := range commandNames {
			if strings.HasPrefix(cn, prefix) {
				matches = append(matches, cn)
			}
		}
		return matches
	}

	// There's a space — the command name is typed, now complete arguments
	parts := strings.SplitN(input, " ", 2)
	cmdName := strings.ToLower(parts[0])
	argPart := ""
	if len(parts) > 1 {
		argPart = parts[1]
	}

	switch cmdName {
	case "open", "rm":
		return completeNotePaths(argPart, noteItems)
	case "tag", "untag":
		return completeTagCommand(argPart, noteItems, tagList)
	case "mv":
		return completeMvCommand(argPart, noteItems)
	case "cd":
		return completeFolders(argPart, noteItems)
	case "new":
		return completeFolders(argPart, noteItems)
	default:
		return nil
	}
}

// completeNotePaths suggests the next path segment matching the given prefix,
// similar to shell tab-completion. Folders get a trailing "/" suffix.
func completeNotePaths(prefix string, noteItems []components.NoteItem) []string {
	return completePathSegments(prefix, noteItems, true)
}

// completeFolders suggests the next folder segment matching the given prefix.
func completeFolders(prefix string, noteItems []components.NoteItem) []string {
	return completePathSegments(prefix, noteItems, false)
}

// completeTagCommand handles completion for "tag <path> <tag...>" and "untag <path> <tag...>".
// If the first arg (note path) isn't complete, suggest paths. Otherwise, suggest tags.
func completeTagCommand(argPart string, noteItems []components.NoteItem, tagList []string) []string {
	// Split on first space only — the note path may contain spaces
	spaceIdx := strings.Index(argPart, " ")

	// No space yet: still typing the note path
	if spaceIdx < 0 {
		return completeNotePaths(argPart, noteItems)
	}

	// Space found — check if the path before the space is a valid note
	candidatePath := argPart[:spaceIdx]
	isNote := false
	for _, item := range noteItems {
		if strings.EqualFold(item.Path, candidatePath) {
			isNote = true
			break
		}
	}

	if !isNote {
		// The space might be part of a folder name — keep completing paths
		return completeNotePaths(argPart, noteItems)
	}

	// Path is confirmed, complete tags from whatever comes after
	tagPart := strings.TrimLeft(argPart[spaceIdx:], " ")
	// Get the last word being typed
	lastSpace := strings.LastIndex(tagPart, " ")
	lastArg := tagPart
	if lastSpace >= 0 {
		lastArg = tagPart[lastSpace+1:]
	}
	return completeTags(lastArg, tagList)
}

// completeMvCommand handles completion for "mv <old> <new>".
func completeMvCommand(argPart string, noteItems []components.NoteItem) []string {
	// Split on first space — the source path may contain spaces
	spaceIdx := strings.Index(argPart, " ")

	// No space: still typing the source path (notes + folders)
	if spaceIdx < 0 {
		return completePathSegments(argPart, noteItems, true)
	}

	// Space found — check if the path before it is a valid note or folder
	candidatePath := argPart[:spaceIdx]
	isSource := false

	// Check if it's a note path
	for _, item := range noteItems {
		if strings.EqualFold(item.Path, candidatePath) {
			isSource = true
			break
		}
	}

	// Check if it's a folder path (with or without trailing slash)
	if !isSource {
		cleanCandidate := strings.TrimSuffix(candidatePath, "/")
		for _, item := range noteItems {
			folder := item.Folder
			for folder != "" && folder != "." {
				if strings.EqualFold(folder, cleanCandidate) {
					isSource = true
					break
				}
				folder = filepath.Dir(folder)
				if folder == "." {
					break
				}
			}
			if isSource {
				break
			}
		}
	}

	if !isSource {
		// The space might be part of a path — keep completing
		return completePathSegments(argPart, noteItems, true)
	}

	// Source confirmed, complete destination (folders)
	destPart := strings.TrimLeft(argPart[spaceIdx:], " ")
	return completeFolders(destPart, noteItems)
}

// completePathSegments is the core hierarchical path completer. Given a typed
// prefix it collects every note path (and, when includeFiles is true, file
// basenames) that start with the prefix, then returns only the *next* segment
// after the prefix directory. Folder segments get a trailing "/".
func completePathSegments(prefix string, noteItems []components.NoteItem, includeFiles bool) []string {
	lowerPrefix := strings.ToLower(prefix)

	// Collect all candidate full paths
	var paths []string
	for _, item := range noteItems {
		paths = append(paths, item.Path)
	}
	// Also add folder paths (with trailing "/") so we can complete folder names
	seen := make(map[string]bool)
	for _, item := range noteItems {
		if item.Folder != "" && !seen[item.Folder] {
			seen[item.Folder] = true
			paths = append(paths, item.Folder+"/")
		}
	}

	// Determine the directory prefix we're completing inside.
	// e.g. prefix "Work/Proj" → dir "Work/", partial "proj"
	dir := ""
	partial := lowerPrefix
	if idx := strings.LastIndex(prefix, "/"); idx >= 0 {
		dir = prefix[:idx+1]          // e.g. "Work/"
		partial = lowerPrefix[idx+1:] // e.g. "proj"
	}

	segmentSet := make(map[string]bool)
	var segments []string

	for _, p := range paths {
		lp := strings.ToLower(p)
		if !strings.HasPrefix(lp, strings.ToLower(dir)) {
			continue
		}

		// The remainder after the directory prefix
		rest := p[len(dir):]
		lowerRest := strings.ToLower(rest)
		if rest == "" {
			continue
		}

		// Check if the first segment of 'rest' matches the partial input
		if !strings.HasPrefix(lowerRest, partial) {
			continue
		}

		// Extract the next segment (up to the next "/")
		segment := rest
		if slashIdx := strings.Index(rest, "/"); slashIdx >= 0 {
			segment = rest[:slashIdx+1] // include the "/"
		}

		// Skip bare files when only completing folders
		isDir := strings.HasSuffix(segment, "/")
		if !includeFiles && !isDir {
			continue
		}

		if !segmentSet[segment] {
			segmentSet[segment] = true
			segments = append(segments, dir+segment)
		}
	}

	return segments
}

// completeTags suggests tags matching the given prefix.
func completeTags(prefix string, tagList []string) []string {
	prefix = strings.ToLower(prefix)
	var matches []string
	for _, tag := range tagList {
		if strings.HasPrefix(strings.ToLower(tag), prefix) {
			matches = append(matches, tag)
		}
	}
	return matches
}
