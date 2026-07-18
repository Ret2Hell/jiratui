package app

import (
	"image/color"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/theme"
)

func TestCustomThemeSemanticMapping(t *testing.T) {
	theme := testTheme()
	styles := stylesFromPalette(paletteFromTheme(theme))

	assertColor(t, "title", styles.Title.GetForeground(), lipgloss.Color(theme.Accent))
	assertColor(t, "header metadata", styles.HeaderMeta.GetForeground(), lipgloss.Color(theme.BrightFG))
	assertColor(t, "muted text", styles.Muted.GetForeground(), lipgloss.Color(theme.FG))
	assertColor(t, "success", styles.Success.GetForeground(), lipgloss.Color(theme.Green))
	assertColor(t, "warning", styles.WIP.GetForeground(), lipgloss.Color(theme.Yellow))
	assertColor(t, "error", styles.Error.GetForeground(), lipgloss.Color(theme.Red))
	assertColor(t, "selection foreground", styles.Selected.GetForeground(), lipgloss.Color(theme.BrightFG))
	assertColor(t, "selection background", styles.Selected.GetBackground(), lipgloss.Color(theme.Accent))
}

func TestApplyThemeResetsDefaultPaletteAndRestylesComponents(t *testing.T) {
	m := New(config.Default(), filepath.Join(t.TempDir(), "config.yaml"), nil, nil, "", true)
	custom := testTheme()
	m.applyTheme(custom)

	assertColor(t, "spinner", m.spinner.Style.GetForeground(), lipgloss.Color(custom.Accent))
	assertTextInputTheme(t, "filter", m.filterInput.Styles().Focused.Text.GetForeground(), m.filterInput.Styles().Cursor.Color, custom)
	for i, input := range m.setupInputs {
		assertTextInputTheme(t, "setup input", input.Styles().Focused.Text.GetForeground(), input.Styles().Cursor.Color, custom)
		if got := input.Styles().Blurred.Text.GetForeground(); !reflect.DeepEqual(got, lipgloss.Color(custom.FG)) {
			t.Fatalf("setup input %d blurred text = %v, want %v", i, got, lipgloss.Color(custom.FG))
		}
	}
	assertTextInputTheme(t, "create summary", m.createSummary.Styles().Focused.Text.GetForeground(), m.createSummary.Styles().Cursor.Color, custom)
	assertTextareaTheme(t, "create description", m.createDescription.Styles().Focused.Text.GetForeground(), m.createDescription.Styles().Focused.CursorLineNumber.GetForeground(), m.createDescription.Styles().Cursor.Color, custom)
	assertTextareaTheme(t, "report editor", m.reportEditor.Styles().Focused.Text.GetForeground(), m.reportEditor.Styles().Focused.CursorLineNumber.GetForeground(), m.reportEditor.Styles().Cursor.Color, custom)
	assertColor(t, "focused cursor line", m.createDescription.Styles().Focused.CursorLine.GetForeground(), lipgloss.Color(custom.BrightFG))
	assertColor(t, "blurred cursor line", m.createDescription.Styles().Blurred.CursorLine.GetForeground(), lipgloss.Color(custom.FG))

	m.applyTheme(theme.Default())
	assertColor(t, "default title", m.styles.Title.GetForeground(), lipgloss.BrightYellow)
	assertColor(t, "default selection foreground", m.styles.Selected.GetForeground(), lipgloss.BrightWhite)
	assertColor(t, "default selection background", m.styles.Selected.GetBackground(), lipgloss.BrightBlack)
	assertColor(t, "default spinner", m.spinner.Style.GetForeground(), lipgloss.BrightYellow)
	assertColor(t, "default input", m.filterInput.Styles().Focused.Text.GetForeground(), lipgloss.BrightWhite)
	assertColor(t, "default textarea", m.createDescription.Styles().Focused.Text.GetForeground(), lipgloss.BrightWhite)
	if !m.activeTheme.IsDefault() {
		t.Fatalf("active theme = %q, want terminal default", m.activeTheme.Name)
	}
}

func TestNewResolvesThemeAndFallsBackWithoutReplacingStatus(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")

	cfg := config.Default()
	cfg.UI.Theme = "DrAcUlA"
	m := New(cfg, configPath, nil, nil, "", true)
	if m.activeTheme.Name != "dracula" {
		t.Fatalf("active theme = %q, want dracula", m.activeTheme.Name)
	}
	if m.themeDir != filepath.Join(filepath.Dir(configPath), "themes") {
		t.Fatalf("theme dir = %q", m.themeDir)
	}
	if len(m.themeRegistry.Themes) == 0 {
		t.Fatal("theme registry is empty")
	}

	cfg.UI.Theme = theme.LegacyDefaultConfigName
	m = New(cfg, configPath, nil, nil, "", true)
	if !m.activeTheme.IsDefault() || m.status != "" {
		t.Fatalf("legacy alias resolved to %q with status %q", m.activeTheme.Name, m.status)
	}

	cfg.UI.Theme = "missing-theme"
	m = New(cfg, configPath, nil, nil, "", true)
	if !m.activeTheme.IsDefault() || !strings.Contains(m.status, `Theme "missing-theme" not found`) {
		t.Fatalf("unknown theme resolved to %q with status %q", m.activeTheme.Name, m.status)
	}
	m = New(cfg, configPath, nil, nil, "keep this", true)
	if m.status != "keep this" {
		t.Fatalf("status = %q, want stronger initial status", m.status)
	}
}

func testTheme() theme.Theme {
	return theme.Theme{
		Name:     "test",
		Accent:   "#102030",
		BrightFG: "#213141",
		FG:       "#324252",
		Green:    "#435363",
		Yellow:   "#546474",
		Red:      "#657585",
	}
}

func assertTextInputTheme(t *testing.T, name string, text, cursor color.Color, theme theme.Theme) {
	t.Helper()
	assertColor(t, name+" text", text, lipgloss.Color(theme.BrightFG))
	assertColor(t, name+" cursor", cursor, lipgloss.Color(theme.Accent))
}

func assertTextareaTheme(t *testing.T, name string, text, line, cursor color.Color, theme theme.Theme) {
	t.Helper()
	assertColor(t, name+" text", text, lipgloss.Color(theme.BrightFG))
	assertColor(t, name+" line", line, lipgloss.Color(theme.Accent))
	assertColor(t, name+" cursor", cursor, lipgloss.Color(theme.Accent))
}

func assertColor(t *testing.T, name string, got, want color.Color) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s color = %v, want %v", name, got, want)
	}
}
