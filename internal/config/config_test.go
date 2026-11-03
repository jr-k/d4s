package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeConfigFile(t, filepath.Join(ConfigDir(), "config.yaml"), "d4s:\n  readOnly: true\n")
	path := filepath.Join(t.TempDir(), "production.yaml")
	writeConfigFile(t, path, "d4s:\n  refreshRate: 1\n  readOnly: false\n  ui:\n    skin: dracula\n")

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.D4S.ReadOnly || cfg.D4S.RefreshRate != 2 || cfg.D4S.UI.Skin != "dracula" {
		t.Fatalf("unexpected settings: %+v", cfg.D4S)
	}
	if cfg.D4S.APIServerTimeout != "120s" || cfg.D4S.Logger.Tail != 100 {
		t.Fatalf("omitted settings did not retain defaults: %+v", cfg.D4S)
	}
}

func TestLoadFileErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeConfigFile(t, filepath.Join(ConfigDir(), "config.yaml"), "d4s:\n  readOnly: true\n")
	dir := t.TempDir()
	invalidPath := filepath.Join(dir, "invalid.yaml")
	writeConfigFile(t, invalidPath, "d4s: [")
	wrongTypePath := filepath.Join(dir, "wrong-type.yaml")
	writeConfigFile(t, wrongTypePath, "d4s:\n  readOnly: not-a-boolean\n")
	missingPath := filepath.Join(dir, "missing.yaml")
	// An explicit missing .yaml file must not fall back to .yml.
	writeConfigFile(t, filepath.Join(dir, "missing.yml"), "d4s: {}\n")

	for _, test := range []struct {
		name string
		path string
	}{
		{"empty path", ""},
		{"missing file", missingPath},
		{"directory", dir},
		{"invalid YAML", invalidPath},
		{"invalid field type", wrongTypePath},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := LoadFile(test.path)
			if err == nil || cfg != nil {
				t.Fatalf("LoadFile(%q) = (%v, %v), want nil config and error", test.path, cfg, err)
			}
			if !strings.Contains(err.Error(), test.path) {
				t.Fatalf("error does not identify selected path: %v", err)
			}
		})
	}
}

func TestSaveExplicitFile(t *testing.T) {
	xdgRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgRoot)
	defaultPath := filepath.Join(ConfigDir(), "config.yaml")
	defaultData := "d4s:\n  defaultContext: original\n"
	writeConfigFile(t, defaultPath, defaultData)

	workingDir := t.TempDir()
	t.Chdir(workingDir)
	writeConfigFile(t, "production.yml", "d4s:\n  readOnly: true\n")
	cfg, err := LoadFile("production.yml")
	if err != nil {
		t.Fatal(err)
	}
	// Context switching saves a shallow copy of the active configuration.
	saved := *cfg
	saved.D4S.DefaultContext = "hbx-prod"
	t.Chdir(t.TempDir())
	if err := Save(&saved); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(workingDir, "production.yml")
	reloaded, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.D4S.DefaultContext != "hbx-prod" || !reloaded.D4S.ReadOnly {
		t.Fatalf("settings not saved to selected file: %+v", reloaded.D4S)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "filepath:") || strings.Contains(string(data), path) {
		t.Fatal("internal file path was serialized")
	}
	data, err = os.ReadFile(defaultPath)
	if err != nil || string(data) != defaultData {
		t.Fatalf("default configuration was modified: %q, %v", data, err)
	}
	if ConfigDir() != filepath.Join(xdgRoot, "d4s") || LogsDir() != filepath.Join(xdgRoot, "d4s", "logs") {
		t.Fatal("explicit config changed resource directories")
	}
}

func TestDefaultDiscovery(t *testing.T) {
	for _, test := range []struct {
		name         string
		yaml         string
		yml          string
		wantReadOnly bool
	}{
		{name: "missing files"},
		{name: "yaml takes precedence", yaml: "d4s:\n  readOnly: true\n", yml: "d4s:\n  readOnly: false\n", wantReadOnly: true},
		{name: "yml fallback", yml: "d4s:\n  readOnly: true\n", wantReadOnly: true},
		{name: "invalid yaml uses defaults", yaml: "d4s: [", yml: "d4s:\n  readOnly: true\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			if test.yaml != "" {
				writeConfigFile(t, filepath.Join(ConfigDir(), "config.yaml"), test.yaml)
			}
			if test.yml != "" {
				writeConfigFile(t, filepath.Join(ConfigDir(), "config.yml"), test.yml)
			}
			cfg := Load()
			if cfg.D4S.ReadOnly != test.wantReadOnly {
				t.Fatalf("readOnly = %v, want %v", cfg.D4S.ReadOnly, test.wantReadOnly)
			}
			cfg.D4S.DefaultContext = "saved"
			if err := Save(cfg); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(ConfigDir(), "config.yaml"))
			if err != nil || !strings.Contains(string(data), "defaultContext: saved") {
				t.Fatalf("default save did not write config.yaml: %q, %v", data, err)
			}
		})
	}
}

func TestSaveNil(t *testing.T) {
	if err := Save(nil); err == nil {
		t.Fatal("Save(nil) must return an error")
	}
}
