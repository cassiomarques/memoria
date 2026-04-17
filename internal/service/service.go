package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	// commitReqs feeds commit messages to the background git worker.
	commitReqs chan string
	// syncResults delivers push outcomes back to the TUI layer.
	syncResults chan error
	// done signals the git worker to stop on shutdown.
	done chan struct{}
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
	s := &NoteService{
		files:       files,
		meta:        meta,
		search:      idx,
		repo:        repo,
		editor:      ed,
		commitReqs:  make(chan string, 64),
		syncResults: make(chan error, 64),
		done:        make(chan struct{}),
	}
	go s.gitWorker()
	return s
}

// gitWorker is a background goroutine that handles all git operations.
// It commits each change immediately (preserving descriptive history) and
// debounces pushes: after committing, it waits for a quiet period before
// pushing. Rapid actions produce individual commits but a single push.
//
//	toggle done → commit "toggle done TODO/task.md"     ← immediate
//	delete note → commit "delete work/old.md"           ← immediate
//	create todo → commit "create todo TODO/foo.md"      ← immediate
//	... 2s of quiet ...
//	push to origin                                       ← one push
func (s *NoteService) gitWorker() {
	defer close(s.syncResults)

	const pushDelay = 2 * time.Second
	var timer *time.Timer
	pendingPush := false

	for {
		select {
		case <-s.done:
			if timer != nil {
				timer.Stop()
			}
			// Commit+push any remaining queued work before exiting.
			s.drainCommits()
			if pendingPush {
				_ = s.pushIfRemote()
			}
			return

		case msg := <-s.commitReqs:
			if s.repo == nil {
				continue
			}
			if err := s.safeCommit(msg); err != nil {
				s.syncResults <- fmt.Errorf("git commit: %w", err)
				continue
			}
			pendingPush = true
			// Reset the push debounce timer.
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(pushDelay)

		case <-timerChan(timer):
			// Quiet period elapsed — push now.
			timer = nil
			if pendingPush {
				s.syncResults <- s.pushIfRemote()
				pendingPush = false
			}
		}
	}
}

// safeCommit wraps CommitAll with panic recovery. go-git is not fully
// thread-safe and can occasionally panic when the working tree changes
// during a commit. We recover instead of crashing the worker goroutine.
func (s *NoteService) safeCommit(msg string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("git panic (recovered): %v", r)
		}
	}()
	return s.repo.CommitAll(msg)
}

// drainCommits processes any remaining commit requests in the buffer.
func (s *NoteService) drainCommits() {
	for {
		select {
		case msg := <-s.commitReqs:
			if s.repo != nil {
				_ = s.safeCommit(msg)
			}
		default:
			return
		}
	}
}

// timerChan returns the channel for a timer, or a nil channel (blocks forever)
// if the timer is nil.
func timerChan(t *time.Timer) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

// pushIfRemote pushes to "origin" if a remote is configured.
func (s *NoteService) pushIfRemote() error {
	if s.repo == nil || !s.repo.HasRemote("origin") {
		return nil
	}
	return s.repo.Push("origin")
}

// SyncResults returns a channel that delivers git push outcomes.
func (s *NoteService) SyncResults() <-chan error {
	return s.syncResults
}

// requestSync enqueues a git commit message to the background worker.
// The commit and push happen entirely in the background — the caller
// returns immediately so the UI stays responsive.
func (s *NoteService) requestSync(message string) {
	select {
	case s.commitReqs <- message:
	default:
	}
}

// Close shuts down the background git worker. Pending commits are
// flushed and a best-effort push is attempted before returning.
func (s *NoteService) Close() {
	close(s.done)
	for range s.syncResults {
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

	s.requestSync("create " + path)

	return n, nil
}

// Edit updates the content of an existing note, preserving its frontmatter
// metadata (tags, dates, etc.). Returns an error if the note does not exist.
func (s *NoteService) Edit(path string, content string) (*note.Note, error) {
	path = ensureMD(path)

	if !s.files.Exists(path) {
		return nil, fmt.Errorf("note not found: %s", path)
	}

	// Load the existing note to preserve frontmatter
	n, err := s.files.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading note: %w", err)
	}

	n.Content = content
	n.Modified = time.Now()

	if err := s.files.Save(n); err != nil {
		return nil, fmt.Errorf("saving note: %w", err)
	}

	if err := s.meta.UpsertNote(n); err != nil {
		return nil, fmt.Errorf("upserting metadata: %w", err)
	}

	if s.search != nil {
		if err := s.search.Index(n); err != nil {
			return nil, fmt.Errorf("indexing note: %w", err)
		}
	}

	s.requestSync("edit " + path)

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

// FileHash returns the hex-encoded SHA-256 hash of the raw file content
// for the given note path. Use this before opening the editor to capture
// a snapshot, then pass the result to AfterEdit for change detection.
func (s *NoteService) FileHash(path string) (string, error) {
	path = ensureMD(path)
	data, err := os.ReadFile(s.files.AbsPath(path))
	if err != nil {
		return "", fmt.Errorf("reading file for hash: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// AfterEdit reloads a note from disk, updates metadata and search index,
// and commits if content changed. Returns true if the file was modified.
// preEditHash is the hex SHA-256 of the file before the editor was opened;
// if the current file hashes to the same value, no save or sync is performed.
func (s *NoteService) AfterEdit(path string, preEditHash string) (bool, error) {
	path = ensureMD(path)

	// Check whether the file actually changed.
	currentHash, err := s.FileHash(path)
	if err != nil {
		return false, fmt.Errorf("computing post-edit hash: %w", err)
	}
	if preEditHash != "" && currentHash == preEditHash {
		return false, nil
	}

	n, err := s.files.Load(path)
	if err != nil {
		return false, fmt.Errorf("reloading note: %w", err)
	}

	// Re-save to update the frontmatter's "modified" timestamp.
	// Save() sets n.Modified = time.Now() before writing.
	if err := s.files.Save(n); err != nil {
		return false, fmt.Errorf("saving updated timestamp: %w", err)
	}

	if err := s.meta.UpsertNote(n); err != nil {
		return false, fmt.Errorf("upserting metadata: %w", err)
	}

	if s.search != nil {
		if err := s.search.Index(n); err != nil {
			return false, fmt.Errorf("re-indexing note: %w", err)
		}
	}

	s.requestSync("edit " + path)

	return true, nil
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

	s.requestSync("delete " + path)

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

	if deleted > 0 {
		s.requestSync(fmt.Sprintf("delete folder %s (%d notes)", folder, deleted))
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

	s.requestSync("move " + oldPath + " → " + newPath)

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

	s.requestSync("rename folder " + oldFolder + " → " + newFolder)

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

	s.requestSync("add tags to " + path)

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

	s.requestSync("remove tags from " + path)

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
		if info.IsDir() {
			if info.Name() == ".trash" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".md") {
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

	if fixed > 0 {
		s.requestSync(fmt.Sprintf("add frontmatter to %d notes", fixed))
	}

	return fixed, nil
}

// CreateTodoOptions holds the parameters for creating a todo note.
type CreateTodoOptions struct {
	Title   string
	Folder  string
	Tags    []string
	Due     *time.Time
	Content string
}

// CreateTodo creates a new todo note with the given options.
// The title is slugified to form the filename, placed in the given folder.
func (s *NoteService) CreateTodo(opts CreateTodoOptions) (*note.Note, error) {
	slug := note.Slugify(opts.Title)
	if slug == "" {
		return nil, fmt.Errorf("todo title cannot be empty")
	}

	path := filepath.Join(opts.Folder, slug+".md")

	n, err := note.NewNote(path, opts.Content, opts.Tags)
	if err != nil {
		return nil, fmt.Errorf("creating todo: %w", err)
	}
	n.Todo = true
	n.Done = false
	n.Due = opts.Due

	if err := s.files.Save(n); err != nil {
		return nil, fmt.Errorf("saving todo to disk: %w", err)
	}

	if err := s.meta.UpsertNote(n); err != nil {
		return nil, fmt.Errorf("upserting todo metadata: %w", err)
	}

	if s.search != nil {
		if err := s.search.Index(n); err != nil {
			return nil, fmt.Errorf("indexing todo: %w", err)
		}
	}

	s.requestSync("create todo " + path)

	return n, nil
}

// ToggleTodoDone flips the done status of a todo note and persists the change.
func (s *NoteService) ToggleTodoDone(path string) (bool, error) {
	n, err := s.files.Load(path)
	if err != nil {
		return false, fmt.Errorf("loading note: %w", err)
	}
	if !n.Todo {
		return false, fmt.Errorf("%q is not a todo", path)
	}

	n.Done = !n.Done
	n.Modified = time.Now()

	if n.Done {
		now := time.Now()
		n.Completed = &now
	} else {
		n.Completed = nil
		n.Archived = false // unarchive if toggling back to pending
	}

	if err := s.files.Save(n); err != nil {
		return false, fmt.Errorf("saving note: %w", err)
	}
	if err := s.meta.UpsertNote(n); err != nil {
		return false, fmt.Errorf("upserting metadata: %w", err)
	}
	if s.search != nil {
		if err := s.search.Index(n); err != nil {
			return false, fmt.Errorf("indexing note: %w", err)
		}
	}
	s.requestSync("toggle done " + path)

	return n.Done, nil
}

// SetTodoDue updates the due date of a todo note. Pass nil to clear the due date.
func (s *NoteService) SetTodoDue(path string, due *time.Time) error {
	path = ensureMD(path)

	n, err := s.files.Load(path)
	if err != nil {
		return fmt.Errorf("loading note: %w", err)
	}
	if !n.Todo {
		return fmt.Errorf("%q is not a todo", path)
	}

	n.Due = due
	n.Modified = time.Now()

	if err := s.files.Save(n); err != nil {
		return fmt.Errorf("saving note: %w", err)
	}
	if err := s.meta.UpsertNote(n); err != nil {
		return fmt.Errorf("upserting metadata: %w", err)
	}
	if s.search != nil {
		if err := s.search.Index(n); err != nil {
			return fmt.Errorf("indexing note: %w", err)
		}
	}
	s.requestSync("set due " + path)

	return nil
}

// ListTodos returns all todo notes from the metadata store, sorted by due date.
// Archived todos are excluded.
func (s *NoteService) ListTodos() ([]*storage.NoteMeta, error) {
	return s.meta.ListTodos()
}

// ListArchivedTodos returns archived todos.
func (s *NoteService) ListArchivedTodos() ([]*storage.NoteMeta, error) {
	return s.meta.ListArchivedTodos()
}

// ArchiveTodo marks a completed todo as archived.
func (s *NoteService) ArchiveTodo(path string) error {
	path = ensureMD(path)

	n, err := s.files.Load(path)
	if err != nil {
		return fmt.Errorf("loading note: %w", err)
	}
	if !n.Todo {
		return fmt.Errorf("%q is not a todo", path)
	}
	if !n.Done {
		return fmt.Errorf("only completed todos can be archived")
	}

	n.Archived = true
	n.Modified = time.Now()

	if err := s.files.Save(n); err != nil {
		return fmt.Errorf("saving note: %w", err)
	}
	if err := s.meta.UpsertNote(n); err != nil {
		return fmt.Errorf("upserting metadata: %w", err)
	}
	if s.search != nil {
		_ = s.search.Remove(path) // archived notes leave the search index
	}
	s.requestSync("archive " + path)
	return nil
}

// UnarchiveTodo restores an archived todo to the active list.
func (s *NoteService) UnarchiveTodo(path string) error {
	path = ensureMD(path)

	n, err := s.files.Load(path)
	if err != nil {
		return fmt.Errorf("loading note: %w", err)
	}
	if !n.Archived {
		return fmt.Errorf("%q is not archived", path)
	}

	n.Archived = false
	n.Modified = time.Now()

	if err := s.files.Save(n); err != nil {
		return fmt.Errorf("saving note: %w", err)
	}
	if err := s.meta.UpsertNote(n); err != nil {
		return fmt.Errorf("upserting metadata: %w", err)
	}
	if s.search != nil {
		_ = s.search.Index(n) // re-add to search index
	}
	s.requestSync("unarchive " + path)
	return nil
}

// trashDir is the hidden directory inside the notes root used for soft-deleted notes.
const trashDir = ".trash"

// Trash soft-deletes a note by moving it into .trash/ preserving its relative path.
// The note is removed from metadata and search, but the file is kept in .trash/.
func (s *NoteService) Trash(path string) error {
	path = ensureMD(path)
	trashPath := filepath.Join(trashDir, path)

	if err := s.files.Move(path, trashPath); err != nil {
		return fmt.Errorf("moving to trash: %w", err)
	}

	// Best-effort cleanup of metadata and search — the file is already moved.
	_ = s.meta.DeleteNote(path)
	if s.search != nil {
		_ = s.search.Remove(path)
	}
	s.requestSync("trash " + path)
	return nil
}

// TrashFolder soft-deletes all notes in a folder by moving them to .trash/.
func (s *NoteService) TrashFolder(folder string) (int, error) {
	notes, err := s.files.ListAll()
	if err != nil {
		return 0, fmt.Errorf("listing notes: %w", err)
	}

	var trashed int
	for _, n := range notes {
		if n.Folder == folder || strings.HasPrefix(n.Folder, folder+"/") {
			trashPath := filepath.Join(trashDir, n.Path)
			if err := s.files.Move(n.Path, trashPath); err != nil {
				return trashed, fmt.Errorf("trashing %s: %w", n.Path, err)
			}
			_ = s.meta.DeleteNote(n.Path)
			if s.search != nil {
				_ = s.search.Remove(n.Path)
			}
			trashed++
		}
	}

	if trashed > 0 {
		s.requestSync(fmt.Sprintf("trash folder %s (%d notes)", folder, trashed))
	}
	return trashed, nil
}

// ListTrash returns all notes inside .trash/, with paths relative to .trash/.
func (s *NoteService) ListTrash() ([]*note.Note, error) {
	trashRoot := s.files.AbsPath(trashDir)

	info, err := os.Stat(trashRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // no trash dir = empty trash
		}
		return nil, fmt.Errorf("stat trash dir: %w", err)
	}
	if !info.IsDir() {
		return nil, nil
	}

	var notes []*note.Note
	err = filepath.Walk(trashRoot, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() || !strings.HasSuffix(fi.Name(), ".md") {
			return nil
		}
		relToTrash, relErr := filepath.Rel(trashRoot, path)
		if relErr != nil {
			return fmt.Errorf("computing relative path: %w", relErr)
		}
		// Load via the full relative path (.trash/...) so FileStore can find it.
		fullRel := filepath.Join(trashDir, relToTrash)
		n, loadErr := s.files.Load(fullRel)
		if loadErr != nil {
			return fmt.Errorf("loading trashed note %q: %w", relToTrash, loadErr)
		}
		// Override path to be relative to .trash/ so callers see the original path.
		n.Path = relToTrash
		n.Folder = folderFromPath(relToTrash)
		notes = append(notes, n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return notes, nil
}

// RestoreFromTrash moves a trashed note back to its original location.
// The path should be relative to .trash/ (i.e., the original note path).
// Returns an error if a note already exists at the destination.
func (s *NoteService) RestoreFromTrash(path string) error {
	path = ensureMD(path)
	trashPath := filepath.Join(trashDir, path)

	// Check destination doesn't already exist.
	if s.files.Exists(path) {
		return fmt.Errorf("cannot restore: %s already exists", path)
	}

	if err := s.files.Move(trashPath, path); err != nil {
		return fmt.Errorf("restoring from trash: %w", err)
	}

	// Re-index the restored note.
	n, err := s.files.Load(path)
	if err != nil {
		return fmt.Errorf("loading restored note: %w", err)
	}
	_ = s.meta.UpsertNote(n)
	if s.search != nil {
		_ = s.search.Index(n)
	}
	s.requestSync("restore " + path)
	return nil
}

// PermanentlyDeleteFromTrash removes a single note from .trash/ forever.
func (s *NoteService) PermanentlyDeleteFromTrash(path string) error {
	path = ensureMD(path)
	trashPath := filepath.Join(trashDir, path)

	if err := s.files.Delete(trashPath); err != nil {
		return fmt.Errorf("deleting from trash: %w", err)
	}
	s.requestSync("permanently delete " + path)
	return nil
}

// EmptyTrash permanently deletes everything in .trash/.
func (s *NoteService) EmptyTrash() (int, error) {
	notes, err := s.ListTrash()
	if err != nil {
		return 0, err
	}

	for _, n := range notes {
		trashPath := filepath.Join(trashDir, n.Path)
		_ = s.files.Delete(trashPath)
	}

	// Remove the .trash directory itself.
	trashRoot := s.files.AbsPath(trashDir)
	_ = os.RemoveAll(trashRoot)

	if len(notes) > 0 {
		s.requestSync(fmt.Sprintf("empty trash (%d notes)", len(notes)))
	}
	return len(notes), nil
}

// folderFromPath extracts the directory portion of a note path.
func folderFromPath(path string) string {
	dir := filepath.Dir(path)
	if dir == "." {
		return ""
	}
	return dir
}
