package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

var (
	ErrNoRemote        = errors.New("remote not configured")
	ErrNothingToCommit = errors.New("nothing to commit")
)

type Repository struct {
	path string
	repo *gogit.Repository
}

// InitOrOpen opens an existing git repository at path, or initializes a new one.
func InitOrOpen(path string) (*Repository, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("path %q: %w", path, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path %q is not a directory", path)
	}

	repo, err := gogit.PlainOpen(path)
	if err != nil {
		repo, err = gogit.PlainInit(path, false)
		if err != nil {
			return nil, fmt.Errorf("init repo at %q: %w", path, err)
		}
	}

	return &Repository{path: path, repo: repo}, nil
}

// HasChanges reports whether the worktree has any uncommitted changes.
func (r *Repository) HasChanges() (bool, error) {
	wt, err := r.repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("status: %w", err)
	}

	return !status.IsClean(), nil
}

// CommitAll stages all changes and commits with the given message.
// If there are no changes, it returns nil (no-op).
func (r *Repository) CommitAll(message string) error {
	wt, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	// Stage all changes including deletions
	err = wt.AddWithOptions(&gogit.AddOptions{All: true})
	if err != nil {
		return fmt.Errorf("stage changes: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}

	if status.IsClean() {
		return nil
	}

	_, err = wt.Commit("memoria: "+message, &gogit.CommitOptions{
		Author: &object.Signature{
			Name:  "memoria",
			Email: "memoria@local",
			When:  time.Now(),
		},
		AllowEmptyCommits: false,
	})
	if err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// SetRemote adds or updates a named remote with the given URL.
func (r *Repository) SetRemote(name string, url string) error {
	_, err := r.repo.Remote(name)
	if err == nil {
		// Remote exists — delete it so we can recreate with the new URL.
		if err := r.repo.DeleteRemote(name); err != nil {
			return fmt.Errorf("delete existing remote %q: %w", name, err)
		}
	}

	_, err = r.repo.CreateRemote(&config.RemoteConfig{
		Name: name,
		URLs: []string{url},
		Fetch: []config.RefSpec{
			config.RefSpec(fmt.Sprintf("+refs/heads/*:refs/remotes/%s/*", name)),
		},
	})
	if err != nil {
		return fmt.Errorf("create remote %q: %w", name, err)
	}
	return nil
}

// CloneFrom replaces the current repository with a fresh clone from url.
// This is the most reliable way to set up a repo from an existing remote.
// It preserves any non-.git files that don't conflict with the clone.
func (r *Repository) CloneFrom(url string) error {
	// Clone into a temp directory first
	tmpDir, err := os.MkdirTemp(filepath.Dir(r.path), ".memoria-clone-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = gogit.PlainClone(tmpDir, false, &gogit.CloneOptions{
		URL: url,
	})
	if err != nil {
		return fmt.Errorf("cloning: %w", err)
	}

	// Remove the old .git and replace with the cloned one
	oldGit := filepath.Join(r.path, ".git")
	if err := os.RemoveAll(oldGit); err != nil {
		return fmt.Errorf("removing old .git: %w", err)
	}

	newGit := filepath.Join(tmpDir, ".git")
	if err := os.Rename(newGit, oldGit); err != nil {
		return fmt.Errorf("moving .git: %w", err)
	}

	// Re-open the repository
	repo, err := gogit.PlainOpen(r.path)
	if err != nil {
		return fmt.Errorf("reopening repo: %w", err)
	}
	r.repo = repo

	// Checkout the files from the clone into our working directory
	wt, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	head, err := r.repo.Head()
	if err != nil {
		return fmt.Errorf("head: %w", err)
	}

	err = wt.Reset(&gogit.ResetOptions{
		Commit: head.Hash(),
		Mode:   gogit.HardReset,
	})
	if err != nil {
		return fmt.Errorf("checkout: %w", err)
	}

	return nil
}

// HasRemote reports whether a remote with the given name is configured.
func (r *Repository) HasRemote(name string) bool {
	_, err := r.repo.Remote(name)
	return err == nil
}

// Push pushes to the named remote.
func (r *Repository) Push(remoteName string) error {
	if !r.HasRemote(remoteName) {
		return fmt.Errorf("remote %q: %w", remoteName, ErrNoRemote)
	}

	err := r.repo.Push(&gogit.PushOptions{
		RemoteName: remoteName,
	})
	if err != nil {
		if errors.Is(err, gogit.NoErrAlreadyUpToDate) {
			return nil
		}
		return fmt.Errorf("push to %q: %w", remoteName, err)
	}
	return nil
}

// Pull fetches and merges from the named remote.
func (r *Repository) Pull(remoteName string) error {
	if !r.HasRemote(remoteName) {
		return fmt.Errorf("remote %q: %w", remoteName, ErrNoRemote)
	}

	wt, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	err = wt.Pull(&gogit.PullOptions{
		RemoteName: remoteName,
	})
	if err != nil {
		if errors.Is(err, gogit.NoErrAlreadyUpToDate) {
			return nil
		}
		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			return nil
		}
		// Handle non-fast-forward (e.g., pulling into empty/diverged repo):
		// fetch + reset to the remote's HEAD.
		if err := r.fetchAndReset(remoteName); err != nil {
			return fmt.Errorf("pull from %q: %w", remoteName, err)
		}
		return nil
	}
	return nil
}

// fetchAndReset fetches from the remote and resets the worktree to the remote's
// default branch. Used when a normal pull fails (e.g., unrelated histories).
func (r *Repository) fetchAndReset(remoteName string) error {
	err := r.repo.Fetch(&gogit.FetchOptions{
		RemoteName: remoteName,
	})
	if err != nil && !errors.Is(err, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("fetch: %w", err)
	}

	// Find the remote's default branch (try main, then master)
	var remoteRef *plumbing.Reference
	var branchName string
	for _, branch := range []string{"main", "master"} {
		refName := plumbing.NewRemoteReferenceName(remoteName, branch)
		ref, err := r.repo.Reference(refName, true)
		if err == nil {
			remoteRef = ref
			branchName = branch
			break
		}
	}
	if remoteRef == nil {
		return fmt.Errorf("could not find remote branch (tried main, master)")
	}

	// Create local branch pointing at the remote's commit
	localRef := plumbing.NewBranchReferenceName(branchName)
	ref := plumbing.NewHashReference(localRef, remoteRef.Hash())
	if err := r.repo.Storer.SetReference(ref); err != nil {
		return fmt.Errorf("setting local branch ref: %w", err)
	}

	// Point HEAD at the local branch
	headRef := plumbing.NewSymbolicReference(plumbing.HEAD, localRef)
	if err := r.repo.Storer.SetReference(headRef); err != nil {
		return fmt.Errorf("setting HEAD: %w", err)
	}

	// Reset worktree to match
	wt, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("worktree: %w", err)
	}

	err = wt.Reset(&gogit.ResetOptions{
		Commit: remoteRef.Hash(),
		Mode:   gogit.HardReset,
	})
	if err != nil {
		return fmt.Errorf("reset: %w", err)
	}

	// Set up branch tracking so push knows the upstream
	cfg, err := r.repo.Config()
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	if cfg.Branches == nil {
		cfg.Branches = make(map[string]*config.Branch)
	}
	cfg.Branches[branchName] = &config.Branch{
		Name:   branchName,
		Remote: remoteName,
		Merge:  localRef,
	}
	if err := r.repo.SetConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	return nil
}

// CommitAndPush commits all changes and pushes if a remote named "origin" exists.
// If no remote is configured, it only commits.
func (r *Repository) CommitAndPush(message string) error {
	if err := r.CommitAll(message); err != nil {
		return err
	}

	if r.HasRemote("origin") {
		return r.Push("origin")
	}

	return nil
}
