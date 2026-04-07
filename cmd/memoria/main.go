package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/cassiomarques/memoria/internal/config"
	"github.com/cassiomarques/memoria/internal/editor"
	"github.com/cassiomarques/memoria/internal/git"
	"github.com/cassiomarques/memoria/internal/search"
	"github.com/cassiomarques/memoria/internal/service"
	"github.com/cassiomarques/memoria/internal/storage"
	"github.com/cassiomarques/memoria/internal/tui"
)

var version = "dev"

func main() {
	// Handle flags before anything else (os.Exit is fine here, no defers yet)
	var homeDir string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version", "-v":
			fmt.Printf("memoria %s\n", version)
			os.Exit(0)
		case "--home":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "Error: --home requires a directory path")
				os.Exit(1)
			}
			homeDir = args[i+1]
			i++
		}
	}

	if err := run(homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(homeDir string) error {
	// Override config dir if --home was provided
	if homeDir != "" {
		if err := config.SetConfigDir(homeDir); err != nil {
			return fmt.Errorf("invalid --home path: %w", err)
		}
	}

	// Load config (creates default if not found)
	cfg, err := config.Load(config.DefaultConfigPath())
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Ensure directories exist
	if err := config.EnsureDirs(cfg); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	// Acquire instance lock (prevents two instances on the same data dir)
	releaseLock, err := config.AcquireLock(config.DefaultConfigDir())
	if err != nil {
		return err
	}
	defer releaseLock()

	// Save default config on first run
	cfgPath := config.DefaultConfigPath()
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := cfg.Save(cfgPath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not save default config: %v\n", err)
		}
	}

	// Resolve notes directory
	notesDir, err := cfg.ResolveNotesDir()
	if err != nil {
		return fmt.Errorf("resolving notes dir: %w", err)
	}

	// Initialize FileStore
	files, err := storage.NewFileStore(notesDir)
	if err != nil {
		return fmt.Errorf("initializing file store: %w", err)
	}

	// Initialize MetaStore (SQLite)
	dbPath := filepath.Join(config.DefaultConfigDir(), "meta.db")
	meta, err := storage.NewMetaStore(dbPath)
	if err != nil {
		return fmt.Errorf("initializing meta store: %w", err)
	}

	// Initialize SearchIndex (Bleve)
	indexPath := filepath.Join(config.DefaultConfigDir(), "search.bleve")
	idx, err := search.NewSearchIndex(indexPath)
	if err != nil {
		return fmt.Errorf("initializing search index: %w", err)
	}

	// Initialize git repository
	repo, err := git.InitOrOpen(notesDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: git init failed: %v\n", err)
		repo = nil
	}

	// Initialize editor
	ed := editor.New(cfg.ResolveEditor())

	// Create NoteService
	svc := service.New(files, meta, idx, repo, ed)
	defer svc.Close()

	// Initial sync: load notes from disk into metadata + search
	if err := svc.Sync(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: initial sync failed: %v\n", err)
	}

	// Ensure all notes have frontmatter
	if count, err := svc.EnsureFrontmatter(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: frontmatter migration failed: %v\n", err)
	} else if count > 0 {
		fmt.Fprintf(os.Stderr, "Added frontmatter to %d notes\n", count)
	}

	// Create app wired to service
	app := tui.NewAppWithService(svc, tui.AppOptions{
		ExpandFolders:     cfg.ResolveExpandFolders(),
		ShowPinnedNotes:   cfg.ResolveShowPinnedNotes(),
		ShowTimestamps:    cfg.ResolveShowTimestamps(),
		Version:           version,
		DefaultTodoFolder: cfg.ResolveDefaultTodoFolder(),
	})

	// Run Bubble Tea program
	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}

	return nil
}
