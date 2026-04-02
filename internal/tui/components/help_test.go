package components

import (
	"strings"
	"testing"
)

func TestHelp_InitiallyHidden(t *testing.T) {
	h := NewHelp()
	if h.Visible() {
		t.Error("expected help to start hidden")
	}
}

func TestHelp_Toggle(t *testing.T) {
	h := NewHelp()
	h.Toggle()
	if !h.Visible() {
		t.Error("expected help to be visible after Toggle")
	}
	h.Toggle()
	if h.Visible() {
		t.Error("expected help to be hidden after second Toggle")
	}
}

func TestHelp_ViewWhenHidden(t *testing.T) {
	h := NewHelp()
	h.SetSize(80, 40)
	if h.View() != "" {
		t.Error("expected empty view when hidden")
	}
}

func TestHelp_ViewContainsSections(t *testing.T) {
	h := NewHelp()
	h.SetSize(80, 40)
	h.Toggle()

	view := h.View()

	sections := []string{"Navigation", "Commands", "General", "Keyboard Shortcuts"}
	for _, sec := range sections {
		if !strings.Contains(view, sec) {
			t.Errorf("expected view to contain %q", sec)
		}
	}
}

func TestHelp_ViewContainsKeys(t *testing.T) {
	h := NewHelp()
	h.SetSize(80, 40)
	h.Toggle()

	view := h.View()

	keys := []string{"j/k", "gg/G", "Tab", "new", "search", "Esc", "?"}
	for _, k := range keys {
		if !strings.Contains(view, k) {
			t.Errorf("expected view to contain key %q", k)
		}
	}
}

func TestHelp_SetSize(t *testing.T) {
	h := NewHelp()
	h.SetSize(120, 50)
	if h.width != 120 || h.height != 50 {
		t.Errorf("expected size 120x50, got %dx%d", h.width, h.height)
	}
}
