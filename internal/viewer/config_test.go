package viewer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadConfigCreatesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, written, err := LoadConfig(dir, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if !written {
		t.Fatal("expected config file to be written")
	}
	if cfg.RefreshInterval != 5 || cfg.Theme != "light" {
		t.Fatalf("unexpected defaults %+v", cfg)
	}
	info, err := os.Stat(filepath.Join(dir, ConfigFileName))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("config file mode %v, want 0600", info.Mode().Perm())
	}
}

func TestLoadConfigOverridesAndPersists(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := LoadConfig(dir, 10, "dark"); err != nil {
		t.Fatal(err)
	}
	cfg, written, err := LoadConfig(dir, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if written {
		t.Error("unchanged config should not be rewritten")
	}
	if cfg.RefreshInterval != 10 || cfg.Theme != "dark" {
		t.Fatalf("overrides not persisted: %+v", cfg)
	}

	var onDisk Config
	b, _ := os.ReadFile(filepath.Join(dir, ConfigFileName))
	if err := json.Unmarshal(b, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk != cfg {
		t.Errorf("disk %+v != loaded %+v", onDisk, cfg)
	}
}

func TestLoadConfigValidation(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := LoadConfig(dir, 0, "neon"); err == nil {
		t.Error("expected error for invalid theme")
	}
	if _, _, err := LoadConfig(filepath.Join(dir, "missing"), 0, ""); err == nil {
		t.Error("expected error for missing folder")
	}
	file := filepath.Join(dir, "file.log")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadConfig(file, 0, ""); err == nil {
		t.Error("expected error when folder is a file")
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFileName), []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadConfig(dir, 0, ""); err == nil {
		t.Error("expected error for corrupt config")
	}
}
