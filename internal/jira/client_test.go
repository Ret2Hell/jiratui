package jira

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	if issue.DescriptionContent.Editable || issue.DescriptionContent.UnsupportedReason == "" {
		t.Fatalf("rich description should be read-only: %#v", issue.DescriptionContent)
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

func TestClientUploadsImageAttachment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/3/issue/TEST-1/attachments" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Atlassian-Token"); got != "no-check" {
			t.Errorf("X-Atlassian-Token = %q", got)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Errorf("read multipart file: %v", err)
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if header.Filename != "screen.png" || string(data) != "png-data" {
			t.Errorf("attachment = %q / %q", header.Filename, data)
		}
		if got := header.Header.Get("Content-Type"); got != "image/png" {
			t.Errorf("attachment content type = %q", got)
		}
		fmt.Fprint(w, `[{"id":"10001","filename":"screen.png","mimeType":"image/png","content":"https://example.atlassian.net/rest/api/3/attachment/content/10001"}]`)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "user@example.com", "token")
	if err != nil {
		t.Fatal(err)
	}
	image, err := client.AddAttachment(t.Context(), "TEST-1", DescriptionImage{
		ID:       "img-test",
		Filename: "screen.png",
		MIMEType: "image/png",
		Data:     []byte("png-data"),
		Width:    800,
		Height:   600,
	})
	if err != nil {
		t.Fatal(err)
	}
	if image.ID != "img-test" || image.AttachmentID != "10001" || image.Filename != "screen.png" || image.Width != 800 || image.Height != 600 || image.URL == "" || len(image.Data) != 0 {
		t.Fatalf("image metadata = %#v", image)
	}
}

func TestDescriptionImageADFRoundTrip(t *testing.T) {
	adf := map[string]any{
		"type": "doc", "version": 1,
		"content": []any{
			map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "Before"}}},
			map[string]any{
				"type": "mediaSingle", "attrs": map[string]any{"layout": "wrap-left", "width": 60},
				"content": []any{map[string]any{"type": "media", "attrs": map[string]any{
					"type": "external", "url": "https://example.atlassian.net/rest/api/3/attachment/content/10001",
					"alt": "screen.png", "width": 800, "height": 600,
				}}},
			},
			map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": "After"}}},
		},
	}
	description := descriptionFromADF(adf)
	if !description.Editable || len(description.Images) != 1 || len(description.References) != 1 {
		t.Fatalf("description = %#v", description)
	}
	description.EditorText = strings.Replace(description.EditorText, "screen.png", "Login failure", 1)
	roundTrip, err := descriptionToADF(description, false)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(roundTrip)
	if !bytes.Contains(data, []byte(`"layout":"wrap-left"`)) || !bytes.Contains(data, []byte(`"alt":"Login failure"`)) {
		t.Fatalf("round-trip ADF = %s", data)
	}
}

func TestClientDeletesTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/rest/api/3/issue/TEST-1" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "user@example.com", "token")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.DeleteTask(t.Context(), "TEST-1"); err != nil {
		t.Fatal(err)
	}
}

func assertIssueType(t *testing.T, got, want IssueType) {
	t.Helper()
	if got != want {
		t.Fatalf("issue type = %#v, want %#v", got, want)
	}
}
