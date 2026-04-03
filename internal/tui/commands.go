package tui

import (
	"fmt"
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
	"new", "open", "search", "tag", "untag", "ls", "cd", "mv", "rm", "tags", "sync", "remote", "fixfm", "help", "quit", "q",
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

// completeNotePaths suggests note paths matching the given prefix.
func completeNotePaths(prefix string, noteItems []components.NoteItem) []string {
	prefix = strings.ToLower(prefix)
	var matches []string
	for _, item := range noteItems {
		lower := strings.ToLower(item.Path)
		if strings.HasPrefix(lower, prefix) {
			matches = append(matches, item.Path)
		}
	}
	return matches
}

// completeTagCommand handles completion for "tag <path> <tag...>" and "untag <path> <tag...>".
// If the first arg (note path) isn't complete, suggest paths. Otherwise, suggest tags.
func completeTagCommand(argPart string, noteItems []components.NoteItem, tagList []string) []string {
	args := strings.Fields(argPart)

	// If no args yet or still typing first arg (no trailing space), complete note paths
	if len(args) == 0 || (len(args) == 1 && !strings.HasSuffix(argPart, " ")) {
		prefix := ""
		if len(args) == 1 {
			prefix = args[0]
		}
		return completeNotePaths(prefix, noteItems)
	}

	// First arg is set (note path), now complete tags
	lastArg := ""
	if strings.HasSuffix(argPart, " ") {
		lastArg = ""
	} else if len(args) > 1 {
		lastArg = args[len(args)-1]
	}

	return completeTags(lastArg, tagList)
}

// completeMvCommand handles completion for "mv <old> <new>".
func completeMvCommand(argPart string, noteItems []components.NoteItem) []string {
	args := strings.Fields(argPart)

	// Complete the first arg (source path)
	if len(args) == 0 || (len(args) == 1 && !strings.HasSuffix(argPart, " ")) {
		prefix := ""
		if len(args) == 1 {
			prefix = args[0]
		}
		return completeNotePaths(prefix, noteItems)
	}

	// For the second arg, suggest folder prefixes
	lastArg := ""
	if !strings.HasSuffix(argPart, " ") && len(args) > 1 {
		lastArg = args[len(args)-1]
	}
	return completeFolders(lastArg, noteItems)
}

// completeFolders suggests unique folder names from the note list.
func completeFolders(prefix string, noteItems []components.NoteItem) []string {
	prefix = strings.ToLower(prefix)
	seen := make(map[string]bool)
	var matches []string
	for _, item := range noteItems {
		folder := item.Folder
		if folder == "" {
			continue
		}
		if seen[folder] {
			continue
		}
		seen[folder] = true
		if strings.HasPrefix(strings.ToLower(folder), prefix) {
			matches = append(matches, folder)
		}
	}
	return matches
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
