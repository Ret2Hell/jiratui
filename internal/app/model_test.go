package app

import (
	"testing"

	"github.com/Ret2Hell/jiratui/internal/config"
)

func TestNewForceSetupStartsAtJiraStage(t *testing.T) {
	cfg := configuredTestConfig()

	model := New(cfg, "", nil, nil, "Update setup", true)

	if model.screen != screenSetup {
		t.Fatalf("screen = %v, want screenSetup", model.screen)
	}
	if model.setupStage != 0 {
		t.Fatalf("setupStage = %d, want 0", model.setupStage)
	}
	if model.setupFocus != 0 {
		t.Fatalf("setupFocus = %d, want 0", model.setupFocus)
	}
}

func TestNewIncompleteMailSetupResumesAtIONOSStage(t *testing.T) {
	cfg := configuredTestConfig()
	cfg.Mail.From = ""
	cfg.Mail.To = nil

	model := New(cfg, "", nil, nil, "", false)

	if model.setupStage != 1 {
		t.Fatalf("setupStage = %d, want 1", model.setupStage)
	}
	if model.setupFocus != 4 {
		t.Fatalf("setupFocus = %d, want 4", model.setupFocus)
	}
}

func configuredTestConfig() config.Config {
	cfg := config.Default()
	cfg.Jira.BaseURL = "https://example.atlassian.net"
	cfg.Jira.Username = "user@example.com"
	cfg.Jira.AccountID = "account-id"
	cfg.Jira.ProjectKey = "TEST"
	cfg.Jira.ProjectID = "10000"
	cfg.Jira.ProjectName = "Test"
	cfg.Jira.BoardID = 1
	cfg.Jira.IssueTypeTaskID = "10001"
	cfg.Jira.StoryPointsFieldID = "customfield_10016"
	cfg.Mail.From = "mail@example.com"
	cfg.Mail.To = []string{"recipient@example.com"}
	return cfg
}
