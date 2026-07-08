// Package localstore persists the local-first jiratui working set.
package localstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/jira"
	"github.com/Ret2Hell/jiratui/internal/service"
)

var unsafePathRE = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// State is the cached local working set used before Jira responds.
type State struct {
	ProjectName string             `json:"project_name"`
	Sprint      jira.Sprint        `json:"sprint"`
	Issues      []jira.Issue       `json:"issues"`
	Draft       service.DailyDraft `json:"draft"`
	SavedAt     time.Time          `json:"saved_at"`
}

// Load reads the cached working set. Missing cache is not an error.
func Load(cfg config.Config) (State, bool, error) {
	path, err := path(cfg)
	if err != nil {
		return State{}, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{}, false, nil
		}
		return State{}, false, fmt.Errorf("read local cache: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, false, fmt.Errorf("parse local cache: %w", err)
	}
	return state, true, nil
}

// Save writes the cached working set atomically enough for small local state.
func Save(cfg config.Config, state State) error {
	path, err := path(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create local cache dir: %w", err)
	}
	state.SavedAt = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal local cache: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write local cache: %w", err)
	}
	return nil
}

func path(cfg config.Config) (string, error) {
	dir, err := config.CacheDir()
	if err != nil {
		return "", err
	}
	name := firstNonEmpty(cfg.Jira.ProjectKey, cfg.Jira.ProjectName, "default")
	user := firstNonEmpty(cfg.Jira.Username, "user")
	name = sanitize(strings.ToLower(user + "-" + name))
	return filepath.Join(dir, name+".json"), nil
}

func sanitize(value string) string {
	value = unsafePathRE.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-.")
	if value == "" {
		return "default"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
