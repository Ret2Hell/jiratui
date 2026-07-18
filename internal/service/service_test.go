package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/jira"
	"github.com/Ret2Hell/jiratui/internal/tasksave"
)

func TestSaveTaskCheckpointsCreateImageAndFinalContent(t *testing.T) {
	var created, uploaded, updated int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/search/jql":
			fmt.Fprint(w, `{"issues":[],"isLast":true}`)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/issue":
			created++
			var payload map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if !strings.Contains(string(payload["properties"]), `"id":"save-test"`) {
				t.Errorf("create properties = %s", payload["properties"])
			}
			if strings.Contains(string(payload["fields"]), "mediaSingle") {
				t.Errorf("initial create leaked unresolved image: %s", payload["fields"])
			}
			fmt.Fprint(w, `{"id":"10000","key":"TEST-1"}`)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/agile/1.0/sprint/7/issue":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/TEST-1":
			fmt.Fprint(w, `{"fields":{"attachment":[]}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/issue/TEST-1/attachments":
			uploaded++
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			defer file.Close()
			data, _ := io.ReadAll(file)
			if string(data) != "image-data" || !strings.HasPrefix(header.Filename, "jiratui-img-test-") {
				t.Errorf("attachment = %q / %q", header.Filename, data)
			}
			fmt.Fprintf(w, `[{"id":"10001","filename":%q,"mimeType":"image/png","content":"https://example.invalid/content/10001"}]`, header.Filename)
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/3/issue/TEST-1":
			updated++
			data, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(data), `"type":"external"`) || !strings.Contains(string(data), `"alt":"Login screen"`) {
				t.Errorf("final update = %s", data)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := jira.NewClient(server.URL, "user@example.com", "token")
	if err != nil {
		t.Fatal(err)
	}
	svc := &JiraService{cfg: config.Config{Jira: config.JiraConfig{
		ProjectKey: "TEST", BoardID: 1, IssueTypeTaskID: "10001", AccountID: "account-1",
	}}, jira: client}
	journal := imageCreateJournal(t)
	checkpoints := 0
	saved, err := svc.SaveTask(t.Context(), journal, func(progress tasksave.Journal) error {
		checkpoints++
		if checkpoints == 4 && len(progress.Draft.Description.ReferencedPendingImages()) != 0 {
			t.Error("uploaded image remained pending at its checkpoint")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !saved.Complete() || saved.IssueKey != "TEST-1" || checkpoints != 5 {
		t.Fatalf("saved journal = %#v, checkpoints = %d", saved, checkpoints)
	}
	if created != 1 || uploaded != 1 || updated != 1 {
		t.Fatalf("writes: create=%d upload=%d update=%d", created, uploaded, updated)
	}
}

func TestSaveTaskKeepsFinalStepPendingWhenCheckpointFails(t *testing.T) {
	updates := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/rest/api/3/issue/TEST-1" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		updates++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client, _ := jira.NewClient(server.URL, "user@example.com", "token")
	svc := &JiraService{jira: client}
	journal, err := tasksave.NewUpdate(jira.Issue{Key: "TEST-1", Summary: "Old"}, tasksave.Draft{
		Summary:      "New",
		Description:  jira.PlainDescription(""),
		WriteSummary: true,
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	saved, err := svc.SaveTask(t.Context(), journal, func(tasksave.Journal) error {
		return errors.New("disk full")
	})
	if err == nil || saved.ContentUpdated || saved.Complete() {
		t.Fatalf("save = %#v, err = %v", saved, err)
	}

	saved, err = svc.SaveTask(t.Context(), saved, func(tasksave.Journal) error { return nil })
	if err != nil || !saved.Complete() || updates != 2 {
		t.Fatalf("retry = %#v, err = %v, updates = %d", saved, err, updates)
	}
}

func TestSaveTaskReconcilesCreateByJournalProperty(t *testing.T) {
	creates := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/search/jql":
			fmt.Fprint(w, `{"issues":[{"id":"10000","key":"TEST-1","fields":{"summary":"Task"}}],"isLast":true}`)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/TEST-1/properties":
			fmt.Fprint(w, `{"keys":[{"key":"jiratui.task-save"}]}`)
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/issue/TEST-1/properties/jiratui.task-save":
			fmt.Fprint(w, `{"value":{"id":"save-test"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/issue":
			creates++
			t.Error("reconciled create posted a duplicate task")
		case r.Method == http.MethodPost && r.URL.Path == "/rest/agile/1.0/sprint/7/issue":
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	client, _ := jira.NewClient(server.URL, "user@example.com", "token")
	svc := &JiraService{cfg: config.Config{Jira: config.JiraConfig{
		ProjectKey: "TEST", IssueTypeTaskID: "10001", AccountID: "account-1",
	}}, jira: client}
	journal, err := tasksave.NewCreate("NEW-1", jira.Issue{Key: "NEW-1"}, tasksave.Draft{
		Summary: "Task", Description: jira.PlainDescription("Description"),
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	journal.ID = "save-test"
	journal.AssigneeID = "account-1"
	journal.SprintID = 7
	saved, err := svc.SaveTask(t.Context(), journal, func(tasksave.Journal) error { return nil })
	if err != nil || !saved.Complete() || saved.IssueKey != "TEST-1" || creates != 0 {
		t.Fatalf("save = %#v, err = %v, creates = %d", saved, err, creates)
	}
}

func imageCreateJournal(t *testing.T) tasksave.Journal {
	t.Helper()
	image := jira.DescriptionImage{
		ID: "img-test", Filename: "screen.png", MIMEType: "image/png", Data: []byte("image-data"), Width: 20, Height: 10,
	}
	description := jira.PlainDescription("Before\n" + jira.ImageReferenceToken(image.ID, "Login screen"))
	description.Images = []jira.DescriptionImage{image}
	var err error
	description, err = jira.ParseDescriptionEditor(description.EditorText, description)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := tasksave.NewCreate("NEW-1", jira.Issue{Key: "NEW-1"}, tasksave.Draft{
		Summary: "Task", Description: description,
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	journal.ID = "save-test"
	journal.AssigneeID = "account-1"
	journal.SprintID = 7
	return journal
}
