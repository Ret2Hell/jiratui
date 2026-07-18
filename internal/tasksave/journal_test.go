package tasksave

import (
	"testing"
	"time"

	"github.com/Ret2Hell/jiratui/internal/jira"
)

func TestProjectionDoesNotExposePendingImageBytes(t *testing.T) {
	image, err := jira.NewDescriptionImage("screen.png", "image/png", []byte("private"), 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	description := jira.PlainDescription(jira.ImageReferenceToken(image.ID, "screen"))
	description.Images = []jira.DescriptionImage{image}
	description, err = jira.ParseDescriptionEditor(description.EditorText, description)
	if err != nil {
		t.Fatal(err)
	}
	journal, err := NewCreate("NEW-1", jira.Issue{Key: "NEW-1"}, Draft{Summary: "Task", Description: description}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	projection := journal.Projection()
	if len(projection.DescriptionContent.Images) != 1 || len(projection.DescriptionContent.Images[0].Data) != 0 {
		t.Fatalf("projection retained image bytes: %#v", projection.DescriptionContent.Images)
	}
	if len(journal.Draft.Description.Images[0].Data) == 0 {
		t.Fatal("journal draft lost pending image bytes")
	}
}

func TestJournalExpiryUsesLastAcceptedStep(t *testing.T) {
	created := time.Date(2026, 7, 17, 10, 0, 0, 0, time.UTC)
	journal := Journal{CreatedAt: created, LastAcceptedAt: created.Add(12 * time.Hour)}
	if journal.Expired(created.Add(35 * time.Hour)) {
		t.Fatal("journal expired before 24 hours after its last accepted step")
	}
	if !journal.Expired(created.Add(36 * time.Hour)) {
		t.Fatal("journal did not expire after 24 hours")
	}
}

func TestJournalValidationRejectsUnknownVersion(t *testing.T) {
	journal := Journal{Version: CurrentVersion + 1, ID: "save-test", Kind: KindUpdate, IssueKey: "TEST-1"}
	if err := journal.Validate(); err == nil {
		t.Fatal("unknown journal version was accepted")
	}
}
