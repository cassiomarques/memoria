package git

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"time"
)

// helper: create a bare repo to act as a remote, returns its path.
func createBareRemote(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_, err := gogit.PlainInit(dir, true)
	if err != nil {
		t.Fatalf("init bare repo: %v", err)
	}
	return dir
}

// helper: write a file in a directory.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write file %s: %v", name, err)
	}
}

// helper: remove a file from a directory.
func removeFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.Remove(filepath.Join(dir, name)); err != nil {
		t.Fatalf("remove file %s: %v", name, err)
	}
}

// helper: commit directly using go-git for test setup purposes.
func seedCommit(t *testing.T, repo *gogit.Repository, dir string) {
	t.Helper()
	writeFile(t, dir, ".gitkeep", "")
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wt.Add(".gitkeep"); err != nil {
		t.Fatal(err)
	}
	_, err = wt.Commit("initial", &gogit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test", When: time.Now()},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInitOrOpen_NewRepo(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir)
	if err != nil {
		t.Fatalf("InitOrOpen new: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil Repository")
	}

	// .git should exist
	info, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil {
		t.Fatalf("expected .git dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected .git to be a directory")
	}
}

func TestInitOrOpen_ExistingRepo(t *testing.T) {
	dir := t.TempDir()
	// Init first
	_, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("pre-init: %v", err)
	}

	r, err := InitOrOpen(dir)
	if err != nil {
		t.Fatalf("InitOrOpen existing: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil Repository")
	}
}

func TestInitOrOpen_InvalidPath(t *testing.T) {
	_, err := InitOrOpen("/nonexistent/path/xyz")
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestHasChanges(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Fresh repo, no files — no changes
	has, err := r.HasChanges()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("expected no changes in empty repo")
	}

	// Add a file — should have changes
	writeFile(t, dir, "note.md", "hello")
	has, err = r.HasChanges()
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("expected changes after adding file")
	}
}

func TestCommitAll_NewFiles(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "a.md", "alpha")
	writeFile(t, dir, "b.md", "beta")

	if err := r.CommitAll("add notes"); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}

	// Should be clean now
	has, err := r.HasChanges()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("expected clean after commit")
	}

	// Verify commit message has prefix
	head, err := r.repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	if commit.Message != "memoria: add notes" {
		t.Fatalf("unexpected commit message: %q", commit.Message)
	}
	if commit.Author.Name != "memoria" || commit.Author.Email != "memoria@local" {
		t.Fatalf("unexpected author: %s <%s>", commit.Author.Name, commit.Author.Email)
	}
}

func TestCommitAll_ModifiedFiles(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "note.md", "v1")
	if err := r.CommitAll("create"); err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "note.md", "v2")
	if err := r.CommitAll("update"); err != nil {
		t.Fatal(err)
	}

	has, err := r.HasChanges()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("expected clean after committing modification")
	}
}

func TestCommitAll_DeletedFiles(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "note.md", "content")
	if err := r.CommitAll("create"); err != nil {
		t.Fatal(err)
	}

	removeFile(t, dir, "note.md")

	has, err := r.HasChanges()
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal("expected changes after deletion")
	}

	if err := r.CommitAll("delete note"); err != nil {
		t.Fatal(err)
	}

	has, err = r.HasChanges()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("expected clean after committing deletion")
	}
}

func TestCommitAll_NoChanges(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "note.md", "content")
	if err := r.CommitAll("initial"); err != nil {
		t.Fatal(err)
	}

	// Commit again with no changes — should be no-op
	if err := r.CommitAll("no-op"); err != nil {
		t.Fatalf("expected no error on no-op commit, got: %v", err)
	}
}

func TestSetRemote_And_HasRemote(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	if r.HasRemote("origin") {
		t.Fatal("expected no origin remote initially")
	}

	if err := r.SetRemote("origin", "/some/path"); err != nil {
		t.Fatalf("SetRemote: %v", err)
	}

	if !r.HasRemote("origin") {
		t.Fatal("expected origin remote after SetRemote")
	}

	// Update existing remote
	if err := r.SetRemote("origin", "/another/path"); err != nil {
		t.Fatalf("SetRemote update: %v", err)
	}

	if !r.HasRemote("origin") {
		t.Fatal("expected origin remote after update")
	}
}

func TestPush_NoRemote(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	err = r.Push("origin")
	if err == nil {
		t.Fatal("expected error pushing without remote")
	}
	if !errors.Is(err, ErrNoRemote) {
		t.Fatalf("expected ErrNoRemote, got: %v", err)
	}
}

func TestPull_NoRemote(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	err = r.Pull("origin")
	if err == nil {
		t.Fatal("expected error pulling without remote")
	}
	if !errors.Is(err, ErrNoRemote) {
		t.Fatalf("expected ErrNoRemote, got: %v", err)
	}
}

func TestPush_And_Pull_WithBareRemote(t *testing.T) {
	remoteDir := createBareRemote(t)
	localDir := t.TempDir()

	r, err := InitOrOpen(localDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetRemote("origin", remoteDir); err != nil {
		t.Fatal(err)
	}

	// Create a file and commit
	writeFile(t, localDir, "note.md", "hello")
	if err := r.CommitAll("first note"); err != nil {
		t.Fatal(err)
	}

	// Push to bare remote
	if err := r.Push("origin"); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// Push again (already up to date) — should not error
	if err := r.Push("origin"); err != nil {
		t.Fatalf("Push (up-to-date): %v", err)
	}

	// Clone from remote into a second local to simulate pull
	secondDir := t.TempDir()
	r2, err := InitOrOpen(secondDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r2.SetRemote("origin", remoteDir); err != nil {
		t.Fatal(err)
	}

	// Pull into second repo
	if err := r2.Pull("origin"); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	// Verify the file arrived
	content, err := os.ReadFile(filepath.Join(secondDir, "note.md"))
	if err != nil {
		t.Fatalf("read pulled file: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("unexpected content: %q", string(content))
	}

	// Pull again (already up to date) — should not error
	if err := r2.Pull("origin"); err != nil {
		t.Fatalf("Pull (up-to-date): %v", err)
	}
}

func TestCommitAndPush_NoRemote(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	writeFile(t, dir, "note.md", "content")

	// CommitAndPush without remote should still commit
	if err := r.CommitAndPush("auto save"); err != nil {
		t.Fatalf("CommitAndPush: %v", err)
	}

	has, err := r.HasChanges()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("expected clean after CommitAndPush")
	}

	// Verify commit exists
	head, err := r.repo.Head()
	if err != nil {
		t.Fatal(err)
	}
	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		t.Fatal(err)
	}
	if commit.Message != "memoria: auto save" {
		t.Fatalf("unexpected message: %q", commit.Message)
	}
}

func TestCommitAndPush_WithRemote(t *testing.T) {
	remoteDir := createBareRemote(t)
	localDir := t.TempDir()

	r, err := InitOrOpen(localDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.SetRemote("origin", remoteDir); err != nil {
		t.Fatal(err)
	}

	writeFile(t, localDir, "note.md", "content")

	if err := r.CommitAndPush("sync notes"); err != nil {
		t.Fatalf("CommitAndPush with remote: %v", err)
	}

	// Verify pushed by cloning
	cloneDir := t.TempDir()
	_, err = gogit.PlainClone(cloneDir, false, &gogit.CloneOptions{URL: remoteDir})
	if err != nil {
		t.Fatalf("clone to verify push: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(cloneDir, "note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "content" {
		t.Fatalf("unexpected pushed content: %q", string(content))
	}
}

func TestCommitAll_MixedChanges(t *testing.T) {
	dir := t.TempDir()
	r, err := InitOrOpen(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Create multiple files
	writeFile(t, dir, "a.md", "alpha")
	writeFile(t, dir, "b.md", "beta")
	writeFile(t, dir, "c.md", "gamma")
	if err := r.CommitAll("initial"); err != nil {
		t.Fatal(err)
	}

	// Modify one, delete another, add a new one
	writeFile(t, dir, "a.md", "alpha-v2")
	removeFile(t, dir, "b.md")
	writeFile(t, dir, "d.md", "delta")

	if err := r.CommitAll("mixed changes"); err != nil {
		t.Fatal(err)
	}

	has, err := r.HasChanges()
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Fatal("expected clean after committing mixed changes")
	}
}
