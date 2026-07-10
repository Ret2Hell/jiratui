package jira

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

// IssueStatusChanges returns status changes for an issue from Jira changelog.
func (c *Client) IssueStatusChanges(ctx context.Context, issue Issue) ([]StatusChange, error) {
	changes := []StatusChange{}
	startAt := 0
	for {
		params := url.Values{"startAt": {fmt.Sprint(startAt)}, "maxResults": {"100"}}
		path := fmt.Sprintf("/rest/api/3/issue/%s/changelog", url.PathEscape(issue.Key))
		var raw struct {
			StartAt    int `json:"startAt"`
			MaxResults int `json:"maxResults"`
			Total      int `json:"total"`
			Values     []struct {
				Created string `json:"created"`
				Author  struct {
					AccountID string `json:"accountId"`
				} `json:"author"`
				Items []struct {
					Field      string `json:"field"`
					FromString string `json:"fromString"`
					ToString   string `json:"toString"`
				} `json:"items"`
			} `json:"values"`
		}
		if err := c.do(ctx, http.MethodGet, path, params, nil, &raw); err != nil {
			return nil, err
		}
		for _, history := range raw.Values {
			at := parseJiraTime(history.Created)
			for _, item := range history.Items {
				if item.Field != "status" {
					continue
				}
				changes = append(changes, StatusChange{
					IssueKey:     issue.Key,
					IssueSummary: issue.Summary,
					FromStatus:   item.FromString,
					ToStatus:     item.ToString,
					ToCategory:   StatusCategoryForName(item.ToString),
					AuthorID:     history.Author.AccountID,
					At:           at,
				})
			}
		}
		startAt = raw.StartAt + raw.MaxResults
		if startAt >= raw.Total || len(raw.Values) == 0 {
			break
		}
	}
	return changes, nil
}

// StatusCategoryForName maps common Jira status names to a report bucket.
func StatusCategoryForName(status string) string {
	normalized := normalizeStatus(status)
	switch normalized {
	case "done", "closed", "resolved", "complete", "completed", "merged", "released":
		return "done"
	case "in progress", "development", "developing", "in review", "review", "code review", "qa", "testing", "in testing", "doing":
		return "indeterminate"
	case "blocked", "impediment", "stuck":
		return "blocked"
	}
	if containsAnyStatusTerm(normalized, "blocked", "impediment", "stuck") {
		return "blocked"
	}
	if containsAnyStatusTerm(normalized, "done", "complete", "resolved", "closed", "merged", "released") {
		return "done"
	}
	if containsAnyStatusTerm(normalized, "progress", "development", "review", "qa", "test", "doing") {
		return "indeterminate"
	}
	return "new"
}

func containsAnyStatusTerm(value string, terms ...string) bool {
	return slices.ContainsFunc(terms, func(term string) bool {
		return containsStatusTerm(value, term)
	})
}

func containsStatusTerm(value string, term string) bool {
	if term == "" {
		return false
	}
	return strings.Contains(value, term)
}

// IsSameLocalDay reports whether t falls on day in loc.
func IsSameLocalDay(t time.Time, day time.Time, loc *time.Location) bool {
	if t.IsZero() {
		return false
	}
	a := t.In(loc)
	b := day.In(loc)
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func normalizeStatus(status string) string {
	return stringsLowerTrim(status)
}

func stringsLowerTrim(value string) string {
	out := make([]rune, 0, len(value))
	lastSpace := false
	for _, r := range value {
		if r == '_' || r == '-' || r == '\t' || r == '\n' || r == '\r' {
			r = ' '
		}
		if r == ' ' {
			if lastSpace {
				continue
			}
			lastSpace = true
		} else {
			lastSpace = false
		}
		if r >= 'A' && r <= 'Z' {
			r += 'a' - 'A'
		}
		out = append(out, r)
	}
	for len(out) > 0 && out[0] == ' ' {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == ' ' {
		out = out[:len(out)-1]
	}
	return string(out)
}
