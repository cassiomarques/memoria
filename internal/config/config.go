package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	NotesDir      string `yaml:"notes_dir"`
	GitRemote     string `yaml:"git_remote,omitempty"`
	Editor        string `yaml:"editor,omitempty"`
	ExpandFolders *bool  `yaml:"expand_folders,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	expandDefault := true
	return &Config{
		NotesDir:      filepath.Join(DefaultConfigDir(), "notes"),
		ExpandFolders: &expandDefault,
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

// ResolveExpandFolders returns whether folders should start expanded.
// Defaults to true if not set.
func (c *Config) ResolveExpandFolders() bool {
	if c.ExpandFolders == nil {
		return true
	}
	return *c.ExpandFolders
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
// If overrideDir is set (via SetConfigDir), it returns that instead.
func DefaultConfigDir() string {
	if configDirOverride != "" {
		return configDirOverride
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".memoria")
}

// SetConfigDir overrides the default config directory.
// Call this before any other config functions.
func SetConfigDir(dir string) error {
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dir = filepath.Join(home, dir[1:])
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	configDirOverride = abs
	return nil
}

var configDirOverride string

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

// AcquireLock tries to acquire an exclusive instance lock for the given config
// directory. It writes the current PID to a lock file. If another living
// process already holds the lock, it returns a descriptive error.
// The returned function releases the lock and should be deferred by the caller.
func AcquireLock(configDir string) (release func(), err error) {
	lockPath := filepath.Join(configDir, "memoria.lock")

	// Check for an existing lock
	if data, readErr := os.ReadFile(lockPath); readErr == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 && pid != os.Getpid() && isProcessAlive(pid) {
			return nil, fmt.Errorf(
				"memoria is already running in another terminal (PID %d)\n"+
					"If this is wrong, remove the lock file: %s",
				pid, lockPath,
			)
		}
	}

	// Write our PID
	if writeErr := os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())), 0644); writeErr != nil {
		return nil, fmt.Errorf("could not create lock file: %w", writeErr)
	}

	release = func() { os.Remove(lockPath) }
	return release, nil
}

// isProcessAlive checks whether a process with the given PID is still running.
func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 doesn't actually send a signal — it just checks if the process exists.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}
