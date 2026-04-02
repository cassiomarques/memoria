package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/cassiomarques/remember/internal/tui/theme"
)

// CommandBar is a text input bar at the bottom of the screen.
type CommandBar struct {
	input         textinput.Model
	active        bool
	styles        theme.Styles
	width         int
	suggestions   []string
	suggestionIdx int
	customLabel   string // overrides "CMD" badge when non-empty
}

// NewCommandBar creates a CommandBar styled with Catppuccin Mocha colors.
func NewCommandBar() CommandBar {
	ti := textinput.New()
	ti.Placeholder = "type a command..."
	ti.Prompt = "> "
	ti.CharLimit = 256

	s := theme.DefaultStyles()

	tiStyles := textinput.DefaultDarkStyles()
	tiStyles.Focused.Text = lipgloss.NewStyle().Foreground(theme.ColorText)
	tiStyles.Focused.Placeholder = lipgloss.NewStyle().Foreground(theme.ColorOverlay0)
	tiStyles.Focused.Prompt = lipgloss.NewStyle().Foreground(theme.ColorMauve).Bold(true)
	tiStyles.Blurred.Text = lipgloss.NewStyle().Foreground(theme.ColorSubtext0)
	tiStyles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(theme.ColorSurface2)
	tiStyles.Blurred.Prompt = lipgloss.NewStyle().Foreground(theme.ColorOverlay0)
	ti.SetStyles(tiStyles)

	return CommandBar{
		input:  ti,
		styles: s,
	}
}

func (c *CommandBar) SetWidth(w int) {
	c.width = w
	c.input.SetWidth(w - 2) // account for padding
}

// Focus activates the command bar and focuses the text input.
func (c *CommandBar) Focus() tea.Cmd {
	c.active = true
	return c.input.Focus()
}

// Blur deactivates the command bar.
func (c *CommandBar) Blur() {
	c.active = false
	c.input.Blur()
}

// Active reports whether the command bar is focused.
func (c CommandBar) Active() bool { return c.active }

// Value returns the current input text.
func (c CommandBar) Value() string { return c.input.Value() }

// Reset clears the input text.
func (c *CommandBar) Reset() {
	c.input.SetValue("")
	c.suggestions = nil
	c.suggestionIdx = -1
	c.customLabel = ""
}

// SetLabel overrides the "CMD" badge shown when the command bar is active.
func (c *CommandBar) SetLabel(label string) {
	c.customLabel = label
}

// SetPlaceholder changes the placeholder text shown when the input is empty.
func (c *CommandBar) SetPlaceholder(text string) {
	c.input.Placeholder = text
}

// SetSuggestions sets the completion suggestions for the current input.
func (c *CommandBar) SetSuggestions(suggestions []string) {
	c.suggestions = suggestions
	c.suggestionIdx = -1
}

// CycleSuggestion cycles through suggestions and applies the current one.
// Returns true if a suggestion was applied.
func (c *CommandBar) CycleSuggestion() bool {
	if len(c.suggestions) == 0 {
		return false
	}
	c.suggestionIdx++
	if c.suggestionIdx >= len(c.suggestions) {
		c.suggestionIdx = 0
	}

	suggestion := c.suggestions[c.suggestionIdx]

	// Replace the last "word" in the input with the suggestion.
	// For partial command names, replace the whole input.
	// For arguments, replace the last token after the command.
	val := c.input.Value()
	spaceIdx := strings.LastIndex(val, " ")
	if spaceIdx == -1 {
		// Completing a command name
		c.input.SetValue(suggestion)
	} else {
		// Completing an argument
		c.input.SetValue(val[:spaceIdx+1] + suggestion)
	}
	// Move cursor to end
	c.input.CursorEnd()
	return true
}

func (c CommandBar) Init() tea.Cmd { return nil }

func (c CommandBar) Update(msg tea.Msg) (CommandBar, tea.Cmd) {
	if !c.active {
		return c, nil
	}
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return c, cmd
}

func (c CommandBar) View() string {
	style := c.styles.CommandInput.Width(c.width)

	if !c.active {
		hint := lipgloss.NewStyle().
			Foreground(theme.ColorOverlay0).
			Render("Press : to enter a command")
		return style.Render(hint)
	}

	badgeText := "CMD"
	if c.customLabel != "" {
		badgeText = c.customLabel
	}
	label := lipgloss.NewStyle().
		Foreground(theme.ColorCrust).
		Background(theme.ColorMauve).
		Bold(true).
		Padding(0, 1).
		Render(badgeText)

	inputView := c.input.View()

	// Show ghost suggestion text
	ghost := ""
	if len(c.suggestions) > 0 && c.suggestionIdx == -1 {
		hint := c.suggestions[0]
		val := c.input.Value()
		spaceIdx := strings.LastIndex(val, " ")
		var prefix string
		if spaceIdx == -1 {
			prefix = strings.ToLower(val)
		} else {
			prefix = strings.ToLower(val[spaceIdx+1:])
		}
		if prefix != "" && strings.HasPrefix(strings.ToLower(hint), prefix) {
			ghost = hint[len(prefix):]
		}
	}

	if ghost != "" {
		ghostStyle := lipgloss.NewStyle().Foreground(theme.ColorOverlay0)
		return style.Render(label + " " + inputView + ghostStyle.Render(ghost))
	}

	return style.Render(label + " " + inputView)
}
