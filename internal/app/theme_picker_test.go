package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/theme"
)

func TestOpenThemePickerReloadsAndKeepsSelectionNameStable(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	themeDir := filepath.Join(dir, "themes")
	writeTestTheme(t, themeDir, "alpha", "#111111")

	cfg := config.Default()
	cfg.UI.Theme = "AlPhA"
	m := New(cfg, configPath, nil, nil, "", true)
	original := m.activeTheme
	writeTestTheme(t, themeDir, "aardvark", "#222222")
	writeTestTheme(t, themeDir, "alpha", "#333333")
	if err := os.WriteFile(filepath.Join(themeDir, "invalid.toml"), []byte("accent = '#fff'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	m.screen = screenMain
	m.openThemePicker()

	if m.screen != screenTheme {
		t.Fatalf("screen = %v, want theme picker", m.screen)
	}
	if m.themePicker.snapshotTheme != original {
		t.Fatalf("snapshot theme = %#v, want %#v", m.themePicker.snapshotTheme, original)
	}
	if m.themePicker.snapshotConfigName != "alpha" || m.themePicker.snapshotConfig != "AlPhA" {
		t.Fatalf("snapshot names = %q / %q", m.themePicker.snapshotConfigName, m.themePicker.snapshotConfig)
	}
	if m.themePicker.selectedName != "alpha" || m.selectableThemes()[m.themePicker.cursor].Name != "alpha" {
		t.Fatalf("selection = %q at %d", m.themePicker.selectedName, m.themePicker.cursor)
	}
	if m.themePicker.cursor < 2 {
		t.Fatalf("cursor = %d, want selection repositioned after newly sorted theme", m.themePicker.cursor)
	}
	if m.activeTheme.Accent != "#333333" {
		t.Fatalf("preview accent = %q, want reloaded palette", m.activeTheme.Accent)
	}
	if len(m.themeWarnings) == 0 || !strings.Contains(m.status, "invalid") {
		t.Fatalf("warnings/status = %v / %q", m.themeWarnings, m.status)
	}
}

func TestThemePickerCancelRestoresExactThemeAndConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	themeDir := filepath.Join(dir, "themes")
	writeTestTheme(t, themeDir, "custom", "#123456")
	cfg := config.Default()
	cfg.UI.Theme = "CuStOm"
	m := New(cfg, configPath, nil, nil, "", true)
	original := m.activeTheme

	writeTestTheme(t, themeDir, "custom", "#654321")
	m.screen = screenMain
	m.openThemePicker()
	m.moveThemePicker(1, true)
	m.cancelThemePicker()

	if m.activeTheme != original {
		t.Fatalf("cancel restored %#v, want exact %#v", m.activeTheme, original)
	}
	if m.cfg.UI.Theme != "CuStOm" || m.screen != screenMain {
		t.Fatalf("cancel config/screen = %q / %v", m.cfg.UI.Theme, m.screen)
	}
}

func TestThemePickerConfirmCanonicalizesAndSavesCompleteConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := configuredTestConfig()
	cfg.UI.Theme = theme.DefaultConfigName
	m := New(cfg, configPath, nil, nil, "", true)
	m.screen = screenMain
	m.openThemePicker()
	items := m.selectableThemes()
	m.setThemePickerCursor(themeIndexByName(items, "dracula"))

	cmd := m.confirmThemePicker()
	if cmd == nil || m.cfg.UI.Theme != "dracula" || m.activeTheme.Name != "dracula" || m.screen != screenMain {
		t.Fatalf("confirm state = cmd:%v config:%q active:%q screen:%v", cmd != nil, m.cfg.UI.Theme, m.activeTheme.Name, m.screen)
	}
	msg := cmd()
	if _, ok := msg.(themeSavedMsg); !ok {
		t.Fatalf("save message = %T, want themeSavedMsg", msg)
	}
	m.Update(msg)
	loaded, _, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.UI.Theme != "dracula" || loaded.Jira.ProjectKey != cfg.Jira.ProjectKey || loaded.Mail.From != cfg.Mail.From {
		t.Fatalf("saved config lost data: %#v", loaded)
	}
	if m.err != nil || !strings.Contains(m.status, "saved") {
		t.Fatalf("success err/status = %v / %q", m.err, m.status)
	}
}

func TestThemeSaveFailureKeepsSelectedThemeWithoutGenericErrorSideEffects(t *testing.T) {
	dir := t.TempDir()
	blockedParent := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blockedParent, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := New(config.Default(), filepath.Join(blockedParent, "config.yaml"), nil, nil, "", true)
	m.applyTheme(testTheme())
	m.cfg.UI.Theme = testTheme().Name
	m.loading = true

	msg := m.saveThemeCmd(testTheme().Name)()
	failure, ok := msg.(themeSaveFailedMsg)
	if !ok || failure.Err == nil {
		t.Fatalf("save message = %#v, want failure", msg)
	}
	m.Update(msg)
	if m.activeTheme != testTheme() || m.cfg.UI.Theme != testTheme().Name {
		t.Fatalf("failure reverted selected theme/config: %#v / %q", m.activeTheme, m.cfg.UI.Theme)
	}
	if !m.loading || m.err == nil || !strings.Contains(m.status, "Could not save theme") {
		t.Fatalf("failure side effects loading/err/status = %v / %v / %q", m.loading, m.err, m.status)
	}
}

func TestThemePickerNavigationWrapsPagesAndPreviews(t *testing.T) {
	m := newMainTestModel(t, 60, 12)
	m.openThemePicker()
	items := m.selectableThemes()
	m.setThemePickerCursor(0)
	m.moveThemePicker(-1, true)
	if m.themePicker.cursor != len(items)-1 || m.activeTheme != items[len(items)-1] {
		t.Fatalf("wrapped selection = %d / %q", m.themePicker.cursor, m.activeTheme.Name)
	}
	m.moveThemePicker(1, true)
	if m.themePicker.cursor != 0 || !m.activeTheme.IsDefault() {
		t.Fatalf("down wrap = %d / %q", m.themePicker.cursor, m.activeTheme.Name)
	}
	m.updateThemePicker(keyPressForBinding("pgdown"))
	if m.themePicker.cursor == 0 || m.themePicker.scroll == 0 {
		t.Fatalf("page down cursor/scroll = %d/%d", m.themePicker.cursor, m.themePicker.scroll)
	}
	m.updateThemePicker(keyPressForBinding("end"))
	if m.themePicker.cursor != len(items)-1 {
		t.Fatalf("end cursor = %d", m.themePicker.cursor)
	}
	m.updateThemePicker(keyPressForBinding("home"))
	if m.themePicker.cursor != 0 || m.themePicker.scroll != 0 {
		t.Fatalf("home cursor/scroll = %d/%d", m.themePicker.cursor, m.themePicker.scroll)
	}
}

func TestLowercaseTodoAndUppercaseThemeBindingsRemainDistinct(t *testing.T) {
	main := mainBindings()
	if binding, ok := bindingForKey(main, "t"); !ok || binding.Command != cmdTodo {
		t.Fatalf("t binding = %#v / %v", binding, ok)
	}
	if binding, ok := bindingForKey(main, "shift+t"); !ok || binding.Command != cmdTheme {
		t.Fatalf("T binding = %#v / %v", binding, ok)
	}
	if keyLabel("shift+t") != "T" {
		t.Fatalf("shift+t label = %q", keyLabel("shift+t"))
	}

	m := newMainTestModel(t, 80, 20)
	m.updateMain(keyPressForBinding("t"))
	if m.screen != screenMain {
		t.Fatalf("lowercase t opened screen %v", m.screen)
	}
	m.updateMain(keyPressForBinding("shift+t"))
	if m.screen != screenTheme {
		t.Fatalf("uppercase T opened screen %v", m.screen)
	}
}

func TestKeybindingsMenuListsAndExecutesThemeAction(t *testing.T) {
	m := newMainTestModel(t, 90, 24)
	m.modalParent = screenMain
	plain := ansi.Strip(strings.Join(m.keybindingLines(70), "\n"))
	if !strings.Contains(plain, "choose application theme") || !strings.Contains(plain, "T") {
		t.Fatalf("theme action missing from keybindings:\n%s", plain)
	}

	m.screen = screenMain
	m.openKeybindingsModal()
	m.updateKeybindingsModal(keyPressForBinding("/"))
	for _, char := range "theme" {
		m.updateKeybindingsModal(tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	m.updateKeybindingsModal(keyPressForBinding("enter"))
	if m.screen != screenTheme {
		t.Fatalf("menu theme action opened screen %v", m.screen)
	}
}

func TestThemePickerRenderFitsMinimumAndMarksPreviewAndActive(t *testing.T) {
	m := newMainTestModel(t, 30, 10)
	m.openThemePicker()
	content := m.View().Content
	lines := strings.Split(content, "\n")
	if len(lines) != 10 {
		t.Fatalf("height = %d, want 10", len(lines))
	}
	for i, line := range lines {
		if width := ansi.StringWidth(line); width != 30 {
			t.Fatalf("line %d width = %d, want 30", i, width)
		}
	}
	plain := ansi.Strip(content)
	for _, want := range []string{"Application theme", "Preview:", "active"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("render missing %q:\n%s", want, plain)
		}
	}
}

func writeTestTheme(t *testing.T, dir, name, accent string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	contents := strings.Join([]string{
		"accent = '" + accent + "'",
		"bright_fg = '#eeeeee'",
		"fg = '#999999'",
		"green = '#00aa00'",
		"yellow = '#aaaa00'",
		"red = '#aa0000'",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, name+".toml"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
