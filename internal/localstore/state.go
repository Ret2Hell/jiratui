// Package localstore persists the local-first jiratui working set.
package localstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Ret2Hell/jiratui/internal/config"
	"github.com/Ret2Hell/jiratui/internal/jira"
	"github.com/Ret2Hell/jiratui/internal/service"
	"github.com/Ret2Hell/jiratui/internal/tasksave"
)

var unsafePathRE = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

var (
	cacheMu        sync.Mutex
	cacheRevisions = make(map[string]uint64)
)

// State is the cached local working set used before Jira responds.
type State struct {
	ProjectName string             `json:"project_name"`
	Sprint      jira.Sprint        `json:"sprint"`
	Issues      []jira.Issue       `json:"issues"`
	Draft       service.DailyDraft `json:"draft"`
	SavedAt     time.Time          `json:"saved_at"`
	Revision    uint64             `json:"revision"`
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
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create local cache dir: %w", err)
	}
	latest := cacheRevisions[path]
	if data, err := os.ReadFile(path); err == nil {
		var existing struct {
			Revision uint64 `json:"revision"`
		}
		if json.Unmarshal(data, &existing) == nil {
			latest = max(latest, existing.Revision)
		}
	}
	if state.Revision > 0 && state.Revision < latest {
		return nil
	}
	state.SavedAt = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal local cache: %w", err)
	}
	if err := writePrivateFile(path, data); err != nil {
		return fmt.Errorf("write local cache: %w", err)
	}
	cacheRevisions[path] = max(latest, state.Revision)
	return nil
}

// SaveTaskJournal atomically checkpoints a partial save and its pending image data.
func SaveTaskJournal(cfg config.Config, journal tasksave.Journal) error {
	dir, err := taskJournalDir(cfg)
	if err != nil {
		return err
	}
	if sanitize(journal.ID) != journal.ID || journal.ID == "" {
		return fmt.Errorf("invalid task-save id %q", journal.ID)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create task-save dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure task-save dir: %w", err)
	}
	imageDir := filepath.Join(dir, journal.ID+".images")
	pending := journal.Draft.Description.ReferencedPendingImages()
	if len(pending) > 0 {
		if err := os.MkdirAll(imageDir, 0o700); err != nil {
			return fmt.Errorf("create task-save image dir: %w", err)
		}
		if err := os.Chmod(imageDir, 0o700); err != nil {
			return fmt.Errorf("secure task-save image dir: %w", err)
		}
	}
	expected := make(map[string]bool, len(pending))
	for _, image := range pending {
		if sanitize(image.ID) != image.ID || image.ID == "" {
			return fmt.Errorf("invalid description image id %q", image.ID)
		}
		name := image.ID + ".bin"
		expected[name] = true
		if err := writePrivateFile(filepath.Join(imageDir, name), image.Data); err != nil {
			return fmt.Errorf("write task-save image %q: %w", image.Filename, err)
		}
	}
	metadata := journal
	metadata.Draft.Description = journal.Draft.Description.WithoutImageData()
	metadata.Issue.DescriptionContent = journal.Issue.DescriptionContent.WithoutImageData()
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal task-save journal: %w", err)
	}
	if err := writePrivateFile(filepath.Join(dir, journal.ID+".json"), data); err != nil {
		return fmt.Errorf("write task-save journal: %w", err)
	}
	if entries, err := os.ReadDir(imageDir); err == nil {
		for _, entry := range entries {
			if !expected[entry.Name()] {
				if err := os.Remove(filepath.Join(imageDir, entry.Name())); err != nil {
					return fmt.Errorf("remove completed task-save image: %w", err)
				}
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read task-save image dir: %w", err)
	}
	if len(expected) == 0 {
		if err := os.Remove(imageDir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove empty task-save image dir: %w", err)
		}
	}
	return nil
}

// LoadTaskJournals reads all resumable task saves and restores pending image data.
func LoadTaskJournals(cfg config.Config) ([]tasksave.Journal, error) {
	dir, err := taskJournalDir(cfg)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read task-save dir: %w", err)
	}
	journals := make([]tasksave.Journal, 0)
	var loadErrors []error
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("read task-save journal %q: %w", entry.Name(), err))
			continue
		}
		var journal tasksave.Journal
		if err := json.Unmarshal(data, &journal); err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("parse task-save journal %q: %w", entry.Name(), err))
			continue
		}
		if strings.TrimSuffix(filepath.Base(entry.Name()), filepath.Ext(entry.Name())) != journal.ID {
			loadErrors = append(loadErrors, fmt.Errorf("task-save journal %q has mismatched id %q", entry.Name(), journal.ID))
			continue
		}
		if err := journal.Validate(); err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("validate task-save journal %q: %w", entry.Name(), err))
			continue
		}
		valid := true
		for index := range journal.Draft.Description.Images {
			image := &journal.Draft.Description.Images[index]
			path := filepath.Join(dir, journal.ID+".images", image.ID+".bin")
			image.Data, err = os.ReadFile(path)
			if errors.Is(err, os.ErrNotExist) {
				image.Data = nil
				continue
			}
			if err != nil {
				loadErrors = append(loadErrors, fmt.Errorf("read task-save image %q: %w", image.Filename, err))
				valid = false
				break
			}
		}
		if !valid {
			continue
		}
		if missing := journal.Draft.Description.MissingPendingImageData(); len(missing) > 0 {
			loadErrors = append(loadErrors, fmt.Errorf("task-save journal %q is missing image data for %q", entry.Name(), missing[0].Filename))
			valid = false
		}
		if !valid {
			continue
		}
		journals = append(journals, journal)
	}
	slices.SortFunc(journals, func(a, b tasksave.Journal) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return journals, errors.Join(loadErrors...)
}

// DeleteTaskJournal removes one completed, abandoned, or expired local save.
func DeleteTaskJournal(cfg config.Config, id string) error {
	dir, err := taskJournalDir(cfg)
	if err != nil {
		return err
	}
	if sanitize(id) != id || id == "" {
		return fmt.Errorf("invalid task-save id %q", id)
	}
	if err := os.Remove(filepath.Join(dir, id+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove task-save journal: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(dir, id+".images")); err != nil {
		return fmt.Errorf("remove task-save images: %w", err)
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

func taskJournalDir(cfg config.Config) (string, error) {
	cachePath, err := path(cfg)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(cachePath, filepath.Ext(cachePath)) + "-task-saves", nil
}

func writePrivateFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
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
