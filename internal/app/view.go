package app

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Ret2Hell/jiratui/internal/jira"
)

// View renders the root TUI.
func (m *Model) View() tea.View {
	m.setComponentWidths()
	var content string
	if minWidth, minHeight := m.minimumScreenSize(); m.width < minWidth || m.height < minHeight {
		content = m.renderTooSmall(minWidth, minHeight)
	} else {
		switch m.screen {
		case screenSetup:
			content = m.renderSetup()
		case screenCreate:
			content = m.renderCreateModal()
		case screenDelete:
			content = m.renderDeleteModal()
		case screenPoints:
			content = m.renderMain()
		case screenReport:
			content = m.renderReport()
		case screenHelp:
			content = m.renderKeybindingsModal()
		default:
			content = m.renderMain()
		}
	}
	content = m.zones.Scan(content)
	if m.width > 0 && m.height > 0 {
		content = strings.Join(normalizeLines(content, m.width, m.height), "\n")
	}
	view := tea.NewView(content)
	view.AltScreen = true
	if m.cfg.UI.Mouse {
		view.MouseMode = tea.MouseModeCellMotion
	}
	view.WindowTitle = "jiratui"
	return view
}

func (m *Model) minimumScreenSize() (int, int) {
	switch m.screen {
	case screenSetup:
		return 40, 12
	case screenCreate:
		return 40, 12
	case screenDelete:
		return 30, 8
	case screenReport:
		return 40, 10
	case screenHelp:
		return 30, 10
	default:
		return 20, 6
	}
}

func (m *Model) renderTooSmall(minWidth, minHeight int) string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	message := fmt.Sprintf("Terminal too small — resize to at least %d×%d", minWidth, minHeight)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, truncatePlain(message, m.width))
}

func (m *Model) renderMain() string {
	layout := calculateMainLayout(m.width, m.height, 1, m.focus, defaultLayoutOptions())
	if layout.Unusable {
		message := "Terminal too small — resize to at least 20×5"
		return strings.Join(normalizeLines(lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, truncatePlain(message, m.width)), m.width, m.height), "\n")
	}
	m.repairViewports()
	header := strings.Join(normalizeLines(m.renderHeader(), layout.Header.Width, layout.Header.Height), "\n")
	footer := strings.Join(normalizeLines(m.renderBindingFooter(), layout.Footer.Width, layout.Footer.Height), "\n")
	tickets := m.renderTicketsPanel(layout.Tickets)
	body := tickets
	if !layout.TicketsOnly {
		details := m.renderDetailsPanel(layout.Details)
		if layout.Stacked {
			body = lipgloss.JoinVertical(lipgloss.Left, tickets, details)
		} else {
			body = joinFixedHorizontal(tickets, details, layout.Tickets.Width, layout.Details.Width, layout.Tickets.Height)
		}
	}
	return strings.Join(normalizeLines(lipgloss.JoinVertical(lipgloss.Left, header, body, footer), m.width, m.height), "\n")
}

func (m *Model) renderTicketsPanel(r rect) string {
	visible := m.visibleIssues()
	page := max(0, r.Height-2)
	if m.showFilterLine() {
		page--
	}
	var position *listPosition
	if len(visible) > 0 {
		position = &listPosition{Current: m.selected, Total: len(visible)}
	}
	return m.renderPanelSpec(panelSpec{Title: "Tickets", Active: m.focus == focusTickets, Content: m.renderTickets(max(0, r.Width-4), max(0, r.Height-2)), Width: r.Width, Height: r.Height, Position: position, Scrollbar: &scrollbarState{ContentSize: len(visible), PageSize: max(0, page), Offset: m.ticketViewport.Offset}})
}

func (m *Model) renderDetailsPanel(r rect) string {
	lines := m.detailsLines(max(1, r.Width-4))
	page := max(0, r.Height-2)
	start, end := visibleRange(m.detailsViewport.Offset, len(lines), page)
	return m.renderPanelSpec(panelSpec{Title: "Details", Active: m.focus == focusDetails, Content: strings.Join(lines[start:end], "\n"), Width: r.Width, Height: r.Height, Scrollbar: &scrollbarState{ContentSize: len(lines), PageSize: page, Offset: m.detailsViewport.Offset}})
}

func (m *Model) renderHeader() string {
	width := max(10, m.width)
	project := firstNonEmpty(m.projectName, m.cfg.Jira.ProjectName, "No project")
	sprint := firstNonEmpty(m.sprint.Name, "No active sprint")
	points := m.statusPoints()
	leftParts := []string{
		m.stat("project", project, m.styles.HeaderMeta),
		m.stat("sprint", sprint, m.styles.HeaderMeta),
		m.stat("todo", pointValueString(points.todo), m.styles.Todo),
		m.stat("progress", pointValueString(points.inProgress), m.styles.WIP),
		m.stat("done", pointValueString(points.done), m.styles.Done),
	}
	left := strings.Join(leftParts, m.styles.Muted.Render("  •  "))
	right := m.renderHeaderStatus()
	return joinLeftRight(left, right, width)
}

func (m *Model) renderHeaderStatus() string {
	if m.loading {
		return m.spinner.View() + " " + m.styles.Subtitle.Render("loading")
	}
	if pending := m.pendingSyncCount(); pending > 0 {
		return m.spinner.View() + " " + m.styles.Subtitle.Render(fmt.Sprintf("syncing %d", pending))
	}
	if m.syncingSprint {
		return m.spinner.View() + " " + m.styles.Subtitle.Render("syncing Jira")
	}
	if m.refreshingReport {
		return m.spinner.View() + " " + m.styles.Subtitle.Render("refreshing report")
	}
	if m.err != nil {
		return m.styles.Error.Render(m.err.Error())
	}
	if m.status != "" {
		return m.styles.Success.Render(m.status)
	}
	return ""
}

func (m *Model) stat(label string, value string, valueStyle lipgloss.Style) string {
	return m.styles.StatLabel.Render(label+" ") + valueStyle.Render(value)
}

func (m *Model) renderTickets(width int, height int) string {
	visible := m.visibleIssues()
	if len(visible) == 0 {
		if m.loading && len(m.issues) == 0 {
			return m.loadingTicketsView(width, height)
		}
		if m.filterInput.Value() != "" {
			return m.emptyTicketsView("No tickets match filter.", width, height)
		}
		return m.emptyTicketsView("No assigned tickets in active sprint.\nPress n to create a task.", width, height)
	}
	rowsH := max(0, height)
	showFilter := m.showFilterLine()
	if showFilter {
		rowsH = max(0, height-1)
	}
	m.ticketViewport.Offset = ensureVisible(m.ticketViewport.Offset, m.selected, len(visible), rowsH)
	start, end := visibleRange(m.ticketViewport.Offset, len(visible), rowsH)
	lines := make([]string, 0, rowsH+2)
	for row := start; row < end; row++ {
		item := visible[row]
		issue := item.Issue
		cursor := " "
		if row == m.selected {
			cursor = ">"
		}
		points := pointsString(issue.StoryPoints)
		editingPoints := m.screen == screenPoints && row == m.selected
		if editingPoints {
			points = "‹" + pointsString(selectedStoryPoints(m.pointSelected)) + "›"
		}
		line := fmt.Sprintf("%s %-9s %-2s %5s  %s", cursor, issue.Key, m.statusIcon(issue), points, issue.Summary)
		line = truncatePlain(line, width)
		if editingPoints {
			line = m.styles.Selected.Render(padRight(line, width))
		} else if row == m.selected {
			line = m.styles.Selected.Render(padRight(line, width))
		} else {
			line = m.styleIssueLine(issue, line)
		}
		line = m.zones.Mark(fmt.Sprintf("%sticket-%d", m.prefix, item.Index), line)
		lines = append(lines, line)
	}
	for len(lines) < rowsH {
		lines = append(lines, "")
	}
	if showFilter {
		lines = append(lines, m.filterLine())
	}
	return strings.Join(lines, "\n")
}

func (m *Model) loadingTicketsView(width int, height int) string {
	message := m.spinner.View() + " " + m.styles.Subtitle.Render("Loading active sprint tickets…")
	return m.emptyTicketsView(message, width, height)
}

func (m *Model) emptyTicketsView(message string, width int, height int) string {
	width, height = max(0, width), max(0, height)
	if height == 0 {
		return ""
	}
	contentHeight := height
	if m.showFilterLine() {
		contentHeight--
	}
	lines := normalizeLines(message, width, max(0, contentHeight))
	if m.showFilterLine() {
		lines = append(lines, truncatePlain(m.filterLine(), width))
	}
	return strings.Join(normalizeLines(strings.Join(lines, "\n"), width, height), "\n")
}

func (m *Model) detailsLines(width int) []string {
	issue, ok := m.selectedIssue()
	if !ok {
		if m.loading && len(m.issues) == 0 {
			return []string{"Loading sprint details…"}
		}
		return []string{"Select a ticket to see details."}
	}
	points := pointsString(issue.StoryPoints)
	if m.screen == screenPoints {
		points = pointsString(selectedStoryPoints(m.pointSelected))
	}
	meta := []string{
		"",
		fmt.Sprintf("Status: %s", m.styleStatus(issue.Status).Render(issue.Status.Name)),
		fmt.Sprintf("Points: %s", points),
		fmt.Sprintf("Type:   %s", issue.IssueType.Name),
	}
	if m.screen == screenPoints {
		meta = append(meta, "", m.styles.Success.Render("Editing story points"), m.styles.Muted.Render("←/→ or ↑/↓ change · enter save · esc cancel"))
	}
	if issue.Assignee != nil {
		meta = append(meta, fmt.Sprintf("Assignee: %s", issue.Assignee.DisplayName))
	}
	summaryLines := wrapWords(issue.Summary, width)
	lines := []string{m.styles.Title.Render(issue.Key)}
	lines = append(lines, summaryLines...)
	if strings.TrimSpace(issue.Description) != "" {
		lines = append(lines, "", m.styles.Subtitle.Render("Description"))
		lines = append(lines, wrapText(issue.Description, width)...)
	}
	lines = append(lines, meta...)
	return lines
}

func (m *Model) renderSetup() string {
	footer := m.renderBindingFooter()
	bodyHeight := max(1, m.height-lipgloss.Height(footer))
	var lines []string
	lines = append(lines, m.styles.Title.Render(fmt.Sprintf("jiratui setup · step %d/2", m.setupStage+1)))
	if m.setupStage == 0 {
		lines = append(lines, m.styles.Subtitle.Render("Jira first. Enter credentials and project key; account, board, task type, and story points are auto-discovered."))
	} else {
		lines = append(lines, m.styles.Subtitle.Render("IONOS draft saving. Use the full mailbox email and mailbox password, not the IONOS control-panel password."))
	}
	if m.loading {
		lines = append(lines, m.spinner.View()+" "+m.styles.Subtitle.Render(m.status))
	} else if m.err != nil {
		for _, line := range wrapWords(m.err.Error(), max(1, m.width-4)) {
			lines = append(lines, m.styles.Error.Render(line))
		}
	} else if m.status != "" {
		lines = append(lines, m.styles.Success.Render(m.status))
	}
	lines = append(lines, "")
	for i := m.setupStageStart(); i < m.setupStageEnd(); i++ {
		label := m.setupLabels[i]
		prefix := "  "
		if i == m.setupFocus {
			prefix = "> "
		}
		lines = append(lines, fmt.Sprintf("%s%s", prefix, m.styles.InputLabel.Render(label)))
		lines = append(lines, "  "+m.setupInputs[i].View())
	}
	body := lipgloss.Place(max(1, m.width), bodyHeight, lipgloss.Left, lipgloss.Top, strings.Join(lines, "\n"), lipgloss.WithWhitespaceChars(" "))
	return lipgloss.JoinVertical(lipgloss.Left, body, footer)
}

func (m *Model) renderCreateModal() string {
	background := m.mainBackground()
	summaryRect, descriptionRect := m.createPopupRects()
	contentWidth := max(1, summaryRect.Width-4)

	summary := createSingleLineField(
		m.createSummary.Value(),
		m.createSummary.Position(),
		m.createSummary.Placeholder,
		m.createFocus == 0,
		contentWidth,
	)
	if m.createSummary.Value() == "" {
		summary = m.styles.Muted.Render(summary)
	} else {
		summary = m.styles.HeaderMeta.Render(summary)
	}

	m.createDescription.SetWidth(contentWidth)
	m.createDescription.SetHeight(max(1, descriptionRect.Height-2))
	title := "New task"
	if m.editingTaskKey != "" {
		title = "Edit " + m.editingTaskKey
	}
	summaryFooter := ""
	if m.createFocus == 0 {
		summaryFooter = "enter save  •  esc cancel"
	}
	descriptionFooter := ""
	if m.createFocus == 1 {
		descriptionFooter = "ctrl+s save  •  esc cancel"
	}
	summaryPanel := m.renderPanelSpec(panelSpec{
		Title:   title + " · Summary",
		Active:  m.createFocus == 0,
		Content: summary,
		Footer:  summaryFooter,
		Width:   summaryRect.Width,
		Height:  summaryRect.Height,
	})
	descriptionPanel := m.renderPanelSpec(panelSpec{
		Title:   "Description",
		Active:  m.createFocus == 1,
		Content: m.createDescription.View(),
		Footer:  descriptionFooter,
		Width:   descriptionRect.Width,
		Height:  descriptionRect.Height,
	})
	popup := lipgloss.JoinVertical(lipgloss.Left, summaryPanel, descriptionPanel)
	return overlayCentered(background, popup, m.width, m.height)
}

func (m *Model) renderDeleteModal() string {
	background := m.mainBackground()
	width := popupWidth(m.width, 72, 48)
	contentWidth := max(1, width-4)
	lines := []string{
		"Permanently delete " + m.styles.Title.Render(m.deletingTaskKey) + "?",
		m.styles.Muted.Render(truncatePlain(m.deletingTaskSummary, contentWidth)),
		"",
	}
	if m.loading {
		lines = append(lines, m.spinner.View()+" "+m.styles.Muted.Render("Deleting…"))
	} else if m.err != nil {
		lines = append(lines, m.styles.Error.Render(truncatePlain(m.err.Error(), contentWidth)))
	} else {
		lines = append(lines, m.styles.Muted.Render("enter confirm  •  esc cancel"))
	}
	height := popupHeight(m.height, len(lines), 7)
	return m.renderPopupPanel(background, "Delete task", strings.Join(lines, "\n"), width, height, nil)
}

func (m *Model) renderReport() string {
	background := m.mainBackground()
	width := popupWidth(m.width, 100, 80)
	height := min(max(10, m.height*3/4), max(8, m.height-2))
	contentWidth := max(1, width-4)
	editorHeight := max(1, height-6)
	m.reportEditor.SetWidth(contentWidth)
	m.reportEditor.SetHeight(editorHeight)
	actions := m.styles.Muted.Render(compactBindingLine(m.activeBindings()))
	content := strings.Join([]string{
		"",
		m.reportEditor.View(),
		"",
		actions,
	}, "\n")
	return m.renderPopupPanel(background, "Daily Report", content, width, height, nil)
}

func (m *Model) showFilterLine() bool {
	return m.filtering || strings.TrimSpace(m.filterInput.Value()) != ""
}

func (m *Model) filterLine() string {
	if m.filtering {
		return m.filterInput.View()
	}
	if value := strings.TrimSpace(m.filterInput.Value()); value != "" {
		return m.styles.Muted.Render("filter: " + value + "  (press / to edit, esc clear while editing)")
	}
	return ""
}

func (m *Model) statusIcon(issue jira.Issue) string {
	if !m.cfg.UI.Icons {
		switch issue.Status.Category.Key {
		case "done":
			return "D"
		case "indeterminate":
			return "W"
		default:
			return "T"
		}
	}
	if jira.StatusCategoryForName(issue.Status.Name) == "blocked" {
		return "!"
	}
	switch issue.Status.Category.Key {
	case "done":
		return "✓"
	case "indeterminate":
		return "…"
	default:
		return "·"
	}
}

type statusPoints struct {
	done       float64
	inProgress float64
	todo       float64
}

func (m *Model) statusPoints() statusPoints {
	return statusPoints{
		done:       m.totals.Done,
		inProgress: m.totals.InProgress + m.totals.Blocked,
		todo:       m.totals.Todo,
	}
}

func (m *Model) styleIssueLine(issue jira.Issue, line string) string {
	return m.styleStatus(issue.Status).Render(line)
}

func (m *Model) styleStatus(status jira.Status) lipgloss.Style {
	if jira.StatusCategoryForName(status.Name) == "blocked" {
		return m.styles.Blocked
	}
	switch status.Category.Key {
	case "done":
		return m.styles.Done
	case "indeterminate":
		return m.styles.WIP
	default:
		return m.styles.Todo
	}
}

func overlayCentered(background string, overlay string, width int, height int) string {
	width = max(1, width)
	height = max(1, height)
	bgLines := normalizeLines(background, width, height)
	overlayLines := slices.Collect(strings.SplitSeq(overlay, "\n"))
	overlayWidth := min(width, maxLineWidth(overlayLines))
	overlayHeight := min(height, len(overlayLines))
	x := max(0, (width-overlayWidth)/2)
	y := max(0, (height-overlayHeight)/2)
	for row := range overlayHeight {
		idx := y + row
		line := padRight(truncatePlain(overlayLines[row], overlayWidth), overlayWidth)
		base := bgLines[idx]
		left := ansi.Cut(base, 0, x)
		right := ansi.Cut(base, min(width, x+overlayWidth), width)
		bgLines[idx] = truncatePlain(padRight(left, x)+line+right, width)
	}
	return strings.Join(bgLines, "\n")
}

func normalizeLines(value string, width int, height int) []string {
	lines := slices.Collect(strings.SplitSeq(value, "\n"))
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		lines[i] = padRight(truncatePlain(lines[i], width), width)
	}
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}
	return lines
}

func joinFixedHorizontal(left string, right string, leftW int, rightW int, height int) string {
	leftLines := normalizeLines(left, leftW, height)
	rightLines := normalizeLines(right, rightW, height)
	lines := make([]string, height)
	for i := range lines {
		lines[i] = leftLines[i] + rightLines[i]
	}
	return strings.Join(lines, "\n")
}

func maxLineWidth(lines []string) int {
	width := 0
	for _, line := range lines {
		width = max(width, ansi.StringWidth(line))
	}
	return width
}

func createSingleLineField(value string, cursorPosition int, placeholder string, focused bool, width int) string {
	width = max(1, width)
	value = strings.ReplaceAll(value, "\n", " ")
	if value == "" {
		if focused && width > 2 {
			return "▌ " + truncatePlain(placeholder, width-2)
		}
		return truncatePlain(placeholder, width)
	}

	runes := []rune(value)
	cursorPosition = min(max(0, cursorPosition), len(runes))
	if focused {
		runes = slices.Insert(runes, cursorPosition, '▌')
	}
	if len(runes) <= width {
		return string(runes)
	}

	focusPosition := cursorPosition
	if !focused {
		focusPosition = 0
	}
	prefix := focusPosition > width/2
	suffix := true
	bodyWidth := width
	if prefix {
		bodyWidth--
	}
	if suffix {
		bodyWidth--
	}
	bodyWidth = max(1, bodyWidth)

	start := 0
	if focused {
		start = max(0, focusPosition-bodyWidth/2)
		if start+bodyWidth > len(runes) {
			start = max(0, len(runes)-bodyWidth)
		}
	}
	prefix = start > 0
	suffix = start+bodyWidth < len(runes)
	bodyWidth = width
	if prefix {
		bodyWidth--
	}
	if suffix {
		bodyWidth--
	}
	bodyWidth = max(1, bodyWidth)
	if start+bodyWidth > len(runes) {
		start = max(0, len(runes)-bodyWidth)
	}
	end := min(len(runes), start+bodyWidth)

	var out strings.Builder
	if start > 0 {
		out.WriteRune('…')
	}
	out.WriteString(string(runes[start:end]))
	if end < len(runes) {
		out.WriteRune('…')
	}
	return truncatePlain(out.String(), width)
}

func wrapText(value string, width int) []string {
	lines := make([]string, 0)
	for line := range strings.SplitSeq(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		lines = append(lines, wrapWords(line, width)...)
	}
	return lines
}

func wrapWords(value string, width int) []string {
	width = max(1, width)
	words := slices.Collect(strings.FieldsSeq(value))
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if ansi.StringWidth(candidate) > width {
			lines = append(lines, truncatePlain(current, width))
			current = word
			continue
		}
		current = candidate
	}
	lines = append(lines, truncatePlain(current, width))
	return lines
}

func truncatePlain(value string, width int) string {
	width = max(0, width)
	if ansi.StringWidth(value) <= width {
		return value
	}
	if width == 0 {
		return ""
	}
	if width == 1 {
		return "…"
	}
	return ansi.Truncate(value, width, "…")
}

func padRight(value string, width int) string {
	missing := width - ansi.StringWidth(value)
	if missing <= 0 {
		return value
	}
	return value + strings.Repeat(" ", missing)
}

func joinLeftRight(left string, right string, width int) string {
	if right == "" {
		return truncatePlain(left, width)
	}
	leftWidth := ansi.StringWidth(left)
	rightWidth := ansi.StringWidth(right)
	if leftWidth+rightWidth+1 > width {
		reservedRight := min(rightWidth, max(0, width/3))
		if reservedRight == 0 {
			return truncatePlain(left, width)
		}
		right = truncatePlain(right, reservedRight)
		rightWidth = ansi.StringWidth(right)
		left = truncatePlain(left, max(0, width-rightWidth-1))
		leftWidth = ansi.StringWidth(left)
	}
	gap := max(1, width-leftWidth-rightWidth)
	return left + strings.Repeat(" ", gap) + right
}
