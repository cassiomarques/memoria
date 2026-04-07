package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cassiomarques/memoria/internal/tui/theme"
)

// CommandBar is a text input bar at the bottom of the screen.
type CommandBar struct {
	input         textinput.Model
	active        bool
	styles        theme.Styles
	width         int
	suggestions   []string
	suggestionIdx int
	showMenu      bool   // whether the suggestion menu is visible
	customLabel   string // overrides "CMD" badge when non-empty
}

// NewCommandBar creates a CommandBar styled with the active theme colors.
func NewCommandBar() CommandBar {
	ti := textinput.New()
	ti.Placeholder = "type a command..."
	ti.Prompt = "> "
	ti.CharLimit = 256

	s := theme.DefaultStyles()

	var tiStyles textinput.Styles
	if theme.IsLight() {
		tiStyles = textinput.DefaultLightStyles()
	} else {
		tiStyles = textinput.DefaultDarkStyles()
	}
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

// CycleSuggestion shows the suggestion menu on first press, cycles on subsequent presses.
// Returns true if suggestions are being shown.
func (c *CommandBar) CycleSuggestion() bool {
	if len(c.suggestions) == 0 {
		return false
	}

	if !c.showMenu {
		// First tab: show menu, select first item
		c.showMenu = true
		c.suggestionIdx = 0
		return true
	}

	// Subsequent tabs: cycle forward
	c.suggestionIdx++
	if c.suggestionIdx >= len(c.suggestions) {
		c.suggestionIdx = 0
	}
	return true
}

// PrevSuggestion moves selection up in the suggestion menu.
func (c *CommandBar) PrevSuggestion() {
	if !c.showMenu || len(c.suggestions) == 0 {
		return
	}
	c.suggestionIdx--
	if c.suggestionIdx < 0 {
		c.suggestionIdx = len(c.suggestions) - 1
	}
}

// NextSuggestion moves selection down in the suggestion menu.
func (c *CommandBar) NextSuggestion() {
	if !c.showMenu || len(c.suggestions) == 0 {
		return
	}
	c.suggestionIdx++
	if c.suggestionIdx >= len(c.suggestions) {
		c.suggestionIdx = 0
	}
}

// AcceptSuggestion applies the currently selected suggestion to the input.
// Returns true if a suggestion was applied.
func (c *CommandBar) AcceptSuggestion() bool {
	if !c.showMenu || len(c.suggestions) == 0 || c.suggestionIdx < 0 {
		return false
	}

	suggestion := c.suggestions[c.suggestionIdx]
	val := c.input.Value()

	// Find the first space (command separator).
	firstSpace := strings.Index(val, " ")
	if firstSpace == -1 {
		c.input.SetValue(suggestion)
	} else {
		// For multi-arg commands (e.g. "mv src dest"), find the start of the
		// last argument and replace from there. The suggestion returned by
		// Completions corresponds to the last (incomplete) argument.
		cmdAndArgs := val[:firstSpace+1]
		argsPart := val[firstSpace+1:]

		// Find where the last argument starts by looking for the last space
		// in the arguments portion.
		lastSpace := strings.LastIndex(argsPart, " ")
		if lastSpace == -1 {
			// Only one argument so far
			c.input.SetValue(cmdAndArgs + suggestion)
		} else {
			// Multiple arguments — keep everything up to and including the
			// last space, then append the suggestion.
			c.input.SetValue(cmdAndArgs + argsPart[:lastSpace+1] + suggestion)
		}
	}
	c.input.CursorEnd()
	c.DismissMenu()
	return true
}

// DismissMenu hides the suggestion menu without applying.
func (c *CommandBar) DismissMenu() {
	c.showMenu = false
	c.suggestionIdx = -1
}

// ShowingMenu reports whether the suggestion menu is visible.
func (c CommandBar) ShowingMenu() bool {
	return c.showMenu && len(c.suggestions) > 0
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
	inputLine := style.Render(label + " " + inputView)

	// Render suggestion menu above the input line
	if c.showMenu && len(c.suggestions) > 0 {
		menuStyle := lipgloss.NewStyle().
			Background(theme.ColorSurface0).
			Foreground(theme.ColorText).
			Padding(0, 1)
		selectedStyle := lipgloss.NewStyle().
			Background(theme.ColorMauve).
			Foreground(theme.ColorCrust).
			Bold(true).
			Padding(0, 1)

		maxVisible := 8
		items := c.suggestions
		if len(items) > maxVisible {
			// Show a window around the selected item
			start := c.suggestionIdx - maxVisible/2
			if start < 0 {
				start = 0
			}
			end := start + maxVisible
			if end > len(items) {
				end = len(items)
				start = end - maxVisible
				if start < 0 {
					start = 0
				}
			}
			items = items[start:end]
		}

		var menuLines []string
		for i, s := range items {
			// Find the actual index in full suggestion list
			actualIdx := i
			if len(c.suggestions) > maxVisible {
				start := c.suggestionIdx - maxVisible/2
				if start < 0 {
					start = 0
				}
				if start+maxVisible > len(c.suggestions) {
					start = len(c.suggestions) - maxVisible
					if start < 0 {
						start = 0
					}
				}
				actualIdx = start + i
			}

			if actualIdx == c.suggestionIdx {
				menuLines = append(menuLines, selectedStyle.Render(s))
			} else {
				menuLines = append(menuLines, menuStyle.Render(s))
			}
		}

		// Show count if truncated
		header := ""
		if len(c.suggestions) > maxVisible {
			countStyle := lipgloss.NewStyle().
				Foreground(theme.ColorOverlay0).
				Padding(0, 1)
			header = countStyle.Render(fmt.Sprintf("%d/%d", c.suggestionIdx+1, len(c.suggestions)))
		}

		menu := strings.Join(menuLines, "\n")
		if header != "" {
			menu = header + "\n" + menu
		}
		return menu + "\n" + inputLine
	}

	// Show ghost suggestion text when menu is not visible
	ghost := ""
	if len(c.suggestions) > 0 && !c.showMenu {
		hint := c.suggestions[0]
		val := c.input.Value()
		// Extract the argument portion (after command name)
		firstSpace := strings.Index(val, " ")
		var argPart string
		if firstSpace == -1 {
			argPart = strings.ToLower(val)
		} else {
			argPart = strings.ToLower(val[firstSpace+1:])
		}
		if argPart != "" && strings.HasPrefix(strings.ToLower(hint), argPart) {
			ghost = hint[len(argPart):]
		}
	}

	if ghost != "" {
		ghostStyle := lipgloss.NewStyle().Foreground(theme.ColorOverlay0)
		return style.Render(label + " " + inputView + ghostStyle.Render(ghost))
	}

	return inputLine
}
