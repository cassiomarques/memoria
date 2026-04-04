package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	glamour "charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/cassiomarques/memoria/internal/tui/theme"
)

const placeholderText = "Press p to preview a note"

// Preview is a Glamour-based markdown preview pane with scrollable viewport.
type Preview struct {
	content  string // raw markdown
	rendered string // glamour-rendered output
	viewport viewport.Model
	width    int
	height   int
	title    string
	visible  bool
	focused  bool
	styles   theme.Styles
}

// NewPreview creates a new Preview component.
func NewPreview() Preview {
	vp := viewport.New()
	vp.SoftWrap = true

	return Preview{
		viewport: vp,
		styles:   theme.DefaultStyles(),
	}
}

// SetContent sets the raw markdown, renders it with Glamour, and updates the viewport.
func (p *Preview) SetContent(title string, markdown string) {
	p.title = title
	p.content = markdown
	p.renderContent()
}

func (p *Preview) renderContent() {
	if p.content == "" {
		p.rendered = ""
		p.viewport.SetContent("")
		return
	}

	// Glamour rendering width accounts for the border (2 chars)
	renderWidth := p.width - 2
	if renderWidth < 10 {
		renderWidth = 10
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(renderWidth),
	)
	if err != nil {
		p.rendered = p.content
		p.viewport.SetContent(p.content)
		return
	}

	rendered, err := renderer.Render(p.content)
	if err != nil {
		p.rendered = p.content
		p.viewport.SetContent(p.content)
		return
	}

	p.rendered = strings.TrimRight(rendered, "\n")
	p.viewport.SetContent(p.rendered)
}

// SetSize updates viewport dimensions.
func (p *Preview) SetSize(width, height int) {
	p.width = width
	p.height = height

	// Account for top border (1) + title line (1) + bottom border (1)
	vpHeight := height - 2
	if vpHeight < 1 {
		vpHeight = 1
	}
	// Account for left/right border
	vpWidth := width - 2
	if vpWidth < 1 {
		vpWidth = 1
	}
	p.viewport.SetWidth(vpWidth)
	p.viewport.SetHeight(vpHeight)

	// Re-render content for new width
	if p.content != "" {
		p.renderContent()
	}
}

// Toggle shows/hides the preview.
func (p *Preview) Toggle() { p.visible = !p.visible }

// Visible reports whether the preview is visible.
func (p Preview) Visible() bool { return p.visible }

// SetFocused sets the focus state for visual styling.
func (p *Preview) SetFocused(focused bool) { p.focused = focused }

// Focused reports whether the preview is focused.
func (p Preview) Focused() bool { return p.focused }

// ScrollUp scrolls the viewport up.
func (p *Preview) ScrollUp() { p.viewport.ScrollUp(1) }

// ScrollDown scrolls the viewport down.
func (p *Preview) ScrollDown() { p.viewport.ScrollDown(1) }

// ScrollToTop scrolls to the top.
func (p *Preview) ScrollToTop() { p.viewport.GotoTop() }

// ScrollToBottom scrolls to the bottom.
func (p *Preview) ScrollToBottom() { p.viewport.GotoBottom() }

func (p Preview) Init() tea.Cmd { return nil }

func (p Preview) Update(msg tea.Msg) (Preview, tea.Cmd) {
	var cmd tea.Cmd
	p.viewport, cmd = p.viewport.Update(msg)
	return p, cmd
}

func (p Preview) View() string {
	borderColor := theme.ColorSurface2
	if p.focused {
		borderColor = theme.ColorMauve
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColorMauve).
		Background(theme.ColorSurface0).
		Padding(0, 1)

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(p.width - 2). // account for border chars
		Height(p.height - 2)

	if p.content == "" {
		placeholder := lipgloss.NewStyle().
			Foreground(theme.ColorOverlay0).
			Width(p.width-4).
			Height(p.height-4).
			Align(lipgloss.Center, lipgloss.Center)
		return borderStyle.Render(placeholder.Render("Press p to preview a note"))
	}

	// Title bar with scroll position
	titleText := titleStyle.Render(" " + p.title + " ")
	scrollPct := p.viewport.ScrollPercent()
	scrollInfo := lipgloss.NewStyle().
		Foreground(theme.ColorOverlay0).
		Render(fmt.Sprintf(" %d%%", int(scrollPct*100)))
	titleLine := titleText + scrollInfo

	vpView := p.viewport.View()
	content := lipgloss.JoinVertical(lipgloss.Left, titleLine, vpView)

	return borderStyle.Render(content)
}
