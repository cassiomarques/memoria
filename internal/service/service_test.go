package service

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cassiomarques/memoria/internal/editor"
	"github.com/cassiomarques/memoria/internal/git"
	"github.com/cassiomarques/memoria/internal/search"
	"github.com/cassiomarques/memoria/internal/storage"
)

// setupService creates a fully wired NoteService backed by temp directories,
// in-memory SQLite, in-memory Bleve, and a temp git repo.
func setupService(t *testing.T) *NoteService {
	t.Helper()

	root := t.TempDir()

	files, err := storage.NewFileStore(root)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}

	meta, err := storage.NewMemoryMetaStore()
	if err != nil {
		t.Fatalf("NewMemoryMetaStore: %v", err)
	}
	t.Cleanup(func() { meta.Close() })

	idx, err := search.NewMemorySearchIndex()
	if err != nil {
		t.Fatalf("NewMemorySearchIndex: %v", err)
	}
	t.Cleanup(func() { idx.Close() })

	repo, err := git.InitOrOpen(root)
	if err != nil {
		t.Fatalf("InitOrOpen: %v", err)
	}

	ed := editor.New("cat") // harmless editor for tests

	svc := New(files, meta, idx, repo, ed)
	t.Cleanup(func() { svc.Close() })
	return svc
}

// waitCommit polls until the commit count exceeds prev, or times out.
// Used to wait for the background git worker to process a commit.
func waitCommit(t *testing.T, root string, prev int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if countCommits(t, root) > prev {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for commit (still at %d)", prev)
}

func TestCreate(t *testing.T) {
	svc := setupService(t)

	n, err := svc.Create("hello", "Hello world!", []string{"greeting"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Path should have .md appended
	if n.Path != "hello.md" {
		t.Errorf("expected path hello.md, got %s", n.Path)
	}

	// Should exist on disk
	if !svc.files.Exists("hello.md") {
		t.Error("note file does not exist on disk")
	}

	// Should exist in metadata
	nm, err := svc.meta.GetNote("hello.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if nm.Title != "hello" {
		t.Errorf("expected title 'hello', got %q", nm.Title)
	}
	if len(nm.Tags) != 1 || nm.Tags[0] != "greeting" {
		t.Errorf("expected tags [greeting], got %v", nm.Tags)
	}

	// Should be in search index
	cnt, err := svc.search.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if cnt != 1 {
		t.Errorf("expected 1 doc in search, got %d", cnt)
	}
}

func TestCreateWithMDExtension(t *testing.T) {
	svc := setupService(t)

	n, err := svc.Create("already.md", "content", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if n.Path != "already.md" {
		t.Errorf("expected path already.md, got %s", n.Path)
	}
}

func TestCreateThenOpen(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("openme.md", "Open this note", []string{"test"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	n, err := svc.Open("openme.md")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	if n.Path != "openme.md" {
		t.Errorf("expected path openme.md, got %s", n.Path)
	}
	if !strings.Contains(n.Content, "Open this note") {
		t.Errorf("expected content to contain 'Open this note', got %q", n.Content)
	}
}

func TestAfterEdit(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("editable.md", "Original content", []string{"v1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Capture real pre-edit hash, then simulate editing the file on disk
	preHash, err := svc.FileHash("editable.md")
	if err != nil {
		t.Fatalf("FileHash: %v", err)
	}

	absPath := svc.files.AbsPath("editable.md")
	newContent := "---\ntags:\n    - v1\n    - v2\ncreated: " + time.Now().Format(time.RFC3339) + "\nmodified: " + time.Now().Format(time.RFC3339) + "\n---\nEdited content"
	if err := os.WriteFile(absPath, []byte(newContent), 0o644); err != nil {
		t.Fatalf("writing edited content: %v", err)
	}

	changed, err := svc.AfterEdit("editable.md", preHash)
	if err != nil {
		t.Fatalf("AfterEdit: %v", err)
	}

	if !changed {
		t.Error("expected AfterEdit to report changes")
	}

	// Verify re-indexed: reload and check content
	n, err := svc.Get("editable.md")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !strings.Contains(n.Content, "Edited content") {
		t.Errorf("expected edited content, got %q", n.Content)
	}

	// Verify metadata updated with new tag
	nm, err := svc.meta.GetNote("editable.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	hasV2 := false
	for _, tag := range nm.Tags {
		if tag == "v2" {
			hasV2 = true
		}
	}
	if !hasV2 {
		t.Errorf("expected tag v2 in metadata, got %v", nm.Tags)
	}
}

func TestAfterEdit_UpdatesModifiedTimestamp(t *testing.T) {
	svc := setupService(t)

	n, err := svc.Create("timestamped.md", "Initial content", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Write the file with a backdated modified timestamp to make the
	// comparison unambiguous (frontmatter uses second-level precision).
	absPath := svc.files.AbsPath("timestamped.md")
	backdated := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	edited := "---\ncreated: " + n.Created.Format(time.RFC3339) +
		"\nmodified: " + backdated.Format(time.RFC3339) +
		"\n---\nEdited content"
	if err := os.WriteFile(absPath, []byte(edited), 0o644); err != nil {
		t.Fatalf("writing edited content: %v", err)
	}

	_, err = svc.AfterEdit("timestamped.md", "")
	if err != nil {
		t.Fatalf("AfterEdit: %v", err)
	}

	// Verify Modified was updated in the reloaded note
	got, err := svc.Get("timestamped.md")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Modified.After(backdated) {
		t.Errorf("expected Modified to advance past backdated time: backdated=%v, got=%v",
			backdated, got.Modified)
	}

	// Verify the frontmatter on disk was updated too
	raw, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	if strings.Contains(string(raw), backdated.Format(time.RFC3339)) {
		t.Error("expected frontmatter on disk to have a newer modified timestamp")
	}

	// Verify SQLite metadata matches
	nm, err := svc.meta.GetNote("timestamped.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if !nm.Modified.After(backdated) {
		t.Errorf("expected metadata Modified to advance: backdated=%v, got=%v",
			backdated, nm.Modified)
	}
}

func TestAfterEdit_NoChangeSkipsSave(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("noop.md", "Unchanged content", []string{"stable"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Capture the file bytes and hash before the simulated editor open
	absPath := svc.files.AbsPath("noop.md")
	beforeBytes, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}
	preHash, err := svc.FileHash("noop.md")
	if err != nil {
		t.Fatalf("FileHash: %v", err)
	}

	// Simulate editor open+close with no changes (or :wq with no edits)
	changed, err := svc.AfterEdit("noop.md", preHash)
	if err != nil {
		t.Fatalf("AfterEdit: %v", err)
	}
	if changed {
		t.Error("expected AfterEdit to report no changes")
	}

	// Verify the file on disk was NOT rewritten (bytes identical)
	afterBytes, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("reading file after: %v", err)
	}
	if string(beforeBytes) != string(afterBytes) {
		t.Error("expected file bytes to remain identical when no changes were made")
	}
}

func TestDelete(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("deleteme.md", "To be deleted", []string{"temp"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete("deleteme.md"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify removed from disk
	if svc.files.Exists("deleteme.md") {
		t.Error("note still exists on disk after delete")
	}

	// Verify removed from metadata
	_, err = svc.meta.GetNote("deleteme.md")
	if !errors.Is(err, storage.ErrNoteNotFound) {
		t.Errorf("expected ErrNoteNotFound, got %v", err)
	}

	// Verify removed from search (count should be 0)
	cnt, err := svc.search.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if cnt != 0 {
		t.Errorf("expected 0 docs in search after delete, got %d", cnt)
	}
}

func TestMove(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("old-name.md", "Moving note", []string{"move"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Move("old-name.md", "subdir/new-name"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	// Old path gone
	if svc.files.Exists("old-name.md") {
		t.Error("old path still exists on disk")
	}
	_, err = svc.meta.GetNote("old-name.md")
	if !errors.Is(err, storage.ErrNoteNotFound) {
		t.Errorf("expected ErrNoteNotFound for old path, got %v", err)
	}

	// New path exists
	if !svc.files.Exists("subdir/new-name.md") {
		t.Error("new path does not exist on disk")
	}
	nm, err := svc.meta.GetNote("subdir/new-name.md")
	if err != nil {
		t.Fatalf("GetNote new path: %v", err)
	}
	if nm.Folder != "subdir" {
		t.Errorf("expected folder 'subdir', got %q", nm.Folder)
	}

	// Search should find at new path
	results, err := svc.Search("Moving", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Error("search returned no results after move")
	} else if results[0].Path != "subdir/new-name.md" {
		t.Errorf("expected search result at subdir/new-name.md, got %s", results[0].Path)
	}
}

func TestMove_DestinationIsFolder(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("my-note.md", "Content here", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Move to a folder path (trailing slash) — should keep original filename
	if err := svc.Move("my-note.md", "archive/"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	// Old path gone
	if svc.files.Exists("my-note.md") {
		t.Error("old path still exists on disk")
	}

	// New path should be archive/my-note.md
	if !svc.files.Exists("archive/my-note.md") {
		t.Error("expected archive/my-note.md to exist")
	}
	nm, err := svc.meta.GetNote("archive/my-note.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if nm.Folder != "archive" {
		t.Errorf("expected folder 'archive', got %q", nm.Folder)
	}
}

func TestMove_DestinationIsExistingFolder(t *testing.T) {
	svc := setupService(t)

	// Create a note in a folder so the folder exists on disk
	_, err := svc.Create("target/existing.md", "Existing", nil)
	if err != nil {
		t.Fatalf("Create existing: %v", err)
	}

	// Create the note we want to move
	_, err = svc.Create("moveme.md", "Move me", nil)
	if err != nil {
		t.Fatalf("Create moveme: %v", err)
	}

	// Move to "target" (no trailing slash) — should detect existing dir
	if err := svc.Move("moveme.md", "target"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	if svc.files.Exists("moveme.md") {
		t.Error("old path still exists")
	}
	if !svc.files.Exists("target/moveme.md") {
		t.Error("expected target/moveme.md to exist")
	}
	// Original note in target should still be there
	if !svc.files.Exists("target/existing.md") {
		t.Error("existing note in target folder was lost")
	}
}

func TestMove_UnnestToParentFolder(t *testing.T) {
	svc := setupService(t)

	// Create a nested note: Personal/Projects/my_file.md
	_, err := svc.Create("Personal/Projects/my_file.md", "Nested note", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Move to parent: Personal/ (trailing slash)
	if err := svc.Move("Personal/Projects/my_file.md", "Personal/"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	// Old path gone
	if svc.files.Exists("Personal/Projects/my_file.md") {
		t.Error("old path still exists")
	}

	// New path should be Personal/my_file.md
	if !svc.files.Exists("Personal/my_file.md") {
		t.Error("expected Personal/my_file.md to exist")
	}

	nm, err := svc.meta.GetNote("Personal/my_file.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if nm.Folder != "Personal" {
		t.Errorf("expected folder 'Personal', got %q", nm.Folder)
	}
}

func TestAddTags(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("tagged.md", "Tag test", []string{"initial"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	n, err := svc.AddTags("tagged.md", []string{"extra", "bonus"})
	if err != nil {
		t.Fatalf("AddTags: %v", err)
	}

	if !n.HasTag("initial") || !n.HasTag("extra") || !n.HasTag("bonus") {
		t.Errorf("expected all three tags, got %v", n.Tags)
	}

	// Check frontmatter on disk
	loaded, err := svc.files.Load("tagged.md")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.HasTag("extra") {
		t.Error("tag 'extra' not persisted in frontmatter")
	}

	// Check metadata
	nm, err := svc.meta.GetNote("tagged.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	tagSet := make(map[string]bool)
	for _, tag := range nm.Tags {
		tagSet[tag] = true
	}
	if !tagSet["extra"] || !tagSet["bonus"] {
		t.Errorf("metadata tags missing extra/bonus: %v", nm.Tags)
	}
}

func TestRemoveTags(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("untagged.md", "Untag test", []string{"keep", "remove"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	n, err := svc.RemoveTags("untagged.md", []string{"remove"})
	if err != nil {
		t.Fatalf("RemoveTags: %v", err)
	}

	if n.HasTag("remove") {
		t.Error("tag 'remove' still present after RemoveTags")
	}
	if !n.HasTag("keep") {
		t.Error("tag 'keep' should still be present")
	}

	// Check metadata
	nm, err := svc.meta.GetNote("untagged.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	for _, tag := range nm.Tags {
		if tag == "remove" {
			t.Error("metadata still has 'remove' tag")
		}
	}
}

func TestSearch(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("golang.md", "Go is a great programming language", []string{"programming"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = svc.Create("python.md", "Python is also popular", []string{"programming"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_, err = svc.Create("cooking.md", "How to make pasta", []string{"food"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	results, err := svc.Search("programming", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) < 2 {
		t.Errorf("expected at least 2 results for 'programming', got %d", len(results))
	}

	// Cooking note should not appear
	for _, r := range results {
		if r.Path == "cooking.md" {
			t.Error("cooking note should not match 'programming' search")
		}
	}
}

func TestSearchFuzzy(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("important.md", "This is an important document about algorithms", []string{"cs"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Typo: "algoritms" instead of "algorithms"
	results, err := svc.SearchFuzzy("algoritms", 10)
	if err != nil {
		t.Fatalf("SearchFuzzy: %v", err)
	}
	if len(results) == 0 {
		t.Error("expected fuzzy search to find 'algorithms' with typo 'algoritms'")
	}
}

func TestListTags(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("a.md", "Note A", []string{"go", "web"})
	if err != nil {
		t.Fatalf("Create a: %v", err)
	}
	_, err = svc.Create("b.md", "Note B", []string{"go", "cli"})
	if err != nil {
		t.Fatalf("Create b: %v", err)
	}
	_, err = svc.Create("c.md", "Note C", []string{"web"})
	if err != nil {
		t.Fatalf("Create c: %v", err)
	}

	tags, err := svc.ListTags()
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}

	tagMap := make(map[string]int)
	for _, ti := range tags {
		tagMap[ti.Tag] = ti.Count
	}

	if tagMap["go"] != 2 {
		t.Errorf("expected 'go' count 2, got %d", tagMap["go"])
	}
	if tagMap["web"] != 2 {
		t.Errorf("expected 'web' count 2, got %d", tagMap["web"])
	}
	if tagMap["cli"] != 1 {
		t.Errorf("expected 'cli' count 1, got %d", tagMap["cli"])
	}
}

func TestListByTag(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("x.md", "Note X", []string{"shared"})
	if err != nil {
		t.Fatalf("Create x: %v", err)
	}
	_, err = svc.Create("y.md", "Note Y", []string{"shared", "extra"})
	if err != nil {
		t.Fatalf("Create y: %v", err)
	}
	_, err = svc.Create("z.md", "Note Z", []string{"other"})
	if err != nil {
		t.Fatalf("Create z: %v", err)
	}

	metas, err := svc.ListByTag("shared")
	if err != nil {
		t.Fatalf("ListByTag: %v", err)
	}

	if len(metas) != 2 {
		t.Fatalf("expected 2 notes with tag 'shared', got %d", len(metas))
	}

	paths := make(map[string]bool)
	for _, m := range metas {
		paths[m.Path] = true
	}
	if !paths["x.md"] || !paths["y.md"] {
		t.Errorf("expected x.md and y.md, got %v", paths)
	}
}

func TestListByFolder(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("work/task1.md", "Task 1", nil)
	if err != nil {
		t.Fatalf("Create task1: %v", err)
	}
	_, err = svc.Create("work/task2.md", "Task 2", nil)
	if err != nil {
		t.Fatalf("Create task2: %v", err)
	}
	_, err = svc.Create("personal/diary.md", "Dear diary", nil)
	if err != nil {
		t.Fatalf("Create diary: %v", err)
	}

	notes, err := svc.List("work")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(notes) != 2 {
		t.Fatalf("expected 2 notes in 'work' folder, got %d", len(notes))
	}

	for _, n := range notes {
		if n.Folder != "work" {
			t.Errorf("expected folder 'work', got %q for %s", n.Folder, n.Path)
		}
	}
}

func TestListAll(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("one.md", "One", nil)
	if err != nil {
		t.Fatalf("Create one: %v", err)
	}
	_, err = svc.Create("sub/two.md", "Two", nil)
	if err != nil {
		t.Fatalf("Create two: %v", err)
	}

	notes, err := svc.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}

	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}
}

func TestSync(t *testing.T) {
	svc := setupService(t)

	// Create some notes
	_, err := svc.Create("sync1.md", "Sync test one", []string{"sync"})
	if err != nil {
		t.Fatalf("Create sync1: %v", err)
	}
	_, err = svc.Create("sync2.md", "Sync test two", []string{"sync"})
	if err != nil {
		t.Fatalf("Create sync2: %v", err)
	}

	// Manually add a file on disk that isn't in metadata/search
	absPath := filepath.Join(svc.files.Root(), "sync3.md")
	content := "---\ntags:\n    - manual\ncreated: " + time.Now().Format(time.RFC3339) + "\nmodified: " + time.Now().Format(time.RFC3339) + "\n---\nManually added"
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := svc.Sync(); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// sync3.md should now be in metadata
	nm, err := svc.meta.GetNote("sync3.md")
	if err != nil {
		t.Fatalf("GetNote sync3: %v", err)
	}
	if nm.Title != "sync3" {
		t.Errorf("expected title 'sync3', got %q", nm.Title)
	}

	// Should be in search index too
	cnt, err := svc.search.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if cnt != 3 {
		t.Errorf("expected 3 docs in search after sync, got %d", cnt)
	}
}

func TestGet(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("getme.md", "Get this note", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	n, err := svc.Get("getme.md")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if n.Path != "getme.md" {
		t.Errorf("expected path getme.md, got %s", n.Path)
	}
}

func TestEditorCommand(t *testing.T) {
	svc := setupService(t)
	if cmd := svc.EditorCommand(); cmd != "cat" {
		t.Errorf("expected editor command 'cat', got %q", cmd)
	}
}

func TestAbsPath(t *testing.T) {
	svc := setupService(t)
	abs := svc.AbsPath("test.md")
	if !filepath.IsAbs(abs) {
		t.Errorf("expected absolute path, got %s", abs)
	}
	if !strings.HasSuffix(abs, "test.md") {
		t.Errorf("expected path ending in test.md, got %s", abs)
	}
}

func TestNilOptionalComponents(t *testing.T) {
	root := t.TempDir()

	files, err := storage.NewFileStore(root)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	meta, err := storage.NewMemoryMetaStore()
	if err != nil {
		t.Fatalf("NewMemoryMetaStore: %v", err)
	}
	t.Cleanup(func() { meta.Close() })

	// No search, no git, no editor
	svc := New(files, meta, nil, nil, nil)
	t.Cleanup(func() { svc.Close() })

	n, err := svc.Create("minimal.md", "Minimal note", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if n.Path != "minimal.md" {
		t.Errorf("expected path minimal.md, got %s", n.Path)
	}

	// Search should return error when not configured
	_, err = svc.Search("anything", 10)
	if err == nil {
		t.Error("expected error when search index is nil")
	}

	// EditorCommand should return empty
	if cmd := svc.EditorCommand(); cmd != "" {
		t.Errorf("expected empty editor command, got %q", cmd)
	}
}

func TestEnsureFrontmatter_AddsMissingFrontmatter(t *testing.T) {
	svc := setupService(t)

	// Write a file directly without frontmatter
	absPath := filepath.Join(svc.files.Root(), "no-fm.md")
	if err := os.WriteFile(absPath, []byte("# My Note\n\nSome content here\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	count, err := svc.EnsureFrontmatter()
	if err != nil {
		t.Fatalf("EnsureFrontmatter: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 note fixed, got %d", count)
	}

	// Read back raw content and verify frontmatter was added
	raw, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasPrefix(string(raw), "---\n") {
		t.Error("expected frontmatter to be added to file")
	}
	if !strings.Contains(string(raw), "Some content here") {
		t.Error("expected original content to be preserved")
	}
}

func TestEnsureFrontmatter_SkipsExistingFrontmatter(t *testing.T) {
	svc := setupService(t)

	// Create a note normally (with frontmatter)
	_, err := svc.Create("has-fm.md", "Already has frontmatter", []string{"test"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Read the content before
	absPath := svc.files.AbsPath("has-fm.md")
	before, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	count, err := svc.EnsureFrontmatter()
	if err != nil {
		t.Fatalf("EnsureFrontmatter: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 notes fixed, got %d", count)
	}

	// Verify file wasn't modified
	after, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(before) != string(after) {
		t.Error("file with existing frontmatter should not be modified")
	}
}

func TestEnsureFrontmatter_UsesFileMtime(t *testing.T) {
	svc := setupService(t)

	// Write a file directly without frontmatter
	absPath := filepath.Join(svc.files.Root(), "old-note.md")
	if err := os.WriteFile(absPath, []byte("# Old Note\n\nContent\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Set a known mtime
	knownTime := time.Date(2022, 3, 15, 10, 30, 0, 0, time.UTC)
	if err := os.Chtimes(absPath, knownTime, knownTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	count, err := svc.EnsureFrontmatter()
	if err != nil {
		t.Fatalf("EnsureFrontmatter: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 note fixed, got %d", count)
	}

	// Reload the note and check Created timestamp matches the known mtime
	n, err := svc.files.Load("old-note.md")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !n.Created.Equal(knownTime) {
		t.Errorf("Created = %v, want %v", n.Created, knownTime)
	}
}

func TestDeleteFolder(t *testing.T) {
	svc := setupService(t)

	// Create notes in testfolder
	_, err := svc.Create("testfolder/note1", "Note 1", nil)
	if err != nil {
		t.Fatalf("Create note1: %v", err)
	}
	_, err = svc.Create("testfolder/note2", "Note 2", nil)
	if err != nil {
		t.Fatalf("Create note2: %v", err)
	}
	_, err = svc.Create("testfolder/note3", "Note 3", nil)
	if err != nil {
		t.Fatalf("Create note3: %v", err)
	}
	// Create a note in another folder
	_, err = svc.Create("other/keep.md", "Keep me", nil)
	if err != nil {
		t.Fatalf("Create keep: %v", err)
	}

	count, err := svc.DeleteFolder("testfolder")
	if err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 deleted, got %d", count)
	}

	// Verify the notes are gone from ListAll
	all, err := svc.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	for _, n := range all {
		if strings.HasPrefix(n.Path, "testfolder/") {
			t.Errorf("note %s should have been deleted", n.Path)
		}
	}

	// Notes in other folders are unaffected
	if len(all) != 1 {
		t.Errorf("expected 1 remaining note, got %d", len(all))
	}

	// Verify the folder directory no longer exists on disk
	folderPath := filepath.Join(svc.files.Root(), "testfolder")
	if _, err := os.Stat(folderPath); !os.IsNotExist(err) {
		t.Errorf("expected testfolder directory to be removed, but it still exists")
	}
}

func TestDeleteFolder_Empty(t *testing.T) {
	svc := setupService(t)

	count, err := svc.DeleteFolder("nonexistent")
	if err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 deleted for non-existent folder, got %d", count)
	}
}

func TestDeleteFolder_NestedFolder(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("parent/child/note1", "Child 1", nil)
	if err != nil {
		t.Fatalf("Create child/note1: %v", err)
	}
	_, err = svc.Create("parent/child/note2", "Child 2", nil)
	if err != nil {
		t.Fatalf("Create child/note2: %v", err)
	}
	_, err = svc.Create("parent/other.md", "Other note", nil)
	if err != nil {
		t.Fatalf("Create parent/other: %v", err)
	}

	count, err := svc.DeleteFolder("parent/child")
	if err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 deleted, got %d", count)
	}

	// Verify only child notes are deleted
	all, err := svc.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected 1 remaining note, got %d", len(all))
	}
	if all[0].Path != "parent/other.md" {
		t.Errorf("expected parent/other.md to remain, got %s", all[0].Path)
	}
}

func TestOpen_AddsFrontmatterOnAccess(t *testing.T) {
	svc := setupService(t)

	// Write a file directly without frontmatter
	absPath := filepath.Join(svc.files.Root(), "open-no-fm.md")
	if err := os.WriteFile(absPath, []byte("# Open Me\n\nNo frontmatter\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	n, err := svc.Open("open-no-fm.md")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if n.Path != "open-no-fm.md" {
		t.Errorf("expected path open-no-fm.md, got %s", n.Path)
	}

	// Read back raw content and verify frontmatter was added
	raw, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.HasPrefix(string(raw), "---\n") {
		t.Error("expected frontmatter to be added after Open")
	}
	if !strings.Contains(string(raw), "No frontmatter") {
		t.Error("expected original content to be preserved after Open")
	}
}

// --- Bookmark/Pin service tests ---

func TestTogglePin(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("todo.md", "", []string{})
	if err != nil {
		t.Fatal(err)
	}

	// Pin
	pinned, err := svc.TogglePin("todo.md")
	if err != nil {
		t.Fatalf("TogglePin: %v", err)
	}
	if !pinned {
		t.Error("expected note to be pinned")
	}

	// Verify
	isPinned, err := svc.IsPinned("todo.md")
	if err != nil {
		t.Fatal(err)
	}
	if !isPinned {
		t.Error("expected IsPinned to return true")
	}

	// Unpin
	pinned, err = svc.TogglePin("todo.md")
	if err != nil {
		t.Fatalf("TogglePin (unpin): %v", err)
	}
	if pinned {
		t.Error("expected note to be unpinned")
	}
}

func TestListPinned(t *testing.T) {
	svc := setupService(t)

	for _, name := range []string{"a.md", "b.md", "c.md"} {
		if _, err := svc.Create(name, "", nil); err != nil {
			t.Fatal(err)
		}
	}

	for _, name := range []string{"a.md", "c.md"} {
		if _, err := svc.TogglePin(name); err != nil {
			t.Fatal(err)
		}
	}

	pins, err := svc.ListPinned()
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 2 {
		t.Fatalf("expected 2 pinned, got %d", len(pins))
	}
	if pins[0] != "a.md" || pins[1] != "c.md" {
		t.Errorf("unexpected pin order: %v", pins)
	}
}

func TestListRecent(t *testing.T) {
	svc := setupService(t)

	for _, name := range []string{"first.md", "second.md", "third.md"} {
		if _, err := svc.Create(name, "", nil); err != nil {
			t.Fatal(err)
		}
		// Small delay so modified times differ
		time.Sleep(50 * time.Millisecond)
	}

	recent, err := svc.ListRecent(2)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2, got %d", len(recent))
	}
	// Most recently created should be first
	if recent[0].Path != "third.md" {
		t.Errorf("expected 'third.md' first, got %q", recent[0].Path)
	}
}

// --- Folder rename (moveFolder) tests ---

func TestMoveFolder(t *testing.T) {
	svc := setupService(t)

	// Create a folder with 3 notes inside
	_, err := svc.Create("OldFolder/note1", "First note", []string{"tag1"})
	if err != nil {
		t.Fatalf("Create note1: %v", err)
	}
	_, err = svc.Create("OldFolder/note2", "Second note", []string{"tag2"})
	if err != nil {
		t.Fatalf("Create note2: %v", err)
	}
	_, err = svc.Create("OldFolder/note3", "Third note", nil)
	if err != nil {
		t.Fatalf("Create note3: %v", err)
	}

	// Move the folder
	if err := svc.Move("OldFolder/", "NewFolder"); err != nil {
		t.Fatalf("Move folder: %v", err)
	}

	// Verify old folder no longer exists on disk
	oldDir := filepath.Join(svc.files.Root(), "OldFolder")
	if _, statErr := os.Stat(oldDir); !os.IsNotExist(statErr) {
		t.Error("old folder still exists on disk after rename")
	}

	// Verify new folder exists on disk
	newDir := filepath.Join(svc.files.Root(), "NewFolder")
	info, statErr := os.Stat(newDir)
	if statErr != nil {
		t.Fatalf("new folder does not exist on disk: %v", statErr)
	}
	if !info.IsDir() {
		t.Error("expected NewFolder to be a directory")
	}

	// Verify all notes have updated paths in metadata
	for _, name := range []string{"note1.md", "note2.md", "note3.md"} {
		newPath := "NewFolder/" + name
		nm, getErr := svc.meta.GetNote(newPath)
		if getErr != nil {
			t.Errorf("GetNote(%s): %v", newPath, getErr)
			continue
		}
		if nm.Folder != "NewFolder" {
			t.Errorf("expected folder 'NewFolder' for %s, got %q", newPath, nm.Folder)
		}
	}

	// Verify old paths no longer exist in metadata
	for _, name := range []string{"note1.md", "note2.md", "note3.md"} {
		oldPath := "OldFolder/" + name
		_, getErr := svc.meta.GetNote(oldPath)
		if !errors.Is(getErr, storage.ErrNoteNotFound) {
			t.Errorf("expected ErrNoteNotFound for old path %s, got %v", oldPath, getErr)
		}
	}

	// Verify notes are loadable from disk at new paths
	for _, name := range []string{"note1.md", "note2.md", "note3.md"} {
		newPath := "NewFolder/" + name
		if !svc.files.Exists(newPath) {
			t.Errorf("expected %s to exist on disk", newPath)
		}
	}
}

func TestMoveFolder_SameNameIsNoop(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("MyFolder/note1", "Content", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Move to the same name — should be a no-op
	if err := svc.Move("MyFolder/", "MyFolder"); err != nil {
		t.Fatalf("Move same name: %v", err)
	}

	// Folder and note should still exist
	if !svc.files.Exists("MyFolder/note1.md") {
		t.Error("note should still exist after no-op move")
	}
	nm, getErr := svc.meta.GetNote("MyFolder/note1.md")
	if getErr != nil {
		t.Fatalf("GetNote: %v", getErr)
	}
	if nm.Folder != "MyFolder" {
		t.Errorf("expected folder 'MyFolder', got %q", nm.Folder)
	}
}

func TestMoveFolder_DetectsDirectoryWithoutTrailingSlash(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("SrcFolder/note1", "Content", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Move without trailing slash — should still detect the directory
	if err := svc.Move("SrcFolder", "DstFolder"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	// Verify move happened
	if !svc.files.Exists("DstFolder/note1.md") {
		t.Error("expected DstFolder/note1.md to exist")
	}
	_, getErr := svc.meta.GetNote("SrcFolder/note1.md")
	if !errors.Is(getErr, storage.ErrNoteNotFound) {
		t.Errorf("expected ErrNoteNotFound for old path, got %v", getErr)
	}
}

func TestMoveFolder_NestedNotes(t *testing.T) {
	svc := setupService(t)

	// Create notes at multiple depth levels within the folder
	_, err := svc.Create("Parent/note1", "Note 1", nil)
	if err != nil {
		t.Fatalf("Create note1: %v", err)
	}
	_, err = svc.Create("Parent/Sub/note2", "Note 2", nil)
	if err != nil {
		t.Fatalf("Create note2: %v", err)
	}

	if err := svc.Move("Parent/", "Renamed"); err != nil {
		t.Fatalf("Move: %v", err)
	}

	// Top-level note should be moved
	if !svc.files.Exists("Renamed/note1.md") {
		t.Error("expected Renamed/note1.md to exist")
	}
	nm, getErr := svc.meta.GetNote("Renamed/note1.md")
	if getErr != nil {
		t.Fatalf("GetNote Renamed/note1.md: %v", getErr)
	}
	if nm.Folder != "Renamed" {
		t.Errorf("expected folder 'Renamed', got %q", nm.Folder)
	}

	// Nested note should be moved
	if !svc.files.Exists("Renamed/Sub/note2.md") {
		t.Error("expected Renamed/Sub/note2.md to exist")
	}
	nm2, getErr := svc.meta.GetNote("Renamed/Sub/note2.md")
	if getErr != nil {
		t.Fatalf("GetNote Renamed/Sub/note2.md: %v", getErr)
	}
	if nm2.Folder != "Renamed/Sub" {
		t.Errorf("expected folder 'Renamed/Sub', got %q", nm2.Folder)
	}
}

// --- Todo integration tests ---

func TestCreateTodo(t *testing.T) {
	svc := setupService(t)

	due := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	n, err := svc.CreateTodo(CreateTodoOptions{
		Title:  "fix auth bug",
		Folder: "TODO",
		Tags:   []string{"work"},
		Due:    &due,
	})
	if err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}

	if n.Path != "TODO/fix-auth-bug.md" {
		t.Errorf("expected path TODO/fix-auth-bug.md, got %s", n.Path)
	}
	if !n.Todo {
		t.Error("expected Todo=true")
	}
	if n.Done {
		t.Error("expected Done=false")
	}
	if n.Due == nil || n.Due.Format(time.DateOnly) != "2026-04-15" {
		t.Errorf("expected due=2026-04-15, got %v", n.Due)
	}
	if len(n.Tags) != 1 || n.Tags[0] != "work" {
		t.Errorf("expected tags=[work], got %v", n.Tags)
	}

	// Verify it was persisted to disk
	loaded, err := svc.Get(n.Path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !loaded.Todo {
		t.Error("loaded note should have Todo=true")
	}

	// Verify it's in the todos list
	todos, err := svc.ListTodos()
	if err != nil {
		t.Fatalf("ListTodos: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}
	if todos[0].Path != n.Path {
		t.Errorf("expected todo path %s, got %s", n.Path, todos[0].Path)
	}
}

func TestToggleTodoDone(t *testing.T) {
	svc := setupService(t)

	n, err := svc.CreateTodo(CreateTodoOptions{
		Title:  "buy milk",
		Folder: "TODO",
	})
	if err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}

	// Toggle to done
	done, err := svc.ToggleTodoDone(n.Path)
	if err != nil {
		t.Fatalf("ToggleTodoDone: %v", err)
	}
	if !done {
		t.Error("expected done=true after first toggle")
	}

	// Verify on disk
	loaded, err := svc.Get(n.Path)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !loaded.Done {
		t.Error("expected loaded note Done=true")
	}

	// Toggle back to undone
	done, err = svc.ToggleTodoDone(n.Path)
	if err != nil {
		t.Fatalf("ToggleTodoDone: %v", err)
	}
	if done {
		t.Error("expected done=false after second toggle")
	}
}

func TestToggleTodoDone_NonTodo(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("regular/note.md", "content", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = svc.ToggleTodoDone("regular/note.md")
	if err == nil {
		t.Error("expected error when toggling non-todo note")
	}
}

func TestCreateTodo_EmptyTitle(t *testing.T) {
	svc := setupService(t)

	_, err := svc.CreateTodo(CreateTodoOptions{
		Title:  "",
		Folder: "TODO",
	})
	if err == nil {
		t.Error("expected error for empty title")
	}
}

func TestEdit(t *testing.T) {
	svc := setupService(t)

	// Create a note first
	_, err := svc.Create("work/notes", "original content", []string{"work"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Edit it
	n, err := svc.Edit("work/notes.md", "updated content")
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}

	if n.Content != "updated content" {
		t.Errorf("expected content 'updated content', got %q", n.Content)
	}

	// Verify on disk via Get
	got, err := svc.Get("work/notes.md")
	if err != nil {
		t.Fatalf("Get after edit: %v", err)
	}
	if got.Content != "updated content" {
		t.Errorf("expected content on disk 'updated content', got %q", got.Content)
	}

	// Tags should be preserved
	nm, err := svc.meta.GetNote("work/notes.md")
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if len(nm.Tags) != 1 || nm.Tags[0] != "work" {
		t.Errorf("expected tags preserved [work], got %v", nm.Tags)
	}
}

func TestEdit_NonexistentFails(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Edit("does/not/exist.md", "content")
	if err == nil {
		t.Error("expected error when editing nonexistent note")
	}
}

func TestEdit_AppendsMDExtension(t *testing.T) {
	svc := setupService(t)

	_, err := svc.Create("myfile", "original", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	n, err := svc.Edit("myfile", "edited")
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if n.Path != "myfile.md" {
		t.Errorf("expected path myfile.md, got %s", n.Path)
	}
}
