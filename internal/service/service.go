// Package service coordinates Jira, report, and mail operations for the TUI.
package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/jira"
	"github.com/Ret2Hell/jiratui/internal/mail"
	"github.com/Ret2Hell/jiratui/internal/report"
)

// SprintData is the data needed by the main screen.
type SprintData struct {
	ProjectName string
	Sprint      jira.Sprint
	Issues      []jira.Issue
}

// DailyDraft is a generated report and its target email metadata.
type DailyDraft struct {
	Subject string
	Body    string
}

// TaskInput contains TUI task creation fields.
type TaskInput struct {
	Summary     string
	StoryPoints *float64
}

// Service defines the app workflow operations.
type Service interface {
	LoadSprint(context.Context) (SprintData, error)
	CreateTask(context.Context, TaskInput) (jira.Issue, error)
	UpdateTaskSummary(context.Context, string, string) error
	Transitions(context.Context, string) ([]jira.Transition, error)
	TransitionIssue(context.Context, string, string) error
	MoveToStatus(context.Context, string, string) (jira.Status, error)
	UpdateStoryPoints(context.Context, string, *float64) error
	GenerateReport(context.Context, []jira.Issue) (DailyDraft, error)
	SaveDraft(context.Context, DailyDraft) error
}

// Factory creates a Service from config.
type Factory func(config.Config) (Service, error)

// NewJiraService creates a production service using Jira Cloud and IONOS IMAP.
func NewJiraService(cfg config.Config) (Service, error) {
	token, err := config.JiraToken()
	if err != nil {
		return nil, fmt.Errorf("jira token: %w", err)
	}
	client, err := jira.NewClient(cfg.Jira.BaseURL, cfg.Jira.Username, token)
	if err != nil {
		return nil, err
	}
	mailPassword, _ := config.MailPassword()
	mailClient := mail.Client{
		Host:          cfg.Mail.IMAPHost,
		Port:          cfg.Mail.IMAPPort,
		UseTLS:        cfg.Mail.TLS,
		Username:      firstNonEmpty(cfg.Mail.Username, cfg.Mail.From),
		Password:      mailPassword,
		DraftsMailbox: cfg.Mail.DraftsMailbox,
	}
	return &JiraService{cfg: cfg, jira: client, mail: mailClient}, nil
}

// JiraService implements Service with real Jira and IMAP clients.
type JiraService struct {
	cfg        config.Config
	jira       *jira.Client
	mail       mail.Client
	lastSprint jira.Sprint
}

// LoadSprint loads current active sprint issues assigned to the user.
func (s *JiraService) LoadSprint(ctx context.Context) (SprintData, error) {
	sprint, err := s.jira.ActiveSprint(ctx, s.cfg.Jira.BoardID)
	if err != nil {
		return SprintData{}, err
	}
	s.lastSprint = sprint
	issues, err := s.jira.SearchMySprintIssues(ctx, s.cfg.Jira.ProjectKey, sprint.ID, s.cfg.Jira.StoryPointsFieldID)
	if err != nil {
		return SprintData{}, err
	}
	return SprintData{ProjectName: s.cfg.Jira.ProjectName, Sprint: sprint, Issues: issues}, nil
}

// CreateTask creates a task assigned to the current Jira account and adds it to the active sprint.
func (s *JiraService) CreateTask(ctx context.Context, input TaskInput) (jira.Issue, error) {
	if strings.TrimSpace(input.Summary) == "" {
		return jira.Issue{}, fmt.Errorf("summary is required")
	}
	assignee, err := s.currentUser(ctx)
	if err != nil {
		return jira.Issue{}, err
	}
	sprint, err := s.jira.ActiveSprint(ctx, s.cfg.Jira.BoardID)
	if err != nil {
		return jira.Issue{}, err
	}
	issue, err := s.jira.CreateTask(ctx, jira.CreateTaskInput{
		ProjectKey:      s.cfg.Jira.ProjectKey,
		IssueTypeID:     s.cfg.Jira.IssueTypeTaskID,
		AssigneeID:      assignee.AccountID,
		Summary:         strings.TrimSpace(input.Summary),
		Description:     "",
		StoryPoints:     input.StoryPoints,
		StoryPointsID:   s.cfg.Jira.StoryPointsFieldID,
		AdditionalField: s.cfg.Jira.CreateDefaults,
	})
	if err != nil {
		return jira.Issue{}, err
	}
	if err := s.jira.AssignIssue(ctx, issue.Key, assignee.AccountID); err != nil {
		return issue, fmt.Errorf("created %s but could not assign it to you: %w", issue.Key, err)
	}
	issue.Assignee = &assignee
	issue.IssueType = jira.IssueType{ID: s.cfg.Jira.IssueTypeTaskID, Name: "Task"}
	issue.Status = jira.Status{Name: "To Do", Category: jira.StatusCategory{Key: "new", Name: "To Do"}}
	if err := s.jira.AddIssuesToSprint(ctx, sprint.ID, []string{issue.Key}); err != nil {
		return issue, fmt.Errorf("created %s but could not add it to sprint: %w", issue.Key, err)
	}
	return issue, nil
}

// UpdateTaskSummary updates an issue's summary.
func (s *JiraService) UpdateTaskSummary(ctx context.Context, issueKey, summary string) error {
	if strings.TrimSpace(summary) == "" {
		return fmt.Errorf("summary is required")
	}
	return s.jira.UpdateSummary(ctx, issueKey, strings.TrimSpace(summary))
}

// Transitions returns valid issue transitions.
func (s *JiraService) Transitions(ctx context.Context, issueKey string) ([]jira.Transition, error) {
	return s.jira.Transitions(ctx, issueKey)
}

// TransitionIssue applies a transition id.
func (s *JiraService) TransitionIssue(ctx context.Context, issueKey, transitionID string) error {
	return s.jira.TransitionIssue(ctx, issueKey, transitionID)
}

// MoveToStatus applies the first transition matching a target bucket such as done or indeterminate.
func (s *JiraService) MoveToStatus(ctx context.Context, issueKey, target string) (jira.Status, error) {
	transitions, err := s.jira.Transitions(ctx, issueKey)
	if err != nil {
		return jira.Status{}, err
	}
	i := slices.IndexFunc(transitions, func(transition jira.Transition) bool {
		return statusMatches(transition.ToStatus, target)
	})
	if i < 0 {
		return jira.Status{}, fmt.Errorf("no transition to %s available for %s", target, issueKey)
	}
	transition := transitions[i]
	return transition.ToStatus, s.jira.TransitionIssue(ctx, issueKey, transition.ID)
}

// UpdateStoryPoints updates the configured story-points field.
func (s *JiraService) UpdateStoryPoints(ctx context.Context, issueKey string, points *float64) error {
	return s.jira.UpdateStoryPoints(ctx, issueKey, s.cfg.Jira.StoryPointsFieldID, points)
}

// GenerateReport generates a daily report from visible sprint issues and changelogs.
func (s *JiraService) GenerateReport(ctx context.Context, issues []jira.Issue) (DailyDraft, error) {
	loc := time.Local
	if s.cfg.Report.Timezone != "" && s.cfg.Report.Timezone != "Local" {
		if loaded, err := time.LoadLocation(s.cfg.Report.Timezone); err == nil {
			loc = loaded
		}
	}
	changes := []jira.StatusChange{}
	for _, issue := range issues {
		issueChanges, err := s.jira.IssueStatusChanges(ctx, issue)
		if err != nil {
			continue
		}
		changes = append(changes, issueChanges...)
	}
	now := time.Now().In(loc)
	body := report.GenerateDaily(issues, changes, report.Options{
		ProjectLabel:        s.cfg.Report.ProjectLabel,
		ProjectName:         s.cfg.Jira.ProjectName,
		SprintName:          s.lastSprint.Name,
		DeliveryDefault:     s.cfg.Report.DeliveryDefault,
		BlockersDefault:     s.cfg.Report.BlockersDefault,
		TodoNextLimit:       s.cfg.Report.TodoNextLimit,
		OnlyMyStatusChanges: s.cfg.Report.OnlyMyStatusChanges,
		CurrentAccountID:    s.cfg.Jira.AccountID,
		IssueBaseURL:        s.cfg.Jira.BaseURL,
		Location:            loc,
		Day:                 now,
	})
	return DailyDraft{Subject: "Daily Report", Body: body}, nil
}

// SaveDraft appends the report as a draft email.
func (s *JiraService) SaveDraft(ctx context.Context, draft DailyDraft) error {
	return s.mail.SaveDraft(ctx, mail.Draft{
		From:    s.cfg.Mail.From,
		To:      s.cfg.Mail.To,
		CC:      s.cfg.Mail.CC,
		Subject: draft.Subject,
		Body:    draft.Body,
		Date:    time.Now(),
	})
}

func statusMatches(status jira.Status, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	if status.Category.Key == target {
		return true
	}
	return jira.StatusCategoryForName(status.Name) == target || strings.EqualFold(status.Name, target)
}

func (s *JiraService) currentUser(ctx context.Context) (jira.User, error) {
	if strings.TrimSpace(s.cfg.Jira.AccountID) != "" {
		return jira.User{
			AccountID:   strings.TrimSpace(s.cfg.Jira.AccountID),
			DisplayName: s.cfg.Jira.DisplayName,
			Email:       s.cfg.Jira.Username,
		}, nil
	}
	user, err := s.jira.Myself(ctx)
	if err != nil {
		return jira.User{}, fmt.Errorf("resolve current Jira user for assignment: %w", err)
	}
	if strings.TrimSpace(user.AccountID) == "" {
		return jira.User{}, fmt.Errorf("resolve current Jira user for assignment: missing account id")
	}
	s.cfg.Jira.AccountID = user.AccountID
	s.cfg.Jira.DisplayName = user.DisplayName
	return user, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
