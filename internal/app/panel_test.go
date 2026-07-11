package app

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestPanelExactANSIWidthAndChrome(t *testing.T) {
	m := &Model{styles: newStyles()}
	got := m.renderPanelSpec(panelSpec{Title: "Tïckets 界", Active: true, Content: "first line that is too long\nsecond", Width: 24, Height: 6, Position: &listPosition{Current: 1, Total: 18}, Scrollbar: &scrollbarState{ContentSize: 20, PageSize: 4, Offset: 0}})
	lines := strings.Split(got, "\n")
	if len(lines) != 6 {
		t.Fatalf("height=%d", len(lines))
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w != 24 {
			t.Fatalf("line %d width=%d: %q", i, w, line)
		}
	}
	plain := ansi.Strip(got)
	if !strings.Contains(plain, "◆ Tïckets") {
		t.Fatalf("active marker/title missing: %q", plain)
	}
	if !strings.Contains(plain, "2 of 18") {
		t.Fatalf("position missing: %q", plain)
	}
	if !strings.Contains(plain, "▐") {
		t.Fatalf("scrollbar missing: %q", plain)
	}
	if !strings.HasPrefix(strings.Split(plain, "\n")[0], "╭") || !strings.HasSuffix(strings.Split(plain, "\n")[5], "╯") {
		t.Fatal("corners corrupt")
	}
}

func TestPanelOmitsMetadataWhenItCannotFit(t *testing.T) {
	m := &Model{styles: newStyles()}
	got := ansi.Strip(m.renderPanelSpec(panelSpec{Title: "X", Width: 8, Height: 4, Position: &listPosition{Current: 999, Total: 1000}}))
	if strings.Contains(got, "of") {
		t.Fatalf("metadata should be omitted: %q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if ansi.StringWidth(line) != 8 {
			t.Fatalf("width=%d", ansi.StringWidth(line))
		}
	}
}

func TestPanelDoesNotWrapContent(t *testing.T) {
	m := &Model{styles: newStyles()}
	got := ansi.Strip(m.renderPanelSpec(panelSpec{Title: "X", Width: 12, Height: 4, Content: strings.Repeat("a", 40)}))
	if len(strings.Split(got, "\n")) != 4 {
		t.Fatal("content wrapped")
	}
}
