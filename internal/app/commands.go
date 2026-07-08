package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/jira"
	"github.com/Ret2Hell/jiratui/internal/localstore"
	"github.com/Ret2Hell/jiratui/internal/mail"
	"github.com/Ret2Hell/jiratui/internal/report"
	"github.com/Ret2Hell/jiratui/internal/service"
)

func (m *Model) loadCacheCmd() tea.Cmd {
	return func() tea.Msg {
		state, ok, err := localstore.Load(m.cfg)
		return cacheLoadedMsg{State: state, OK: ok, Err: err}
	}
}

func (m *Model) saveCacheCmd() tea.Cmd {
	state := localstore.State{
		ProjectName: firstNonEmpty(m.projectName, m.cfg.Jira.ProjectName),
		Sprint:      m.sprint,
		Issues:      append([]jira.Issue(nil), m.issues...),
		Draft:       m.reportDraft,
	}
	cfg := m.cfg
	return func() tea.Msg {
		if err := localstore.Save(cfg, state); err != nil {
			return errMsg{Err: err}
		}
		return cacheSavedMsg{}
	}
}

func (m *Model) loadSprintCmd() tea.Cmd {
	return func() tea.Msg {
		if m.service == nil {
			return errMsg{Err: fmt.Errorf("service is not configured")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		data, err := m.service.LoadSprint(ctx)
		if err != nil {
			return errMsg{Err: err}
		}
		return sprintLoadedMsg{Data: data}
	}
}

func (m *Model) createTaskCmd() tea.Cmd {
	if m.service == nil {
		return func() tea.Msg { return errMsg{Err: fmt.Errorf("service is not configured")} }
	}
	summary := strings.TrimSpace(m.createSummary.Value())
	if summary == "" {
		return func() tea.Msg { return errMsg{Err: fmt.Errorf("summary is required")} }
	}
	m.err = nil
	m.tempIssueSeq++
	tempKey := fmt.Sprintf("NEW-%d", m.tempIssueSeq)
	issue := jira.Issue{
		ID:        tempKey,
		Key:       tempKey,
		Summary:   summary,
		Status:    optimisticStatus("new"),
		IssueType: jira.IssueType{ID: m.cfg.Jira.IssueTypeTaskID, Name: "Task"},
		Assignee: &jira.User{
			AccountID:   m.cfg.Jira.AccountID,
			DisplayName: firstNonEmpty(m.cfg.Jira.DisplayName, "You"),
			Email:       m.cfg.Jira.Username,
		},
		Updated: time.Now(),
	}
	m.pendingCreates[tempKey] = issue
	m.issues = append(m.issues, issue)
	m.selected = len(m.visibleIssues()) - 1
	m.screen = screenMain
	m.createSummary.SetValue("")
	m.status = tempKey + " queued"
	m.recalcTotals()
	input := service.TaskInput{Summary: summary}
	createCmd := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		created, err := m.service.CreateTask(ctx, input)
		if err != nil {
			return taskCreateFailedMsg{TempKey: tempKey, Err: err}
		}
		return taskCreatedMsg{TempKey: tempKey, Issue: created}
	}
	return tea.Batch(m.saveCacheCmd(), createCmd)
}

func (m *Model) quickMoveCmd(target string) tea.Cmd {
	issue, ok := m.selectedIssue()
	if !ok || m.service == nil {
		return nil
	}
	next := optimisticStatus(target)
	if statusEqual(issue.Status, next) || statusCategory(issue.Status) == next.Category.Key {
		m.status = issue.Key + " is already " + next.Name
		m.err = nil
		return nil
	}
	m.startStatusSync(issue.Key, issue.Status, next)
	return tea.Batch(m.saveCacheCmd(), func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		status, err := m.service.MoveToStatus(ctx, issue.Key, target)
		if err != nil {
			return issueTransitionFailedMsg{Key: issue.Key, Target: optimisticStatus(target), Err: err}
		}
		return issueTransitionedMsg{Key: issue.Key, Status: status}
	})
}

func (m *Model) updatePointsCmd() tea.Cmd {
	issue, ok := m.selectedIssue()
	if !ok || m.service == nil {
		return nil
	}
	points := selectedStoryPoints(m.pointSelected)
	m.err = nil
	if _, exists := m.pendingPointOriginals[issue.Key]; !exists {
		m.pendingPointOriginals[issue.Key] = cloneFloat(m.pointOriginal)
	}
	m.updateIssuePoints(issue.Key, points)
	m.recalcTotals()
	m.pointEditingKey = ""
	m.pointOriginal = nil
	m.screen = screenMain
	m.status = issue.Key + " points queued"
	issueKey := issue.Key
	return tea.Batch(m.saveCacheCmd(), func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := m.service.UpdateStoryPoints(ctx, issueKey, points); err != nil {
			return pointsUpdateFailedMsg{Key: issueKey, Points: points, Err: err}
		}
		return pointsUpdatedMsg{Key: issueKey, Points: points}
	})
}

func (m *Model) openReportCmd() tea.Cmd {
	if m.refreshingReport {
		m.openReportWhenReady = true
		m.status = "Refreshing report; opening when ready"
		return nil
	}
	if strings.TrimSpace(m.reportDraft.Body) == "" {
		m.refreshLocalDraft()
	}
	m.screen = screenReport
	m.reportEditor.SetValue(m.reportDraft.Body)
	m.reportEditor.Focus()
	return nil
}

func (m *Model) generateReportCmd(open bool) tea.Cmd {
	if m.service == nil {
		return func() tea.Msg { return errMsg{Err: fmt.Errorf("service is not configured")} }
	}
	issues := append([]jira.Issue(nil), m.issues...)
	version := m.reportVersion
	m.refreshingReport = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		draft, err := m.service.GenerateReport(ctx, issues)
		if err != nil {
			return errMsg{Err: err}
		}
		return reportGeneratedMsg{Draft: draft, Open: open, Version: version}
	}
}

func (m *Model) refreshLocalDraft() {
	m.reportVersion++
	issues := append([]jira.Issue(nil), m.issues...)
	loc := time.Local
	if m.cfg.Report.Timezone != "" && m.cfg.Report.Timezone != "Local" {
		if loaded, err := time.LoadLocation(m.cfg.Report.Timezone); err == nil {
			loc = loaded
		}
	}
	now := time.Now().In(loc)
	changes := make([]jira.StatusChange, 0, len(m.localStatusChanges))
	for _, change := range m.localStatusChanges {
		changes = append(changes, change)
	}
	body := report.GenerateDaily(issues, changes, report.Options{
		ProjectLabel:        m.cfg.Report.ProjectLabel,
		ProjectName:         firstNonEmpty(m.projectName, m.cfg.Jira.ProjectName),
		SprintName:          m.sprint.Name,
		DeliveryDefault:     m.cfg.Report.DeliveryDefault,
		BlockersDefault:     m.cfg.Report.BlockersDefault,
		TodoNextLimit:       m.cfg.Report.TodoNextLimit,
		OnlyMyStatusChanges: m.cfg.Report.OnlyMyStatusChanges,
		CurrentAccountID:    m.cfg.Jira.AccountID,
		IssueBaseURL:        m.cfg.Jira.BaseURL,
		Location:            loc,
		Day:                 now,
	})
	m.reportDraft = service.DailyDraft{Subject: "Daily Report", Body: body}
}

func (m *Model) saveDraftCmd() tea.Cmd {
	draft := m.reportDraft
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := m.service.SaveDraft(ctx, draft); err != nil {
			return errMsg{Err: err}
		}
		return draftSavedMsg{}
	}
}

func (m *Model) saveJiraSetupCmd() tea.Cmd {
	cfg, jiraToken, err := m.parseJiraSetup()
	if err != nil {
		return func() tea.Msg { return errMsg{Err: err} }
	}
	return func() tea.Msg {
		tokenForDiscovery := jiraToken
		if tokenForDiscovery == "" {
			var err error
			tokenForDiscovery, err = config.JiraToken()
			if err != nil {
				return errMsg{Err: fmt.Errorf("jira API token is required for setup discovery: %w", err)}
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		discovered, err := discoverJiraSetup(ctx, cfg, tokenForDiscovery)
		if err != nil {
			return errMsg{Err: err}
		}
		if jiraToken != "" {
			if err := config.SetJiraToken(jiraToken); err != nil {
				return errMsg{Err: err}
			}
		}
		if err := config.Save(m.configPath, discovered); err != nil {
			return errMsg{Err: err}
		}
		return jiraSetupSavedMsg{Config: discovered}
	}
}

func (m *Model) saveSetupCmd() tea.Cmd {
	cfg, jiraToken, mailPassword, err := m.parseSetup()
	if err != nil {
		return func() tea.Msg { return errMsg{Err: err} }
	}
	return func() tea.Msg {
		tokenForDiscovery := jiraToken
		if tokenForDiscovery == "" {
			var err error
			tokenForDiscovery, err = config.JiraToken()
			if err != nil {
				return errMsg{Err: fmt.Errorf("jira API token is required for setup discovery: %w", err)}
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		discovered, err := discoverJiraSetup(ctx, cfg, tokenForDiscovery)
		if err != nil {
			return errMsg{Err: err}
		}
		cfg = discovered
		if err := validateMailSetup(ctx, cfg, mailPassword); err != nil {
			return errMsg{Err: err}
		}
		if jiraToken != "" {
			if err := config.SetJiraToken(jiraToken); err != nil {
				return errMsg{Err: err}
			}
		}
		if mailPassword != "" {
			if err := config.SetMailPassword(mailPassword); err != nil {
				return errMsg{Err: err}
			}
		}
		if err := config.Save(m.configPath, cfg); err != nil {
			return errMsg{Err: err}
		}
		if m.factory == nil {
			return errMsg{Err: fmt.Errorf("service factory is not configured")}
		}
		svc, err := m.factory(cfg)
		if err != nil {
			return errMsg{Err: err}
		}
		return setupSavedMsg{Config: cfg, Service: svc}
	}
}

func validateMailSetup(ctx context.Context, cfg config.Config, password string) error {
	if password == "" {
		var err error
		password, err = config.MailPassword()
		if err != nil {
			return fmt.Errorf("IONOS mailbox password is required: %w", err)
		}
	}
	client := mail.Client{
		Host:          cfg.Mail.IMAPHost,
		Port:          cfg.Mail.IMAPPort,
		UseTLS:        cfg.Mail.TLS,
		Username:      firstNonEmpty(cfg.Mail.Username, cfg.Mail.From),
		Password:      password,
		DraftsMailbox: cfg.Mail.DraftsMailbox,
	}
	if err := client.CheckLogin(ctx); err != nil {
		return fmt.Errorf("validate IONOS IMAP: %w", err)
	}
	return nil
}

func discoverJiraSetup(ctx context.Context, cfg config.Config, token string) (config.Config, error) {
	client, err := jira.NewClient(cfg.Jira.BaseURL, cfg.Jira.Username, token)
	if err != nil {
		return cfg, err
	}
	user, err := client.Myself(ctx)
	if err != nil {
		return cfg, fmt.Errorf("validate Jira credentials: %w", err)
	}
	cfg.Jira.AccountID = user.AccountID
	cfg.Jira.DisplayName = user.DisplayName

	project, err := discoverProject(ctx, client, cfg.Jira.ProjectKey)
	if err != nil {
		return cfg, err
	}
	cfg.Jira.ProjectID = project.ID
	cfg.Jira.ProjectKey = project.Key
	cfg.Jira.ProjectName = project.Name
	if cfg.Report.ProjectLabel == "" && project.Name != "" {
		cfg.Report.ProjectLabel = "Project Name - " + project.Name
	}

	issueTypes, err := client.ProjectIssueTypes(ctx, project.ID)
	if err != nil || len(issueTypes) == 0 {
		issueTypes = project.IssueTypes
	}
	cfg.Jira.IssueTypeTaskID, err = discoverTaskIssueTypeID(issueTypes)
	if err != nil {
		return cfg, err
	}

	board, err := discoverBoardWithActiveSprint(ctx, client, project.Key)
	if err != nil {
		return cfg, err
	}
	cfg.Jira.BoardID = board.ID

	cfg.Jira.StoryPointsFieldID, err = discoverStoryPointsFieldID(ctx, client, board.ID)
	if err != nil {
		return cfg, err
	}
	if !cfg.IsJiraConfigured() {
		return cfg, fmt.Errorf("auto-discovery completed but Jira config is still incomplete")
	}
	return cfg, nil
}

func discoverProject(ctx context.Context, client *jira.Client, projectKey string) (jira.Project, error) {
	project, err := client.Project(ctx, projectKey)
	if err == nil {
		return project, nil
	}
	projects, searchErr := client.SearchProjects(ctx, projectKey)
	if searchErr != nil {
		return jira.Project{}, fmt.Errorf("find Jira project %q: %w", projectKey, err)
	}
	for _, candidate := range projects {
		if strings.EqualFold(candidate.Key, projectKey) || strings.EqualFold(candidate.Name, projectKey) {
			project, err := client.Project(ctx, candidate.Key)
			if err != nil {
				return candidate, nil
			}
			return project, nil
		}
	}
	return jira.Project{}, fmt.Errorf("find Jira project %q: no exact project match", projectKey)
}

func discoverTaskIssueTypeID(issueTypes []jira.IssueType) (string, error) {
	for _, issueType := range issueTypes {
		if !issueType.Subtask && strings.EqualFold(issueType.Name, "Task") {
			return issueType.ID, nil
		}
	}
	for _, issueType := range issueTypes {
		name := strings.ToLower(issueType.Name)
		if !issueType.Subtask && strings.Contains(name, "task") {
			return issueType.ID, nil
		}
	}
	return "", fmt.Errorf("could not auto-discover a Task issue type for this project")
}

func discoverBoardWithActiveSprint(ctx context.Context, client *jira.Client, projectKey string) (jira.Board, error) {
	boards, err := client.SearchBoards(ctx, projectKey)
	if err != nil {
		return jira.Board{}, fmt.Errorf("find agile board for project %s: %w", projectKey, err)
	}
	if len(boards) == 0 {
		return jira.Board{}, fmt.Errorf("no agile board found for project %s", projectKey)
	}
	for _, board := range boards {
		if _, err := client.ActiveSprint(ctx, board.ID); err == nil {
			return board, nil
		}
	}
	if len(boards) == 1 {
		return boards[0], nil
	}
	return jira.Board{}, fmt.Errorf("no active sprint found on agile boards for project %s", projectKey)
}

func discoverStoryPointsFieldID(ctx context.Context, client *jira.Client, boardID int) (string, error) {
	if boardID > 0 {
		boardConfig, err := client.BoardConfiguration(ctx, boardID)
		if err == nil && strings.TrimSpace(boardConfig.EstimationFieldID) != "" {
			return boardConfig.EstimationFieldID, nil
		}
	}
	fields, err := client.Fields(ctx)
	if err != nil {
		return "", fmt.Errorf("discover story points field: %w", err)
	}
	if fieldID := selectStoryPointsFieldID(fields); fieldID != "" {
		return fieldID, nil
	}
	return "", fmt.Errorf("could not auto-discover Jira story points field")
}

func selectStoryPointsFieldID(fields []jira.Field) string {
	preferred := []string{"story point estimate", "story points", "story point"}
	for _, expected := range preferred {
		for _, field := range fields {
			if strings.EqualFold(strings.TrimSpace(field.Name), expected) {
				return field.ID
			}
		}
	}
	for _, field := range fields {
		name := strings.ToLower(field.Name)
		if strings.Contains(name, "story") && strings.Contains(name, "point") && field.SchemaType == "number" {
			return field.ID
		}
	}
	for _, field := range fields {
		name := strings.ToLower(field.Name)
		if strings.Contains(name, "story") && strings.Contains(name, "point") {
			return field.ID
		}
	}
	return ""
}
