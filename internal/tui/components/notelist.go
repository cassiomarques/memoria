package components

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cassiomarques/remember/internal/tui/theme"
)

// NoteItem represents a note entry for the list.
type NoteItem struct {
	Path     string
	Title    string
	Folder   string
	Tags     []string
	Modified time.Time
}

// NoteList is a scrollable, filterable list of notes with vim-style navigation.
type NoteList struct {
	items  []NoteItem
	cursor int
	offset int // first visible item index
	height int
	width  int
	styles theme.Styles

	// gg detection: true after first 'g' press
	pendingG bool
}

// NewNoteList creates a NoteList with default styles.
func NewNoteList() NoteList {
	return NoteList{
		styles: theme.DefaultStyles(),
	}
}

func (n *NoteList) SetItems(items []NoteItem) {
	n.items = items
	n.cursor = 0
	n.offset = 0
}

func (n *NoteList) SetSize(width, height int) {
	n.width = width
	n.height = height
}

// SelectedItem returns the currently selected item, or nil if the list is empty.
func (n NoteList) SelectedItem() *NoteItem {
	if len(n.items) == 0 {
		return nil
	}
	return &n.items[n.cursor]
}

// Cursor returns the current cursor position.
func (n NoteList) Cursor() int { return n.cursor }

// ItemCount returns the number of items.
func (n NoteList) ItemCount() int { return len(n.items) }

// ItemAt returns the item at the given index, or nil if out of range.
func (n NoteList) ItemAt(index int) *NoteItem {
	if index < 0 || index >= len(n.items) {
		return nil
	}
	return &n.items[index]
}

func (n NoteList) Init() tea.Cmd { return nil }

func (n NoteList) Update(msg tea.Msg) (NoteList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		key := msg.String()

		// Handle gg (go to top)
		if n.pendingG {
			n.pendingG = false
			if key == "g" {
				n.cursor = 0
				n.offset = 0
				return n, nil
			}
			// Not a second g; fall through to normal handling
		}

		switch key {
		case "j", "down":
			n.moveDown()
		case "k", "up":
			n.moveUp()
		case "g":
			n.pendingG = true
		case "G", "shift+g":
			n.moveToBottom()
		case "ctrl+d":
			n.pageDown()
		case "ctrl+u":
			n.pageUp()
		}
	}
	return n, nil
}

func (n *NoteList) moveDown() {
	if n.cursor < len(n.items)-1 {
		n.cursor++
		n.ensureVisible()
	}
}

func (n *NoteList) moveUp() {
	if n.cursor > 0 {
		n.cursor--
		n.ensureVisible()
	}
}

func (n *NoteList) moveToBottom() {
	if len(n.items) > 0 {
		n.cursor = len(n.items) - 1
		n.ensureVisible()
	}
}

func (n *NoteList) pageDown() {
	visible := n.visibleCount()
	n.cursor += visible / 2
	if n.cursor >= len(n.items) {
		n.cursor = len(n.items) - 1
	}
	if n.cursor < 0 {
		n.cursor = 0
	}
	n.ensureVisible()
}

func (n *NoteList) pageUp() {
	visible := n.visibleCount()
	n.cursor -= visible / 2
	if n.cursor < 0 {
		n.cursor = 0
	}
	n.ensureVisible()
}

// Each item takes 3 lines (title + metadata + blank separator)
const linesPerItem = 3

func (n *NoteList) visibleCount() int {
	if n.height <= 0 {
		return 1
	}
	vc := n.height / linesPerItem
	if vc < 1 {
		return 1
	}
	return vc
}

func (n *NoteList) ensureVisible() {
	visible := n.visibleCount()
	if n.cursor < n.offset {
		n.offset = n.cursor
	}
	if n.cursor >= n.offset+visible {
		n.offset = n.cursor - visible + 1
	}
}

func (n NoteList) View() string {
	if len(n.items) == 0 {
		empty := lipgloss.NewStyle().
			Foreground(theme.ColorOverlay0).
			Width(n.width).
			Align(lipgloss.Center).
			Padding(2, 0)
		return empty.Render("No notes yet. Press : to create one.")
	}

	visible := n.visibleCount()
	end := n.offset + visible
	if end > len(n.items) {
		end = len(n.items)
	}

	var rows []string
	for i := n.offset; i < end; i++ {
		rows = append(rows, n.renderItem(i))
	}

	content := strings.Join(rows, "\n")

	// Scroll indicators when the list extends beyond the visible area
	canScrollUp := n.offset > 0
	canScrollDown := end < len(n.items)
	if canScrollUp || canScrollDown {
		indicator := lipgloss.NewStyle().Foreground(theme.ColorOverlay1)
		var parts []string
		if canScrollUp {
			parts = append(parts, indicator.Render("▲"))
		}
		if canScrollDown {
			parts = append(parts, indicator.Render("▼"))
		}
		scrollHint := strings.Join(parts, " ")
		content += "\n" + lipgloss.NewStyle().
			Width(n.width).
			Align(lipgloss.Right).
			Render(scrollHint)
	}

	return content
}

func (n NoteList) renderItem(index int) string {
	item := n.items[index]
	selected := index == n.cursor

	// Selection indicator
	indicator := "  "
	if selected {
		indicator = lipgloss.NewStyle().Foreground(theme.ColorMauve).Bold(true).Render("▸ ")
	}

	// Humanize and truncate title
	maxTitleW := n.width - 6 // account for indicator and padding
	if maxTitleW < 10 {
		maxTitleW = 10
	}
	displayTitle := humanizeTitle(item.Title)
	if len(displayTitle) > maxTitleW {
		displayTitle = displayTitle[:maxTitleW-1] + "…"
	}

	titleStyle := n.styles.NoteItem
	if selected {
		titleStyle = n.styles.NoteItemSel
	}

	titleText := indicator + titleStyle.Render(displayTitle)
	titleLine := lipgloss.NewStyle().Width(n.width).Render(titleText)

	// Metadata line: folder, tags, date — with breathing room
	var metaParts []string
	if item.Folder != "" {
		metaParts = append(metaParts, n.styles.FolderPath.Render("📁 "+item.Folder))
	}
	for _, tag := range item.Tags {
		metaParts = append(metaParts, n.styles.Tag.Render("#"+tag))
	}
	metaParts = append(metaParts, lipgloss.NewStyle().
		Foreground(theme.ColorOverlay0).
		Render(formatTime(item.Modified)))

	meta := "    " + strings.Join(metaParts, "  ")

	// Blank line for breathing room between items
	return titleLine + "\n" + meta + "\n"
}

// humanizeTitle converts a raw filename-based title into a readable form.
// e.g. "running_azure_blob_storage" → "Running Azure Blob Storage"
func humanizeTitle(title string) string {
	// Replace underscores and hyphens with spaces
	s := strings.ReplaceAll(title, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")

	// Title-case each word
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func formatTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return t.Format("Jan 02")
	}
}
