// Package config loads and persists lazyjira configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// AppName is the application name used for XDG paths and keyring service names.
	AppName = "lazyjira"
)

// Config contains all non-secret application settings.
type Config struct {
	Jira   JiraConfig   `yaml:"jira"`
	Mail   MailConfig   `yaml:"mail"`
	Report ReportConfig `yaml:"report"`
	UI     UIConfig     `yaml:"ui"`
}

// JiraConfig contains Jira connection and workflow settings.
type JiraConfig struct {
	BaseURL            string         `yaml:"base_url"`
	Username           string         `yaml:"username"`
	AccountID          string         `yaml:"account_id"`
	DisplayName        string         `yaml:"display_name"`
	ProjectKey         string         `yaml:"project_key"`
	ProjectID          string         `yaml:"project_id"`
	ProjectName        string         `yaml:"project_name"`
	BoardID            int            `yaml:"board_id"`
	IssueTypeTaskID    string         `yaml:"issue_type_task_id"`
	StoryPointsFieldID string         `yaml:"story_points_field_id"`
	StatusGroups       StatusGroups   `yaml:"status_groups"`
	CreateDefaults     map[string]any `yaml:"create_defaults,omitempty"`
}

// StatusGroups maps Jira status names to lazyjira buckets.
type StatusGroups struct {
	Done       []string `yaml:"done"`
	InProgress []string `yaml:"in_progress"`
	Blocked    []string `yaml:"blocked"`
}

// MailConfig contains IONOS IMAP draft settings.
type MailConfig struct {
	Provider        string   `yaml:"provider"`
	IMAPHost        string   `yaml:"imap_host"`
	IMAPPort        int      `yaml:"imap_port"`
	TLS             bool     `yaml:"tls"`
	Username        string   `yaml:"username"`
	From            string   `yaml:"from"`
	To              []string `yaml:"to"`
	CC              []string `yaml:"cc,omitempty"`
	DraftsMailbox   string   `yaml:"drafts_mailbox"`
	SubjectTemplate string   `yaml:"subject_template"`
}

// ReportConfig contains daily report generation settings.
type ReportConfig struct {
	ProjectLabel        string `yaml:"project_label"`
	Timezone            string `yaml:"timezone"`
	OnlyMyStatusChanges bool   `yaml:"only_my_status_changes"`
	BlockersDefault     string `yaml:"blockers_default"`
	DeliveryDefault     string `yaml:"delivery_default"`
	TodoNextLimit       int    `yaml:"todo_next_limit"`
}

// UIConfig contains terminal UI preferences.
type UIConfig struct {
	Mouse      bool   `yaml:"mouse"`
	Icons      bool   `yaml:"icons"`
	Animations bool   `yaml:"animations"`
	Theme      string `yaml:"theme"`
}

// Default returns a config with production-safe defaults.
func Default() Config {
	return Config{
		Jira: JiraConfig{
			StatusGroups: StatusGroups{
				Done:       []string{"done", "closed", "resolved"},
				InProgress: []string{"in progress", "development", "in review", "review"},
				Blocked:    []string{"blocked", "impediment"},
			},
			CreateDefaults: map[string]any{},
		},
		Mail: MailConfig{
			Provider:        "ionos",
			IMAPHost:        "imap.ionos.de",
			IMAPPort:        993,
			TLS:             true,
			DraftsMailbox:   "Entwürfe",
			SubjectTemplate: "Daily Report",
		},
		Report: ReportConfig{
			Timezone:            "Local",
			OnlyMyStatusChanges: true,
			BlockersDefault:     "None",
			DeliveryDefault:     "G",
			TodoNextLimit:       1,
		},
		UI: UIConfig{
			Mouse:      true,
			Icons:      true,
			Animations: true,
			Theme:      "default-dark",
		},
	}
}

// IsJiraConfigured reports whether Jira setup/discovery has completed.
func (c Config) IsJiraConfigured() bool {
	return c.Jira.BaseURL != "" &&
		c.Jira.Username != "" &&
		c.Jira.AccountID != "" &&
		c.Jira.ProjectKey != "" &&
		c.Jira.ProjectID != "" &&
		c.Jira.ProjectName != "" &&
		c.Jira.BoardID > 0 &&
		c.Jira.IssueTypeTaskID != "" &&
		c.Jira.StoryPointsFieldID != ""
}

// IsConfigured reports whether the config has enough data to start the daily sprint view.
func (c Config) IsConfigured() bool {
	return c.IsJiraConfigured() &&
		c.Mail.From != "" &&
		len(c.Mail.To) > 0
}

// Load reads the config from path. If path is empty, the XDG default path is used.
// Missing config files are not considered an error; the default config is returned.
func Load(path string) (Config, string, error) {
	resolved, err := Path(path)
	if err != nil {
		return Config{}, "", err
	}

	cfg := Default()
	data, err := os.ReadFile(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, resolved, nil
		}
		return Config{}, resolved, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, resolved, fmt.Errorf("parse config: %w", err)
	}
	return cfg, resolved, nil
}

// Save writes cfg to path using user-only permissions.
func Save(path string, cfg Config) error {
	resolved, err := Path(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(resolved, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
