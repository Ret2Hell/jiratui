// Package service coordinates Jira, report, and mail operations for the TUI.
package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/jira"
	"github.com/Ret2Hell/jiratui/internal/mail"
	"github.com/Ret2Hell/jiratui/internal/report"
	"github.com/Ret2Hell/jiratui/internal/tasksave"
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

// Service defines the app workflow operations.
type Service interface {
	LoadSprint(context.Context) (SprintData, error)
	SaveTask(context.Context, tasksave.Journal, func(tasksave.Journal) error) (tasksave.Journal, error)
	AttachmentMeta(context.Context) (jira.AttachmentMeta, error)
	DeleteTask(context.Context, string) error
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
	mu         sync.RWMutex
}

// LoadSprint loads current active sprint issues assigned to the user.
func (s *JiraService) LoadSprint(ctx context.Context) (SprintData, error) {
	sprint, err := s.jira.ActiveSprint(ctx, s.cfg.Jira.BoardID)
	if err != nil {
		return SprintData{}, err
	}
	s.mu.Lock()
	s.lastSprint = sprint
	s.mu.Unlock()
	issues, err := s.jira.SearchMySprintIssues(ctx, s.cfg.Jira.ProjectKey, sprint.ID, s.cfg.Jira.StoryPointsFieldID)
	if err != nil {
		return SprintData{}, err
	}
	return SprintData{ProjectName: s.cfg.Jira.ProjectName, Sprint: sprint, Issues: issues}, nil
}

// SaveTask resumes a journal and persists each accepted Jira step before continuing.
func (s *JiraService) SaveTask(ctx context.Context, journal tasksave.Journal, persist func(tasksave.Journal) error) (tasksave.Journal, error) {
	if strings.TrimSpace(journal.Draft.Summary) == "" {
		return journal, fmt.Errorf("summary is required")
	}
	if journal.Draft.WriteDescription {
		description, err := jira.ParseDescriptionEditor(journal.Draft.Description.EditorText, journal.Draft.Description)
		if err != nil {
			return journal, err
		}
		journal.Draft.Description = description
	}
	checkpoint := func(accepted bool) error {
		if accepted {
			journal.LastAcceptedAt = time.Now()
		}
		if persist == nil {
			return nil
		}
		return persist(journal)
	}

	if journal.Kind == tasksave.KindCreate && !journal.IssueCreated {
		assignee := jira.User{
			AccountID:   journal.AssigneeID,
			DisplayName: s.cfg.Jira.DisplayName,
			Email:       s.cfg.Jira.Username,
		}
		if assignee.AccountID == "" {
			var err error
			assignee, err = s.currentUser(ctx)
			if err != nil {
				return journal, err
			}
			journal.AssigneeID = assignee.AccountID
		}
		if journal.SprintID == 0 {
			sprint, err := s.jira.ActiveSprint(ctx, s.cfg.Jira.BoardID)
			if err != nil {
				return journal, err
			}
			journal.SprintID = sprint.ID
		}
		if err := checkpoint(false); err != nil {
			return journal, fmt.Errorf("persist task save before Jira create: %w", err)
		}
		created, found, err := s.jira.FindTaskBySaveID(ctx, s.cfg.Jira.ProjectKey, journal.ID, journal.CreatedAt.Add(-5*time.Minute))
		if err != nil {
			return journal, fmt.Errorf("reconcile Jira task create: %w", err)
		}
		if !found {
			created, err = s.jira.CreateTask(ctx, jira.CreateTaskInput{
				ProjectKey:      s.cfg.Jira.ProjectKey,
				IssueTypeID:     s.cfg.Jira.IssueTypeTaskID,
				AssigneeID:      assignee.AccountID,
				Summary:         journal.Draft.Summary,
				Description:     journal.Draft.Description,
				StoryPoints:     journal.Draft.StoryPoints,
				StoryPointsID:   s.cfg.Jira.StoryPointsFieldID,
				AdditionalField: s.cfg.Jira.CreateDefaults,
				SaveID:          journal.ID,
			})
			if err != nil {
				return journal, err
			}
		}
		issue := journal.Projection()
		issue.ID = created.ID
		issue.Key = created.Key
		issue.Assignee = &assignee
		issue.IssueType = jira.IssueType{ID: s.cfg.Jira.IssueTypeTaskID, Name: "Task"}
		issue.Status = jira.Status{Name: "To Do", Category: jira.StatusCategory{Key: "new", Name: "To Do"}}
		journal.Issue = issue
		journal.IssueKey = issue.Key
		journal.IssueCreated = true
		journal.ContentUpdated = len(journal.Draft.Description.ReferencedPendingImages()) == 0
		if err := checkpoint(true); err != nil {
			return journal, fmt.Errorf("created %s but could not checkpoint it: %w", issue.Key, err)
		}
	}

	if !journal.IssueCreated || journal.IssueKey == "" {
		return journal, fmt.Errorf("task save has no Jira issue")
	}
	if journal.Kind == tasksave.KindCreate && !journal.AddedToSprint {
		if err := s.jira.AddIssuesToSprint(ctx, journal.SprintID, []string{journal.IssueKey}); err != nil {
			return journal, fmt.Errorf("created %s but could not add it to sprint: %w", journal.IssueKey, err)
		}
		journal.AddedToSprint = true
		if err := checkpoint(true); err != nil {
			return journal, fmt.Errorf("added %s to sprint but could not checkpoint it: %w", journal.IssueKey, err)
		}
	}

	if journal.Draft.WriteDescription {
		for _, image := range journal.Draft.Description.ReferencedPendingImages() {
			originalFilename := image.Filename
			filename := attachmentFilename(image)
			uploaded, found, err := s.jira.FindAttachmentByFilename(ctx, journal.IssueKey, filename, image)
			if err != nil {
				return journal, fmt.Errorf("reconcile description image %q: %w", originalFilename, err)
			}
			if !found {
				image.Filename = filename
				uploaded, err = s.jira.AddAttachment(ctx, journal.IssueKey, image)
				if err != nil {
					return journal, fmt.Errorf("upload description image %q: %w", originalFilename, err)
				}
			}
			description, err := journal.Draft.Description.WithUploadedImage(uploaded)
			if err != nil {
				return journal, err
			}
			journal.Draft.Description = description
			journal.Issue = journal.Projection()
			if err := checkpoint(true); err != nil {
				return journal, fmt.Errorf("uploaded description image %q but could not checkpoint it: %w", image.Filename, err)
			}
		}
	}

	if !journal.ContentUpdated {
		var summary *string
		var description *jira.Description
		if journal.Kind == tasksave.KindUpdate && journal.Draft.WriteSummary {
			summary = &journal.Draft.Summary
		}
		if journal.Draft.WriteDescription {
			description = &journal.Draft.Description
		}
		if err := s.jira.UpdateTaskFields(ctx, journal.IssueKey, summary, description); err != nil {
			return journal, fmt.Errorf("update %s content: %w", journal.IssueKey, err)
		}
		journal.ContentUpdated = true
		journal.Issue = journal.Projection()
		if err := checkpoint(true); err != nil {
			journal.ContentUpdated = false
			return journal, fmt.Errorf("updated %s but could not checkpoint it: %w", journal.IssueKey, err)
		}
	}
	return journal, nil
}

func attachmentFilename(image jira.DescriptionImage) string {
	name := strings.TrimSpace(image.Filename)
	if name == "" {
		name = "image"
	}
	return "jiratui-" + image.ID + "-" + name
}

// AttachmentMeta returns Jira's attachment availability and tenant size limit.
func (s *JiraService) AttachmentMeta(ctx context.Context) (jira.AttachmentMeta, error) {
	return s.jira.AttachmentMeta(ctx)
}

// DeleteTask permanently deletes an issue.
func (s *JiraService) DeleteTask(ctx context.Context, issueKey string) error {
	return s.jira.DeleteTask(ctx, issueKey)
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
	s.mu.RLock()
	sprintName := s.lastSprint.Name
	s.mu.RUnlock()
	body := report.GenerateDaily(issues, changes, report.Options{
		ProjectLabel:        s.cfg.Report.ProjectLabel,
		ProjectName:         s.cfg.Jira.ProjectName,
		SprintName:          sprintName,
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
