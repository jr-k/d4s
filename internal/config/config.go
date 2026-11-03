package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	D4S D4SConfig `yaml:"d4s"`

	// filePath is set only for explicitly selected files, so Save preserves
	// the default discovery behavior for configurations loaded with Load.
	filePath string
}

type D4SConfig struct {
	RefreshRate      float64 `yaml:"refreshRate"`
	APIServerTimeout string  `yaml:"apiServerTimeout"`
	ReadOnly         bool    `yaml:"readOnly"`
	DefaultContext   string  `yaml:"defaultContext"`
	DefaultView      string  `yaml:"defaultView"`
	NoExitOnCtrlC    bool    `yaml:"noExitOnCtrlC"`

	UI    UIConfig              `yaml:"ui"`
	Views map[string]ViewConfig `yaml:"views,omitempty"`

	SkipLatestRevCheck bool `yaml:"skipLatestRevCheck"`

	Logger   LoggerConfig   `yaml:"logger"`
	ShellPod ShellPodConfig `yaml:"shellPod"`
}

type UIConfig struct {
	EnableMouse bool   `yaml:"enableMouse"`
	Headless    bool   `yaml:"headless"`
	Logoless    bool   `yaml:"logoless"`
	Crumbsless  bool   `yaml:"crumbsless"`
	Invert      bool   `yaml:"invert"`
	Skin        string `yaml:"skin"`
}

type ViewConfig struct {
	Columns []string `yaml:"columns,omitempty"`
}

type LoggerConfig struct {
	Tail              int  `yaml:"tail"`
	SinceSeconds      int  `yaml:"sinceSeconds"`
	TextWrap          bool `yaml:"textWrap"`
	DisableAutoscroll bool `yaml:"disableAutoscroll"`
	ShowTime          bool `yaml:"showTime"`
}

type ShellPodConfig struct {
	Image string `yaml:"image"`
}

// GetAPIServerTimeout parses the apiServerTimeout string into a time.Duration.
func (c *D4SConfig) GetAPIServerTimeout() time.Duration {
	if c.APIServerTimeout == "" {
		return 120 * time.Second
	}
	d, err := time.ParseDuration(c.APIServerTimeout)
	if err != nil {
		return 120 * time.Second
	}
	return d
}

// GetRefreshInterval returns the refresh rate as a time.Duration, enforcing a 2s minimum.
func (c *D4SConfig) GetRefreshInterval() time.Duration {
	rate := c.RefreshRate
	if rate < 2.0 {
		rate = 2.0
	}
	return time.Duration(rate * float64(time.Second))
}

// GetLogSince returns the "since" parameter for log streaming.
// A value of -1 means tail (no since filter), 0 or positive is seconds.
func (c *LoggerConfig) GetLogSince() string {
	if c.SinceSeconds < 0 {
		return ""
	}
	return fmt.Sprintf("%ds", c.SinceSeconds)
}

// GetLogSinceLabel returns a human-readable label for the since config.
func (c *LoggerConfig) GetLogSinceLabel() string {
	if c.SinceSeconds < 0 {
		return "Tail"
	}
	if c.SinceSeconds == 0 {
		return "All"
	}
	d := time.Duration(c.SinceSeconds) * time.Second
	if d >= time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	if d >= time.Minute {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%ds", c.SinceSeconds)
}

// GetLogTail returns the tail line count as a string.
func (c *LoggerConfig) GetLogTail() string {
	if c.Tail <= 0 {
		return "100"
	}
	return fmt.Sprintf("%d", c.Tail)
}

// DefaultConfig returns a Config with all default values applied.
func DefaultConfig() *Config {
	return &Config{
		D4S: D4SConfig{
			RefreshRate:      2.0,
			APIServerTimeout: "120s",
			ReadOnly:         false,
			DefaultContext:   "",
			DefaultView:      "",
			NoExitOnCtrlC:    false,
			UI: UIConfig{
				EnableMouse: false,
				Headless:    false,
				Logoless:    false,
				Crumbsless:  false,
				Invert:      false,
			},
			SkipLatestRevCheck: false,
			Logger: LoggerConfig{
				Tail:              100,
				SinceSeconds:      -1,
				TextWrap:          false,
				DisableAutoscroll: false,
				ShowTime:          false,
			},
			ShellPod: ShellPodConfig{
				Image: "ghcr.io/jr-k/nget:latest",
			},
		},
	}
}

// configDir returns the d4s config directory, respecting XDG_CONFIG_HOME.
func configDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "d4s")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "d4s")
}

// ConfigDir returns the d4s config directory, respecting XDG_CONFIG_HOME.
func ConfigDir() string {
	return configDir()
}

// LogsDir returns the directory used for d4s log exports.
func LogsDir() string {
	dir := configDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "logs")
}

// ensureConfigDirs creates the config directory and skins subdirectory if they don't exist.
func ensureConfigDirs() {
	dir := configDir()
	if dir == "" {
		return
	}
	_ = os.MkdirAll(filepath.Join(dir, "skins"), 0o755)
}

// Load reads the config from $XDG_CONFIG_HOME/d4s/config.yaml.
// If the file doesn't exist or can't be parsed, defaults are returned.
func Load() *Config {
	cfg := DefaultConfig()

	dir := configDir()
	if dir == "" {
		return cfg
	}

	ensureConfigDirs()

	configPath := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Try config.yml as fallback
		configPath = filepath.Join(dir, "config.yml")
		data, err = os.ReadFile(configPath)
		if err != nil {
			return cfg
		}
	}

	parsed, err := parseConfig(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "d4s: warning: failed to parse %s: %v\n", configPath, err)
		return cfg
	}

	return parsed
}

// LoadFile reads exactly the specified file, returning an error if it cannot be
// read or parsed. Save writes subsequent changes back to the same file.
func LoadFile(path string) (*Config, error) {
	if path == "" {
		return nil, errors.New("configuration file path cannot be empty")
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve configuration file %q: %w", path, err)
	}
	data, err := os.ReadFile(absolutePath)
	if err != nil {
		return nil, fmt.Errorf("read configuration file %q: %w", path, err)
	}
	cfg, err := parseConfig(data)
	if err != nil {
		return nil, fmt.Errorf("parse configuration file %q: %w", path, err)
	}
	cfg.filePath = absolutePath
	return cfg, nil
}

func parseConfig(data []byte) (*Config, error) {
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Enforce minimum refresh rate.
	if cfg.D4S.RefreshRate < 2.0 && cfg.D4S.RefreshRate > 0 {
		cfg.D4S.RefreshRate = 2.0
	}
	return cfg, nil
}

// Save writes the config back to the explicitly selected file, or to
// $XDG_CONFIG_HOME/d4s/config.yaml when default discovery was used.
func Save(cfg *Config) error {
	if cfg == nil {
		return errors.New("nil config")
	}

	configPath := cfg.filePath
	if configPath == "" {
		dir := configDir()
		if dir == "" {
			return errors.New("unable to determine config directory")
		}
		ensureConfigDirs()
		configPath = filepath.Join(dir, "config.yaml")
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0o644)
}
