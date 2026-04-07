package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Catppuccin Mocha palette (dark — default)
var mochaPalette = palette{
	Rosewater: "#f5e0dc", Flamingo: "#f2cdcd", Pink: "#f5c2e7", Mauve: "#cba6f7",
	Red: "#f38ba8", Maroon: "#eba0ac", Peach: "#fab387", Yellow: "#f9e2af",
	Green: "#a6e3a1", Teal: "#94e2d5", Sky: "#89dceb", Sapphire: "#74c7ec",
	Blue: "#89b4fa", Lavender: "#b4befe",
	Text: "#cdd6f4", Subtext1: "#bac2de", Subtext0: "#a6adc8",
	Overlay2: "#9399b2", Overlay1: "#7f849c", Overlay0: "#6c7086",
	Surface2: "#585b70", Surface1: "#45475a", Surface0: "#313244",
	Base: "#1e1e2e", Mantle: "#181825", Crust: "#11111b",
}

// Catppuccin Latte palette (light)
var lattePalette = palette{
	Rosewater: "#dc8a78", Flamingo: "#dd7878", Pink: "#ea76cb", Mauve: "#8839ef",
	Red: "#d20f39", Maroon: "#e64553", Peach: "#fe640b", Yellow: "#df8e1d",
	Green: "#40a02b", Teal: "#179299", Sky: "#04a5e5", Sapphire: "#209fb5",
	Blue: "#1e66f5", Lavender: "#7287fd",
	Text: "#4c4f69", Subtext1: "#5c5f77", Subtext0: "#6c6f85",
	Overlay2: "#7c7f93", Overlay1: "#8c8fa1", Overlay0: "#9ca0b0",
	Surface2: "#acb0be", Surface1: "#bcc0cc", Surface0: "#ccd0da",
	Base: "#eff1f5", Mantle: "#e6e9ef", Crust: "#dce0e8",
}

type palette struct {
	Rosewater, Flamingo, Pink, Mauve           string
	Red, Maroon, Peach, Yellow                 string
	Green, Teal, Sky, Sapphire, Blue, Lavender string
	Text, Subtext1, Subtext0                   string
	Overlay2, Overlay1, Overlay0               string
	Surface2, Surface1, Surface0               string
	Base, Mantle, Crust                        string
}

// Active color variables — set by Init().
var (
	ColorRosewater color.Color
	ColorFlamingo  color.Color
	ColorPink      color.Color
	ColorMauve     color.Color
	ColorRed       color.Color
	ColorMaroon    color.Color
	ColorPeach     color.Color
	ColorYellow    color.Color
	ColorGreen     color.Color
	ColorTeal      color.Color
	ColorSky       color.Color
	ColorSapphire  color.Color
	ColorBlue      color.Color
	ColorLavender  color.Color
	ColorText      color.Color
	ColorSubtext1  color.Color
	ColorSubtext0  color.Color
	ColorOverlay2  color.Color
	ColorOverlay1  color.Color
	ColorOverlay0  color.Color
	ColorSurface2  color.Color
	ColorSurface1  color.Color
	ColorSurface0  color.Color
	ColorBase      color.Color
	ColorMantle    color.Color
	ColorCrust     color.Color
)

func init() {
	applyPalette(mochaPalette)
}

// Init sets the active color palette. Pass "light" for Catppuccin Latte,
// anything else (including "") defaults to Catppuccin Mocha (dark).
func Init(mode string) {
	if mode == "light" {
		activeMode = "light"
		applyPalette(lattePalette)
	} else {
		activeMode = "dark"
		applyPalette(mochaPalette)
	}
}

var activeMode = "dark"

// GlamourStyle returns the glamour style name matching the active theme.
func GlamourStyle() string {
	return activeMode
}

// IsLight reports whether the active theme is a light theme.
func IsLight() bool {
	return activeMode == "light"
}

func applyPalette(p palette) {
	ColorRosewater = lipgloss.Color(p.Rosewater)
	ColorFlamingo = lipgloss.Color(p.Flamingo)
	ColorPink = lipgloss.Color(p.Pink)
	ColorMauve = lipgloss.Color(p.Mauve)
	ColorRed = lipgloss.Color(p.Red)
	ColorMaroon = lipgloss.Color(p.Maroon)
	ColorPeach = lipgloss.Color(p.Peach)
	ColorYellow = lipgloss.Color(p.Yellow)
	ColorGreen = lipgloss.Color(p.Green)
	ColorTeal = lipgloss.Color(p.Teal)
	ColorSky = lipgloss.Color(p.Sky)
	ColorSapphire = lipgloss.Color(p.Sapphire)
	ColorBlue = lipgloss.Color(p.Blue)
	ColorLavender = lipgloss.Color(p.Lavender)
	ColorText = lipgloss.Color(p.Text)
	ColorSubtext1 = lipgloss.Color(p.Subtext1)
	ColorSubtext0 = lipgloss.Color(p.Subtext0)
	ColorOverlay2 = lipgloss.Color(p.Overlay2)
	ColorOverlay1 = lipgloss.Color(p.Overlay1)
	ColorOverlay0 = lipgloss.Color(p.Overlay0)
	ColorSurface2 = lipgloss.Color(p.Surface2)
	ColorSurface1 = lipgloss.Color(p.Surface1)
	ColorSurface0 = lipgloss.Color(p.Surface0)
	ColorBase = lipgloss.Color(p.Base)
	ColorMantle = lipgloss.Color(p.Mantle)
	ColorCrust = lipgloss.Color(p.Crust)
}

// Styles holds all TUI styles derived from the active palette.
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

// DefaultStyles returns a Styles instance configured with the active palette.
// Call Init() before this to set light/dark mode.
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
