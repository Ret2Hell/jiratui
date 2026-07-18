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
	ID               string
	Name             string
	UntranslatedName string
	Subtask          bool
}

// Issue is the minimal issue shape used by jiratui.
type Issue struct {
	ID                 string
	Key                string
	Summary            string
	Description        string
	DescriptionContent Description
	Status             Status
	IssueType          IssueType
	Assignee           *User
	StoryPoints        *float64
	Updated            time.Time
}

// Description is the editable projection and preservation state of Jira ADF.
type Description struct {
	EditorText        string
	Editable          bool
	UnsupportedReason string
	Images            []DescriptionImage
	References        []DescriptionImageReference
	RawADF            []byte
}

// DescriptionImage is one image in a task draft, independent of its references.
type DescriptionImage struct {
	ID           string
	AttachmentID string
	Filename     string
	MIMEType     string
	Data         []byte
	URL          string
	MediaID      string
	Collection   string
	Width        int
	Height       int
}

// DescriptionImageReference is one block-level occurrence of a description image.
type DescriptionImageReference struct {
	ImageID      string
	Alt          string
	Presentation []byte
}

// AttachmentMeta contains Jira's site-wide attachment limits.
type AttachmentMeta struct {
	Enabled     bool
	UploadLimit int
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
	Description     Description
	StoryPoints     *float64
	StoryPointsID   string
	AdditionalField map[string]any
	SaveID          string
}
