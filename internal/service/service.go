package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cassiomarques/memoria/internal/editor"
	"github.com/cassiomarques/memoria/internal/git"
	"github.com/cassiomarques/memoria/internal/note"
	"github.com/cassiomarques/memoria/internal/search"
	"github.com/cassiomarques/memoria/internal/storage"
)

// NoteService orchestrates all note operations across the filesystem,
// metadata store, search index, and git repository.
type NoteService struct {
	files  *storage.FileStore
	meta   *storage.MetaStore
	search *search.SearchIndex
	repo   *git.Repository
	editor *editor.Editor
}

// New creates a NoteService wired to the given subsystems.
// Any parameter except files and meta may be nil (git, search, editor are optional).
func New(
	files *storage.FileStore,
	meta *storage.MetaStore,
	idx *search.SearchIndex,
	repo *git.Repository,
	ed *editor.Editor,
) *NoteService {
	return &NoteService{
		files:  files,
		meta:   meta,
		search: idx,
		repo:   repo,
		editor: ed,
	}
}

// ensureMD appends ".md" to path if it doesn't already have it.
func ensureMD(path string) string {
	if !strings.HasSuffix(path, ".md") {
		return path + ".md"
	}
	return path
}

// Create makes a new note and persists it across all stores.
func (s *NoteService) Create(path string, content string, tags []string) (*note.Note, error) {
	path = ensureMD(path)

	n, err := note.NewNote(path, content, tags)
	if err != nil {
		return nil, fmt.Errorf("creating note: %w", err)
	}

	if err := s.files.Save(n); err != nil {
		return nil, fmt.Errorf("saving note to disk: %w", err)
	}

	if err := s.meta.UpsertNote(n); err != nil {
		return nil, fmt.Errorf("upserting metadata: %w", err)
	}

	if s.search != nil {
		if err := s.search.Index(n); err != nil {
			return nil, fmt.Errorf("indexing note: %w", err)
		}
	}

	if s.repo != nil {
		if err := s.repo.CommitAndPush("create " + path); err != nil {
			return nil, fmt.Errorf("git commit: %w", err)
		}
	}

	return n, nil
}

// Open loads a note from disk for viewing/editing.
// If the note lacks YAML frontmatter, it adds it and persists the change.
func (s *NoteService) Open(path string) (*note.Note, error) {
	path = ensureMD(path)

	// Read raw content to check for frontmatter
	absPath := s.files.AbsPath(path)
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("reading note: %w", err)
	}

	n, err := s.files.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading note: %w", err)
	}

	// Add frontmatter if missing
	if !note.HasFrontmatter(string(raw)) {
		info, statErr := os.Stat(absPath)
		if statErr == nil {
			n.Created = info.ModTime()
			n.Modified = info.ModTime()
		}
		if err := s.files.Save(n); err != nil {
			return nil, fmt.Errorf("adding frontmatter: %w", err)
		}
		_ = s.meta.UpsertNote(n)
		if s.search != nil {
			_ = s.search.Index(n)
		}
	}

	return n, nil
}

// AfterEdit reloads a note from disk, updates metadata and search index,
// and commits if content changed. Returns true if the file was modified.
func (s *NoteService) AfterEdit(path string) (bool, error) {
	path = ensureMD(path)

	absPath := s.files.AbsPath(path)
	changed, err := editor.HasChanged("", absPath)
	if err != nil {
		// If we can't determine the hash, reload anyway
		changed = true
	}

	// We always reload — the caller is responsible for checking the hash
	// before and after editing. We detect changes via git.
	n, err := s.files.Load(path)
	if err != nil {
		return false, fmt.Errorf("reloading note: %w", err)
	}

	if err := s.meta.UpsertNote(n); err != nil {
		return false, fmt.Errorf("upserting metadata: %w", err)
	}

	if s.search != nil {
		if err := s.search.Index(n); err != nil {
			return false, fmt.Errorf("re-indexing note: %w", err)
		}
	}

	if s.repo != nil {
		hasChanges, err := s.repo.HasChanges()
		if err != nil {
			return false, fmt.Errorf("checking git changes: %w", err)
		}
		if hasChanges {
			if err := s.repo.CommitAndPush("edit " + path); err != nil {
				return false, fmt.Errorf("git commit: %w", err)
			}
			return true, nil
		}
	}

	_ = changed
	return false, nil
}

// Delete removes a note from all stores.
func (s *NoteService) Delete(path string) error {
	path = ensureMD(path)

	if err := s.files.Delete(path); err != nil {
		return fmt.Errorf("deleting file: %w", err)
	}

	if err := s.meta.DeleteNote(path); err != nil {
		return fmt.Errorf("deleting metadata: %w", err)
	}

	if s.search != nil {
		if err := s.search.Remove(path); err != nil {
			return fmt.Errorf("removing from search: %w", err)
		}
	}

	if s.repo != nil {
		if err := s.repo.CommitAndPush("delete " + path); err != nil {
			return fmt.Errorf("git commit: %w", err)
		}
	}

	return nil
}

// DeleteFolder removes all notes in a folder from all stores.
func (s *NoteService) DeleteFolder(folder string) (int, error) {
	notes, err := s.files.ListAll()
	if err != nil {
		return 0, fmt.Errorf("listing notes: %w", err)
	}

	var deleted int
	for _, n := range notes {
		if n.Folder == folder || strings.HasPrefix(n.Folder, folder+"/") {
			if err := s.files.Delete(n.Path); err != nil {
				return deleted, fmt.Errorf("deleting %s: %w", n.Path, err)
			}
			if err := s.meta.DeleteNote(n.Path); err != nil {
				return deleted, fmt.Errorf("deleting metadata for %s: %w", n.Path, err)
			}
			if s.search != nil {
				_ = s.search.Remove(n.Path)
			}
			deleted++
		}
	}

	if deleted > 0 && s.repo != nil {
		if err := s.repo.CommitAndPush(fmt.Sprintf("delete folder %s (%d notes)", folder, deleted)); err != nil {
			return deleted, fmt.Errorf("git commit: %w", err)
		}
	}

	return deleted, nil
}

// Move renames/moves a note across all stores.
func (s *NoteService) Move(oldPath, newPath string) error {
	// Check if oldPath is a folder (trailing slash or existing directory on disk).
	oldIsDir := strings.HasSuffix(oldPath, "/")
	oldClean := strings.TrimSuffix(oldPath, "/")
	if !oldIsDir && s.files != nil {
		info, err := os.Stat(s.files.AbsPath(oldClean))
		if err == nil && info.IsDir() {
			oldIsDir = true
		}
	}

	if oldIsDir {
		return s.moveFolder(oldClean, strings.TrimSuffix(newPath, "/"))
	}

	oldPath = ensureMD(oldPath)

	// If newPath is a directory (trailing slash, or existing dir on disk), append the original filename.
	isDir := strings.HasSuffix(newPath, "/")
	if !isDir && s.files != nil {
		info, err := os.Stat(s.files.AbsPath(newPath))
		if err == nil && info.IsDir() {
			isDir = true
		}
	}
	if isDir {
		newPath = strings.TrimSuffix(newPath, "/") + "/" + filepath.Base(oldPath)
	} else {
		newPath = ensureMD(newPath)
	}

	if newPath == oldPath {
		return nil
	}

	if err := s.files.Move(oldPath, newPath); err != nil {
		return fmt.Errorf("moving file: %w", err)
	}

	// Determine the new folder from the new path.
	n, err := s.files.Load(newPath)
	if err != nil {
		return fmt.Errorf("loading moved note: %w", err)
	}

	if err := s.meta.MoveNote(oldPath, newPath, n.Folder); err != nil {
		return fmt.Errorf("moving metadata: %w", err)
	}

	if s.search != nil {
		if err := s.search.Remove(oldPath); err != nil {
			return fmt.Errorf("removing old search entry: %w", err)
		}
		if err := s.search.Index(n); err != nil {
			return fmt.Errorf("indexing at new path: %w", err)
		}
	}

	if s.repo != nil {
		if err := s.repo.CommitAndPush("move " + oldPath + " → " + newPath); err != nil {
			return fmt.Errorf("git commit: %w", err)
		}
	}

	return nil
}

// moveFolder renames a folder by moving all notes inside it to the new folder path.
func (s *NoteService) moveFolder(oldFolder, newFolder string) error {
	if oldFolder == newFolder {
		return nil
	}

	// Rename the directory on disk
	absOld := s.files.AbsPath(oldFolder)
	absNew := s.files.AbsPath(newFolder)

	if err := os.MkdirAll(filepath.Dir(absNew), 0o755); err != nil {
		return fmt.Errorf("creating parent dirs: %w", err)
	}
	if err := os.Rename(absOld, absNew); err != nil {
		return fmt.Errorf("renaming folder: %w", err)
	}

	// Update all notes that were under the old folder in metadata and search index
	notes, err := s.meta.ListAll()
	if err != nil {
		return fmt.Errorf("listing notes: %w", err)
	}

	prefix := oldFolder + "/"
	for _, n := range notes {
		if !strings.HasPrefix(n.Path, prefix) && n.Folder != oldFolder {
			continue
		}
		oldNotePath := n.Path
		newNotePath := newFolder + "/" + strings.TrimPrefix(n.Path, prefix)
		newNoteFolder := filepath.Dir(newNotePath)

		if err := s.meta.MoveNote(oldNotePath, newNotePath, newNoteFolder); err != nil {
			return fmt.Errorf("updating metadata for %s: %w", oldNotePath, err)
		}

		if s.search != nil {
			_ = s.search.Remove(oldNotePath)
			if loaded, loadErr := s.files.Load(newNotePath); loadErr == nil {
				_ = s.search.Index(loaded)
			}
		}
	}

	if s.repo != nil {
		if err := s.repo.CommitAndPush("rename folder " + oldFolder + " → " + newFolder); err != nil {
			return fmt.Errorf("git commit: %w", err)
		}
	}

	return nil
}

// Get loads a note from the filesystem.
func (s *NoteService) Get(path string) (*note.Note, error) {
	path = ensureMD(path)
	return s.files.Load(path)
}

// Search performs a query-string search.
func (s *NoteService) Search(query string, limit int) ([]search.SearchResult, error) {
	if s.search == nil {
		return nil, fmt.Errorf("search index not configured")
	}
	return s.search.Search(query, limit)
}

// SearchFuzzy performs a typo-tolerant search.
func (s *NoteService) SearchFuzzy(query string, limit int) ([]search.SearchResult, error) {
	if s.search == nil {
		return nil, fmt.Errorf("search index not configured")
	}
	return s.search.SearchFuzzy(query, limit)
}

// List returns notes in the given folder (non-recursive).
func (s *NoteService) List(folder string) ([]*note.Note, error) {
	return s.files.List(folder)
}

// ListAll returns all notes recursively.
func (s *NoteService) ListAll() ([]*note.Note, error) {
	return s.files.ListAll()
}

// AddTags loads a note, adds the given tags, and persists across all stores.
func (s *NoteService) AddTags(path string, tags []string) (*note.Note, error) {
	path = ensureMD(path)

	n, err := s.files.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading note: %w", err)
	}

	for _, tag := range tags {
		n.AddTag(tag)
	}

	if err := s.files.Save(n); err != nil {
		return nil, fmt.Errorf("saving note: %w", err)
	}

	if err := s.meta.UpsertNote(n); err != nil {
		return nil, fmt.Errorf("upserting metadata: %w", err)
	}

	if s.search != nil {
		if err := s.search.Index(n); err != nil {
			return nil, fmt.Errorf("re-indexing note: %w", err)
		}
	}

	if s.repo != nil {
		if err := s.repo.CommitAndPush("add tags to " + path); err != nil {
			return nil, fmt.Errorf("git commit: %w", err)
		}
	}

	return n, nil
}

// RemoveTags loads a note, removes the given tags, and persists across all stores.
func (s *NoteService) RemoveTags(path string, tags []string) (*note.Note, error) {
	path = ensureMD(path)

	n, err := s.files.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading note: %w", err)
	}

	for _, tag := range tags {
		n.RemoveTag(tag)
	}

	if err := s.files.Save(n); err != nil {
		return nil, fmt.Errorf("saving note: %w", err)
	}

	if err := s.meta.UpsertNote(n); err != nil {
		return nil, fmt.Errorf("upserting metadata: %w", err)
	}

	if s.search != nil {
		if err := s.search.Index(n); err != nil {
			return nil, fmt.Errorf("re-indexing note: %w", err)
		}
	}

	if s.repo != nil {
		if err := s.repo.CommitAndPush("remove tags from " + path); err != nil {
			return nil, fmt.Errorf("git commit: %w", err)
		}
	}

	return n, nil
}

// ListTags returns all tags with their note counts.
func (s *NoteService) ListTags() ([]storage.TagInfo, error) {
	return s.meta.ListAllTags()
}

// ListByTag returns metadata for all notes with the given tag.
func (s *NoteService) ListByTag(tag string) ([]*storage.NoteMeta, error) {
	return s.meta.ListByTag(tag)
}

// TogglePin pins a note if it's unpinned, or unpins it if it's pinned.
// Returns the new pinned state.
func (s *NoteService) TogglePin(path string) (bool, error) {
	pinned, err := s.meta.IsPinned(path)
	if err != nil {
		return false, err
	}
	if pinned {
		return false, s.meta.UnpinNote(path)
	}
	return true, s.meta.PinNote(path)
}

// IsPinned reports whether a note is bookmarked.
func (s *NoteService) IsPinned(path string) (bool, error) {
	return s.meta.IsPinned(path)
}

// ListPinned returns the paths of all bookmarked notes in order.
func (s *NoteService) ListPinned() ([]string, error) {
	return s.meta.ListPinned()
}

// ListRecent returns the most recently modified notes.
func (s *NoteService) ListRecent(limit int) ([]*storage.NoteMeta, error) {
	return s.meta.ListRecent(limit)
}

// Sync pulls from git (if configured), then reloads all notes from disk
// and re-indexes them in metadata and search.
func (s *NoteService) Sync() error {
	if s.repo != nil && s.repo.HasRemote("origin") {
		if err := s.repo.Pull("origin"); err != nil {
			return fmt.Errorf("git pull: %w", err)
		}
	}

	notes, err := s.files.ListAll()
	if err != nil {
		return fmt.Errorf("listing all notes: %w", err)
	}

	// Build set of paths that exist on disk
	diskPaths := make(map[string]bool, len(notes))
	for _, n := range notes {
		diskPaths[n.Path] = true
	}

	// Remove stale metadata entries for files no longer on disk
	allMeta, err := s.meta.ListAll()
	if err == nil {
		for _, m := range allMeta {
			if !diskPaths[m.Path] {
				_ = s.meta.DeleteNote(m.Path)
			}
		}
	}

	for _, n := range notes {
		if err := s.meta.UpsertNote(n); err != nil {
			return fmt.Errorf("upserting metadata for %s: %w", n.Path, err)
		}
	}

	if s.search != nil {
		if err := s.search.Reindex(notes); err != nil {
			return fmt.Errorf("reindexing search: %w", err)
		}
	}

	return nil
}

// HasRemote reports whether a git remote named "origin" is configured.
func (s *NoteService) HasRemote() bool {
	return s.repo != nil && s.repo.HasRemote("origin")
}

// SetRemote configures the git remote and pulls existing notes.
// After pulling, it reloads all notes into metadata and search.
func (s *NoteService) SetRemote(url string) error {
	if s.repo == nil {
		return fmt.Errorf("git repository not initialized")
	}

	// Use a full clone to get a proper repo with correct tracking.
	// This is far more reliable than init + fetch + manual ref setup.
	if err := s.repo.CloneFrom(url); err != nil {
		return fmt.Errorf("cloning from remote: %w", err)
	}

	return s.Sync()
}

// EditorCommand returns the configured editor command string.
func (s *NoteService) EditorCommand() string {
	if s.editor == nil {
		return ""
	}
	return s.editor.Command()
}

// AbsPath returns the absolute filesystem path for a relative note path.
func (s *NoteService) AbsPath(relPath string) string {
	return s.files.AbsPath(relPath)
}

// EnsureFrontmatter scans all notes and adds frontmatter to any that lack it.
// Returns the number of notes that were updated.
func (s *NoteService) EnsureFrontmatter() (int, error) {
	root := s.files.Root()
	var fixed int

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", path, readErr)
		}

		if note.HasFrontmatter(string(raw)) {
			return nil
		}

		relPath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return fmt.Errorf("computing relative path: %w", relErr)
		}

		n, loadErr := s.files.Load(relPath)
		if loadErr != nil {
			return fmt.Errorf("loading note %q: %w", relPath, loadErr)
		}

		// Use file mtime for timestamps instead of time.Now()
		n.Created = info.ModTime()
		n.Modified = info.ModTime()

		if err := s.files.Save(n); err != nil {
			return fmt.Errorf("saving note %q: %w", relPath, err)
		}

		_ = s.meta.UpsertNote(n)
		if s.search != nil {
			_ = s.search.Index(n)
		}

		fixed++
		return nil
	})
	if err != nil {
		return fixed, err
	}

	if fixed > 0 && s.repo != nil {
		if err := s.repo.CommitAndPush(fmt.Sprintf("add frontmatter to %d notes", fixed)); err != nil {
			return fixed, fmt.Errorf("git commit: %w", err)
		}
	}

	return fixed, nil
}
