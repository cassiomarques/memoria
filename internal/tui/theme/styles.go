package theme

import "charm.land/lipgloss/v2"

// Catppuccin Mocha palette
var (
	ColorRosewater = lipgloss.Color("#f5e0dc")
	ColorFlamingo  = lipgloss.Color("#f2cdcd")
	ColorPink      = lipgloss.Color("#f5c2e7")
	ColorMauve     = lipgloss.Color("#cba6f7")
	ColorRed       = lipgloss.Color("#f38ba8")
	ColorMaroon    = lipgloss.Color("#eba0ac")
	ColorPeach     = lipgloss.Color("#fab387")
	ColorYellow    = lipgloss.Color("#f9e2af")
	ColorGreen     = lipgloss.Color("#a6e3a1")
	ColorTeal      = lipgloss.Color("#94e2d5")
	ColorSky       = lipgloss.Color("#89dceb")
	ColorSapphire  = lipgloss.Color("#74c7ec")
	ColorBlue      = lipgloss.Color("#89b4fa")
	ColorLavender  = lipgloss.Color("#b4befe")
	ColorText      = lipgloss.Color("#cdd6f4")
	ColorSubtext1  = lipgloss.Color("#bac2de")
	ColorSubtext0  = lipgloss.Color("#a6adc8")
	ColorOverlay2  = lipgloss.Color("#9399b2")
	ColorOverlay1  = lipgloss.Color("#7f849c")
	ColorOverlay0  = lipgloss.Color("#6c7086")
	ColorSurface2  = lipgloss.Color("#585b70")
	ColorSurface1  = lipgloss.Color("#45475a")
	ColorSurface0  = lipgloss.Color("#313244")
	ColorBase      = lipgloss.Color("#1e1e2e")
	ColorMantle    = lipgloss.Color("#181825")
	ColorCrust     = lipgloss.Color("#11111b")
)

// Styles holds all TUI styles derived from the Catppuccin Mocha palette.
type Styles struct {
	TitleBar       lipgloss.Style
	NoteItem       lipgloss.Style
	NoteItemSel    lipgloss.Style
	StatusBar      lipgloss.Style
	CommandInput   lipgloss.Style
	Tag            lipgloss.Style
	FolderPath     lipgloss.Style
	HelpText       lipgloss.Style
	ErrorMessage   lipgloss.Style
	SuccessMessage lipgloss.Style
}

// DefaultStyles returns a Styles instance configured with Catppuccin Mocha colors.
func DefaultStyles() Styles {
	return Styles{
		TitleBar: lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorLavender).
			Background(ColorMantle).
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(ColorSurface2),

		NoteItem: lipgloss.NewStyle().
			Foreground(ColorText).
			Padding(0, 1),

		NoteItemSel: lipgloss.NewStyle().
			Foreground(ColorCrust).
			Background(ColorMauve).
			Bold(true).
			Padding(0, 1),

		StatusBar: lipgloss.NewStyle().
			Foreground(ColorSubtext1).
			Background(ColorSurface0).
			Padding(0, 1),

		CommandInput: lipgloss.NewStyle().
			Foreground(ColorText).
			Background(ColorMantle).
			Padding(0, 1),

		Tag: lipgloss.NewStyle().
			Foreground(ColorCrust).
			Background(ColorTeal).
			Padding(0, 1).
			MarginRight(1),

		FolderPath: lipgloss.NewStyle().
			Foreground(ColorSapphire).
			Bold(true),

		HelpText: lipgloss.NewStyle().
			Foreground(ColorSurface2),

		ErrorMessage: lipgloss.NewStyle().
			Foreground(ColorRed).
			Bold(true),

		SuccessMessage: lipgloss.NewStyle().
			Foreground(ColorGreen).
			Bold(true),
	}
}
