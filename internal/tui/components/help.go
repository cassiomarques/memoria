package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/cassiomarques/memoria/internal/tui/theme"
)

// Help is a toggleable overlay that displays keyboard shortcuts.
type Help struct {
	visible bool
	width   int
	height  int
}

// NewHelp creates a new Help overlay (initially hidden).
func NewHelp() Help {
	return Help{}
}

// Toggle flips the visibility of the help overlay.
func (h *Help) Toggle() { h.visible = !h.visible }

// Visible reports whether the help overlay is shown.
func (h Help) Visible() bool { return h.visible }

// SetSize updates the available area for centering the overlay.
func (h *Help) SetSize(width, height int) {
	h.width = width
	h.height = height
}

// View renders the help overlay as a centered box with keyboard shortcuts.
func (h Help) View() string {
	if !h.visible {
		return ""
	}

	heading := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColorMauve).
		MarginBottom(1)

	key := lipgloss.NewStyle().
		Foreground(theme.ColorLavender)

	desc := lipgloss.NewStyle().
		Foreground(theme.ColorText)

	dimDesc := lipgloss.NewStyle().
		Foreground(theme.ColorSubtext0)

	line := func(k, d string) string {
		return "  " + key.Render(padRight(k, 10)) + " " + desc.Render(d)
	}

	dimLine := func(k, d string) string {
		return "  " + key.Render(padRight(k, 10)) + " " + dimDesc.Render(d)
	}

	var b strings.Builder

	b.WriteString(heading.Render("Navigation"))
	b.WriteByte('\n')
	b.WriteString(line("j/k", "Move up/down"))
	b.WriteByte('\n')
	b.WriteString(line("gg/G", "Top/bottom"))
	b.WriteByte('\n')
	b.WriteString(line("Ctrl+d/u", "Page down/up"))
	b.WriteByte('\n')
	b.WriteString(line("Tab", "Switch pane"))
	b.WriteByte('\n')
	b.WriteString(line("p", "Toggle preview"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(heading.Render("Commands (press : to enter)"))
	b.WriteByte('\n')
	b.WriteString(line("new", "Create a note"))
	b.WriteByte('\n')
	b.WriteString(line("open", "Open in editor"))
	b.WriteByte('\n')
	b.WriteString(line("search", "Full-text search"))
	b.WriteByte('\n')
	b.WriteString(dimLine("tag/untag", "Manage tags"))
	b.WriteByte('\n')
	b.WriteString(dimLine("ls/cd", "Browse folders"))
	b.WriteByte('\n')
	b.WriteString(dimLine("mv/rm", "Move/delete"))
	b.WriteByte('\n')
	b.WriteString(dimLine("sync", "Git sync"))
	b.WriteByte('\n')
	b.WriteString(dimLine("help", "This screen"))
	b.WriteByte('\n')
	b.WriteByte('\n')

	b.WriteString(heading.Render("General"))
	b.WriteByte('\n')
	b.WriteString(line("q", "Quit"))
	b.WriteByte('\n')
	b.WriteString(line("Esc", "Cancel/back"))
	b.WriteByte('\n')
	b.WriteString(line("?", "Show this help"))

	content := b.String()

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColorMauve).
		Padding(0, 1).
		Render("⌨️  Keyboard Shortcuts")

	boxWidth := 42
	if boxWidth > h.width-4 {
		boxWidth = h.width - 4
	}
	if boxWidth < 20 {
		boxWidth = 20
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorMauve).
		Background(theme.ColorSurface0).
		Foreground(theme.ColorText).
		Padding(1, 2).
		Width(boxWidth)

	inner := lipgloss.JoinVertical(lipgloss.Left, title, "", content)
	rendered := box.Render(inner)

	// Center the box in the available space
	boxH := lipgloss.Height(rendered)
	boxW := lipgloss.Width(rendered)

	padLeft := (h.width - boxW) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	padTop := (h.height - boxH) / 2
	if padTop < 0 {
		padTop = 0
	}

	leftPad := strings.Repeat(" ", padLeft)
	lines := strings.Split(rendered, "\n")
	for i, l := range lines {
		lines[i] = leftPad + l
	}

	topPad := strings.Repeat("\n", padTop)
	return topPad + strings.Join(lines, "\n")
}

// padRight pads s with spaces to the given width.
func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
}
