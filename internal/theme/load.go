package theme

import (
	"cmp"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed themes/*.toml
var builtinThemes embed.FS

// Registry is a sorted collection of selectable hex themes and non-fatal load warnings.
// The synthetic terminal default is resolved by Resolve and is not included in Themes.
type Registry struct {
	Themes   []Theme
	Warnings []error
}

// LoadAll loads embedded themes and valid user .toml files from userDir.
// User themes override built-ins with the same case-insensitive filename.
func LoadAll(userDir string) Registry {
	themes := make(map[string]Theme)
	registry := Registry{}
	loadBuiltin(themes, &registry.Warnings)
	if userDir != "" {
		loadUserDir(userDir, themes, &registry.Warnings)
	}

	registry.Themes = slices.Collect(maps.Values(themes))
	slices.SortFunc(registry.Themes, func(a, b Theme) int {
		if order := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); order != 0 {
			return order
		}
		return cmp.Compare(a.Name, b.Name)
	})
	return registry
}

// Resolve returns the terminal default for an empty value, default, or
// default-dark, otherwise it matches a registry name case-insensitively.
func (r Registry) Resolve(name string) (Theme, bool) {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, DefaultConfigName) || strings.EqualFold(name, LegacyDefaultConfigName) {
		return Default(), true
	}
	for _, item := range r.Themes {
		if strings.EqualFold(item.Name, name) {
			return item, true
		}
	}
	return Theme{}, false
}

func loadBuiltin(themes map[string]Theme, warnings *[]error) {
	entries, err := fs.ReadDir(builtinThemes, "themes")
	if err != nil {
		*warnings = append(*warnings, fmt.Errorf("read built-in theme directory: %w", err))
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		path := "themes/" + entry.Name()
		file, err := builtinThemes.Open(path)
		if err != nil {
			*warnings = append(*warnings, fmt.Errorf("open theme %q: %w", path, err))
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".toml")
		item, parseErr := Parse(name, file)
		closeErr := file.Close()
		if err := errors.Join(parseErr, closeErr); err != nil {
			*warnings = append(*warnings, fmt.Errorf("load built-in theme %q: %w", path, err))
			continue
		}
		themes[strings.ToLower(name)] = item
	}
}

func loadUserDir(dir string, themes map[string]Theme, warnings *[]error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		*warnings = append(*warnings, fmt.Errorf("read custom theme directory %q: %w", dir, err))
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := os.Open(path)
		if err != nil {
			*warnings = append(*warnings, fmt.Errorf("open custom theme %q: %w", path, err))
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".toml")
		item, parseErr := Parse(name, file)
		closeErr := file.Close()
		if err := errors.Join(parseErr, closeErr); err != nil {
			*warnings = append(*warnings, fmt.Errorf("load custom theme %q: %w", path, err))
			continue
		}
		themes[strings.ToLower(name)] = item
	}
}
