package app

import (
	"strings"
	"testing"

	"github.com/Ret2Hell/jiratui/internal/jira"
)

func TestDiscoverTaskIssueTypeID(t *testing.T) {
	tests := []struct {
		name       string
		issueTypes []jira.IssueType
		want       string
		wantErr    string
	}{
		{
			name: "French localized name",
			issueTypes: []jira.IssueType{
				{ID: "10233", Name: "Tâche", UntranslatedName: "Task"},
			},
			want: "10233",
		},
		{
			name: "non-Latin localized name",
			issueTypes: []jira.IssueType{
				{ID: "1", Name: "タスク", UntranslatedName: "Task"},
			},
			want: "1",
		},
		{
			name: "localized fallback without untranslated name",
			issueTypes: []jira.IssueType{
				{ID: "2", Name: "Task"},
			},
			want: "2",
		},
		{
			name: "case and whitespace",
			issueTypes: []jira.IssueType{
				{ID: "3", Name: "Tâche", UntranslatedName: "  tAsK  "},
			},
			want: "3",
		},
		{
			name: "subtask rejected",
			issueTypes: []jira.IssueType{
				{ID: "sub", Name: "Tâche", UntranslatedName: "Task", Subtask: true},
				{ID: "story", Name: "Story", UntranslatedName: "Story"},
			},
			wantErr: "available: Story",
		},
		{
			name: "exact canonical beats fuzzy",
			issueTypes: []jira.IssueType{
				{ID: "request", Name: "Task Request", UntranslatedName: "Task Request"},
				{ID: "task", Name: "Tâche", UntranslatedName: "Task"},
			},
			want: "task",
		},
		{
			name: "canonical beats localized fallback",
			issueTypes: []jira.IssueType{
				{ID: "custom", Name: "Task"},
				{ID: "canonical", Name: "Tâche", UntranslatedName: "Task"},
			},
			want: "canonical",
		},
		{
			name: "custom fuzzy fallback",
			issueTypes: []jira.IssueType{
				{ID: "custom", Name: "Engineering Task"},
			},
			want: "custom",
		},
		{
			name: "no match reports unique non-subtask names",
			issueTypes: []jira.IssueType{
				{ID: "1", Name: "Bug"},
				{ID: "2", Name: "Story"},
				{ID: "3", Name: "bug"},
				{ID: "4", Name: "Subtask", Subtask: true},
			},
			wantErr: "available: Bug, Story",
		},
		{
			name:    "empty input",
			wantErr: "could not auto-discover",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := discoverTaskIssueTypeID(tt.issueTypes)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ID = %q, want %q", got, tt.want)
			}
		})
	}
}
