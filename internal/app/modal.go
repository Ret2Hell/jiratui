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

func (m *Model) createPopupRects() (rect, rect) {
	width := popupWidth(m.width, 100, 80)
	totalHeight := min(max(10, m.height/2), max(10, m.height-2))
	summaryHeight := 3
	descriptionHeight := max(7, totalHeight-summaryHeight)
	totalHeight = summaryHeight + descriptionHeight
	x := max(0, (m.width-width)/2)
	y := max(0, (m.height-totalHeight)/2)
	return rect{X: x, Y: y, Width: width, Height: summaryHeight}, rect{X: x, Y: y + summaryHeight, Width: width, Height: descriptionHeight}
}

func (m *Model) mainBackground() string {
	background := *m
	background.screen = screenMain
	return background.renderMain()
}

func (m *Model) modalBackground() string {
	background := *m
	background.screen = m.modalParent
	switch m.modalParent {
	case screenSetup:
		return background.renderSetup()
	case screenCreate:
		return background.renderCreateModal()
	case screenDelete:
		return background.renderDeleteModal()
	case screenPoints:
		return background.renderMain()
	case screenReport:
		return background.renderReport()
	default:
		return background.renderMain()
	}
}

type keybindingMenuRow struct {
	Line       string
	Binding    binding
	Selectable bool
}

func (m *Model) renderKeybindingsModal() string {
	background := m.modalBackground()
	width := popupWidth(m.width, 90, 60)
	contentWidth := max(1, width-4)
	rows := m.keybindingMenuRows(contentWidth)
	lineCount, pageSize := m.keybindingsModalMetrics()
	m.keybindingsSelected = min(max(0, m.keybindingsSelected), max(0, m.selectableKeybindingCount()-1))
	m.ensureSelectedKeybindingVisible(rows, pageSize)
	m.keybindingsViewport.Offset = clampOffset(m.keybindingsViewport.Offset, lineCount, pageSize)
	start, end := visibleRange(m.keybindingsViewport.Offset, lineCount, pageSize)
	filterRows := 0
	lines := make([]string, 0, pageSize+1)
	if m.keybindingsFiltering || m.keybindingsFilter != "" {
		filterRows = 1
		lines = append(lines, m.styles.FooterKey.Render("/ ")+m.keybindingsFilter+"▌")
	}
	for _, row := range rows[start:end] {
		lines = append(lines, row.Line)
	}
	for len(lines) < pageSize+filterRows {
		lines = append(lines, "")
	}
	footer := "/ filter  •  esc close"
	panel := m.renderPanelSpec(panelSpec{
		Title:     "Keybindings",
		Active:    true,
		Content:   strings.Join(lines, "\n"),
		Footer:    footer,
		Width:     width,
		Height:    pageSize + filterRows + 2,
		Scrollbar: &scrollbarState{ContentSize: lineCount, PageSize: pageSize, Offset: m.keybindingsViewport.Offset},
	})
	return overlayCentered(background, panel, m.width, m.height)
}

func (m *Model) keybindingsModalMetrics() (int, int) {
	lineCount := len(m.keybindingMenuRows(max(1, popupWidth(m.width, 90, 60)-4)))
	height := popupHeight(m.height, lineCount, 10)
	pageSize := max(1, height-2)
	if m.keybindingsFiltering || m.keybindingsFilter != "" {
		pageSize--
	}
	return lineCount, max(1, pageSize)
}

func (m *Model) keybindingMenuGroups() []bindingGroup {
	main := mainBindings()
	application := bindingsForCommands(main, cmdHelp, cmdQuit)
	if m.modalParent == screenMain {
		return []bindingGroup{
			{Title: "Tasks", Bindings: bindingsForCommands(main, cmdNew, cmdEdit, cmdDelete, cmdPoints)},
			{Title: "Workflow", Bindings: bindingsForCommands(main, cmdTodo, cmdProgress, cmdDone)},
			{Title: "View", Bindings: bindingsForCommands(main, cmdReport, cmdFilter, cmdRefresh)},
			{Title: "Navigation", Bindings: bindingsForCommands(main, cmdUp, cmdDown, cmdPageUp, cmdPageDown, cmdHome, cmdEnd, cmdFocus)},
			{Title: "Application", Bindings: application},
		}
	}
	var title string
	var contextual []binding
	switch m.modalParent {
	case screenCreate:
		title = "Task form"
		contextual = createBindings()
		if m.createFocus == 1 {
			contextual[0].Keys = []string{"ctrl+s"}
		}
	case screenDelete:
		title, contextual = "Delete confirmation", deleteBindings()
	case screenPoints:
		title, contextual = "Story points", pointsBindings()
	case screenReport:
		title, contextual = "Daily report", reportBindings()
	case screenSetup:
		title, contextual = "Setup", setupBindings("continue or save setup")
	default:
		title = "Actions"
	}
	return []bindingGroup{{Title: title, Bindings: contextual}, {Title: "Application", Bindings: application}}
}

func (m *Model) keybindingMenuRows(width int) []keybindingMenuRow {
	groups := m.keybindingMenuGroups()
	query := strings.ToLower(strings.TrimSpace(m.keybindingsFilter))
	keyWidth := 0
	for _, group := range groups {
		for _, item := range group.Bindings {
			keyWidth = max(keyWidth, lipgloss.Width(bindingKeyList(item)))
		}
	}
	keyWidth = min(keyWidth, max(8, width/3))
	selected := 0
	rows := make([]keybindingMenuRow, 0, 32)
	for _, group := range groups {
		items := make([]binding, 0, len(group.Bindings))
		for _, item := range group.Bindings {
			haystack := strings.ToLower(group.Title + " " + bindingKeyList(item) + " " + item.Description)
			if query == "" || strings.Contains(haystack, query) {
				items = append(items, item)
			}
		}
		if len(items) == 0 {
			continue
		}
		if len(rows) > 0 {
			rows = append(rows, keybindingMenuRow{})
		}
		rows = append(rows, keybindingMenuRow{Line: m.styles.ModalSection.Render(group.Title)})
		for _, item := range items {
			keyLabel := truncatePlain(bindingKeyList(item), keyWidth)
			paddedKey := strings.Repeat(" ", max(0, keyWidth-ansiWidth(keyLabel))) + keyLabel
			line := m.styles.FooterKey.Render(paddedKey) + "  " + m.styles.Footer.Render(item.Description)
			if selected == m.keybindingsSelected {
				line = m.styles.Selected.Width(width).Render(padRight(paddedKey+"  "+item.Description, width))
			}
			rows = append(rows, keybindingMenuRow{Line: truncatePlain(line, width), Binding: item, Selectable: true})
			selected++
		}
	}
	if len(rows) == 0 {
		rows = append(rows, keybindingMenuRow{Line: m.styles.Muted.Render("No matching keybindings")})
	}
	return rows
}

func (m *Model) keybindingLines(width int) []string {
	rows := m.keybindingMenuRows(width)
	lines := make([]string, len(rows))
	for i, row := range rows {
		lines[i] = row.Line
	}
	return lines
}

func (m *Model) selectableKeybindingCount() int {
	count := 0
	for _, row := range m.keybindingMenuRows(max(1, popupWidth(m.width, 90, 60)-4)) {
		if row.Selectable {
			count++
		}
	}
	return count
}

func (m *Model) selectedKeybinding() (binding, bool) {
	selected := 0
	for _, row := range m.keybindingMenuRows(max(1, popupWidth(m.width, 90, 60)-4)) {
		if !row.Selectable {
			continue
		}
		if selected == m.keybindingsSelected {
			return row.Binding, true
		}
		selected++
	}
	return binding{}, false
}

func (m *Model) ensureSelectedKeybindingVisible(rows []keybindingMenuRow, pageSize int) {
	selected, line := 0, 0
	for i, row := range rows {
		if row.Selectable {
			if selected == m.keybindingsSelected {
				line = i
				break
			}
			selected++
		}
	}
	if line < m.keybindingsViewport.Offset {
		m.keybindingsViewport.Offset = line
	} else if line >= m.keybindingsViewport.Offset+pageSize {
		m.keybindingsViewport.Offset = line - pageSize + 1
	}
}
