package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/cassiomarques/memoria/internal/tui/theme"
)

// StatusBar displays folder path, tag filter, sync status, and note count.
type StatusBar struct {
	folder       string
	tagFilter    string
	noteCount    int
	synced       bool
	width        int
	message      string
	messageStyle lipgloss.Style
	styles       theme.Styles
}

// NewStatusBar creates a new StatusBar with default styles.
func NewStatusBar() StatusBar {
	return StatusBar{
		styles: theme.DefaultStyles(),
	}
}

func (s *StatusBar) SetFolder(folder string)       { s.folder = folder }
func (s *StatusBar) SetTagFilter(tagFilter string)  { s.tagFilter = tagFilter }
func (s *StatusBar) SetNoteCount(noteCount int)     { s.noteCount = noteCount }
func (s *StatusBar) SetSynced(synced bool)          { s.synced = synced }
func (s *StatusBar) SetWidth(width int)             { s.width = width }
func (s *StatusBar) SetMessage(msg string, style lipgloss.Style) {
	s.message = msg
	s.messageStyle = style
}
func (s *StatusBar) ClearMessage() { s.message = "" }

func (s StatusBar) Init() tea.Cmd { return nil }

func (s StatusBar) Update(msg tea.Msg) (StatusBar, tea.Cmd) {
	return s, nil
}

func (s StatusBar) View() string {
	sep := lipgloss.NewStyle().Foreground(theme.ColorOverlay0).Render(" │ ")

	// Left side: message (if any) — otherwise empty
	var left string
	if s.message != "" {
		left = s.messageStyle.Render(s.message)
	}

	// Right side: note count + sync status
	var syncText string
	if s.synced {
		syncText = lipgloss.NewStyle().Foreground(theme.ColorGreen).Render("synced")
	} else {
		syncText = lipgloss.NewStyle().Foreground(theme.ColorYellow).Render("unsynced")
	}

	countText := fmt.Sprintf("%d notes", s.noteCount)
	right := countText + sep + syncText

	// Pad to fill the full width (account for style's horizontal padding of 2)
	innerWidth := s.width - 2
	usedWidth := lipgloss.Width(left) + lipgloss.Width(right)
	padding := innerWidth - usedWidth
	if padding < 1 {
		padding = 1
	}

	bar := left + strings.Repeat(" ", padding) + right

	return s.styles.StatusBar.Width(s.width).Render(bar)
}
