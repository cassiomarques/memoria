package components

import (
	"strings"
	"testing"
)

func TestPreview_Toggle(t *testing.T) {
	p := NewPreview()

	if p.Visible() {
		t.Error("expected preview to start hidden")
	}

	p.Toggle()
	if !p.Visible() {
		t.Error("expected preview to be visible after Toggle")
	}

	p.Toggle()
	if p.Visible() {
		t.Error("expected preview to be hidden after second Toggle")
	}
}

func TestPreview_SetContentRendersMarkdown(t *testing.T) {
	p := NewPreview()
	p.SetSize(80, 24)
	p.SetContent("Test Note", "# Hello\n\nThis is **bold** text.")

	if p.content != "# Hello\n\nThis is **bold** text." {
		t.Errorf("expected raw content to be stored, got %q", p.content)
	}
	if p.rendered == "" {
		t.Error("expected rendered content to be non-empty")
	}
	if p.title != "Test Note" {
		t.Errorf("expected title 'Test Note', got %q", p.title)
	}
}

func TestPreview_SetSizeUpdatesDimensions(t *testing.T) {
	p := NewPreview()
	p.SetSize(100, 40)

	if p.width != 100 {
		t.Errorf("expected width 100, got %d", p.width)
	}
	if p.height != 40 {
		t.Errorf("expected height 40, got %d", p.height)
	}
}

func TestPreview_EmptyContentShowsPlaceholder(t *testing.T) {
	p := NewPreview()
	p.SetSize(60, 20)

	view := p.View()
	if !strings.Contains(view, placeholderText) {
		t.Errorf("expected placeholder text in view, got: %q", view)
	}
}

func TestPreview_SetContentThenClear(t *testing.T) {
	p := NewPreview()
	p.SetSize(80, 24)

	p.SetContent("Note", "# Heading")
	if p.rendered == "" {
		t.Error("expected rendered content after SetContent")
	}

	p.SetContent("", "")
	if p.rendered != "" {
		t.Error("expected empty rendered content after clearing")
	}

	view := p.View()
	if !strings.Contains(view, placeholderText) {
		t.Error("expected placeholder after clearing content")
	}
}

func TestPreview_FocusedState(t *testing.T) {
	p := NewPreview()
	if p.Focused() {
		t.Error("expected unfocused by default")
	}
	p.SetFocused(true)
	if !p.Focused() {
		t.Error("expected focused after SetFocused(true)")
	}
	p.SetFocused(false)
	if p.Focused() {
		t.Error("expected unfocused after SetFocused(false)")
	}
}

func TestPreview_ViewWithContent(t *testing.T) {
	p := NewPreview()
	p.SetSize(80, 24)
	p.SetContent("My Note", "Some markdown content")

	view := p.View()
	if view == "" {
		t.Error("expected non-empty view with content")
	}
	if !strings.Contains(view, "My Note") {
		t.Errorf("expected title in view, got: %q", view)
	}
}

func TestPreview_EstimateSourceLine_AtTop(t *testing.T) {
	p := NewPreview()
	p.SetSize(80, 24)
	p.SetContent("Test", "line1\nline2\nline3\nline4\nline5")

	// At top, should return line 1
	line := p.EstimateSourceLine()
	if line != 1 {
		t.Errorf("expected line 1 at top, got %d", line)
	}
}

func TestPreview_EstimateSourceLine_EmptyContent(t *testing.T) {
	p := NewPreview()
	p.SetSize(80, 24)

	line := p.EstimateSourceLine()
	if line != 1 {
		t.Errorf("expected line 1 for empty content, got %d", line)
	}
}
