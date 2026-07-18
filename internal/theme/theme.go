// Package theme loads cliamp-compatible color themes.
package theme

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	// DefaultName is the display name of the terminal-palette theme.
	DefaultName = "Default - Terminal colors"
	// DefaultConfigName is the canonical configuration value for the terminal-palette theme.
	DefaultConfigName = "default"
	// LegacyDefaultConfigName is the former configuration value accepted as an alias for default.
	LegacyDefaultConfigName = "default-dark"
)

// Theme is a named cliamp-compatible six-color palette.
type Theme struct {
	Name     string
	Accent   string
	BrightFG string
	FG       string
	Green    string
	Yellow   string
	Red      string
}

// Default returns the explicit ANSI terminal-palette theme.
func Default() Theme {
	return Theme{
		Name:     DefaultName,
		Accent:   "11",
		BrightFG: "15",
		FG:       "7",
		Green:    "10",
		Yellow:   "11",
		Red:      "9",
	}
}

// IsDefault reports whether t is the terminal-palette theme.
func (t Theme) IsDefault() bool {
	return t == Default()
}

// Parse reads and validates the six flat keys in a cliamp theme file.
// Blank lines, full-line comments, unknown keys, and lines without an equals
// sign are ignored for compatibility with cliamp.
func Parse(name string, r io.Reader) (Theme, error) {
	t := Theme{Name: name}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		switch key {
		case "accent":
			t.Accent = value
		case "bright_fg":
			t.BrightFG = value
		case "fg":
			t.FG = value
		case "green":
			t.Green = value
		case "yellow":
			t.Yellow = value
		case "red":
			t.Red = value
		}
	}
	if err := scanner.Err(); err != nil {
		return Theme{}, fmt.Errorf("read theme %q: %w", name, err)
	}
	if err := validate(t); err != nil {
		return Theme{}, fmt.Errorf("validate theme %q: %w", name, err)
	}
	return t, nil
}

func validate(t Theme) error {
	colors := []struct {
		key   string
		value string
	}{
		{"accent", t.Accent},
		{"bright_fg", t.BrightFG},
		{"fg", t.FG},
		{"green", t.Green},
		{"yellow", t.Yellow},
		{"red", t.Red},
	}

	var errs []error
	for _, color := range colors {
		if color.value == "" {
			errs = append(errs, fmt.Errorf("missing %s", color.key))
			continue
		}
		if !validHexColor(color.value) {
			errs = append(errs, fmt.Errorf("%s must be #RGB or #RRGGBB, got %q", color.key, color.value))
		}
	}
	return errors.Join(errs...)
}

func validHexColor(value string) bool {
	if len(value) != 4 && len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, char := range value[1:] {
		if char < '0' || char > '9' {
			if char < 'a' || char > 'f' {
				if char < 'A' || char > 'F' {
					return false
				}
			}
		}
	}
	return true
}
