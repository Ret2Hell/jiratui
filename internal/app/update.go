package app

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/jira"
)

// Update handles Bubble Tea messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.setComponentWidths()
	case cacheLoadedMsg:
		if msg.Err != nil {
			m.status = msg.Err.Error()
			break
		}
		if msg.OK && len(m.issues) == 0 {
			m.projectName = msg.State.ProjectName
			m.sprint = msg.State.Sprint
			m.issues = msg.State.Issues
			m.reportDraft = msg.State.Draft
			if strings.TrimSpace(m.reportDraft.Body) == "" {
				m.refreshLocalDraft()
			}
			m.loading = false
			m.status = fmt.Sprintf("Loaded %d cached tickets", len(m.issues))
			m.recalcTotals()
		}
	case cacheSavedMsg:
	case sprintLoadedMsg:
		m.loading = false
		m.syncingSprint = false
		m.err = nil
		m.projectName = msg.Data.ProjectName
		m.sprint = msg.Data.Sprint
		m.mergeSprintData(msg.Data.Issues)
		m.selected = min(m.selected, max(0, len(m.visibleIssues())-1))
		m.status = fmt.Sprintf("Synced %d tickets", len(m.issues))
		m.recalcTotals()
		m.refreshLocalDraft()
		cmds = append(cmds, m.saveCacheCmd(), m.generateReportCmd(false))
	case taskCreatedMsg:
		m.err = nil
		delete(m.pendingCreates, msg.TempKey)
		m.replaceIssue(msg.TempKey, msg.Issue)
		m.status = "Created " + msg.Issue.Key
		m.recalcTotals()
		cmds = append(cmds, m.saveCacheCmd())
	case taskCreateFailedMsg:
		delete(m.pendingCreates, msg.TempKey)
		m.removeIssue(msg.TempKey)
		m.err = msg.Err
		m.status = "Create failed: " + msg.Err.Error()
		m.recalcTotals()
		cmds = append(cmds, m.saveCacheCmd())
	case issueTransitionedMsg:
		if current, ok := m.issueByKey(msg.Key); !ok || statusEqual(current.Status, msg.Status) {
			delete(m.pendingStatusOriginal, msg.Key)
			m.updateIssueStatus(msg.Key, msg.Status)
			m.status = msg.Key + " → " + msg.Status.Name
		}
		m.screen = screenMain
		m.recalcTotals()
		m.refreshLocalDraft()
		cmds = append(cmds, m.saveCacheCmd(), m.generateReportCmd(false))
	case issueTransitionFailedMsg:
		if current, ok := m.issueByKey(msg.Key); ok && statusEqual(current.Status, msg.Target) {
			if original, ok := m.pendingStatusOriginal[msg.Key]; ok {
				m.updateIssueStatus(msg.Key, original)
				delete(m.pendingStatusOriginal, msg.Key)
			}
			delete(m.localStatusChanges, msg.Key)
		}
		m.err = msg.Err
		m.status = msg.Key + " status failed: " + msg.Err.Error()
		m.recalcTotals()
		m.refreshLocalDraft()
		cmds = append(cmds, m.saveCacheCmd())
	case pointsUpdatedMsg:
		if current, ok := m.issueByKey(msg.Key); !ok || pointsEqual(current.StoryPoints, msg.Points) {
			delete(m.pendingPointOriginals, msg.Key)
			m.updateIssuePoints(msg.Key, msg.Points)
			m.status = msg.Key + " points synced"
		}
		m.screen = screenMain
		m.recalcTotals()
		cmds = append(cmds, m.saveCacheCmd())
	case pointsUpdateFailedMsg:
		if current, ok := m.issueByKey(msg.Key); ok && pointsEqual(current.StoryPoints, msg.Points) {
			if original, ok := m.pendingPointOriginals[msg.Key]; ok {
				m.updateIssuePoints(msg.Key, cloneFloat(original))
				delete(m.pendingPointOriginals, msg.Key)
			}
		}
		m.err = msg.Err
		m.status = msg.Key + " points failed: " + msg.Err.Error()
		m.recalcTotals()
		cmds = append(cmds, m.saveCacheCmd())
	case reportGeneratedMsg:
		m.refreshingReport = false
		if msg.Version != m.reportVersion {
			break
		}
		m.reportDraft = msg.Draft
		openReport := msg.Open || m.openReportWhenReady
		m.openReportWhenReady = false
		if openReport {
			m.screen = screenReport
			m.reportEditor.SetValue(msg.Draft.Body)
			m.reportEditor.Focus()
		} else if m.screen != screenReport {
			m.status = "Report ready"
		}
		cmds = append(cmds, m.saveCacheCmd())
	case draftSavedMsg:
		m.loading = false
		m.screen = screenMain
		m.status = "Saved report to mail drafts"
	case jiraSetupSavedMsg:
		m.loading = false
		m.err = nil
		m.cfg = msg.Config
		m.setupStage = 1
		m.status = "Jira setup saved. Complete IONOS setup."
		m.focusSetup(m.setupStageStart())
	case setupSavedMsg:
		m.loading = true
		m.syncingSprint = true
		m.cfg = msg.Config
		m.service = msg.Service
		m.screen = screenMain
		m.status = "Setup saved"
		cmds = append(cmds, m.loadCacheCmd(), m.loadSprintCmd())
	case errMsg:
		m.refreshingReport = false
		m.openReportWhenReady = false
		m.syncingSprint = false
		m.loading = false
		m.err = msg.Err
		m.status = msg.Err.Error()
	}

	if key, ok := msg.(tea.KeyPressMsg); ok {
		if key.Keystroke() == "ctrl+c" {
			return m, tea.Quit
		}
		switch m.screen {
		case screenSetup:
			cmds = append(cmds, m.updateSetup(key))
		case screenMain:
			cmds = append(cmds, m.updateMain(key))
		case screenCreate:
			cmds = append(cmds, m.updateCreate(key, msg))
		case screenPoints:
			cmds = append(cmds, m.updatePoints(key, msg))
		case screenReport:
			cmds = append(cmds, m.updateReport(key, msg))
		case screenHelp:
			if key.Keystroke() == "esc" || key.String() == "q" || key.String() == "?" {
				m.screen = screenMain
			}
		}
	}

	if _, ok := msg.(tea.PasteMsg); ok {
		cmds = append(cmds, m.updatePaste(msg))
	}

	if mouse, ok := msg.(tea.MouseClickMsg); ok && m.screen == screenMain {
		m.updateMouse(mouse)
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) updatePaste(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch m.screen {
	case screenSetup:
		m.setupInputs[m.setupFocus], cmd = m.setupInputs[m.setupFocus].Update(msg)
	case screenMain:
		if m.filtering {
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.selected = 0
			m.recalcTotals()
			m.refreshLocalDraft()
		}
	case screenCreate:
		m.createSummary, cmd = m.createSummary.Update(msg)
	case screenPoints:
		// Story points are selected from fixed Fibonacci chips; paste is ignored.
	case screenReport:
		m.reportEditor, cmd = m.reportEditor.Update(msg)
	}
	return cmd
}

func (m *Model) updateSetup(key tea.KeyPressMsg) tea.Cmd {
	switch key.Keystroke() {
	case "tab":
		m.focusSetup(m.setupFocus + 1)
		return nil
	case "enter":
		if m.setupStage == 0 {
			m.loading = true
			m.err = nil
			m.status = "Discovering Jira setup"
			return m.saveJiraSetupCmd()
		}
		m.loading = true
		m.err = nil
		m.status = "Discovering Jira setup"
		return m.saveSetupCmd()
	case "shift+tab":
		if m.setupFocus == m.setupStageStart() && m.setupStage > 0 {
			m.setupStage--
			m.focusSetup(m.setupStageEnd() - 1)
			return nil
		}
		m.focusSetup(m.setupFocus - 1)
		return nil
	case "q":
		return tea.Quit
	}
	var cmd tea.Cmd
	m.setupInputs[m.setupFocus], cmd = m.setupInputs[m.setupFocus].Update(key)
	return cmd
}

func (m *Model) updateMain(key tea.KeyPressMsg) tea.Cmd {
	if key.String() == "R" || key.String() == "M" {
		return m.openReportCmd()
	}
	if key.String() == "?" {
		m.screen = screenHelp
		return nil
	}
	if m.filtering {
		switch key.Keystroke() {
		case "esc":
			m.filtering = false
			m.filterInput.SetValue("")
			m.filterInput.Blur()
			m.recalcTotals()
			m.refreshLocalDraft()
			return nil
		case "enter":
			m.filtering = false
			m.filterInput.Blur()
			return nil
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(key)
		m.selected = 0
		m.recalcTotals()
		m.refreshLocalDraft()
		return cmd
	}

	switch key.Keystroke() {
	case "q":
		return tea.Quit
	case "?":
		m.screen = screenHelp
	case "r":
		m.syncingSprint = true
		m.loading = len(m.issues) == 0
		return m.loadSprintCmd()
	case "up", "k":
		m.moveSelection(-1)
	case "down", "j":
		m.moveSelection(1)
	case "tab":
		if m.focus == focusTickets {
			m.focus = focusDetails
		} else {
			m.focus = focusTickets
		}
	case "/":
		m.filtering = true
		m.filterInput.Focus()
	case "n":
		m.openCreate()
	case "enter":
		m.openPoints()
	case "i", "p":
		return m.quickMoveCmd("indeterminate")
	case "t":
		return m.quickMoveCmd("new")
	case "d", "x":
		return m.quickMoveCmd("done")
	case "m", "shift+r":
		return m.openReportCmd()
	}
	return nil
}

func (m *Model) updateCreate(key tea.KeyPressMsg, msg tea.Msg) tea.Cmd {
	switch key.Keystroke() {
	case "esc":
		m.screen = screenMain
		return nil
	case "enter":
		return m.createTaskCmd()
	case "tab":
		m.focusCreate(m.createFocus + 1)
		return nil
	case "shift+tab":
		m.focusCreate(m.createFocus - 1)
		return nil
	}
	var cmd tea.Cmd
	m.createSummary, cmd = m.createSummary.Update(msg)
	return cmd
}

func (m *Model) updatePoints(key tea.KeyPressMsg, _ tea.Msg) tea.Cmd {
	switch key.Keystroke() {
	case "esc":
		m.cancelPointEdit()
		m.screen = screenMain
		return nil
	case "enter":
		return m.updatePointsCmd()
	case "up", "left", "k", "h":
		m.movePointSelection(-1)
	case "down", "right", "j", "l":
		m.movePointSelection(1)
	case "0", "1", "2", "3", "4", "5", "6":
		m.pointSelected = int(key.String()[0] - '0')
		m.applyPointSelection()
	}
	return nil
}

func (m *Model) updateReport(key tea.KeyPressMsg, msg tea.Msg) tea.Cmd {
	switch key.Keystroke() {
	case "esc":
		m.screen = screenMain
		return nil
	case "ctrl+s":
		m.reportDraft.Body = m.reportEditor.Value()
		m.loading = true
		return tea.Batch(m.saveCacheCmd(), m.saveDraftCmd())
	}
	var cmd tea.Cmd
	m.reportEditor, cmd = m.reportEditor.Update(msg)
	return cmd
}

func (m *Model) updateMouse(msg tea.MouseClickMsg) {
	visible := m.visibleIssues()
	for row, item := range visible {
		z := m.zones.Get(fmt.Sprintf("%sticket-%d", m.prefix, item.Index))
		if z != nil && z.InBounds(msg) {
			m.selected = row
			return
		}
	}
}

func (m *Model) focusSetup(next int) {
	m.setupInputs[m.setupFocus].Blur()
	start := m.setupStageStart()
	end := m.setupStageEnd()
	if next < start {
		next = end - 1
	}
	if next >= end {
		next = start
	}
	m.setupFocus = next
	m.setupInputs[m.setupFocus].Focus()
}

func (m *Model) setupStageStart() int {
	if m.setupStage == 0 {
		return 0
	}
	return 4
}

func (m *Model) setupStageEnd() int {
	if m.setupStage == 0 {
		return 4
	}
	return len(m.setupInputs)
}

func (m *Model) focusCreate(_ int) {
	m.createFocus = 0
	m.createSummary.Focus()
}

func (m *Model) openCreate() {
	m.createSummary.SetValue("")
	m.createFocus = 0
	m.focusCreate(0)
	m.screen = screenCreate
}

func (m *Model) openPoints() {
	issue, ok := m.selectedIssue()
	if !ok {
		return
	}
	m.pointSelected = pointIndex(issue.StoryPoints)
	m.pointEditingKey = issue.Key
	m.pointOriginal = cloneFloat(issue.StoryPoints)
	m.status = "Choose story points for " + issue.Key
	m.screen = screenPoints
}

func (m *Model) movePointSelection(delta int) {
	m.pointSelected += delta
	if m.pointSelected < 0 {
		m.pointSelected = len(storyPointValues()) - 1
	}
	if m.pointSelected >= len(storyPointValues()) {
		m.pointSelected = 0
	}
	m.applyPointSelection()
}

func (m *Model) applyPointSelection() {
	if m.pointEditingKey == "" {
		return
	}
	m.updateIssuePoints(m.pointEditingKey, selectedStoryPoints(m.pointSelected))
	m.recalcTotals()
}

func (m *Model) cancelPointEdit() {
	if m.pointEditingKey == "" {
		return
	}
	m.updateIssuePoints(m.pointEditingKey, cloneFloat(m.pointOriginal))
	m.recalcTotals()
	m.pointEditingKey = ""
	m.pointOriginal = nil
}

func (m *Model) mergeSprintData(remote []jira.Issue) {
	localByKey := make(map[string]jira.Issue, len(m.issues))
	for _, issue := range m.issues {
		localByKey[issue.Key] = issue
	}
	merged := make([]jira.Issue, 0, len(remote)+len(m.pendingCreates))
	for _, issue := range m.issues {
		if _, pending := m.pendingCreates[issue.Key]; pending {
			merged = append(merged, issue)
		}
	}
	for _, issue := range remote {
		if local, ok := localByKey[issue.Key]; ok {
			if _, pending := m.pendingStatusOriginal[issue.Key]; pending {
				issue.Status = local.Status
			}
			if _, pending := m.pendingPointOriginals[issue.Key]; pending {
				issue.StoryPoints = cloneFloat(local.StoryPoints)
			}
		}
		merged = append(merged, issue)
	}
	m.issues = merged
}

func (m *Model) replaceIssue(oldKey string, next jira.Issue) {
	i := slices.IndexFunc(m.issues, func(issue jira.Issue) bool { return issue.Key == oldKey })
	if i < 0 {
		m.issues = append(m.issues, next)
		return
	}
	m.issues[i] = next
}

func (m *Model) removeIssue(key string) {
	i := slices.IndexFunc(m.issues, func(issue jira.Issue) bool { return issue.Key == key })
	if i < 0 {
		return
	}
	m.issues = slices.Delete(m.issues, i, i+1)
	if m.selected >= len(m.visibleIssues()) {
		m.selected = max(0, len(m.visibleIssues())-1)
	}
}

func (m *Model) startStatusSync(key string, original jira.Status, next jira.Status) {
	m.err = nil
	if _, exists := m.pendingStatusOriginal[key]; !exists {
		m.pendingStatusOriginal[key] = original
	}
	m.trackLocalStatusChange(key, next)
	m.screen = screenMain
	m.status = key + " → " + next.Name + " queued"
	m.updateIssueStatus(key, next)
	m.recalcTotals()
	m.refreshLocalDraft()
}

func (m *Model) trackLocalStatusChange(key string, next jira.Status) {
	category := next.Category.Key
	if category == "" {
		category = jira.StatusCategoryForName(next.Name)
	}
	if category != "done" && category != "indeterminate" {
		delete(m.localStatusChanges, key)
		return
	}
	issue, _ := m.issueByKey(key)
	m.localStatusChanges[key] = jira.StatusChange{
		IssueKey:     key,
		IssueSummary: issue.Summary,
		ToStatus:     next.Name,
		ToCategory:   category,
		AuthorID:     m.cfg.Jira.AccountID,
		At:           time.Now(),
	}
}

func statusCategory(status jira.Status) string {
	if status.Category.Key != "" {
		return status.Category.Key
	}
	return jira.StatusCategoryForName(status.Name)
}

func optimisticStatus(target string) jira.Status {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "done":
		return jira.Status{Name: "Done", Category: jira.StatusCategory{Key: "done", Name: "Done"}}
	case "indeterminate", "progress", "in progress":
		return jira.Status{Name: "In Progress", Category: jira.StatusCategory{Key: "indeterminate", Name: "In Progress"}}
	default:
		return jira.Status{Name: "To Do", Category: jira.StatusCategory{Key: "new", Name: "To Do"}}
	}
}

func (m *Model) issueByKey(key string) (jira.Issue, bool) {
	i := slices.IndexFunc(m.issues, func(issue jira.Issue) bool { return issue.Key == key })
	if i < 0 {
		return jira.Issue{}, false
	}
	return m.issues[i], true
}

func statusEqual(a jira.Status, b jira.Status) bool {
	if a.ID != "" && b.ID != "" && a.ID == b.ID {
		return true
	}
	return strings.EqualFold(a.Name, b.Name) && a.Category.Key == b.Category.Key
}

func pointsEqual(a *float64, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (m *Model) updateIssueStatus(key string, status jira.Status) {
	for i := range m.issues {
		if m.issues[i].Key == key {
			m.issues[i].Status = status
			m.issues[i].Updated = time.Now()
			return
		}
	}
}

func (m *Model) updateIssuePoints(key string, points *float64) {
	for i := range m.issues {
		if m.issues[i].Key == key {
			m.issues[i].StoryPoints = points
			m.issues[i].Updated = time.Now()
			return
		}
	}
}

func (m *Model) parseJiraSetup() (config.Config, string, error) {
	cfg := m.cfg
	cfg.Jira.BaseURL = strings.TrimSpace(m.setupInputs[0].Value())
	cfg.Jira.Username = strings.TrimSpace(m.setupInputs[1].Value())
	jiraToken := strings.TrimSpace(m.setupInputs[2].Value())
	cfg.Jira.ProjectKey = strings.TrimSpace(m.setupInputs[3].Value())
	if cfg.Jira.BaseURL == "" || cfg.Jira.Username == "" || cfg.Jira.ProjectKey == "" {
		return cfg, "", fmt.Errorf("jira base URL, email, and project key are required")
	}
	return cfg, jiraToken, nil
}

func (m *Model) parseSetup() (config.Config, string, string, error) {
	cfg, jiraToken, err := m.parseJiraSetup()
	if err != nil {
		return cfg, "", "", err
	}
	cfg.Mail.IMAPHost = "imap.ionos.de"
	mailAddress := strings.TrimSpace(m.setupInputs[4].Value())
	cfg.Mail.From = mailAddress
	cfg.Mail.Username = mailAddress
	mailPassword := strings.TrimSpace(m.setupInputs[5].Value())
	cfg.Mail.To = splitCSV(m.setupInputs[6].Value())
	if cfg.Mail.From == "" || len(cfg.Mail.To) == 0 {
		return cfg, "", "", fmt.Errorf("mail sender and at least one report recipient are required")
	}
	return cfg, jiraToken, mailPassword, nil
}

func splitCSV(value string) []string {
	var out []string
	for part := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
