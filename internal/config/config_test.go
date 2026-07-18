package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefault(t *testing.T) {
	t.Parallel()

	cfg := Default()
	if cfg.UI.Theme != "default" {
		t.Fatalf("UI.Theme = %q, want default", cfg.UI.Theme)
	}
	if !cfg.UI.Mouse || !cfg.UI.Icons || !cfg.UI.Animations {
		t.Fatalf("UI defaults = %+v, want all enabled", cfg.UI)
	}
	if cfg.Mail.IMAPHost != "imap.ionos.de" || cfg.Mail.IMAPPort != 993 || !cfg.Mail.TLS {
		t.Fatalf("Mail defaults = %+v", cfg.Mail)
	}
	if cfg.Report.Timezone != "Local" || cfg.Report.TodoNextLimit != 1 {
		t.Fatalf("Report defaults = %+v", cfg.Report)
	}
	if len(cfg.Jira.StatusGroups.Done) == 0 || cfg.Jira.CreateDefaults == nil {
		t.Fatalf("Jira defaults = %+v", cfg.Jira)
	}
}

func TestLoadMissingReturnsDefaultsAndResolvedPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	cfg, resolved, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if resolved != path {
		t.Fatalf("Load() path = %q, want %q", resolved, path)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Fatalf("Load() config differs from Default():\n got: %#v\nwant: %#v", cfg, Default())
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Jira: JiraConfig{
			BaseURL:            "https://jira.example.com",
			Username:           "user@example.com",
			AccountID:          "account-1",
			DisplayName:        "Example User",
			ProjectKey:         "APP",
			ProjectID:          "10000",
			ProjectName:        "Application",
			BoardID:            42,
			IssueTypeTaskID:    "10001",
			StoryPointsFieldID: "customfield_10016",
			StatusGroups: StatusGroups{
				Done:       []string{"Complete"},
				InProgress: []string{"Building"},
				Blocked:    []string{"Waiting"},
			},
			CreateDefaults: map[string]any{"priority": "High", "labels": []any{"one", "two"}},
		},
		Mail: MailConfig{
			Provider:        "custom",
			IMAPHost:        "mail.example.com",
			IMAPPort:        1993,
			TLS:             true,
			Username:        "mail-user",
			From:            "from@example.com",
			To:              []string{"to@example.com"},
			CC:              []string{"cc@example.com"},
			DraftsMailbox:   "Drafts",
			SubjectTemplate: "Status {{.Date}}",
		},
		Report: ReportConfig{
			ProjectLabel:        "Project X",
			Timezone:            "Europe/Berlin",
			OnlyMyStatusChanges: true,
			BlockersDefault:     "No blockers",
			DeliveryDefault:     "Friday",
			TodoNextLimit:       7,
		},
		UI: UIConfig{Mouse: false, Icons: true, Animations: false, Theme: "catppuccin"},
	}
	path := filepath.Join(t.TempDir(), "config", "settings.yaml")
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, resolved, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if resolved != path {
		t.Fatalf("Load() path = %q, want %q", resolved, path)
	}
	if !reflect.DeepEqual(loaded, cfg) {
		t.Fatalf("round trip changed config:\n got: %#v\nwant: %#v", loaded, cfg)
	}
}

func TestSaveAtomicallyReplacesConfigWithPrivatePermissions(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "config")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("old: incomplete\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Default()
	cfg.UI.Theme = "dracula"
	cfg.Mail.To = []string{}
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	assertMode(t, dir, 0o755)
	assertMode(t, path, 0o600)
	newInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(oldInfo, newInfo) {
		t.Fatal("Save() rewrote the existing file instead of atomically replacing it")
	}
	loaded, _, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(loaded, cfg) {
		t.Fatalf("saved config differs:\n got: %#v\nwant: %#v", loaded, cfg)
	}
	assertNoTemporaryConfigs(t, dir)
}

func TestSaveRenameFailureCleansTemporaryFile(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "config")
	path := filepath.Join(dir, "config.yaml")
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(path, "marker")
	if err := os.WriteFile(marker, []byte("unchanged"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := Save(path, Default())
	if err == nil || !strings.Contains(err.Error(), "replace config") {
		t.Fatalf("Save() error = %v, want replace config error", err)
	}
	data, readErr := os.ReadFile(marker)
	if readErr != nil || string(data) != "unchanged" {
		t.Fatalf("destination changed after failed save: data=%q error=%v", data, readErr)
	}
	assertNoTemporaryConfigs(t, dir)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %#o, want %#o", path, got, want)
	}
}

func assertNoTemporaryConfigs(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".config-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary config files remain: %v", matches)
	}
}
