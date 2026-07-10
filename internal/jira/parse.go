package jira

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type sprintJSON struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type statusJSON struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	StatusCategory struct {
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"statusCategory"`
}

type issueJSON struct {
	ID     string         `json:"id"`
	Key    string         `json:"key"`
	Fields map[string]any `json:"fields"`
}

type issueTypeJSON struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	UntranslatedName string `json:"untranslatedName"`
	Subtask          bool   `json:"subtask"`
}

func (raw issueTypeJSON) issueType() IssueType {
	return IssueType(raw)
}

func parseSprint(raw sprintJSON) Sprint {
	return Sprint{
		ID:        raw.ID,
		Name:      raw.Name,
		State:     raw.State,
		StartDate: parseJiraTime(raw.StartDate),
		EndDate:   parseJiraTime(raw.EndDate),
	}
}

func parseIssue(raw issueJSON, storyPointsFieldID string) Issue {
	fields := raw.Fields
	issue := Issue{ID: raw.ID, Key: raw.Key}
	issue.Summary, _ = fields["summary"].(string)
	issue.Status = parseStatusFromAny(fields["status"])
	issue.IssueType = parseIssueType(fields["issuetype"])
	issue.Assignee = parseUserPtr(fields["assignee"])
	issue.Updated = parseJiraTime(asString(fields["updated"]))
	if storyPointsFieldID != "" {
		issue.StoryPoints = parseFloatPtr(fields[storyPointsFieldID])
	}
	return issue
}

func parseStatus(raw statusJSON) Status {
	return Status{
		ID:   raw.ID,
		Name: raw.Name,
		Category: StatusCategory{
			Key:  strings.ToLower(raw.StatusCategory.Key),
			Name: raw.StatusCategory.Name,
		},
	}
}

func parseStatusFromAny(value any) Status {
	var raw statusJSON
	if remarshal(value, &raw) != nil {
		return Status{}
	}
	return parseStatus(raw)
}

func parseIssueType(value any) IssueType {
	var raw issueTypeJSON
	if remarshal(value, &raw) != nil {
		return IssueType{}
	}
	return raw.issueType()
}

func parseUserPtr(value any) *User {
	if value == nil {
		return nil
	}
	var raw struct {
		AccountID    string `json:"accountId"`
		DisplayName  string `json:"displayName"`
		EmailAddress string `json:"emailAddress"`
	}
	if remarshal(value, &raw) != nil || raw.AccountID == "" {
		return nil
	}
	return &User{AccountID: raw.AccountID, DisplayName: raw.DisplayName, Email: raw.EmailAddress}
}

func parseFloatPtr(value any) *float64 {
	switch v := value.(type) {
	case nil:
		return nil
	case float64:
		return new(v)
	case int:
		return new(float64(v))
	case json.Number:
		f, err := v.Float64()
		if err == nil {
			return new(f)
		}
	case string:
		if v == "" {
			return nil
		}
		f, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return new(f)
		}
	}
	return nil
}

func parseJiraTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	layouts := []string{time.RFC3339Nano, "2006-01-02T15:04:05.000-0700", "2006-01-02T15:04:05.000Z"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

func remarshal(input any, out any) error {
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
