// Package jira contains a small Jira Cloud API client tailored to jiratui's workflow.
package jira

import "time"

// User is a Jira user.
type User struct {
	AccountID   string
	DisplayName string
	Email       string
}

// Project is a Jira project visible to the authenticated user.
type Project struct {
	ID         string
	Key        string
	Name       string
	IssueTypes []IssueType
}

// Board is a Jira agile board.
type Board struct {
	ID   int
	Name string
	Type string
}

// Sprint is a Jira agile sprint.
type Sprint struct {
	ID        int
	Name      string
	State     string
	StartDate time.Time
	EndDate   time.Time
}

// StatusCategory is Jira's status category.
type StatusCategory struct {
	Key  string
	Name string
}

// Status is a Jira issue status.
type Status struct {
	ID       string
	Name     string
	Category StatusCategory
}

// IssueType is a Jira issue type.
type IssueType struct {
	ID      string
	Name    string
	Subtask bool
}

// Issue is the minimal issue shape used by jiratui.
type Issue struct {
	ID          string
	Key         string
	Summary     string
	Status      Status
	IssueType   IssueType
	Assignee    *User
	StoryPoints *float64
	Updated     time.Time
}

// Transition is a valid Jira workflow transition for an issue.
type Transition struct {
	ID       string
	Name     string
	ToStatus Status
}

// Field is a Jira field exposed by the REST API.
type Field struct {
	ID         string
	Name       string
	Custom     bool
	SchemaType string
}

// BoardConfiguration contains agile board metadata needed by setup discovery.
type BoardConfiguration struct {
	EstimationFieldID   string
	EstimationFieldName string
}

// StatusChange is a status transition extracted from an issue changelog.
type StatusChange struct {
	IssueKey     string
	IssueSummary string
	FromStatus   string
	ToStatus     string
	ToCategory   string
	AuthorID     string
	At           time.Time
}

// CreateTaskInput is the payload for creating a Task in the active sprint.
type CreateTaskInput struct {
	ProjectKey      string
	IssueTypeID     string
	AssigneeID      string
	Summary         string
	Description     string
	StoryPoints     *float64
	StoryPointsID   string
	AdditionalField map[string]any
}
