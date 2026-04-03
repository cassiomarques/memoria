package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	expectedNotesDir := filepath.Join(home, ".memoria", "notes")
	if cfg.NotesDir != expectedNotesDir {
		t.Errorf("NotesDir = %q, want %q", cfg.NotesDir, expectedNotesDir)
	}
	if cfg.GitRemote != "" {
		t.Errorf("GitRemote = %q, want empty string", cfg.GitRemote)
	}
	if cfg.Editor != "" {
		t.Errorf("Editor = %q, want empty string", cfg.Editor)
	}
}

func TestLoadNonExistentFile(t *testing.T) {
	cfg, err := Load("/no/such/path/config.yaml")
	if err != nil {
		t.Fatalf("Load non-existent file returned error: %v", err)
	}

	defaultCfg := DefaultConfig()
	if cfg.NotesDir != defaultCfg.NotesDir {
		t.Errorf("NotesDir = %q, want default %q", cfg.NotesDir, defaultCfg.NotesDir)
	}
	if cfg.GitRemote != defaultCfg.GitRemote {
		t.Errorf("GitRemote = %q, want default %q", cfg.GitRemote, defaultCfg.GitRemote)
	}
	if cfg.Editor != defaultCfg.Editor {
		t.Errorf("Editor = %q, want default %q", cfg.Editor, defaultCfg.Editor)
	}
}

func TestLoadValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := []byte("notes_dir: /custom/notes\ngit_remote: git@github.com:user/repo.git\neditor: nvim\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.NotesDir != "/custom/notes" {
		t.Errorf("NotesDir = %q, want %q", cfg.NotesDir, "/custom/notes")
	}
	if cfg.GitRemote != "git@github.com:user/repo.git" {
		t.Errorf("GitRemote = %q, want %q", cfg.GitRemote, "git@github.com:user/repo.git")
	}
	if cfg.Editor != "nvim" {
		t.Errorf("Editor = %q, want %q", cfg.Editor, "nvim")
	}
}

func TestLoadMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	content := []byte(":\n  :\n  invalid: [yaml\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load malformed YAML should return error, got nil")
	}
}

func TestSaveThenLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")

	original := &Config{
		NotesDir:  "/my/notes",
		GitRemote: "git@github.com:user/repo.git",
		Editor:    "code",
	}

	if err := original.Save(path); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if loaded.NotesDir != original.NotesDir {
		t.Errorf("NotesDir = %q, want %q", loaded.NotesDir, original.NotesDir)
	}
	if loaded.GitRemote != original.GitRemote {
		t.Errorf("GitRemote = %q, want %q", loaded.GitRemote, original.GitRemote)
	}
	if loaded.Editor != original.Editor {
		t.Errorf("Editor = %q, want %q", loaded.Editor, original.Editor)
	}
}

func TestSaveCreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "config.yaml")

	cfg := DefaultConfig()
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save should create parent dirs, got error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}
}

func TestSaveOmitsEmptyOptionalFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &Config{
		NotesDir:  "/notes",
		GitRemote: "",
		Editor:    "",
	}

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}

	content := string(data)
	if strings.Contains(content, "git_remote") {
		t.Error("saved YAML should omit empty git_remote")
	}
	if strings.Contains(content, "editor") {
		t.Error("saved YAML should omit empty editor")
	}
}

func TestResolveEditor(t *testing.T) {
	tests := []struct {
		name     string
		editor   string
		envEditor  string
		envVisual  string
		want     string
	}{
		{
			name:   "configured editor takes precedence",
			editor: "code",
			envEditor: "vim",
			envVisual: "nano",
			want:   "code",
		},
		{
			name:   "falls back to EDITOR env var",
			editor: "",
			envEditor: "emacs",
			envVisual: "",
			want:   "emacs",
		},
		{
			name:   "falls back to VISUAL env var",
			editor: "",
			envEditor: "",
			envVisual: "gedit",
			want:   "gedit",
		},
		{
			name:   "falls back to vim",
			editor: "",
			envEditor: "",
			envVisual: "",
			want:   "vim",
		},
		{
			name:   "EDITOR takes precedence over VISUAL",
			editor: "",
			envEditor: "emacs",
			envVisual: "gedit",
			want:   "emacs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("EDITOR", tt.envEditor)
			t.Setenv("VISUAL", tt.envVisual)

			cfg := &Config{Editor: tt.editor}
			got := cfg.ResolveEditor()
			if got != tt.want {
				t.Errorf("ResolveEditor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveNotesDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	tests := []struct {
		name    string
		dir     string
		want    string
		wantErr bool
	}{
		{
			name: "expands tilde",
			dir:  "~/mynotes",
			want: filepath.Join(home, "mynotes"),
		},
		{
			name: "expands tilde alone",
			dir:  "~",
			want: home,
		},
		{
			name: "absolute path unchanged",
			dir:  "/absolute/path",
			want: "/absolute/path",
		},
		{
			name: "tilde with nested path",
			dir:  "~/.memoria/notes",
			want: filepath.Join(home, ".memoria", "notes"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{NotesDir: tt.dir}
			got, err := cfg.ResolveNotesDir()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ResolveNotesDir() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ResolveNotesDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultConfigDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	got := DefaultConfigDir()
	want := filepath.Join(home, ".memoria")
	if got != want {
		t.Errorf("DefaultConfigDir() = %q, want %q", got, want)
	}
}

func TestDefaultConfigPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	got := DefaultConfigPath()
	want := filepath.Join(home, ".memoria", "config.yaml")
	if got != want {
		t.Errorf("DefaultConfigPath() = %q, want %q", got, want)
	}
}

func TestEnsureDirs(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		NotesDir: filepath.Join(dir, "notes", "sub"),
	}

	if err := EnsureDirs(cfg); err != nil {
		t.Fatalf("EnsureDirs returned error: %v", err)
	}

	// Check notes dir was created
	info, err := os.Stat(cfg.NotesDir)
	if err != nil {
		t.Fatalf("notes dir was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("notes dir is not a directory")
	}
}

func TestEnsureDirsExpandsTilde(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		NotesDir: filepath.Join(dir, "tildetest", "notes"),
	}

	if err := EnsureDirs(cfg); err != nil {
		t.Fatalf("EnsureDirs returned error: %v", err)
	}

	if _, err := os.Stat(cfg.NotesDir); os.IsNotExist(err) {
		t.Fatal("EnsureDirs did not create the notes directory")
	}
}
