package jira

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientDecodesProjectIssueTypeNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/project/TEST" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("expand"); got != "issueTypes" {
			t.Errorf("expand = %q, want issueTypes", got)
		}
		fmt.Fprint(w, `{
			"id":"10207",
			"key":"TEST",
			"name":"Test",
			"issueTypes":[
				{"id":"10233","name":"Tâche","untranslatedName":"Task","subtask":false},
				{"id":"10234","name":"Bug","subtask":false}
			]
		}`)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "user@example.com", "token")
	if err != nil {
		t.Fatal(err)
	}
	project, err := client.Project(t.Context(), "TEST")
	if err != nil {
		t.Fatal(err)
	}
	if len(project.IssueTypes) != 2 {
		t.Fatalf("issue type count = %d, want 2", len(project.IssueTypes))
	}
	assertIssueType(t, project.IssueTypes[0], IssueType{ID: "10233", Name: "Tâche", UntranslatedName: "Task"})
	assertIssueType(t, project.IssueTypes[1], IssueType{ID: "10234", Name: "Bug"})
}

func TestClientDecodesProjectIssueTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issuetype/project" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("projectId"); got != "10207" {
			t.Errorf("projectId = %q, want 10207", got)
		}
		fmt.Fprint(w, `[
			{"id":"10233","name":"Tâche","untranslatedName":"Task","subtask":false},
			{"id":"10237","name":"Sous-tâche","untranslatedName":"Subtask","subtask":true}
		]`)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "user@example.com", "token")
	if err != nil {
		t.Fatal(err)
	}
	issueTypes, err := client.ProjectIssueTypes(t.Context(), "10207")
	if err != nil {
		t.Fatal(err)
	}
	if len(issueTypes) != 2 {
		t.Fatalf("issue type count = %d, want 2", len(issueTypes))
	}
	assertIssueType(t, issueTypes[0], IssueType{ID: "10233", Name: "Tâche", UntranslatedName: "Task"})
	assertIssueType(t, issueTypes[1], IssueType{ID: "10237", Name: "Sous-tâche", UntranslatedName: "Subtask", Subtask: true})
}

func TestParseIssueTypeRetainsUntranslatedName(t *testing.T) {
	got := parseIssueType(map[string]any{
		"id":               "10233",
		"name":             "Tâche",
		"untranslatedName": "Task",
		"subtask":          false,
	})
	assertIssueType(t, got, IssueType{ID: "10233", Name: "Tâche", UntranslatedName: "Task"})
}

func TestParseIssueDescriptionFromADF(t *testing.T) {
	issue := parseIssue(issueJSON{Fields: map[string]any{
		"description": map[string]any{
			"type": "doc",
			"content": []any{
				map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "First paragraph"}}},
				map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Second paragraph"}}},
			},
		},
	}}, "")
	if issue.Description != "First paragraph\nSecond paragraph" {
		t.Fatalf("description = %q", issue.Description)
	}
}

func TestParseIssueDescriptionPreservesRichInlineNodes(t *testing.T) {
	issue := parseIssue(issueJSON{Fields: map[string]any{
		"description": map[string]any{
			"type": "doc",
			"content": []any{map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{"type": "text", "text": "Ask "},
					map[string]any{"type": "mention", "attrs": map[string]any{"text": "@Ada"}},
					map[string]any{"type": "text", "text": " "},
					map[string]any{"type": "emoji", "attrs": map[string]any{"text": "👍"}},
					map[string]any{"type": "text", "text": " "},
					map[string]any{"type": "inlineCard", "attrs": map[string]any{"url": "https://example.com/spec"}},
				},
			}},
		},
	}}, "")
	if issue.Description != "Ask @Ada 👍 https://example.com/spec" {
		t.Fatalf("description = %q", issue.Description)
	}
}

func TestClientUpdatesSummaryAndDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/rest/api/3/issue/TEST-1" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		var payload struct {
			Fields map[string]json.RawMessage `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
			return
		}
		var summary string
		if err := json.Unmarshal(payload.Fields["summary"], &summary); err != nil || summary != "Updated summary" {
			t.Errorf("summary = %q, err = %v", summary, err)
		}
		var description struct {
			Type    string `json:"type"`
			Content []any  `json:"content"`
		}
		if err := json.Unmarshal(payload.Fields["description"], &description); err != nil {
			t.Errorf("decode description: %v", err)
			return
		}
		if description.Type != "doc" || len(description.Content) != 2 {
			t.Errorf("description payload = %#v", description)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "user@example.com", "token")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.UpdateTask(t.Context(), "TEST-1", "Updated summary", "First line\nSecond line"); err != nil {
		t.Fatal(err)
	}
}

func assertIssueType(t *testing.T, got, want IssueType) {
	t.Helper()
	if got != want {
		t.Fatalf("issue type = %#v, want %#v", got, want)
	}
}
