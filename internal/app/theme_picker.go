package app

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Ret2Hell/jiratui/internal/theme"
)

func canonicalThemeName(t theme.Theme) string {
	if t.IsDefault() {
		return theme.DefaultConfigName
	}
	return t.Name
}

func (m *Model) selectableThemes() []theme.Theme {
	themes := make([]theme.Theme, 1, len(m.themeRegistry.Themes)+1)
	themes[0] = theme.Default()
	return append(themes, m.themeRegistry.Themes...)
}

func (m *Model) openThemePicker() {
	snapshotTheme := m.activeTheme
	activeName := canonicalThemeName(snapshotTheme)

	m.themeRegistry = theme.LoadAll(m.themeDir)
	m.themeWarnings = slices.Clone(m.themeRegistry.Warnings)
	if m.themeDirErr != nil {
		m.themeWarnings = append(m.themeWarnings, m.themeDirErr)
	}
	m.themePicker = themePickerState{
		snapshotTheme:      snapshotTheme,
		snapshotConfigName: activeName,
		snapshotConfig:     m.cfg.UI.Theme,
		activeName:         activeName,
		selectedName:       activeName,
	}

	items := m.selectableThemes()
	if refreshed, ok := m.themeRegistry.Resolve(activeName); ok {
		m.themePicker.selectedName = canonicalThemeName(refreshed)
		m.themePicker.cursor = themeIndexByName(items, m.themePicker.selectedName)
		m.applyTheme(refreshed)
	} else {
		m.themePicker.cursor = 0
		m.themePicker.selectedName = theme.DefaultConfigName
		m.applyTheme(items[0])
	}
	if len(m.themeWarnings) > 0 {
		m.status = fmt.Sprintf("Theme warning (%d): %v", len(m.themeWarnings), m.themeWarnings[0])
	}
	m.screen = screenTheme
}

func themeIndexByName(items []theme.Theme, name string) int {
	for i, item := range items {
		if strings.EqualFold(canonicalThemeName(item), name) {
			return i
		}
	}
	return 0
}

func (m *Model) updateThemePicker(key tea.KeyPressMsg) tea.Cmd {
	binding, ok := bindingForKey(themePickerBindings(), key.Keystroke())
	if !ok {
		return nil
	}
	_, pageSize := m.themePickerMetrics()
	switch binding.Command {
	case cmdCancel:
		m.cancelThemePicker()
	case cmdSave:
		return m.confirmThemePicker()
	case cmdUp:
		m.moveThemePicker(-1, true)
	case cmdDown:
		m.moveThemePicker(1, true)
	case cmdPageUp:
		m.moveThemePicker(-max(1, pageSize), false)
	case cmdPageDown:
		m.moveThemePicker(max(1, pageSize), false)
	case cmdHome:
		m.setThemePickerCursor(0)
	case cmdEnd:
		m.setThemePickerCursor(len(m.selectableThemes()) - 1)
	}
	return nil
}

func (m *Model) moveThemePicker(delta int, wrap bool) {
	items := m.selectableThemes()
	m.reconcileThemePickerCursor(items)
	count := len(items)
	if count == 0 {
		return
	}
	next := m.themePicker.cursor + delta
	if wrap {
		next = (next%count + count) % count
	} else {
		next = min(max(0, next), count-1)
	}
	m.setThemePickerCursor(next)
}

func (m *Model) setThemePickerCursor(cursor int) {
	items := m.selectableThemes()
	if len(items) == 0 {
		return
	}
	m.themePicker.cursor = min(max(0, cursor), len(items)-1)
	selected := items[m.themePicker.cursor]
	m.themePicker.selectedName = canonicalThemeName(selected)
	m.applyTheme(selected)
	_, pageSize := m.themePickerMetrics()
	m.themePicker.scroll = ensureVisible(m.themePicker.scroll, m.themePicker.cursor, len(items), pageSize)
}

func (m *Model) cancelThemePicker() {
	m.applyTheme(m.themePicker.snapshotTheme)
	m.cfg.UI.Theme = m.themePicker.snapshotConfig
	m.screen = screenMain
}

func (m *Model) confirmThemePicker() tea.Cmd {
	items := m.selectableThemes()
	if len(items) == 0 {
		return nil
	}
	m.reconcileThemePickerCursor(items)
	selected := items[min(max(0, m.themePicker.cursor), len(items)-1)]
	name := canonicalThemeName(selected)
	m.applyTheme(selected)
	m.cfg.UI.Theme = name
	m.screen = screenMain
	m.err = nil
	m.status = fmt.Sprintf("Saving theme %q", name)
	return m.saveThemeCmd(name)
}

func (m *Model) scrollThemePicker(msg tea.MouseWheelMsg) {
	switch msg.Button {
	case tea.MouseWheelUp:
		m.moveThemePicker(-3, true)
	case tea.MouseWheelDown:
		m.moveThemePicker(3, true)
	}
}

func (m *Model) themePickerMetrics() (int, int) {
	items := m.selectableThemes()
	height := popupHeight(m.height, len(items)+2, 8)
	return height, max(1, height-4)
}

func (m *Model) renderThemePicker() string {
	background := m.mainBackground()
	items := m.selectableThemes()
	width := popupWidth(m.width, 72, 40)
	height, pageSize := m.themePickerMetrics()
	contentWidth := max(1, width-4)

	m.reconcileThemePickerCursor(items)
	m.themePicker.scroll = ensureVisible(m.themePicker.scroll, m.themePicker.cursor, len(items), pageSize)
	start, end := visibleRange(m.themePicker.scroll, len(items), pageSize)
	selectedName := theme.DefaultConfigName
	if len(items) > 0 {
		selectedName = items[m.themePicker.cursor].Name
	}
	lines := []string{m.styles.ModalSection.Render(truncatePlain("* active  Preview: "+selectedName, contentWidth)), ""}
	for i := start; i < end; i++ {
		item := items[i]
		marker := "  "
		if strings.EqualFold(canonicalThemeName(item), m.themePicker.activeName) {
			marker = "* "
			item.Name += " (active)"
		}
		line := marker + item.Name
		if i == m.themePicker.cursor {
			line = m.styles.Selected.Render(padRight(truncatePlain("> "+line, contentWidth), contentWidth))
		} else {
			line = truncatePlain("  "+line, contentWidth)
		}
		lines = append(lines, line)
	}
	for len(lines) < pageSize+2 {
		lines = append(lines, "")
	}
	panel := m.renderPanelSpec(panelSpec{
		Title:     "Application theme",
		Active:    true,
		Content:   strings.Join(lines, "\n"),
		Footer:    "enter select  •  esc/q/T cancel",
		Width:     width,
		Height:    height,
		Scrollbar: &scrollbarState{ContentSize: len(items), PageSize: pageSize, Offset: m.themePicker.scroll},
	})
	return overlayCentered(background, panel, m.width, m.height)
}

func (m *Model) reconcileThemePickerCursor(items []theme.Theme) {
	if len(items) == 0 {
		m.themePicker.cursor = 0
		return
	}
	if m.themePicker.selectedName == "" {
		m.themePicker.cursor = min(max(0, m.themePicker.cursor), len(items)-1)
		m.themePicker.selectedName = canonicalThemeName(items[m.themePicker.cursor])
		return
	}
	m.themePicker.cursor = themeIndexByName(items, m.themePicker.selectedName)
	m.themePicker.selectedName = canonicalThemeName(items[m.themePicker.cursor])
}
