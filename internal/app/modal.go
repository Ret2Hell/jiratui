package app

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func popupWidth(screenWidth, maxWidth, minimumWidth int) int {
	available := max(8, screenWidth-2)
	width := min(4*max(0, screenWidth)/7, maxWidth)
	if width < minimumWidth {
		width = min(available, minimumWidth)
	}
	return min(max(8, width), available)
}

func popupHeight(screenHeight, contentHeight, minimumHeight int) int {
	available := max(4, screenHeight-2)
	height := contentHeight + 2
	height = min(height, max(4, screenHeight*3/4))
	height = max(minimumHeight, height)
	return min(height, available)
}

func (m *Model) renderPopupPanel(background, title, content string, width, height int, scrollbar *scrollbarState) string {
	panel := m.renderPanelSpec(panelSpec{
		Title:     title,
		Active:    true,
		Content:   content,
		Width:     width,
		Height:    height,
		Scrollbar: scrollbar,
	})
	return overlayCentered(background, panel, m.width, m.height)
}

func (m *Model) modalBackground() string {
	background := *m
	background.screen = m.modalParent
	switch m.modalParent {
	case screenSetup:
		return background.renderSetup()
	case screenCreate:
		return background.renderCreateModal()
	case screenPoints:
		return background.renderMain()
	case screenReport:
		return background.renderReport()
	default:
		return background.renderMain()
	}
}

func (m *Model) renderKeybindingsModal() string {
	background := m.modalBackground()
	width := popupWidth(m.width, 90, 60)
	contentWidth := max(1, width-4)
	allLines := m.keybindingLines(contentWidth)
	_, pageSize := m.keybindingsModalMetrics()
	height := pageSize + 3 // Frame plus one navigation hint row.
	m.keybindingsViewport.Offset = clampOffset(m.keybindingsViewport.Offset, len(allLines), pageSize)
	start, end := visibleRange(m.keybindingsViewport.Offset, len(allLines), pageSize)
	lines := append([]string(nil), allLines[start:end]...)
	for len(lines) < pageSize {
		lines = append(lines, "")
	}
	hint := m.styles.Muted.Render("↑/↓ scroll  •  esc close")
	lines = append(lines, truncatePlain(hint, contentWidth))
	return m.renderPopupPanel(
		background,
		"Keybindings",
		strings.Join(lines, "\n"),
		width,
		height,
		&scrollbarState{ContentSize: len(allLines), PageSize: pageSize, Offset: m.keybindingsViewport.Offset},
	)
}

func (m *Model) keybindingsModalMetrics() (int, int) {
	lineCount := len(m.keybindingLines(max(1, popupWidth(m.width, 90, 60)-4)))
	height := popupHeight(m.height, lineCount+1, 10)
	return lineCount, max(1, height-3)
}

func (m *Model) keybindingLines(width int) []string {
	groups := allBindingGroups()
	keyWidth := 0
	for _, group := range groups {
		for _, item := range group.Bindings {
			keyWidth = max(keyWidth, lipgloss.Width(bindingKeyList(item)))
		}
	}
	keyWidth = min(keyWidth, max(8, width/3))

	lines := make([]string, 0, 40)
	for groupIndex, group := range groups {
		if groupIndex > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, m.styles.ModalSection.Render("--- "+group.Title+" ---"))
		for _, item := range group.Bindings {
			keys := m.styles.FooterKey.Render(padRight(truncatePlain(bindingKeyList(item), keyWidth), keyWidth))
			description := m.styles.Footer.Render(item.Description)
			lines = append(lines, truncatePlain(keys+"  "+description, width))
		}
	}
	return lines
}
