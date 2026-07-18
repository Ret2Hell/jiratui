package localstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/jira"
	"github.com/Ret2Hell/jiratui/internal/tasksave"
)

func TestTaskJournalStoresPrivateImageSidecarAndRestoresIt(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cfg := config.Config{Jira: config.JiraConfig{ProjectKey: "TEST", Username: "user@example.com"}}
	journal := testImageJournal(t)
	if err := SaveTaskJournal(cfg, journal); err != nil {
		t.Fatal(err)
	}
	dir, err := taskJournalDir(cfg)
	if err != nil {
		t.Fatal(err)
	}
	assertMode(t, dir, 0o700)
	assertMode(t, filepath.Join(dir, journal.ID+".json"), 0o600)
	imagePath := filepath.Join(dir, journal.ID+".images", journal.Draft.Description.Images[0].ID+".bin")
	assertMode(t, imagePath, 0o600)

	data, err := os.ReadFile(filepath.Join(dir, journal.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var metadata tasksave.Journal
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	if len(metadata.Draft.Description.Images[0].Data) != 0 || len(metadata.Issue.DescriptionContent.Images[0].Data) != 0 {
		t.Fatal("journal metadata retained image bytes")
	}

	loaded, err := LoadTaskJournals(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || string(loaded[0].Draft.Description.Images[0].Data) != "private-image" {
		t.Fatalf("loaded journals = %#v", loaded)
	}
}

func TestTaskJournalRemovesSidecarOnlyAfterImageCheckpoint(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cfg := config.Config{Jira: config.JiraConfig{ProjectKey: "TEST", Username: "user@example.com"}}
	journal := testImageJournal(t)
	if err := SaveTaskJournal(cfg, journal); err != nil {
		t.Fatal(err)
	}
	uploaded := journal.Draft.Description.Images[0]
	uploaded.AttachmentID = "10001"
	uploaded.URL = "https://example.invalid/content/10001"
	uploaded.Data = nil
	description, err := journal.Draft.Description.WithUploadedImage(uploaded)
	if err != nil {
		t.Fatal(err)
	}
	journal.Draft.Description = description
	if err := SaveTaskJournal(cfg, journal); err != nil {
		t.Fatal(err)
	}
	dir, _ := taskJournalDir(cfg)
	imagePath := filepath.Join(dir, journal.ID+".images", uploaded.ID+".bin")
	if _, err := os.Stat(imagePath); !os.IsNotExist(err) {
		t.Fatalf("completed image sidecar still exists: %v", err)
	}
	loaded, err := LoadTaskJournals(cfg)
	if err != nil || len(loaded) != 1 || loaded[0].Draft.Description.Images[0].URL == "" {
		t.Fatalf("loaded checkpoint = %#v, err = %v", loaded, err)
	}
}

func TestMalformedTaskJournalDoesNotHideValidJournal(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cfg := config.Config{Jira: config.JiraConfig{ProjectKey: "TEST", Username: "user@example.com"}}
	journal := testImageJournal(t)
	if err := SaveTaskJournal(cfg, journal); err != nil {
		t.Fatal(err)
	}
	dir, _ := taskJournalDir(cfg)
	if err := os.WriteFile(filepath.Join(dir, "save-corrupt.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadTaskJournals(cfg)
	if err == nil || len(loaded) != 1 || loaded[0].ID != journal.ID {
		t.Fatalf("loaded journals = %#v, err = %v", loaded, err)
	}
}

func TestMissingPendingImageDataMakesJournalUnrecoverable(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cfg := config.Config{Jira: config.JiraConfig{ProjectKey: "TEST", Username: "user@example.com"}}
	journal := testImageJournal(t)
	if err := SaveTaskJournal(cfg, journal); err != nil {
		t.Fatal(err)
	}
	dir, _ := taskJournalDir(cfg)
	imagePath := filepath.Join(dir, journal.ID+".images", journal.Draft.Description.Images[0].ID+".bin")
	if err := os.Remove(imagePath); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadTaskJournals(cfg)
	if err == nil || len(loaded) != 0 {
		t.Fatalf("loaded journals = %#v, err = %v", loaded, err)
	}
}

func testImageJournal(t *testing.T) tasksave.Journal {
	t.Helper()
	image, err := jira.NewDescriptionImage("screen.png", "image/png", []byte("private-image"), 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	description := jira.PlainDescription(jira.ImageReferenceToken(image.ID, "screen"))
	description.Images = []jira.DescriptionImage{image}
	description, err = jira.ParseDescriptionEditor(description.EditorText, description)
	if err != nil {
		t.Fatal(err)
	}
	issue := jira.Issue{Key: "NEW-1", DescriptionContent: description.WithoutImageData()}
	journal, err := tasksave.NewCreate("NEW-1", issue, tasksave.Draft{Summary: "Task", Description: description}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	return journal
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}
