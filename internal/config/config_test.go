package config

import (
	"os"
	"path/filepath"
	"strconv"
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
		name      string
		editor    string
		envEditor string
		envVisual string
		want      string
	}{
		{
			name:      "configured editor takes precedence",
			editor:    "code",
			envEditor: "vim",
			envVisual: "nano",
			want:      "code",
		},
		{
			name:      "falls back to EDITOR env var",
			editor:    "",
			envEditor: "emacs",
			envVisual: "",
			want:      "emacs",
		},
		{
			name:      "falls back to VISUAL env var",
			editor:    "",
			envEditor: "",
			envVisual: "gedit",
			want:      "gedit",
		},
		{
			name:      "falls back to vim",
			editor:    "",
			envEditor: "",
			envVisual: "",
			want:      "vim",
		},
		{
			name:      "EDITOR takes precedence over VISUAL",
			editor:    "",
			envEditor: "emacs",
			envVisual: "gedit",
			want:      "emacs",
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

func TestAcquireLock_Success(t *testing.T) {
	dir := t.TempDir()

	release, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock returned error: %v", err)
	}
	defer release()

	// Lock file should exist with our PID
	data, err := os.ReadFile(filepath.Join(dir, "memoria.lock"))
	if err != nil {
		t.Fatalf("could not read lock file: %v", err)
	}

	got := strings.TrimSpace(string(data))
	want := strconv.Itoa(os.Getpid())
	if got != want {
		t.Errorf("lock file PID = %q, want %q", got, want)
	}
}

func TestAcquireLock_ReleaseCleansUp(t *testing.T) {
	dir := t.TempDir()

	release, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock returned error: %v", err)
	}

	lockPath := filepath.Join(dir, "memoria.lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Fatal("lock file should exist before release")
	}

	release()

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatal("lock file should be removed after release")
	}
}

func TestAcquireLock_StaleLockOverwritten(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "memoria.lock")

	// Write a lock file with a PID that definitely doesn't exist
	if err := os.WriteFile(lockPath, []byte("999999999"), 0644); err != nil {
		t.Fatalf("could not write stale lock: %v", err)
	}

	release, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock should overwrite stale lock, got error: %v", err)
	}
	defer release()

	data, _ := os.ReadFile(lockPath)
	got := strings.TrimSpace(string(data))
	want := strconv.Itoa(os.Getpid())
	if got != want {
		t.Errorf("lock file should have our PID %q, got %q", want, got)
	}
}

func TestAcquireLock_AlreadyRunning(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "memoria.lock")

	// Use parent PID — it's always alive and not our own PID
	ppid := os.Getppid()
	if err := os.WriteFile(lockPath, []byte(strconv.Itoa(ppid)), 0644); err != nil {
		t.Fatalf("could not write lock: %v", err)
	}

	_, err := AcquireLock(dir)
	if err == nil {
		t.Fatal("AcquireLock should return error when another process holds the lock")
	}

	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("error should mention 'already running', got: %v", err)
	}
}

func TestAcquireLock_OwnPidAllowed(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "memoria.lock")

	// Write our own PID — should be allowed (re-acquire)
	if err := os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
		t.Fatalf("could not write lock: %v", err)
	}

	release, err := AcquireLock(dir)
	if err != nil {
		t.Fatalf("AcquireLock should allow re-acquire by same PID, got error: %v", err)
	}
	defer release()
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

func TestResolveDefaultTodoFolder(t *testing.T) {
	// Default (nil) should return "TODO"
	cfg := DefaultConfig()
	if got := cfg.ResolveDefaultTodoFolder(); got != "TODO" {
		t.Errorf("default ResolveDefaultTodoFolder() = %q, want %q", got, "TODO")
	}

	// Custom value
	custom := "my-tasks"
	cfg.DefaultTodoFolder = &custom
	if got := cfg.ResolveDefaultTodoFolder(); got != "my-tasks" {
		t.Errorf("custom ResolveDefaultTodoFolder() = %q, want %q", got, "my-tasks")
	}
}

func TestResolveTodosEnabled(t *testing.T) {
	// Default (nil) should return true
	cfg := DefaultConfig()
	if got := cfg.ResolveTodosEnabled(); !got {
		t.Error("default ResolveTodosEnabled() should be true")
	}

	// Explicitly disabled
	disabled := false
	cfg.TodosEnabled = &disabled
	if got := cfg.ResolveTodosEnabled(); got {
		t.Error("ResolveTodosEnabled() should be false when set to false")
	}

	// Explicitly enabled
	enabled := true
	cfg.TodosEnabled = &enabled
	if got := cfg.ResolveTodosEnabled(); !got {
		t.Error("ResolveTodosEnabled() should be true when set to true")
	}
}

func TestResolveTheme(t *testing.T) {
	cfg := DefaultConfig()

	// Default is "dark"
	if got := cfg.ResolveTheme(); got != "dark" {
		t.Errorf("ResolveTheme() default = %q, want 'dark'", got)
	}

	// Set to "light"
	light := "light"
	cfg.Theme = &light
	if got := cfg.ResolveTheme(); got != "light" {
		t.Errorf("ResolveTheme() = %q, want 'light'", got)
	}
}
