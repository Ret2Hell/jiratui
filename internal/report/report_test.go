package report

import (
	"strings"
	"testing"
	"time"

	"github.com/Ret2Hell/lazyjira/internal/jira"
)

func TestGenerateDaily(t *testing.T) {
	now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	issues := []jira.Issue{
		{Key: "R2-425", Summary: "Fixed youtube captions detection", Status: jira.Status{Name: "Done", Category: jira.StatusCategory{Key: "done"}}},
		{Key: "R2-426", Summary: "Adjusting score for exercises attempts", Status: jira.Status{Name: "In Progress", Category: jira.StatusCategory{Key: "indeterminate"}}},
		{Key: "R2-427", Summary: "Implement caching for youtube subtitles extraction", Status: jira.Status{Name: "To Do", Category: jira.StatusCategory{Key: "new"}}},
	}
	changes := []jira.StatusChange{
		{IssueKey: "R2-425", ToCategory: "done", At: now},
		{IssueKey: "R2-426", ToCategory: "indeterminate", At: now},
	}

	body := GenerateDaily(issues, changes, Options{ProjectName: "OteraX", SprintName: "Sprint 63", Day: now, Location: time.UTC, DeliveryDefault: "G", BlockersDefault: "None", TodoNextLimit: 1, IssueBaseURL: "https://teachinghero.atlassian.net"})

	for _, want := range []string{
		"📍 Project Name - OteraX",
		"- Fixed youtube captions detection. [R2-425](https://teachinghero.atlassian.net/browse/R2-425)",
		"- Adjusting score for exercises attempts. [R2-426](https://teachinghero.atlassian.net/browse/R2-426)",
		"🚦Sprint 63 Delivery: G",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("report missing %q:\n%s", want, body)
		}
	}
}

func TestGenerateDailyOnlyReportsTicketsChangedTodayToDoneOrProgress(t *testing.T) {
	today := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	yesterday := today.Add(-24 * time.Hour)
	issues := []jira.Issue{
		{Key: "R2-1", Summary: "Finished yesterday", Status: jira.Status{Name: "Done", Category: jira.StatusCategory{Key: "done"}}},
		{Key: "R2-2", Summary: "Still being worked on", Status: jira.Status{Name: "In Progress", Category: jira.StatusCategory{Key: "indeterminate"}}},
	}
	changes := []jira.StatusChange{
		{IssueKey: "R2-1", ToCategory: "done", At: yesterday},
	}

	body := GenerateDaily(issues, changes, Options{ProjectName: "OteraX", SprintName: "Sprint 63", Day: today, Location: time.UTC, DeliveryDefault: "G", BlockersDefault: "None"})

	if strings.Contains(body, "Finished yesterday") {
		t.Fatalf("old done issue should not be reported as done today:\n%s", body)
	}
	if strings.Contains(body, "Still being worked on") {
		t.Fatalf("current in-progress issue should not be reported without a progress change today:\n%s", body)
	}
}

func TestGenerateDailyLastStatusChangeWins(t *testing.T) {
	day := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)
	issues := []jira.Issue{
		{Key: "R2-1", Summary: "Reopened task", Status: jira.Status{Name: "In Progress", Category: jira.StatusCategory{Key: "indeterminate"}}},
	}
	changes := []jira.StatusChange{
		{IssueKey: "R2-1", ToCategory: "done", At: day.Add(1 * time.Hour)},
		{IssueKey: "R2-1", ToCategory: "indeterminate", At: day.Add(2 * time.Hour)},
	}

	body := GenerateDaily(issues, changes, Options{ProjectName: "OteraX", SprintName: "Sprint 63", Day: day, Location: time.UTC, DeliveryDefault: "G", BlockersDefault: "None"})

	doneSection := body[strings.Index(body, "✅ Done"):strings.Index(body, "🚧 Work In-Progress")]
	if strings.Contains(doneSection, "Reopened task") {
		t.Fatalf("reopened task should not remain in done section:\n%s", body)
	}
	if !strings.Contains(body, "- Reopened task. [R2-1]") {
		t.Fatalf("reopened task should be reported as WIP:\n%s", body)
	}
}

func TestTotals(t *testing.T) {
	p3 := 3.0
	p5 := 5.0
	got := Totals([]jira.Issue{
		{Status: jira.Status{Category: jira.StatusCategory{Key: "done"}}, StoryPoints: &p3},
		{Status: jira.Status{Category: jira.StatusCategory{Key: "indeterminate"}}, StoryPoints: &p5},
	})
	if got.Total != 8 || got.Done != 3 || got.InProgress != 5 {
		t.Fatalf("unexpected totals: %+v", got)
	}
}
