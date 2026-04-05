package components

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cassiomarques/memoria/internal/tui/theme"
)

// NoteItem represents a note entry for the list.
type NoteItem struct {
	Path     string
	Title    string
	Folder   string
	Tags     []string
	Modified time.Time
	Pinned   bool
}

// treeNode represents a node in the folder tree (either a folder or a note).
type treeNode struct {
	name        string
	fullPath    string // full folder path for folders (e.g., "Projects/CodeCoverage")
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
	items        []NoteItem
	tree         []*treeNode
	flatVisible  []*treeNode
	cursor       int
	offset       int
	height       int
	width        int
	styles       theme.Styles
	expandAll    bool // whether all folders start expanded
	showPinned   bool // whether to show virtual "Pinned" section at top
	showModified bool // whether to show modification timestamps
	filterText   string

	// gg detection: true after first 'g' press
	pendingG bool
}

// NewNoteList creates a NoteList with default styles.
func NewNoteList() NoteList {
	return NoteList{
		styles:     theme.DefaultStyles(),
		expandAll:  true, // default: all folders expanded
		showPinned: true, // default: show pinned section
	}
}

// SetExpandAll sets whether all folders start expanded when items are loaded.
func (n *NoteList) SetExpandAll(v bool) {
	n.expandAll = v
}

// SetShowPinned sets whether the virtual "📌 Pinned" section appears at the top.
func (n *NoteList) SetShowPinned(v bool) {
	n.showPinned = v
}

// ToggleShowModified toggles whether modification timestamps are shown next to notes.
func (n *NoteList) ToggleShowModified() bool {
	n.showModified = !n.showModified
	return n.showModified
}

// SetShowModified sets whether modification timestamps are shown next to notes.
func (n *NoteList) SetShowModified(v bool) {
	n.showModified = v
}

func (n *NoteList) SetItems(items []NoteItem) {
	// Remember current selection to restore after rebuild
	var selectedPath string
	if sel := n.SelectedItem(); sel != nil {
		selectedPath = sel.Path
	}

	n.items = items
	n.tree = buildTree(n.items, n.expandAll, n.showPinned)
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

// SelectedIsFolder reports whether the cursor is on a folder node.
func (n NoteList) SelectedIsFolder() bool {
	if len(n.flatVisible) == 0 || n.cursor >= len(n.flatVisible) {
		return false
	}
	return n.flatVisible[n.cursor].isFolder
}

// SelectedFolderPath returns the full path of the selected folder, or "" if not a folder.
func (n NoteList) SelectedFolderPath() string {
	if len(n.flatVisible) == 0 || n.cursor >= len(n.flatVisible) {
		return ""
	}
	node := n.flatVisible[n.cursor]
	if !node.isFolder {
		return ""
	}
	return node.fullPath
}

// SelectedFolderNoteCount returns the number of notes under the selected folder.
func (n NoteList) SelectedFolderNoteCount() int {
	if len(n.flatVisible) == 0 || n.cursor >= len(n.flatVisible) {
		return 0
	}
	node := n.flatVisible[n.cursor]
	if !node.isFolder {
		return 0
	}
	return node.countNotes()
}

// SelectedIsExpanded reports whether the selected folder is expanded.
func (n NoteList) SelectedIsExpanded() bool {
	if len(n.flatVisible) == 0 || n.cursor >= len(n.flatVisible) {
		return false
	}
	node := n.flatVisible[n.cursor]
	return node.isFolder && node.expanded
}

// CollapseSelected collapses the selected folder.
// If on a note, collapses the parent folder and moves cursor there.
func (n *NoteList) CollapseSelected() {
	if len(n.flatVisible) == 0 || n.cursor >= len(n.flatVisible) {
		return
	}
	node := n.flatVisible[n.cursor]

	if node.isFolder && node.expanded {
		node.expanded = false
		n.rebuildFlatVisible()
		n.clampCursor()
		return
	}

	// On a note (or already-collapsed folder): find parent folder and collapse it
	if !node.isFolder {
		targetDepth := node.depth - 1
		for i := n.cursor - 1; i >= 0; i-- {
			candidate := n.flatVisible[i]
			if candidate.isFolder && candidate.depth <= targetDepth {
				candidate.expanded = false
				n.cursor = i
				n.rebuildFlatVisible()
				// Find the parent folder again in rebuilt list
				for j, fn := range n.flatVisible {
					if fn == candidate {
						n.cursor = j
						break
					}
				}
				n.clampCursor()
				return
			}
		}
	}
}

// ExpandSelected expands the selected folder.
func (n *NoteList) ExpandSelected() {
	if len(n.flatVisible) == 0 || n.cursor >= len(n.flatVisible) {
		return
	}
	node := n.flatVisible[n.cursor]
	if !node.isFolder || node.expanded {
		return
	}
	node.expanded = true
	n.rebuildFlatVisible()
	n.clampCursor()
}

// ExpandAll expands every folder in the tree.
func (n *NoteList) ExpandAll() {
	n.walkTree(func(node *treeNode) {
		if node.isFolder {
			node.expanded = true
		}
	})
	n.rebuildFlatVisible()
	n.clampCursor()
}

// CollapseAll collapses every folder in the tree.
func (n *NoteList) CollapseAll() {
	n.walkTree(func(node *treeNode) {
		if node.isFolder {
			node.expanded = false
		}
	})
	n.rebuildFlatVisible()
	n.clampCursor()
}

// walkTree calls fn for every node in the tree (depth-first).
func (n *NoteList) walkTree(fn func(*treeNode)) {
	var walk func([]*treeNode)
	walk = func(nodes []*treeNode) {
		for _, node := range nodes {
			fn(node)
			if node.isFolder {
				walk(node.children)
			}
		}
	}
	walk(n.tree)
}

func (n *NoteList) clampCursor() {
	if n.cursor >= len(n.flatVisible) {
		n.cursor = len(n.flatVisible) - 1
	}
	if n.cursor < 0 {
		n.cursor = 0
	}
	n.ensureVisible()
}

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
	if msg, ok := msg.(tea.KeyPressMsg); ok {
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
	n.clampCursor()
}

func (n *NoteList) moveDown() {
	if n.cursor < len(n.flatVisible)-1 {
		n.cursor++
		n.ensureVisible()
	}
}

// MoveDown moves the cursor down by one item (public, for use by App in filter mode).
func (n *NoteList) MoveDown() { n.moveDown() }

func (n *NoteList) moveUp() {
	if n.cursor > 0 {
		n.cursor--
		n.ensureVisible()
	}
}

// MoveUp moves the cursor up by one item (public, for use by App in filter mode).
func (n *NoteList) MoveUp() { n.moveUp() }

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
	if node.noteItem != nil && node.noteItem.Pinned {
		displayTitle = "📌 " + displayTitle
	}

	// Build optional timestamp suffix
	timeSuffix := ""
	if n.showModified && node.noteItem != nil && !node.noteItem.Modified.IsZero() {
		timeSuffix = " " + formatRelativeTime(node.noteItem.Modified)
	}

	overhead := 4 + 2*node.depth + len(timeSuffix)
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

	timeStyle := lipgloss.NewStyle().Foreground(theme.ColorOverlay1)

	if node.depth > 0 {
		connector := "├── "
		if node.isLastChild {
			connector = "└── "
		}
		connStyle := lipgloss.NewStyle().Foreground(theme.ColorOverlay0)
		line := indicator + indent + connStyle.Render(connector) + titleStyle.Render(displayTitle) + timeStyle.Render(timeSuffix)
		return lipgloss.NewStyle().Width(n.width).Render(line)
	}

	// Root-level note
	line := indicator + titleStyle.Render(displayTitle) + timeStyle.Render(timeSuffix)
	return lipgloss.NewStyle().Width(n.width).Render(line)
}

// buildTree constructs a hierarchical tree from a flat list of NoteItems.
func buildTree(items []NoteItem, expandAll bool, showPinned bool) []*treeNode {
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
						fullPath: strings.Join(segments[:depth+1], "/"),
						isFolder: true,
						expanded: expandAll || depth == 0,
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

	// Separate root children: folders, then root notes
	var folders, rootNotes []*treeNode
	for _, c := range root.children {
		switch {
		case c.isFolder:
			folders = append(folders, c)
		default:
			rootNotes = append(rootNotes, c)
		}
	}

	result := make([]*treeNode, 0, 1+len(folders)+len(rootNotes))

	// Add virtual "📌 Pinned" section at the very top if enabled and there are pinned notes
	if showPinned {
		var pinnedNodes []*treeNode
		collectPinned(root, &pinnedNodes)
		if len(pinnedNodes) > 0 {
			pinnedFolder := &treeNode{
				name:     "📌 Pinned",
				fullPath: "__pinned__",
				isFolder: true,
				expanded: true,
				depth:    0,
				children: make([]*treeNode, 0, len(pinnedNodes)),
			}
			for _, pn := range pinnedNodes {
				// Create shallow copy so they appear under the virtual section
				child := &treeNode{
					name:     humanizeTitle(pn.noteItem.Title),
					isFolder: false,
					noteItem: pn.noteItem,
					depth:    1,
				}
				pinnedFolder.children = append(pinnedFolder.children, child)
			}
			setLastChildFlags(pinnedFolder.children)
			result = append(result, pinnedFolder)
		}
	}

	result = append(result, folders...)
	result = append(result, rootNotes...)
	setLastChildFlags(result)
	return result
}

// collectPinned recursively gathers all pinned note nodes.
func collectPinned(node *treeNode, result *[]*treeNode) {
	for _, c := range node.children {
		if c.isFolder {
			collectPinned(c, result)
		} else if c.noteItem != nil && c.noteItem.Pinned {
			*result = append(*result, c)
		}
	}
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

// fuzzyMatch returns true if every character in pattern appears (in order,
// case-insensitive) somewhere in target. When it matches it also returns a
// score — lower is better. Consecutive matching characters and matches at
// word boundaries score better.
func fuzzyMatch(pattern, target string) (bool, int) {
	if pattern == "" {
		return true, 0
	}

	pLower := strings.ToLower(pattern)
	tLower := strings.ToLower(target)

	pi := 0
	score := 0
	prevMatchIdx := -1

	for ti := 0; ti < len(tLower) && pi < len(pLower); ti++ {
		if tLower[ti] == pLower[pi] {
			// Bonus for consecutive matches
			if prevMatchIdx >= 0 && ti == prevMatchIdx+1 {
				score -= 5
			}
			// Bonus for match at start or after a word boundary
			if ti == 0 || tLower[ti-1] == ' ' || tLower[ti-1] == '/' || tLower[ti-1] == '_' || tLower[ti-1] == '-' {
				score -= 3
			}
			// Penalty proportional to distance from last match
			if prevMatchIdx >= 0 {
				score += ti - prevMatchIdx - 1
			}
			prevMatchIdx = ti
			pi++
		}
	}

	if pi < len(pLower) {
		return false, 0
	}
	return true, score
}

// NoteMatchesFilter checks whether a NoteItem matches the filter pattern.
// It fuzzy-matches against the title, folder path, and tags, returning the
// best (lowest) score found.
func NoteMatchesFilter(item *NoteItem, pattern string) (bool, int) {
	bestScore := int(^uint(0) >> 1) // max int
	matched := false

	if ok, s := fuzzyMatch(pattern, item.Title); ok {
		matched = true
		if s < bestScore {
			bestScore = s
		}
	}
	if ok, s := fuzzyMatch(pattern, item.Path); ok {
		matched = true
		if s < bestScore {
			bestScore = s
		}
	}
	if ok, s := fuzzyMatch(pattern, item.Folder); ok {
		matched = true
		if s < bestScore {
			bestScore = s
		}
	}
	for _, tag := range item.Tags {
		if ok, s := fuzzyMatch(pattern, tag); ok {
			matched = true
			if s < bestScore {
				bestScore = s
			}
		}
	}

	return matched, bestScore
}

// SetFilter applies a fuzzy filter to the note list. Only notes matching the
// pattern are shown; folders that contain no matching notes are hidden. An
// empty pattern restores the full list.
func (n *NoteList) SetFilter(pattern string) {
	n.filterText = pattern
	if pattern == "" {
		// Restore full list
		n.tree = buildTree(n.items, n.expandAll, n.showPinned)
		n.flatVisible = nil
		n.rebuildFlatVisible()
		n.cursor = 0
		n.offset = 0
		return
	}

	// Filter items and sort by score
	type scored struct {
		item  NoteItem
		score int
	}
	var matches []scored
	for i := range n.items {
		if ok, s := NoteMatchesFilter(&n.items[i], pattern); ok {
			matches = append(matches, scored{n.items[i], s})
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score < matches[j].score
	})

	filtered := make([]NoteItem, len(matches))
	for i, m := range matches {
		filtered[i] = m.item
	}

	n.tree = buildTree(filtered, true, false) // always expand when filtering, no pinned section
	n.flatVisible = nil
	n.rebuildFlatVisible()
	n.cursor = 0
	n.offset = 0
}

// SetFilteredItems replaces the displayed items with a pre-filtered set
// (e.g. from Bleve search) and records the filter text. The original items
// are preserved so AllItems() still returns the full set.
func (n *NoteList) SetFilteredItems(items []NoteItem, filterText string) {
	n.filterText = filterText
	n.tree = buildTree(items, true, false)
	n.flatVisible = nil
	n.rebuildFlatVisible()
	n.cursor = 0
	n.offset = 0
}

// AllItems returns the full (unfiltered) item list.
func (n *NoteList) AllItems() []NoteItem {
	return n.items
}

// ClearFilter removes any active filter and restores the full note list.
func (n *NoteList) ClearFilter() {
	n.SetFilter("")
}

// IsFiltering returns true when a filter is active.
func (n NoteList) IsFiltering() bool {
	return n.filterText != ""
}

// FilterText returns the current filter pattern.
func (n NoteList) FilterText() string {
	return n.filterText
}

// FilteredCount returns the number of notes (not folders) currently visible.
// When no filter is active this equals ItemCount().
func (n *NoteList) FilteredCount() int {
	count := 0
	n.walkTree(func(node *treeNode) {
		if !node.isFolder {
			count++
		}
	})
	return count
}

// formatRelativeTime returns a human-friendly relative time string.
func formatRelativeTime(t time.Time) string {
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		m := int(diff.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case diff < 24*time.Hour:
		h := int(diff.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	case diff < 48*time.Hour:
		return "yesterday"
	case diff < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	default:
		return t.Format("Jan 02")
	}
}
