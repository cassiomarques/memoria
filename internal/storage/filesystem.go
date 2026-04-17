package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cassiomarques/memoria/internal/note"
)

// FileStore implements note storage backed by the local filesystem.
type FileStore struct {
	root string // absolute path to the notes directory
}

// NewFileStore creates a new FileStore rooted at the given directory.
// It ensures the root directory exists, creating it if necessary.
func NewFileStore(root string) (*FileStore, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving root path: %w", err)
	}

	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return nil, fmt.Errorf("creating root directory: %w", err)
	}

	return &FileStore{root: absRoot}, nil
}

// Root returns the absolute path to the notes directory.
func (fs *FileStore) Root() string {
	return fs.root
}

// Save writes a note to disk at root/note.Path.
// It creates parent directories as needed and updates n.Modified before writing.
func (fs *FileStore) Save(n *note.Note) error {
	absPath := fs.AbsPath(n.Path)

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating parent directories: %w", err)
	}

	n.Modified = time.Now()

	content := n.FullContent()
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing note file: %w", err)
	}

	return nil
}

// Load reads a note file at root/relPath and parses it.
func (fs *FileStore) Load(relPath string) (*note.Note, error) {
	absPath := fs.AbsPath(relPath)

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading note file: %w", err)
	}

	n, err := note.ParseNote(relPath, string(data))
	if err != nil {
		return nil, fmt.Errorf("parsing note: %w", err)
	}

	return n, nil
}

// Delete removes the note file at root/relPath.
// Returns os.ErrNotExist if the file does not exist.
// Also removes empty parent directories up to (but not including) root.
func (fs *FileStore) Delete(relPath string) error {
	absPath := fs.AbsPath(relPath)

	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return os.ErrNotExist
		}
		return err
	}

	if err := os.Remove(absPath); err != nil {
		return fmt.Errorf("deleting note file: %w", err)
	}

	fs.cleanEmptyParents(filepath.Dir(absPath))
	return nil
}

// Move renames/moves a note from oldPath to newPath.
// Creates destination parent directories and removes empty source parent directories.
func (fs *FileStore) Move(oldPath, newPath string) error {
	absOld := fs.AbsPath(oldPath)
	absNew := fs.AbsPath(newPath)

	if _, err := os.Stat(absOld); err != nil {
		if os.IsNotExist(err) {
			return os.ErrNotExist
		}
		return err
	}

	newDir := filepath.Dir(absNew)
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		return fmt.Errorf("creating destination directories: %w", err)
	}

	if err := os.Rename(absOld, absNew); err != nil {
		return fmt.Errorf("moving note: %w", err)
	}

	fs.cleanEmptyParents(filepath.Dir(absOld))
	return nil
}

// List returns all notes in the given folder (non-recursive), sorted by path.
// If folder is "", it lists from root.
func (fs *FileStore) List(folder string) ([]*note.Note, error) {
	dir := fs.root
	if folder != "" {
		dir = filepath.Join(fs.root, folder)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	var notes []*note.Note
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		relPath := entry.Name()
		if folder != "" {
			relPath = filepath.Join(folder, entry.Name())
		}

		n, err := fs.Load(relPath)
		if err != nil {
			return nil, fmt.Errorf("loading note %q: %w", relPath, err)
		}
		notes = append(notes, n)
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Path < notes[j].Path
	})

	return notes, nil
}

// ListAll returns all notes recursively under the root, sorted by path.
// The .trash directory is excluded from the listing.
func (fs *FileStore) ListAll() ([]*note.Note, error) {
	var notes []*note.Note

	err := filepath.Walk(fs.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // skip files/dirs that vanish during walk (e.g. git repacking)
			}
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".trash" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		relPath, err := filepath.Rel(fs.root, path)
		if err != nil {
			return fmt.Errorf("computing relative path: %w", err)
		}

		n, loadErr := fs.Load(relPath)
		if loadErr != nil {
			return fmt.Errorf("loading note %q: %w", relPath, loadErr)
		}
		notes = append(notes, n)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Path < notes[j].Path
	})

	return notes, nil
}

// Exists checks if a note file exists at root/relPath.
func (fs *FileStore) Exists(relPath string) bool {
	info, err := os.Stat(fs.AbsPath(relPath))
	return err == nil && !info.IsDir()
}

// AbsPath returns the absolute filesystem path for a relative note path.
func (fs *FileStore) AbsPath(relPath string) string {
	return filepath.Join(fs.root, relPath)
}

// cleanEmptyParents removes empty directories from dir up to (but not including) root.
func (fs *FileStore) cleanEmptyParents(dir string) {
	for dir != fs.root && strings.HasPrefix(dir, fs.root) {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		_ = os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}
