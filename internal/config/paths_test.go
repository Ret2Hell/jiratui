package config

import (
	"path/filepath"
	"testing"
)

func TestPathExplicitOverridesEnvironmentAndBecomesAbsolute(t *testing.T) {
	t.Setenv("JIRATUI_CONFIG_FILE", filepath.Join(t.TempDir(), "environment.yaml"))
	explicit := filepath.Join("relative", "config.yaml")
	want, err := filepath.Abs(explicit)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Path(explicit)
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestPathUsesConfiguredFile(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom.yaml")
	t.Setenv("JIRATUI_CONFIG_FILE", want)

	got, err := Path("")
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestPathUsesXDGDefault(t *testing.T) {
	base := t.TempDir()
	t.Setenv("JIRATUI_CONFIG_FILE", "")
	t.Setenv("XDG_CONFIG_HOME", base)

	got, err := Path("")
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	want := filepath.Join(base, AppName, "config.yaml")
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestThemeDirAdjacentToExplicitConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "profile", "settings.yaml")
	got, err := ThemeDir(configPath)
	if err != nil {
		t.Fatalf("ThemeDir() error = %v", err)
	}
	want := filepath.Join(filepath.Dir(configPath), "themes")
	if got != want {
		t.Fatalf("ThemeDir() = %q, want %q", got, want)
	}
}

func TestThemeDirEmptyUsesResolvedConfig(t *testing.T) {
	base := t.TempDir()
	t.Setenv("JIRATUI_CONFIG_FILE", "")
	t.Setenv("XDG_CONFIG_HOME", base)

	got, err := ThemeDir("")
	if err != nil {
		t.Fatalf("ThemeDir() error = %v", err)
	}
	want := filepath.Join(base, AppName, "themes")
	if got != want {
		t.Fatalf("ThemeDir() = %q, want %q", got, want)
	}
}

func TestThemeDirEmptyUsesConfiguredFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "alternate", "jiratui.yml")
	t.Setenv("JIRATUI_CONFIG_FILE", configPath)

	got, err := ThemeDir("")
	if err != nil {
		t.Fatalf("ThemeDir() error = %v", err)
	}
	want := filepath.Join(filepath.Dir(configPath), "themes")
	if got != want {
		t.Fatalf("ThemeDir() = %q, want %q", got, want)
	}
}

func TestStateAndCacheDirsHonorXDG(t *testing.T) {
	stateBase := t.TempDir()
	cacheBase := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateBase)
	t.Setenv("XDG_CACHE_HOME", cacheBase)

	state, err := StateDir()
	if err != nil {
		t.Fatalf("StateDir() error = %v", err)
	}
	if want := filepath.Join(stateBase, AppName); state != want {
		t.Fatalf("StateDir() = %q, want %q", state, want)
	}
	cache, err := CacheDir()
	if err != nil {
		t.Fatalf("CacheDir() error = %v", err)
	}
	if want := filepath.Join(cacheBase, AppName); cache != want {
		t.Fatalf("CacheDir() = %q, want %q", cache, want)
	}
}
