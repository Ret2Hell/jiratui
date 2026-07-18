package jira

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"
)

func TestLiveAttachmentBackedExternalMedia(t *testing.T) {
	if os.Getenv("JIRATUI_LIVE_JIRA_TEST") != "1" {
		t.Skip("set JIRATUI_LIVE_JIRA_TEST=1 to run the Jira media release gate")
	}
	baseURL := requireEnv(t, "JIRATUI_LIVE_JIRA_URL")
	username := requireEnv(t, "JIRATUI_LIVE_JIRA_USERNAME")
	token := requireEnv(t, "JIRATUI_LIVE_JIRA_TOKEN")
	projectKey := requireEnv(t, "JIRATUI_LIVE_JIRA_PROJECT_KEY")
	issueTypeID := requireEnv(t, "JIRATUI_LIVE_JIRA_ISSUE_TYPE_ID")
	client, err := NewClient(baseURL, username, token)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	created, err := client.CreateTask(ctx, CreateTaskInput{
		ProjectKey: projectKey, IssueTypeID: issueTypeID,
		Summary:     "jiratui external media verification " + time.Now().UTC().Format(time.RFC3339),
		Description: PlainDescription("Before image"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := client.DeleteTask(cleanupContext, created.Key); err != nil {
			t.Logf("delete live verification task %s: %v", created.Key, err)
		}
	})

	var encoded bytes.Buffer
	source := image.NewRGBA(image.Rect(0, 0, 2, 2))
	source.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	pending := DescriptionImage{ID: "img-live", Filename: "jiratui-live.png", MIMEType: "image/png", Data: encoded.Bytes(), Width: 2, Height: 2}
	uploaded, err := client.AddAttachment(ctx, created.Key, pending)
	if err != nil {
		t.Fatal(err)
	}
	description := PlainDescription("Before image\n" + ImageReferenceToken(pending.ID, "Red verification pixel"))
	description.Images = []DescriptionImage{uploaded}
	description, err = ParseDescriptionEditor(description.EditorText, description)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.UpdateTaskFields(ctx, created.Key, nil, &description); err != nil {
		t.Fatalf("Jira rejected attachment-backed external media: %v", err)
	}
	roundTrip, err := client.Issue(ctx, created.Key, "")
	if err != nil {
		t.Fatal(err)
	}
	if !roundTrip.DescriptionContent.Editable || len(roundTrip.DescriptionContent.Images) != 1 || len(roundTrip.DescriptionContent.References) != 1 {
		t.Fatalf("external media did not round-trip as an Editable Description: %#v", roundTrip.DescriptionContent)
	}
}

func requireEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required for the live Jira test", name)
	}
	return value
}
