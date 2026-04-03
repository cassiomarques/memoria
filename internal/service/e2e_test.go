package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/cassiomarques/memoria/internal/storage"
)

// countCommits opens the git repo at root and returns the number of commits.
// Returns 0 for a freshly initialized repo with no commits.
func countCommits(t *testing.T, root string) int {
	t.Helper()
	repo, err := gogit.PlainOpen(root)
	if err != nil {
		t.Fatalf("PlainOpen(%s): %v", root, err)
	}
	iter, err := repo.Log(&gogit.LogOptions{Order: gogit.LogOrderCommitterTime})
	if err != nil {
		// Empty repo with no commits — "reference not found"
		return 0
	}
	count := 0
	_ = iter.ForEach(func(c *object.Commit) error {
		count++
		return nil
	})
	return count
}

func TestE2E_FullLifecycle(t *testing.T) {
	svc := setupService(t)

	// Step 1: Create
	var notePath string
	t.Run("Create", func(t *testing.T) {
		n, err := svc.Create("work/project-ideas", "# Project Ideas\n\nBuild a TUI note-taking app in Go.", []string{"golang", "tui"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		notePath = n.Path // "work/project-ideas.md"

		if !svc.files.Exists(notePath) {
			t.Fatal("note file does not exist on disk")
		}
		nm, err := svc.meta.GetNote(notePath)
		if err != nil {
			t.Fatalf("GetNote: %v", err)
		}
		if len(nm.Tags) != 2 {
			t.Errorf("expected 2 tags, got %v", nm.Tags)
		}
		cnt, err := svc.search.Count()
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if cnt != 1 {
			t.Errorf("expected 1 doc in search, got %d", cnt)
		}
	})

	// Step 2: Simulate edit
	t.Run("Edit", func(t *testing.T) {
		absPath := svc.files.AbsPath(notePath)
		newContent := "---\ntags:\n    - golang\n    - tui\ncreated: " + time.Now().Format(time.RFC3339) + "\nmodified: " + time.Now().Format(time.RFC3339) + "\n---\n# Project Ideas\n\nBuild a TUI note-taking app with Bubble Tea framework."
		if err := os.WriteFile(absPath, []byte(newContent), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		changed, err := svc.AfterEdit(notePath)
		if err != nil {
			t.Fatalf("AfterEdit: %v", err)
		}
		if !changed {
			t.Error("expected AfterEdit to report changes")
		}

		n, err := svc.Get(notePath)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if !strings.Contains(n.Content, "Bubble Tea framework") {
			t.Errorf("expected edited content with 'Bubble Tea framework', got %q", n.Content)
		}
	})

	// Step 3: Search
	t.Run("Search", func(t *testing.T) {
		results, err := svc.Search("Bubble Tea", 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected search to find the note")
		}
		if results[0].Path != notePath {
			t.Errorf("expected result path %s, got %s", notePath, results[0].Path)
		}
	})

	// Step 4: Add tags
	t.Run("AddTags", func(t *testing.T) {
		n, err := svc.AddTags(notePath, []string{"important"})
		if err != nil {
			t.Fatalf("AddTags: %v", err)
		}
		if len(n.Tags) != 3 {
			t.Errorf("expected 3 tags, got %v", n.Tags)
		}

		// Verify in metadata
		nm, err := svc.meta.GetNote(notePath)
		if err != nil {
			t.Fatalf("GetNote: %v", err)
		}
		if len(nm.Tags) != 3 {
			t.Errorf("expected 3 tags in metadata, got %v", nm.Tags)
		}

		// Verify in frontmatter on disk
		loaded, err := svc.files.Load(notePath)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !loaded.HasTag("important") || !loaded.HasTag("golang") || !loaded.HasTag("tui") {
			t.Errorf("expected all 3 tags in frontmatter, got %v", loaded.Tags)
		}
	})

	// Step 5: Move
	newPath := "archive/project-ideas.md"
	t.Run("Move", func(t *testing.T) {
		if err := svc.Move(notePath, "archive/project-ideas"); err != nil {
			t.Fatalf("Move: %v", err)
		}

		// Old path gone
		if svc.files.Exists(notePath) {
			t.Error("old path still exists on disk")
		}
		_, err := svc.meta.GetNote(notePath)
		if err != storage.ErrNoteNotFound {
			t.Errorf("expected ErrNoteNotFound for old path, got %v", err)
		}

		// New path exists
		if !svc.files.Exists(newPath) {
			t.Error("new path does not exist on disk")
		}
		nm, err := svc.meta.GetNote(newPath)
		if err != nil {
			t.Fatalf("GetNote new path: %v", err)
		}
		if nm.Folder != "archive" {
			t.Errorf("expected folder 'archive', got %q", nm.Folder)
		}

		// Search finds at new path
		results, err := svc.Search("Bubble Tea", 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) == 0 || results[0].Path != newPath {
			t.Errorf("expected search result at %s", newPath)
		}
	})

	// Step 6: Delete
	t.Run("Delete", func(t *testing.T) {
		if err := svc.Delete(newPath); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		if svc.files.Exists(newPath) {
			t.Error("note still exists on disk after delete")
		}
		_, err := svc.meta.GetNote(newPath)
		if err != storage.ErrNoteNotFound {
			t.Errorf("expected ErrNoteNotFound, got %v", err)
		}
		cnt, err := svc.search.Count()
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if cnt != 0 {
			t.Errorf("expected 0 docs in search after delete, got %d", cnt)
		}
	})
}

func TestE2E_MultiNoteSearch(t *testing.T) {
	svc := setupService(t)

	// Create 12 notes across folders with various tags and content
	notes := []struct {
		path    string
		content string
		tags    []string
	}{
		{"work/golang-web.md", "Building REST APIs with Go and the net/http package", []string{"golang", "web"}},
		{"work/golang-cli.md", "Creating command line tools with cobra library", []string{"golang", "cli"}},
		{"work/python-ml.md", "Machine learning with Python and scikit-learn", []string{"python", "ml"}},
		{"work/python-web.md", "Flask web application development tutorial", []string{"python", "web"}},
		{"personal/journal-jan.md", "January reflections on productivity and health", []string{"journal", "personal"}},
		{"personal/journal-feb.md", "February goals and new year progress", []string{"journal", "personal"}},
		{"recipes/pasta.md", "Homemade carbonara recipe with guanciale", []string{"food", "italian"}},
		{"recipes/sushi.md", "How to make sushi rolls at home", []string{"food", "japanese"}},
		{"learning/algorithms.md", "Notes on sorting algorithms and complexity analysis", []string{"cs", "algorithms"}},
		{"learning/databases.md", "PostgreSQL performance tuning and query optimization", []string{"cs", "databases"}},
		{"ideas/startup.md", "SaaS startup idea for developer productivity tools", []string{"business", "ideas"}},
		{"ideas/app.md", "Mobile app concept for habit tracking with golang backend", []string{"business", "golang"}},
	}

	for _, tc := range notes {
		if _, err := svc.Create(tc.path, tc.content, tc.tags); err != nil {
			t.Fatalf("Create(%s): %v", tc.path, err)
		}
	}

	t.Run("SearchByContent", func(t *testing.T) {
		results, err := svc.Search("golang", 20)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		// Should find notes with "golang" in content or tags
		if len(results) < 2 {
			t.Errorf("expected at least 2 results for 'golang', got %d", len(results))
		}
		// Verify pasta/sushi notes are NOT in results
		for _, r := range results {
			if strings.HasPrefix(r.Path, "recipes/") {
				t.Errorf("recipe note %s should not match 'golang'", r.Path)
			}
		}
	})

	t.Run("SearchFuzzyWithTypos", func(t *testing.T) {
		// "algorythms" is a typo for "algorithms"
		results, err := svc.SearchFuzzy("algorythms", 10)
		if err != nil {
			t.Fatalf("SearchFuzzy: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected fuzzy search to find 'algorithms' with typo")
		}
		found := false
		for _, r := range results {
			if r.Path == "learning/algorithms.md" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find learning/algorithms.md via fuzzy search")
		}
	})

	t.Run("SearchByTagNameInContent", func(t *testing.T) {
		results, err := svc.Search("web", 20)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) < 2 {
			t.Errorf("expected at least 2 results for 'web', got %d", len(results))
		}
	})

	t.Run("ListByTag", func(t *testing.T) {
		metas, err := svc.ListByTag("golang")
		if err != nil {
			t.Fatalf("ListByTag: %v", err)
		}
		if len(metas) != 3 {
			t.Errorf("expected 3 notes with tag 'golang', got %d", len(metas))
		}

		metas, err = svc.ListByTag("food")
		if err != nil {
			t.Fatalf("ListByTag: %v", err)
		}
		if len(metas) != 2 {
			t.Errorf("expected 2 notes with tag 'food', got %d", len(metas))
		}
	})

	t.Run("ListByFolder", func(t *testing.T) {
		workNotes, err := svc.List("work")
		if err != nil {
			t.Fatalf("List(work): %v", err)
		}
		if len(workNotes) != 4 {
			t.Errorf("expected 4 notes in 'work', got %d", len(workNotes))
		}

		recipeNotes, err := svc.List("recipes")
		if err != nil {
			t.Fatalf("List(recipes): %v", err)
		}
		if len(recipeNotes) != 2 {
			t.Errorf("expected 2 notes in 'recipes', got %d", len(recipeNotes))
		}
	})

	t.Run("ListAllTags", func(t *testing.T) {
		tags, err := svc.ListTags()
		if err != nil {
			t.Fatalf("ListTags: %v", err)
		}
		tagMap := make(map[string]int)
		for _, ti := range tags {
			tagMap[ti.Tag] = ti.Count
		}

		if tagMap["golang"] != 3 {
			t.Errorf("expected 'golang' count 3, got %d", tagMap["golang"])
		}
		if tagMap["web"] != 2 {
			t.Errorf("expected 'web' count 2, got %d", tagMap["web"])
		}
		if tagMap["journal"] != 2 {
			t.Errorf("expected 'journal' count 2, got %d", tagMap["journal"])
		}
		if tagMap["cs"] != 2 {
			t.Errorf("expected 'cs' count 2, got %d", tagMap["cs"])
		}
	})
}

func TestE2E_FolderOperations(t *testing.T) {
	svc := setupService(t)

	// Create notes in nested folders
	t.Run("CreateNested", func(t *testing.T) {
		paths := []string{
			"work/2024/q1/report",
			"work/2024/q2/report",
			"personal/journal",
		}
		for _, p := range paths {
			if _, err := svc.Create(p, fmt.Sprintf("Content of %s", p), []string{"report"}); err != nil {
				t.Fatalf("Create(%s): %v", p, err)
			}
		}

		all, err := svc.ListAll()
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}
		if len(all) != 3 {
			t.Errorf("expected 3 notes, got %d", len(all))
		}
	})

	t.Run("ListEachFolderLevel", func(t *testing.T) {
		// List "personal" folder
		personalNotes, err := svc.List("personal")
		if err != nil {
			t.Fatalf("List(personal): %v", err)
		}
		if len(personalNotes) != 1 {
			t.Errorf("expected 1 note in 'personal', got %d", len(personalNotes))
		}

		// List "work/2024/q1" folder
		q1Notes, err := svc.List("work/2024/q1")
		if err != nil {
			t.Fatalf("List(work/2024/q1): %v", err)
		}
		if len(q1Notes) != 1 {
			t.Errorf("expected 1 note in 'work/2024/q1', got %d", len(q1Notes))
		}

		// List "work/2024/q2" folder
		q2Notes, err := svc.List("work/2024/q2")
		if err != nil {
			t.Fatalf("List(work/2024/q2): %v", err)
		}
		if len(q2Notes) != 1 {
			t.Errorf("expected 1 note in 'work/2024/q2', got %d", len(q2Notes))
		}
	})

	t.Run("MoveBetweenFolders", func(t *testing.T) {
		if err := svc.Move("work/2024/q1/report.md", "archive/2024/q1-report"); err != nil {
			t.Fatalf("Move: %v", err)
		}

		if svc.files.Exists("work/2024/q1/report.md") {
			t.Error("old path still exists")
		}
		if !svc.files.Exists("archive/2024/q1-report.md") {
			t.Error("new path does not exist")
		}

		nm, err := svc.meta.GetNote("archive/2024/q1-report.md")
		if err != nil {
			t.Fatalf("GetNote: %v", err)
		}
		if nm.Folder != "archive/2024" {
			t.Errorf("expected folder 'archive/2024', got %q", nm.Folder)
		}
	})

	t.Run("DeleteCleansEmptyParents", func(t *testing.T) {
		if err := svc.Delete("work/2024/q2/report.md"); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		// The file should be gone
		if svc.files.Exists("work/2024/q2/report.md") {
			t.Error("deleted file still exists")
		}

		// Empty parent dirs should be cleaned up
		q2Dir := filepath.Join(svc.files.Root(), "work", "2024", "q2")
		if _, err := os.Stat(q2Dir); !os.IsNotExist(err) {
			t.Error("expected empty q2 dir to be cleaned up")
		}
	})

	t.Run("ListAllSurvivors", func(t *testing.T) {
		all, err := svc.ListAll()
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}
		// Should have: archive/2024/q1-report.md, personal/journal.md
		if len(all) != 2 {
			paths := make([]string, len(all))
			for i, n := range all {
				paths[i] = n.Path
			}
			t.Errorf("expected 2 surviving notes, got %d: %v", len(all), paths)
		}
	})
}

func TestE2E_TagWorkflow(t *testing.T) {
	svc := setupService(t)

	// Create notes with overlapping tags
	_, err := svc.Create("note-a", "Note A content", []string{"go", "web", "api"})
	if err != nil {
		t.Fatalf("Create A: %v", err)
	}
	_, err = svc.Create("note-b", "Note B content", []string{"go", "cli"})
	if err != nil {
		t.Fatalf("Create B: %v", err)
	}
	_, err = svc.Create("note-c", "Note C content", []string{"web", "frontend"})
	if err != nil {
		t.Fatalf("Create C: %v", err)
	}

	t.Run("InitialTagCounts", func(t *testing.T) {
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
		if tagMap["api"] != 1 {
			t.Errorf("expected 'api' count 1, got %d", tagMap["api"])
		}
		if tagMap["cli"] != 1 {
			t.Errorf("expected 'cli' count 1, got %d", tagMap["cli"])
		}
		if tagMap["frontend"] != 1 {
			t.Errorf("expected 'frontend' count 1, got %d", tagMap["frontend"])
		}
	})

	t.Run("AddTagsIncreasesCount", func(t *testing.T) {
		_, err := svc.AddTags("note-b.md", []string{"web"})
		if err != nil {
			t.Fatalf("AddTags: %v", err)
		}
		tags, err := svc.ListTags()
		if err != nil {
			t.Fatalf("ListTags: %v", err)
		}
		tagMap := make(map[string]int)
		for _, ti := range tags {
			tagMap[ti.Tag] = ti.Count
		}
		if tagMap["web"] != 3 {
			t.Errorf("expected 'web' count 3 after AddTags, got %d", tagMap["web"])
		}
	})

	t.Run("RemoveTagsDecreasesCount", func(t *testing.T) {
		_, err := svc.RemoveTags("note-a.md", []string{"web"})
		if err != nil {
			t.Fatalf("RemoveTags: %v", err)
		}
		tags, err := svc.ListTags()
		if err != nil {
			t.Fatalf("ListTags: %v", err)
		}
		tagMap := make(map[string]int)
		for _, ti := range tags {
			tagMap[ti.Tag] = ti.Count
		}
		// note-a had web removed; note-b and note-c still have web
		if tagMap["web"] != 2 {
			t.Errorf("expected 'web' count 2 after RemoveTags, got %d", tagMap["web"])
		}
	})

	t.Run("DeleteTaggedNoteDecreasesCount", func(t *testing.T) {
		if err := svc.Delete("note-c.md"); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		tags, err := svc.ListTags()
		if err != nil {
			t.Fatalf("ListTags: %v", err)
		}
		tagMap := make(map[string]int)
		for _, ti := range tags {
			tagMap[ti.Tag] = ti.Count
		}
		// note-c had web and frontend; now only note-b has web
		if tagMap["web"] != 1 {
			t.Errorf("expected 'web' count 1 after delete, got %d", tagMap["web"])
		}
		if tagMap["frontend"] != 0 {
			t.Errorf("expected 'frontend' count 0 after delete, got %d", tagMap["frontend"])
		}
	})

	t.Run("TagsPersistThroughMove", func(t *testing.T) {
		// note-a has tags: go, api (web was removed)
		if err := svc.Move("note-a.md", "archive/note-a"); err != nil {
			t.Fatalf("Move: %v", err)
		}

		nm, err := svc.meta.GetNote("archive/note-a.md")
		if err != nil {
			t.Fatalf("GetNote: %v", err)
		}
		tagSet := make(map[string]bool)
		for _, tag := range nm.Tags {
			tagSet[tag] = true
		}
		if !tagSet["go"] || !tagSet["api"] {
			t.Errorf("expected tags [go, api] after move, got %v", nm.Tags)
		}

		// Verify on disk too
		loaded, err := svc.files.Load("archive/note-a.md")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !loaded.HasTag("go") || !loaded.HasTag("api") {
			t.Errorf("expected frontmatter tags [go, api] after move, got %v", loaded.Tags)
		}

		// Verify tag counts still correct
		tags, err := svc.ListTags()
		if err != nil {
			t.Fatalf("ListTags: %v", err)
		}
		tagMap := make(map[string]int)
		for _, ti := range tags {
			tagMap[ti.Tag] = ti.Count
		}
		if tagMap["go"] != 2 {
			t.Errorf("expected 'go' count 2 after move, got %d", tagMap["go"])
		}
	})
}

func TestE2E_SyncRecoversState(t *testing.T) {
	svc := setupService(t)

	// Create several notes via service
	_, err := svc.Create("known-1", "First known note", []string{"sync"})
	if err != nil {
		t.Fatalf("Create known-1: %v", err)
	}
	_, err = svc.Create("known-2", "Second known note", []string{"sync"})
	if err != nil {
		t.Fatalf("Create known-2: %v", err)
	}
	_, err = svc.Create("to-vanish", "This will disappear from disk", []string{"sync", "temp"})
	if err != nil {
		t.Fatalf("Create to-vanish: %v", err)
	}

	// Verify initial state
	cnt, err := svc.search.Count()
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if cnt != 3 {
		t.Fatalf("expected 3 docs initially, got %d", cnt)
	}

	// Manually add a file to disk (bypassing service)
	t.Run("ManuallyAddFile", func(t *testing.T) {
		absPath := filepath.Join(svc.files.Root(), "surprise.md")
		content := "---\ntags:\n    - manual\ncreated: " + time.Now().Format(time.RFC3339) + "\nmodified: " + time.Now().Format(time.RFC3339) + "\n---\nSurprise! I was added directly to disk."
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	})

	// Manually delete a file from disk (bypassing service)
	t.Run("ManuallyDeleteFile", func(t *testing.T) {
		absPath := svc.files.AbsPath("to-vanish.md")
		if err := os.Remove(absPath); err != nil {
			t.Fatalf("Remove: %v", err)
		}
	})

	// Call Sync
	t.Run("SyncRecovers", func(t *testing.T) {
		if err := svc.Sync(); err != nil {
			t.Fatalf("Sync: %v", err)
		}

		// surprise.md should now be in metadata
		nm, err := svc.meta.GetNote("surprise.md")
		if err != nil {
			t.Fatalf("GetNote surprise.md: %v", err)
		}
		if nm.Title != "surprise" {
			t.Errorf("expected title 'surprise', got %q", nm.Title)
		}

		// Search should find the manually added note
		results, err := svc.Search("Surprise", 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) == 0 {
			t.Error("expected search to find 'surprise.md'")
		}

		// Search index should have 3 docs: known-1, known-2, surprise
		// (to-vanish is gone from disk so Reindex drops it)
		cnt, err := svc.search.Count()
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if cnt != 3 {
			t.Errorf("expected 3 docs in search after sync, got %d", cnt)
		}

		// Verify the remaining notes are findable
		for _, path := range []string{"known-1.md", "known-2.md", "surprise.md"} {
			if !svc.files.Exists(path) {
				t.Errorf("expected %s to exist on disk", path)
			}
		}
	})
}

func TestE2E_GitCommitHistory(t *testing.T) {
	svc := setupService(t)
	root := svc.files.Root()

	// Baseline: the setupService creates a git repo; count initial commits
	initialCommits := countCommits(t, root)

	// Step 1: Create a note → verify git has a commit
	t.Run("CreateCommit", func(t *testing.T) {
		_, err := svc.Create("git-test", "Testing git integration", []string{"git"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		commits := countCommits(t, root)
		if commits <= initialCommits {
			t.Errorf("expected new commit after create, got %d (was %d)", commits, initialCommits)
		}

		hasChanges, err := svc.repo.HasChanges()
		if err != nil {
			t.Fatalf("HasChanges: %v", err)
		}
		if hasChanges {
			t.Error("expected no uncommitted changes after create")
		}
	})

	commitsAfterCreate := countCommits(t, root)

	// Step 2: Edit note → verify another commit
	t.Run("EditCommit", func(t *testing.T) {
		absPath := svc.files.AbsPath("git-test.md")
		newContent := "---\ntags:\n    - git\ncreated: " + time.Now().Format(time.RFC3339) + "\nmodified: " + time.Now().Format(time.RFC3339) + "\n---\nEdited content for git commit test"
		if err := os.WriteFile(absPath, []byte(newContent), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		changed, err := svc.AfterEdit("git-test.md")
		if err != nil {
			t.Fatalf("AfterEdit: %v", err)
		}
		if !changed {
			t.Error("expected AfterEdit to report changes")
		}

		commits := countCommits(t, root)
		if commits <= commitsAfterCreate {
			t.Errorf("expected new commit after edit, got %d (was %d)", commits, commitsAfterCreate)
		}

		hasChanges, err := svc.repo.HasChanges()
		if err != nil {
			t.Fatalf("HasChanges: %v", err)
		}
		if hasChanges {
			t.Error("expected no uncommitted changes after edit")
		}
	})

	commitsAfterEdit := countCommits(t, root)

	// Step 3: Delete note → verify another commit
	t.Run("DeleteCommit", func(t *testing.T) {
		if err := svc.Delete("git-test.md"); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		commits := countCommits(t, root)
		if commits <= commitsAfterEdit {
			t.Errorf("expected new commit after delete, got %d (was %d)", commits, commitsAfterEdit)
		}

		hasChanges, err := svc.repo.HasChanges()
		if err != nil {
			t.Fatalf("HasChanges: %v", err)
		}
		if hasChanges {
			t.Error("expected no uncommitted changes after delete")
		}
	})
}
