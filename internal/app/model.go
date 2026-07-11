// Package app implements the Bubble Tea TUI.
package app

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/harmonica"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/jira"
	"github.com/Ret2Hell/jiratui/internal/report"
	"github.com/Ret2Hell/jiratui/internal/service"
)

type screen int

const (
	screenSetup screen = iota
	screenMain
	screenCreate
	screenPoints
	screenReport
	screenHelp
)

type focusArea int

const (
	focusTickets focusArea = iota
	focusDetails
)

type indexedIssue struct {
	Index int
	Issue jira.Issue
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg        config.Config
	configPath string
	factory    service.Factory
	service    service.Service

	width  int
	height int

	screen screen
	focus  focusArea

	styles styles
	zones  *zone.Manager
	prefix string

	spinner             spinner.Model
	loading             bool
	syncingSprint       bool
	refreshingReport    bool
	openReportWhenReady bool
	status              string
	err                 error

	projectName         string
	sprint              jira.Sprint
	issues              []jira.Issue
	selected            int
	ticketViewport      listViewport
	detailsViewport     listViewport
	keybindingsViewport listViewport
	modalParent         screen
	tempIssueSeq        int
	totals              report.PointTotals

	filtering   bool
	filterInput textinput.Model

	setupLabels []string
	setupInputs []textinput.Model
	setupFocus  int
	setupStage  int

	createSummary       textinput.Model
	createFocus         int
	editingTaskKey      string
	editingTaskOriginal string

	pointSelected         int
	pointEditingKey       string
	pointOriginal         *float64
	pendingPointOriginals map[string]*float64
	pendingStatusOriginal map[string]jira.Status
	pendingCreates        map[string]jira.Issue
	localStatusChanges    map[string]jira.StatusChange

	reportEditor  textarea.Model
	reportDraft   service.DailyDraft
	reportVersion int

	selectionSpring harmonica.Spring
	selectionPos    float64
	selectionVel    float64
}

// New creates the root TUI model.
func New(cfg config.Config, configPath string, svc service.Service, factory service.Factory, initialStatus string, forceSetup bool) *Model {
	m := &Model{
		cfg:                   cfg,
		configPath:            configPath,
		factory:               factory,
		service:               svc,
		styles:                newStyles(),
		zones:                 zone.New(),
		spinner:               spinner.New(spinner.WithSpinner(spinner.Dot)),
		status:                initialStatus,
		pendingPointOriginals: make(map[string]*float64),
		pendingStatusOriginal: make(map[string]jira.Status),
		pendingCreates:        make(map[string]jira.Issue),
		localStatusChanges:    make(map[string]jira.StatusChange),
		selectionSpring:       harmonica.NewSpring(harmonica.FPS(60), 10, 0.8),
	}
	m.prefix = m.zones.NewPrefix()
	m.initInputs()
	if forceSetup || svc == nil || !cfg.IsConfigured() {
		m.screen = screenSetup
		if !forceSetup && cfg.IsJiraConfigured() {
			m.setupStage = 1
			m.status = firstNonEmpty(initialStatus, "Jira setup already saved. Complete IONOS setup.")
			m.focusSetup(m.setupStageStart())
		}
	} else {
		m.screen = screenMain
		m.loading = true
		m.syncingSprint = true
	}
	return m
}

// Init starts the first load command when configuration is complete.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{func() tea.Msg { return m.spinner.Tick() }}
	if m.screen == screenMain && m.service != nil {
		cmds = append(cmds, m.loadCacheCmd(), m.loadSprintCmd())
	}
	return tea.Batch(cmds...)
}

func (m *Model) initInputs() {
	m.filterInput = textinput.New()
	m.filterInput.Placeholder = "filter tickets"
	m.filterInput.Prompt = "/ "

	m.setupLabels = []string{
		"Jira base URL",
		"Jira email",
		"Jira API token",
		"Project key",
		"IONOS mailbox email",
		"IONOS mailbox password",
		"Report recipients (comma-separated)",
	}
	values := []string{
		m.cfg.Jira.BaseURL,
		m.cfg.Jira.Username,
		"",
		m.cfg.Jira.ProjectKey,
		firstNonEmpty(m.cfg.Mail.From, m.cfg.Mail.Username),
		"",
		strings.Join(m.cfg.Mail.To, ", "),
	}
	m.setupInputs = make([]textinput.Model, len(m.setupLabels))
	for i := range m.setupInputs {
		input := textinput.New()
		input.Placeholder = m.setupLabels[i]
		input.SetValue(values[i])
		if i == 2 || i == 5 {
			input.EchoMode = textinput.EchoPassword
			input.EchoCharacter = '•'
		}
		if i == 0 {
			input.Focus()
		}
		m.setupInputs[i] = input
	}

	m.createSummary = textinput.New()
	m.createSummary.Placeholder = "What needs to be done?"
	m.createSummary.Focus()

	m.reportEditor = textarea.New()
	m.reportEditor.ShowLineNumbers = false
}

func (m *Model) setComponentWidths() {
	contentWidth := max(1, m.width-4)
	setupWidth := max(12, min(70, contentWidth-24))
	for i := range m.setupInputs {
		m.setupInputs[i].SetWidth(setupWidth)
	}
	m.filterInput.SetWidth(max(1, contentWidth-4))
	createWidth := min(max(1, m.width), popupWidth(m.width, 78, 56))
	createInputWidth := max(1, createWidth-10)
	m.createSummary.SetWidth(createInputWidth)
	m.reportEditor.SetWidth(max(1, contentWidth-8))
	m.reportEditor.SetHeight(max(1, m.height-8))
}

func (m *Model) selectedIssue() (jira.Issue, bool) {
	visible := m.visibleIssues()
	if len(visible) == 0 {
		return jira.Issue{}, false
	}
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(visible) {
		m.selected = len(visible) - 1
	}
	return visible[m.selected].Issue, true
}

func (m *Model) visibleIssues() []indexedIssue {
	filter := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))
	out := make([]indexedIssue, 0, len(m.issues))
	for i, issue := range m.issues {
		if filter == "" || strings.Contains(strings.ToLower(issue.Key+" "+issue.Summary+" "+issue.Status.Name), filter) {
			out = append(out, indexedIssue{Index: i, Issue: issue})
		}
	}
	return out
}

func (m *Model) recalcTotals() {
	visible := m.visibleIssues()
	issues := make([]jira.Issue, 0, len(visible))
	for _, item := range visible {
		issues = append(issues, item.Issue)
	}
	m.totals = report.Totals(issues)
}

func (m *Model) moveSelection(delta int) {
	visible := m.visibleIssues()
	if len(visible) == 0 {
		m.selected = 0
		m.ticketViewport.Offset = 0
		return
	}
	m.selected += delta
	if m.selected < 0 {
		m.selected = 0
	}
	if m.selected >= len(visible) {
		m.selected = len(visible) - 1
	}
	if m.cfg.UI.Animations {
		m.selectionPos, m.selectionVel = m.selectionSpring.Update(m.selectionPos, m.selectionVel, float64(m.selected))
	}
	m.detailsViewport.Offset = 0
	m.repairViewports()
}

func (m *Model) ticketPageSize() int {
	l := calculateMainLayout(m.width, m.height, 1, m.focus, defaultLayoutOptions())
	h := max(0, l.Tickets.Height-2)
	if m.showFilterLine() {
		h--
	}
	return max(0, h)
}

func (m *Model) detailsPanelMetrics() (int, int) {
	l := calculateMainLayout(m.width, m.height, 1, m.focus, defaultLayoutOptions())
	if l.Unusable || l.TicketsOnly {
		return 1, 0
	}
	return max(1, l.Details.Width-4), max(0, l.Details.Height-2)
}

func (m *Model) detailsPageSize() int {
	_, pageSize := m.detailsPanelMetrics()
	return pageSize
}

func (m *Model) repairViewports() {
	visible := m.visibleIssues()
	if len(visible) == 0 {
		m.selected, m.ticketViewport.Offset = 0, 0
	} else {
		m.selected = min(max(0, m.selected), len(visible)-1)
		m.ticketViewport.Offset = ensureVisible(m.ticketViewport.Offset, m.selected, len(visible), m.ticketPageSize())
	}
	detailsWidth, detailsPageSize := m.detailsPanelMetrics()
	m.detailsViewport.Offset = clampOffset(m.detailsViewport.Offset, len(m.detailsLines(detailsWidth)), detailsPageSize)
}

func (m *Model) selectedIssueKey() string {
	if issue, ok := m.selectedIssue(); ok {
		return issue.Key
	}
	return ""
}

func (m *Model) restoreSelection(key string) {
	visible := m.visibleIssues()
	if key != "" {
		for i, item := range visible {
			if item.Issue.Key == key {
				m.selected = i
				m.repairViewports()
				return
			}
		}
	}
	m.repairViewports()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func storyPointValues() []float64 {
	return []float64{0.5, 1, 2, 3, 5, 8, 13}
}

func selectedStoryPoints(index int) *float64 {
	values := storyPointValues()
	index = min(max(0, index), len(values)-1)
	return new(values[index])
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	return new(*value)
}

func (m *Model) pendingSyncCount() int {
	return len(m.pendingCreates) + len(m.pendingStatusOriginal) + len(m.pendingPointOriginals)
}

func pointIndex(points *float64) int {
	if points == nil {
		return 0
	}
	values := storyPointValues()
	closest := 0
	closestDistance := absFloat(values[0] - *points)
	for i, value := range values[1:] {
		distance := absFloat(value - *points)
		if distance < closestDistance {
			closest = i + 1
			closestDistance = distance
		}
	}
	return closest
}

func pointsString(points *float64) string {
	if points == nil {
		return "-"
	}
	return pointValueString(*points)
}

func pointValueString(points float64) string {
	if points == float64(int64(points)) {
		return fmt.Sprintf("%.0f", points)
	}
	return fmt.Sprintf("%.1f", points)
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
