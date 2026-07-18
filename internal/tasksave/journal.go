// Package tasksave defines durable, resumable Jira task-save state.
package tasksave

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/Ret2Hell/jiratui/internal/jira"
)

const (
	// CurrentVersion is the journal schema version written by this release.
	CurrentVersion = 1
	// RetryWindow is how long a partial save can be retried with its local image data.
	RetryWindow = 24 * time.Hour
)

// Kind identifies whether a journal creates a new Jira task or updates one.
type Kind string

const (
	// KindCreate identifies a save that creates a Jira task.
	KindCreate Kind = "create"
	// KindUpdate identifies a save that updates an existing Jira task.
	KindUpdate Kind = "update"
)

// Draft is the user's intended Jira task content.
type Draft struct {
	Summary          string
	Description      jira.Description
	StoryPoints      *float64
	WriteSummary     bool
	WriteDescription bool
}

// Journal records every Jira step accepted during one task save.
type Journal struct {
	Version        int
	ID             string
	Kind           Kind
	TempKey        string
	IssueKey       string
	Issue          jira.Issue
	Draft          Draft
	AssigneeID     string
	SprintID       int
	IssueCreated   bool
	AddedToSprint  bool
	ContentUpdated bool
	CreatedAt      time.Time
	LastAcceptedAt time.Time
}

// NewCreate creates a journal before any Jira write is attempted.
func NewCreate(tempKey string, issue jira.Issue, draft Draft, now time.Time) (Journal, error) {
	id, err := newID()
	if err != nil {
		return Journal{}, err
	}
	draft.Summary = strings.TrimSpace(draft.Summary)
	draft.WriteSummary = true
	draft.WriteDescription = true
	return Journal{
		Version:   CurrentVersion,
		ID:        id,
		Kind:      KindCreate,
		TempKey:   tempKey,
		Issue:     issue,
		Draft:     draft,
		CreatedAt: now,
	}, nil
}

// NewUpdate creates a journal before any Jira write is attempted.
func NewUpdate(issue jira.Issue, draft Draft, now time.Time) (Journal, error) {
	id, err := newID()
	if err != nil {
		return Journal{}, err
	}
	draft.Summary = strings.TrimSpace(draft.Summary)
	return Journal{
		Version:       CurrentVersion,
		ID:            id,
		Kind:          KindUpdate,
		IssueKey:      issue.Key,
		Issue:         issue,
		Draft:         draft,
		IssueCreated:  true,
		AddedToSprint: true,
		CreatedAt:     now,
	}, nil
}

// Complete reports whether every required Jira write has been accepted.
func (j Journal) Complete() bool {
	return j.IssueCreated && j.AddedToSprint && j.ContentUpdated
}

// Validate checks whether journal metadata can be safely resumed.
func (j Journal) Validate() error {
	if j.Version != CurrentVersion {
		return fmt.Errorf("unsupported task-save version %d", j.Version)
	}
	if j.ID == "" {
		return fmt.Errorf("task-save id is required")
	}
	if j.Kind != KindCreate && j.Kind != KindUpdate {
		return fmt.Errorf("invalid task-save kind %q", j.Kind)
	}
	if j.Kind == KindCreate && j.TempKey == "" {
		return fmt.Errorf("create task save has no temporary key")
	}
	if j.Kind == KindUpdate && j.IssueKey == "" {
		return fmt.Errorf("update task save has no Jira key")
	}
	return nil
}

// Expired reports whether Jira accepted no progress recently enough for a direct retry.
func (j Journal) Expired(now time.Time) bool {
	base := j.LastAcceptedAt
	if base.IsZero() {
		base = j.CreatedAt
	}
	return !base.IsZero() && now.Sub(base) >= RetryWindow
}

// Projection returns the issue content the user intended, including partial saves.
func (j Journal) Projection() jira.Issue {
	issue := j.Issue
	if j.IssueKey != "" {
		issue.Key = j.IssueKey
	}
	if j.Draft.WriteSummary {
		issue.Summary = j.Draft.Summary
	}
	if j.Draft.WriteDescription {
		issue.Description = j.Draft.Description.EditorText
		issue.DescriptionContent = j.Draft.Description.WithoutImageData()
	}
	if j.Draft.StoryPoints != nil {
		issue.StoryPoints = j.Draft.StoryPoints
	}
	return issue
}

func newID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create task-save id: %w", err)
	}
	return "save-" + hex.EncodeToString(value[:]), nil
}
