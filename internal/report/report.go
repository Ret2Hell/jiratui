// Package report generates daily report text from Jira issues and changelogs.
package report

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Ret2Hell/jiratui/internal/jira"
)

// Options controls daily report generation.
type Options struct {
	ProjectLabel        string
	ProjectName         string
	SprintName          string
	DeliveryDefault     string
	BlockersDefault     string
	TodoNextLimit       int
	OnlyMyStatusChanges bool
	CurrentAccountID    string
	IssueBaseURL        string
	Location            *time.Location
	Day                 time.Time
}

// PointTotals stores story-point totals by workflow bucket.
type PointTotals struct {
	Todo       float64
	InProgress float64
	Done       float64
	Blocked    float64
	Total      float64
}

// GenerateDaily builds the report body using today's status changes and current sprint state.
func GenerateDaily(issues []jira.Issue, changes []jira.StatusChange, opts Options) string {
	if opts.Location == nil {
		opts.Location = time.Local
	}
	if opts.Day.IsZero() {
		opts.Day = time.Now()
	}
	if opts.TodoNextLimit <= 0 {
		opts.TodoNextLimit = 1
	}
	projectLabel := opts.ProjectLabel
	if projectLabel == "" {
		projectName := opts.ProjectName
		if projectName == "" {
			projectName = "Project"
		}
		projectLabel = "Project Name - " + projectName
	}
	blockersDefault := opts.BlockersDefault
	if blockersDefault == "" {
		blockersDefault = "None"
	}
	delivery := opts.DeliveryDefault
	if delivery == "" {
		delivery = "G"
	}

	issueByKey := map[string]jira.Issue{}
	for _, issue := range issues {
		issueByKey[issue.Key] = issue
	}
	doneKeys := map[string]struct{}{}
	wipKeys := map[string]struct{}{}
	todaysChanges := filterReportChanges(changes, opts)
	for _, change := range todaysChanges {
		category := change.ToCategory
		if category == "" {
			category = jira.StatusCategoryForName(change.ToStatus)
		}
		switch category {
		case "done":
			doneKeys[change.IssueKey] = struct{}{}
			delete(wipKeys, change.IssueKey)
		case "indeterminate":
			wipKeys[change.IssueKey] = struct{}{}
			delete(doneKeys, change.IssueKey)
		default:
			delete(doneKeys, change.IssueKey)
			delete(wipKeys, change.IssueKey)
		}
	}
	done := issuesFromKeys(doneKeys, issueByKey)
	wip := issuesFromKeys(wipKeys, issueByKey)
	todo := []jira.Issue{}

	var b strings.Builder
	fmt.Fprintf(&b, "📍 %s\n\n", projectLabel)
	b.WriteString("✅ Done \n")
	writeIssueLines(&b, done, opts.IssueBaseURL)
	b.WriteString("\n🚧 Work In-Progress\n")
	writeIssueLines(&b, wip, opts.IssueBaseURL)
	b.WriteString("\n❌ Blockers/Pain points\n")
	fmt.Fprintf(&b, "- %s\n", blockersDefault)
	b.WriteString("\n📝 TODO Next\n")
	writeIssueLines(&b, todo, opts.IssueBaseURL)
	fmt.Fprintf(&b, "\n🚦%s Delivery: %s\n", sprintLabel(opts.SprintName), delivery)
	return b.String()
}

// Totals computes story-point totals for the visible issues.
func Totals(issues []jira.Issue) PointTotals {
	var totals PointTotals
	for _, issue := range issues {
		points := 0.0
		if issue.StoryPoints != nil {
			points = *issue.StoryPoints
		}
		totals.Total += points
		switch issueStatusCategory(issue) {
		case "done":
			totals.Done += points
		case "indeterminate":
			totals.InProgress += points
		case "blocked":
			totals.Blocked += points
		default:
			totals.Todo += points
		}
	}
	return totals
}

func filterReportChanges(changes []jira.StatusChange, opts Options) []jira.StatusChange {
	filtered := make([]jira.StatusChange, 0, len(changes))
	for _, change := range changes {
		if opts.OnlyMyStatusChanges && opts.CurrentAccountID != "" && change.AuthorID != "" && change.AuthorID != opts.CurrentAccountID {
			continue
		}
		if !jira.IsSameLocalDay(change.At, opts.Day, opts.Location) {
			continue
		}
		filtered = append(filtered, change)
	}
	sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].At.Before(filtered[j].At) })
	return filtered
}

func issueStatusCategory(issue jira.Issue) string {
	if issue.Status.Category.Key != "" {
		return issue.Status.Category.Key
	}
	return jira.StatusCategoryForName(issue.Status.Name)
}

func issuesFromKeys(keys map[string]struct{}, issueByKey map[string]jira.Issue) []jira.Issue {
	issues := make([]jira.Issue, 0, len(keys))
	for key := range keys {
		if issue, ok := issueByKey[key]; ok {
			issues = append(issues, issue)
		} else {
			issues = append(issues, jira.Issue{Key: key, Summary: key})
		}
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Key < issues[j].Key })
	return issues
}

func writeIssueLines(b *strings.Builder, issues []jira.Issue, issueBaseURL string) {
	if len(issues) == 0 {
		b.WriteString("- None\n")
		return
	}
	for _, issue := range issues {
		fmt.Fprintf(b, "- %s %s\n", sentence(issue.Summary), issueLink(issue.Key, issueBaseURL))
	}
}

func issueLink(key string, baseURL string) string {
	key = strings.TrimSpace(key)
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if key == "" {
		return "[]"
	}
	if baseURL == "" {
		return "[" + key + "]"
	}
	return fmt.Sprintf("[%s](%s/browse/%s)", key, baseURL, key)
}

func sentence(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Untitled."
	}
	if strings.ContainsAny(value[len(value)-1:], ".!?") {
		return value
	}
	return value + "."
}

func sprintLabel(name string) string {
	if strings.TrimSpace(name) == "" {
		return "Sprint"
	}
	return strings.TrimSpace(name)
}
