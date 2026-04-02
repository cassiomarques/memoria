package components

import (
	"strings"
	"testing"
)

func TestCommandBar_NewCommandBar(t *testing.T) {
	cb := NewCommandBar()
	if cb.Active() {
		t.Error("expected command bar to start inactive")
	}
	if cb.Value() != "" {
		t.Error("expected empty initial value")
	}
}

func TestCommandBar_FocusBlur(t *testing.T) {
	cb := NewCommandBar()
	_ = cb.Focus()
	if !cb.Active() {
		t.Error("expected active after Focus")
	}
	cb.Blur()
	if cb.Active() {
		t.Error("expected inactive after Blur")
	}
}

func TestCommandBar_Reset(t *testing.T) {
	cb := NewCommandBar()
	_ = cb.Focus()
	cb.input.SetValue("hello")
	if cb.Value() != "hello" {
		t.Errorf("expected 'hello', got %q", cb.Value())
	}
	cb.Reset()
	if cb.Value() != "" {
		t.Error("expected empty after Reset")
	}
}

func TestCommandBar_ViewNotEmpty(t *testing.T) {
	cb := NewCommandBar()
	cb.SetWidth(80)
	view := cb.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestStatusBar_View(t *testing.T) {
	sb := NewStatusBar()
	sb.SetWidth(120)
	sb.SetFolder("work/projects")
	sb.SetNoteCount(42)
	sb.SetSynced(true)
	sb.SetTagFilter("go")

	view := sb.View()
	if !strings.Contains(view, "work/projects") {
		t.Errorf("expected folder in view, got: %q", view)
	}
	if !strings.Contains(view, "42") {
		t.Errorf("expected note count in view, got: %q", view)
	}
	if !strings.Contains(view, "go") {
		t.Errorf("expected tag filter in view, got: %q", view)
	}
	if !strings.Contains(view, "✓") {
		t.Errorf("expected sync indicator in view, got: %q", view)
	}
}

func TestStatusBar_DefaultFolder(t *testing.T) {
	sb := NewStatusBar()
	sb.SetWidth(80)
	view := sb.View()
	if !strings.Contains(view, "📂 /") {
		t.Error("expected default folder in view")
	}
}
