package theme

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDefaultIsExplicitTerminalPalette(t *testing.T) {
	want := Theme{
		Name:     "Default - Terminal colors",
		Accent:   "11",
		BrightFG: "15",
		FG:       "7",
		Green:    "10",
		Yellow:   "11",
		Red:      "9",
	}
	if got := Default(); got != want {
		t.Fatalf("Default() = %+v, want %+v", got, want)
	}
	if !want.IsDefault() {
		t.Fatal("explicit terminal palette was not recognized as default")
	}
	if (Theme{Name: DefaultName}).IsDefault() {
		t.Fatal("an empty palette must not implicitly represent the default")
	}
}

func TestParse(t *testing.T) {
	input := `
# full-line comment
accent = '#123'
bright_fg = "#ABCDEF"
fg = "#234"
ignored = "anything"
malformed line
green = '#345678'
yellow = "#456"
red = '#56789a'
`
	got, err := Parse("custom", strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	want := Theme{
		Name:     "custom",
		Accent:   "#123",
		BrightFG: "#ABCDEF",
		FG:       "#234",
		Green:    "#345678",
		Yellow:   "#456",
		Red:      "#56789a",
	}
	if got != want {
		t.Fatalf("Parse() = %+v, want %+v", got, want)
	}
}

func TestParseUsesLastRecognizedValue(t *testing.T) {
	input := completeTheme("#111", "#222", "#333", "#444", "#555", "#666") + `accent = "#abc"` + "\n"
	got, err := Parse("duplicate", strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if got.Accent != "#abc" {
		t.Fatalf("Accent = %q, want last value #abc", got.Accent)
	}
}

func TestParseRejectsIncompleteAndInvalidColors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", "missing accent"},
		{"missing one", completeTheme("#111", "#222", "#333", "#444", "#555", ""), "missing red"},
		{"missing hash", completeTheme("123", "#222", "#333", "#444", "#555", "#666"), "accent must be"},
		{"wrong short length", completeTheme("#12", "#222", "#333", "#444", "#555", "#666"), "accent must be"},
		{"wrong long length", completeTheme("#12345", "#222", "#333", "#444", "#555", "#666"), "accent must be"},
		{"non-hex short", completeTheme("#12g", "#222", "#333", "#444", "#555", "#666"), "accent must be"},
		{"non-hex long", completeTheme("#12345z", "#222", "#333", "#444", "#555", "#666"), "accent must be"},
		{"trailing comment", completeTheme("#111 # no inline comments", "#222", "#333", "#444", "#555", "#666"), "accent must be"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse("invalid", strings.NewReader(test.input))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Parse() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestParseReturnsReaderError(t *testing.T) {
	want := errors.New("read failed")
	_, err := Parse("broken", errorReader{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("Parse() error = %v, want wrapping %v", err, want)
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

var _ io.Reader = errorReader{}

func completeTheme(accent, brightFG, fg, green, yellow, red string) string {
	return "accent = \"" + accent + "\"\n" +
		"bright_fg = \"" + brightFG + "\"\n" +
		"fg = \"" + fg + "\"\n" +
		"green = \"" + green + "\"\n" +
		"yellow = \"" + yellow + "\"\n" +
		"red = \"" + red + "\"\n"
}
