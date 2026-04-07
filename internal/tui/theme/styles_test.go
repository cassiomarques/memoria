package theme

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestInit_Dark(t *testing.T) {
	Init("dark")
	// Mocha text is light (#cdd6f4)
	want := lipgloss.Color("#cdd6f4")
	if ColorText != want {
		t.Errorf("dark mode ColorText = %v, want %v", ColorText, want)
	}
}

func TestInit_Light(t *testing.T) {
	Init("light")
	defer Init("dark") // restore for other tests
	// Latte text is dark (#4c4f69)
	want := lipgloss.Color("#4c4f69")
	if ColorText != want {
		t.Errorf("light mode ColorText = %v, want %v", ColorText, want)
	}
}

func TestInit_DefaultIsDark(t *testing.T) {
	Init("")
	want := lipgloss.Color("#cdd6f4")
	if ColorText != want {
		t.Errorf("empty mode should default to dark, ColorText = %v, want %v", ColorText, want)
	}
}

func TestDefaultStyles_NotZero(t *testing.T) {
	Init("dark")
	s := DefaultStyles()
	// Just verify styles are populated (non-zero)
	if s.TitleBar.GetForeground() == nil {
		t.Error("TitleBar foreground should not be nil")
	}
}
