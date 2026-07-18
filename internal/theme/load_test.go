package theme

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadAllBuiltins(t *testing.T) {
	registry := LoadAll("")
	wantNames := []string{
		"ayu-mirage-dark", "catppuccin", "catppuccin-latte", "dracula", "ember",
		"ethereal", "everforest", "flexoki-light", "gruvbox", "hackerman",
		"kanagawa", "matte-black", "miasma", "neon-blade-runner", "nord",
		"osaka-jade", "ristretto", "rose-pine", "tokyo-night", "vantablack",
	}
	if len(registry.Warnings) != 0 {
		t.Fatalf("LoadAll() warnings = %v", registry.Warnings)
	}
	if got := themeNames(registry.Themes); !slices.Equal(got, wantNames) {
		t.Fatalf("built-in names = %q, want %q", got, wantNames)
	}

	dracula, ok := registry.Resolve("dracula")
	if !ok || dracula != (Theme{"dracula", "#bd93f9", "#f8f8f2", "#6272a4", "#50fa7b", "#f1fa8c", "#ff5555"}) {
		t.Fatalf("dracula = %+v, %v", dracula, ok)
	}
	flexoki, ok := registry.Resolve("flexoki-light")
	if !ok || flexoki.Accent != "#205EA6" || flexoki.Red != "#D14D41" {
		t.Fatalf("flexoki-light = %+v, %v", flexoki, ok)
	}
}

func TestLoadAllAddsAndSortsUserThemesCaseInsensitively(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "zebra.toml", "#111")
	writeTheme(t, dir, "Aardvark.toml", "#222")

	registry := LoadAll(dir)
	if len(registry.Warnings) != 0 {
		t.Fatalf("LoadAll() warnings = %v", registry.Warnings)
	}
	names := themeNames(registry.Themes)
	if names[0] != "Aardvark" || names[len(names)-1] != "zebra" {
		t.Fatalf("user themes not sorted case-insensitively: %q", names)
	}
	if got, ok := registry.Resolve("aArDvArK"); !ok || got.Accent != "#222" {
		t.Fatalf("Resolve(Aardvark) = %+v, %v", got, ok)
	}
}

func TestLoadAllUserThemeOverridesBuiltinCaseInsensitively(t *testing.T) {
	dir := t.TempDir()
	writeTheme(t, dir, "DRACULA.toml", "#123")

	registry := LoadAll(dir)
	if len(registry.Themes) != 20 {
		t.Fatalf("theme count = %d, want 20 after override", len(registry.Themes))
	}
	got, ok := registry.Resolve("dracula")
	if !ok || got.Name != "DRACULA" || got.Accent != "#123" {
		t.Fatalf("Resolve(dracula) = %+v, %v", got, ok)
	}
}

func TestLoadAllWarnsAndSkipsInvalidCustomTheme(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "dracula.toml"), []byte(`accent = "#xyz"`), 0o600); err != nil {
		t.Fatal(err)
	}

	registry := LoadAll(dir)
	if len(registry.Warnings) != 1 || !strings.Contains(registry.Warnings[0].Error(), "dracula.toml") {
		t.Fatalf("warnings = %v, want invalid dracula warning", registry.Warnings)
	}
	got, ok := registry.Resolve("dracula")
	if !ok || got.Accent != "#bd93f9" {
		t.Fatalf("invalid override displaced built-in: %+v, %v", got, ok)
	}
}

func TestLoadAllWarnsForUnreadableCustomFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.toml")
	if err := os.Symlink(filepath.Join(dir, "missing-target"), path); err != nil {
		t.Fatal(err)
	}

	registry := LoadAll(dir)
	if len(registry.Warnings) != 1 || !strings.Contains(registry.Warnings[0].Error(), "broken.toml") {
		t.Fatalf("warnings = %v, want unreadable file warning", registry.Warnings)
	}
	if _, ok := registry.Resolve("broken"); ok {
		t.Fatal("unreadable custom theme was selectable")
	}
}

func TestLoadAllIgnoresNonTOMLAndDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("not a theme"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested.toml"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "UPPER.TOML"), []byte("not loaded"), 0o600); err != nil {
		t.Fatal(err)
	}

	registry := LoadAll(dir)
	if len(registry.Themes) != 20 || len(registry.Warnings) != 0 {
		t.Fatalf("LoadAll() = %d themes, warnings %v", len(registry.Themes), registry.Warnings)
	}
}

func TestLoadAllMissingUserDirectory(t *testing.T) {
	registry := LoadAll(filepath.Join(t.TempDir(), "missing"))
	if len(registry.Themes) != 20 || len(registry.Warnings) != 0 {
		t.Fatalf("LoadAll(missing) = %d themes, warnings %v", len(registry.Themes), registry.Warnings)
	}
}

func TestResolve(t *testing.T) {
	registry := LoadAll("")
	for _, name := range []string{"", "   ", "default", "DEFAULT", "default-dark", "DEFAULT-DARK"} {
		t.Run(name, func(t *testing.T) {
			got, ok := registry.Resolve(name)
			if !ok || !got.IsDefault() {
				t.Fatalf("Resolve(%q) = %+v, %v, want default", name, got, ok)
			}
		})
	}
	if got, ok := registry.Resolve("ToKyO-NiGhT"); !ok || got.Name != "tokyo-night" {
		t.Fatalf("case-insensitive Resolve() = %+v, %v", got, ok)
	}
	if got, ok := registry.Resolve("tokyo"); ok || got != (Theme{}) {
		t.Fatalf("partial Resolve() = %+v, %v, want no match", got, ok)
	}
	if got, ok := registry.Resolve("unknown"); ok || got != (Theme{}) {
		t.Fatalf("unknown Resolve() = %+v, %v, want no match", got, ok)
	}
}

func writeTheme(t *testing.T, dir, name, accent string) {
	t.Helper()
	contents := completeTheme(accent, "#fff", "#aaa", "#0f0", "#ff0", "#f00")
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}

func themeNames(themes []Theme) []string {
	return slices.Collect(func(yield func(string) bool) {
		for _, item := range themes {
			if !yield(item.Name) {
				return
			}
		}
	})
}
