package components

import (
	"fmt"
	"sort"
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

// treeNode represents a node in the folder tree (either a folder or a note).
type treeNode struct {
	name        string
	isFolder    bool
	expanded    bool
	depth       int
	noteItem    *NoteItem
	children    []*treeNode
	isLastChild bool
}

func (t *treeNode) countNotes() int {
	if !t.isFolder {
		return 1
	}
	count := 0
	for _, c := range t.children {
		count += c.countNotes()
	}
	return count
}

// NoteList is a scrollable tree view of notes organized by folder.
type NoteList struct {
	items       []NoteItem
	tree        []*treeNode
	flatVisible []*treeNode
	cursor      int
	offset      int
	height      int
	width       int
	styles      theme.Styles

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
	// Remember current selection to restore after rebuild
	var selectedPath string
	if sel := n.SelectedItem(); sel != nil {
		selectedPath = sel.Path
	}

	n.items = items
	n.tree = buildTree(n.items)
	n.flatVisible = nil
	n.rebuildFlatVisible()
	n.cursor = 0
	n.offset = 0

	// Restore cursor to previously selected note
	if selectedPath != "" {
		for i, node := range n.flatVisible {
			if !node.isFolder && node.noteItem != nil && node.noteItem.Path == selectedPath {
				n.cursor = i
				n.ensureVisible()
				break
			}
		}
	}
}

func (n *NoteList) SetSize(width, height int) {
	n.width = width
	n.height = height
}

// SelectedItem returns the currently selected note, or nil if the list is
// empty or a folder is selected.
func (n NoteList) SelectedItem() *NoteItem {
	if len(n.flatVisible) == 0 || n.cursor >= len(n.flatVisible) {
		return nil
	}
	node := n.flatVisible[n.cursor]
	if node.isFolder {
		return nil
	}
	return node.noteItem
}

// Cursor returns the current cursor position in the visible tree.
func (n NoteList) Cursor() int { return n.cursor }

// ItemCount returns the total number of notes (not including folder nodes).
func (n NoteList) ItemCount() int { return len(n.items) }

// ItemAt returns the note at the given index in the flat note list, or nil if out of range.
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
		case "enter":
			n.toggleFolder()
		}
	}
	return n, nil
}

func (n *NoteList) toggleFolder() {
	if len(n.flatVisible) == 0 || n.cursor >= len(n.flatVisible) {
		return
	}
	node := n.flatVisible[n.cursor]
	if !node.isFolder {
		return
	}
	node.expanded = !node.expanded
	n.rebuildFlatVisible()
	if n.cursor >= len(n.flatVisible) {
		n.cursor = len(n.flatVisible) - 1
	}
	if n.cursor < 0 {
		n.cursor = 0
	}
	n.ensureVisible()
}

func (n *NoteList) moveDown() {
	if n.cursor < len(n.flatVisible)-1 {
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
	if len(n.flatVisible) > 0 {
		n.cursor = len(n.flatVisible) - 1
		n.ensureVisible()
	}
}

func (n *NoteList) pageDown() {
	visible := n.visibleCount()
	n.cursor += visible / 2
	if n.cursor >= len(n.flatVisible) {
		n.cursor = len(n.flatVisible) - 1
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

const linesPerItem = 1

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

func (n *NoteList) rebuildFlatVisible() {
	n.flatVisible = n.flatVisible[:0]
	for _, node := range n.tree {
		n.collectVisible(node)
	}
}

func (n *NoteList) collectVisible(node *treeNode) {
	n.flatVisible = append(n.flatVisible, node)
	if node.isFolder && node.expanded {
		for _, child := range node.children {
			n.collectVisible(child)
		}
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
	if end > len(n.flatVisible) {
		end = len(n.flatVisible)
	}

	var rows []string
	for i := n.offset; i < end; i++ {
		rows = append(rows, n.renderNode(i))
	}

	content := strings.Join(rows, "\n")

	// Scroll indicators when the list extends beyond the visible area
	canScrollUp := n.offset > 0
	canScrollDown := end < len(n.flatVisible)
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

func (n NoteList) renderNode(visibleIndex int) string {
	node := n.flatVisible[visibleIndex]
	selected := visibleIndex == n.cursor

	indicator := "  "
	if selected {
		indicator = lipgloss.NewStyle().Foreground(theme.ColorMauve).Bold(true).Render("▸ ")
	}

	indent := strings.Repeat("  ", node.depth)

	if node.isFolder {
		count := node.countNotes()
		name := fmt.Sprintf("📁 %s (%d)", node.name, count)

		var styledName string
		if selected {
			styledName = n.styles.NoteItemSel.Render(name)
		} else {
			styledName = n.styles.FolderPath.Render(name)
		}

		line := indicator + indent + styledName
		return lipgloss.NewStyle().Width(n.width).Render(line)
	}

	// Note rendering
	displayTitle := node.name
	overhead := 4 + 2*node.depth
	if node.depth > 0 {
		overhead += 4 // tree connector
	}
	maxTitleW := n.width - overhead
	if maxTitleW < 10 {
		maxTitleW = 10
	}
	if len(displayTitle) > maxTitleW {
		displayTitle = displayTitle[:maxTitleW-1] + "…"
	}

	titleStyle := n.styles.NoteItem
	if selected {
		titleStyle = n.styles.NoteItemSel
	}

	if node.depth > 0 {
		connector := "├── "
		if node.isLastChild {
			connector = "└── "
		}
		connStyle := lipgloss.NewStyle().Foreground(theme.ColorOverlay0)
		line := indicator + indent + connStyle.Render(connector) + titleStyle.Render(displayTitle)
		return lipgloss.NewStyle().Width(n.width).Render(line)
	}

	// Root-level note
	line := indicator + titleStyle.Render(displayTitle)
	return lipgloss.NewStyle().Width(n.width).Render(line)
}

// buildTree constructs a hierarchical tree from a flat list of NoteItems.
func buildTree(items []NoteItem) []*treeNode {
	if len(items) == 0 {
		return nil
	}

	root := &treeNode{isFolder: true, expanded: true, children: make([]*treeNode, 0)}

	for i := range items {
		item := &items[i]
		target := root

		if item.Folder != "" {
			segments := strings.Split(item.Folder, "/")
			for depth, seg := range segments {
				var found *treeNode
				for _, child := range target.children {
					if child.isFolder && child.name == seg {
						found = child
						break
					}
				}
				if found == nil {
					found = &treeNode{
						name:     seg,
						isFolder: true,
						expanded: true,
						depth:    depth,
						children: make([]*treeNode, 0),
					}
					target.children = append(target.children, found)
				}
				target = found
			}
		}

		note := &treeNode{
			name:     humanizeTitle(item.Title),
			isFolder: false,
			noteItem: item,
		}
		if item.Folder != "" {
			note.depth = len(strings.Split(item.Folder, "/"))
		}
		target.children = append(target.children, note)
	}

	sortTree(root)

	// Separate root children: folders first, then root notes at the end
	var folders, rootNotes []*treeNode
	for _, c := range root.children {
		if c.isFolder {
			folders = append(folders, c)
		} else {
			rootNotes = append(rootNotes, c)
		}
	}

	result := append(folders, rootNotes...)
	setLastChildFlags(result)
	return result
}

func sortTree(node *treeNode) {
	if !node.isFolder || len(node.children) == 0 {
		return
	}
	for _, child := range node.children {
		sortTree(child)
	}
	sort.SliceStable(node.children, func(i, j int) bool {
		ci, cj := node.children[i], node.children[j]
		if ci.isFolder != cj.isFolder {
			return ci.isFolder
		}
		return strings.ToLower(ci.name) < strings.ToLower(cj.name)
	})
}

func setLastChildFlags(nodes []*treeNode) {
	for i, node := range nodes {
		node.isLastChild = (i == len(nodes)-1)
		if node.isFolder {
			setLastChildFlags(node.children)
		}
	}
}

// humanizeTitle converts a raw filename-based title into a readable form.
// e.g. "running_azure_blob_storage" → "Running Azure Blob Storage"
func humanizeTitle(title string) string {
	s := strings.ReplaceAll(title, "_", " ")
	s = strings.ReplaceAll(s, "-", " ")

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
