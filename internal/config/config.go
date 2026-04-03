package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	NotesDir  string `yaml:"notes_dir"`
	GitRemote string `yaml:"git_remote,omitempty"`
	Editor    string `yaml:"editor,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		NotesDir: filepath.Join(home, ".memoria", "notes"),
	}
}

// Load reads a Config from a YAML file at path.
// If the file does not exist, it returns DefaultConfig.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes the Config to a YAML file at path, creating parent directories as needed.
func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ResolveEditor returns the editor to use, checking in order:
// configured editor, $EDITOR, $VISUAL, "vim", "nano".
func (c *Config) ResolveEditor() string {
	if c.Editor != "" {
		return c.Editor
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	return "vim"
}

// ResolveNotesDir expands ~ in NotesDir and returns an absolute path.
func (c *Config) ResolveNotesDir() (string, error) {
	dir := c.NotesDir

	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, dir[1:])
	}

	return filepath.Abs(dir)
}

// DefaultConfigDir returns the default configuration directory (~/.memoria), expanded.
func DefaultConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".memoria")
}

// DefaultConfigPath returns the default configuration file path (~/.memoria/config.yaml), expanded.
func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.yaml")
}

// EnsureDirs creates the notes directory and config directory if they don't exist.
func EnsureDirs(cfg *Config) error {
	resolved, err := cfg.ResolveNotesDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(resolved, 0755); err != nil {
		return err
	}

	return os.MkdirAll(DefaultConfigDir(), 0755)
}
