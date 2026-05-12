// Package git provides a go-git based wrapper for cloning, modifying, and
// pushing to Gitea repositories. It replaces the per-file REST API calls with
// local working-tree operations so that all changes can be committed atomically.
//
// All clones are shallow (depth=1) since RIA never needs history.
// Repos can be either persistent (clone once, pull on reuse) or ephemeral
// (temp dir, removed on Close).
package git

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Repo wraps a cloned git repository with convenience methods for reading,
// writing, and pushing files.
type Repo struct {
	repo      *git.Repository
	wt        *git.Worktree
	dir       string
	auth      *http.BasicAuth
	ephemeral bool // if true, Close() removes the directory
}

// Open opens an existing on-disk repository and returns a Repo handle.
func Open(dir, user, pass string) (*Repo, error) {
	auth := &http.BasicAuth{
		Username: user,
		Password: pass,
	}

	r, err := git.PlainOpen(dir)
	if err != nil {
		return nil, fmt.Errorf("opening repo at %s: %w", dir, err)
	}

	wt, err := r.Worktree()
	if err != nil {
		return nil, fmt.Errorf("getting worktree: %w", err)
	}

	return &Repo{
		repo:      r,
		wt:        wt,
		dir:       dir,
		auth:      auth,
		ephemeral: false,
	}, nil
}

// Pull fetches the latest changes from origin and fast-forwards the working
// tree. If the repository is already up to date it is a no-op.
func (r *Repo) Pull() error {
	err := r.wt.Pull(&git.PullOptions{
		Auth:       r.auth,
		RemoteName: "origin",
		Force:      true,
		Depth:      1,
	})
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil
		}
		return fmt.Errorf("pulling: %w", err)
	}
	return nil
}

// Clone clones the repository at url into the given directory and returns a
// Repo handle. If dir is empty, a temporary directory is created and the repo
// is marked as ephemeral (removed on Close). If dir is non-empty and already
// contains a .git directory, the existing repo is opened and pulled instead
// of cloning fresh.
func Clone(url, user, pass, dir string) (*Repo, error) {
	auth := &http.BasicAuth{
		Username: user,
		Password: pass,
	}

	ephemeral := dir == ""

	// Persistent path: if the directory already has a .git, reuse it.
	if !ephemeral {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			repo, err := Open(dir, user, pass)
			if err != nil {
				return nil, fmt.Errorf("reopening persistent repo at %s: %w", dir, err)
			}
			if err := repo.Pull(); err != nil {
				log.Printf("git: WARNING: pull failed for %s, continuing with stale copy: %v", dir, err)
			}
			return repo, nil
		}
		// Ensure the parent directory exists.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	if ephemeral {
		tmp, err := os.MkdirTemp("", "ria-clone-*")
		if err != nil {
			return nil, fmt.Errorf("creating temp dir: %w", err)
		}
		dir = tmp
	}

	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:   url,
		Auth:  auth,
		Depth: 1,
	})
	if err != nil {
		if ephemeral {
			os.RemoveAll(dir)
		}
		return nil, fmt.Errorf("cloning %s: %w", url, err)
	}

	wt, err := r.Worktree()
	if err != nil {
		if ephemeral {
			os.RemoveAll(dir)
		}
		return nil, fmt.Errorf("getting worktree: %w", err)
	}

	return &Repo{
		repo:      r,
		wt:        wt,
		dir:       dir,
		auth:      auth,
		ephemeral: ephemeral,
	}, nil
}

// Dir returns the path to the working tree root.
func (r *Repo) Dir() string {
	return r.dir
}

// ReadFile reads a file from the working tree. Returns ("", nil) if the
// file does not exist.
func (r *Repo) ReadFile(path string) (string, error) {
	fullPath := filepath.Join(r.dir, path)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return string(data), nil
}

// WriteFile writes content to a file in the working tree, creating parent
// directories as needed.
func (r *Repo) WriteFile(path, content string) error {
	fullPath := filepath.Join(r.dir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("creating directories for %s: %w", path, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// ListFiles returns the names of files in the given directory (non-recursive).
// Returns nil (not an error) if the directory does not exist.
func (r *Repo) ListFiles(dir string) ([]string, error) {
	fullPath := filepath.Join(r.dir, dir)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}

	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

// RemoveFile removes a file from the working tree. It is not an error if the
// file does not exist.
func (r *Repo) RemoveFile(path string) error {
	fullPath := filepath.Join(r.dir, path)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", path, err)
	}
	return nil
}

// HasChanges returns true if the working tree has any uncommitted changes
// (modified, added, or deleted files).
func (r *Repo) HasChanges() (bool, error) {
	status, err := r.wt.Status()
	if err != nil {
		return false, fmt.Errorf("checking status: %w", err)
	}
	return !status.IsClean(), nil
}

// CommitAndPush stages all changes, creates a commit, and pushes to the
// remote origin.
func (r *Repo) CommitAndPush(message string) error {
	// Stage all changes (adds, modifications, deletions).
	if err := r.wt.AddWithOptions(&git.AddOptions{All: true}); err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}

	// Also explicitly add any glob pattern to catch everything; go-git's
	// AddWithOptions{All:true} handles new+modified+deleted files.
	// We use AddGlob as a belt-and-suspenders approach.
	_ = r.wt.AddGlob(".")

	_, err := r.wt.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "ria-bot",
			Email: "ria-bot@obs-platform.local",
			When:  time.Now(),
		},
		All: true,
	})
	if err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	if err := r.repo.Push(&git.PushOptions{
		Auth:       r.auth,
		RemoteName: "origin",
		RefSpecs:   []gitconfig.RefSpec{"refs/heads/*:refs/heads/*"},
	}); err != nil {
		return fmt.Errorf("pushing: %w", err)
	}

	return nil
}

// Close cleans up the repository. For ephemeral repos (component source
// clones) this removes the temporary directory. For persistent repos
// (service-catalog, gitops-deploy) this is a no-op — the directory is kept
// for reuse on subsequent reconciliations.
func (r *Repo) Close() {
	if r.ephemeral && r.dir != "" {
		os.RemoveAll(r.dir)
	}
}

// CloneURL builds the Gitea clone URL for a repository.
func CloneURL(giteaBaseURL, owner, repo string) string {
	base := strings.TrimRight(giteaBaseURL, "/")
	return fmt.Sprintf("%s/%s/%s.git", base, owner, repo)
}
