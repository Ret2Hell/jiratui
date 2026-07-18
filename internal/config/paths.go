package config

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
)

// Path returns the config file path, honoring explicit paths and XDG_CONFIG_HOME.
func Path(explicit string) (string, error) {
	if configured := cmp.Or(explicit, os.Getenv("JIRATUI_CONFIG_FILE")); configured != "" {
		return filepath.Abs(configured)
	}
	base, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, AppName, "config.yaml"), nil
}

// ThemeDir returns the themes directory adjacent to the resolved config file.
func ThemeDir(configPath string) (string, error) {
	resolved, err := Path(configPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(resolved), "themes"), nil
}

// StateDir returns the directory for mutable state and logs.
func StateDir() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, AppName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home dir: %w", err)
	}
	return filepath.Join(home, ".local", "state", AppName), nil
}

// CacheDir returns the directory for cached API responses.
func CacheDir() (string, error) {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, AppName), nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("find cache dir: %w", err)
	}
	return filepath.Join(base, AppName), nil
}

func userConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return xdg, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find config dir: %w", err)
	}
	return base, nil
}
