package viewer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ConfigFileName is stored inside the log folder.
const ConfigFileName = "log-viewer.json"

// LoadConfig reads (or creates) log-viewer.json inside folder and applies the
// overrides. refresh <= 0 and an empty theme mean "keep the saved value".
// It returns the config and whether the file was written.
func LoadConfig(folder string, refresh int, theme string) (Config, bool, error) {
	abs, err := filepath.Abs(folder)
	if err != nil {
		return Config{}, false, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return Config{}, false, err
	}
	if !info.IsDir() {
		return Config{}, false, fmt.Errorf("%s is not a directory", abs)
	}
	if theme != "" && theme != "light" && theme != "dark" {
		return Config{}, false, fmt.Errorf("invalid theme %q (use light or dark)", theme)
	}

	path := filepath.Join(abs, ConfigFileName)
	cfg := Config{RefreshInterval: 5, Theme: "light"}
	data, err := os.ReadFile(path)
	exists := err == nil
	switch {
	case exists:
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, false, fmt.Errorf("parse %s: %w", path, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return Config{}, false, err
	}

	before := cfg
	cfg.LogFolder = abs
	if refresh > 0 {
		cfg.RefreshInterval = refresh
	}
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = 5
	}
	if theme != "" {
		cfg.Theme = theme
	}
	if cfg.Theme != "light" && cfg.Theme != "dark" {
		cfg.Theme = "light"
	}

	if exists && cfg == before {
		return cfg, false, nil
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return Config{}, false, err
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o600); err != nil {
		return Config{}, false, err
	}
	return cfg, true, nil
}
