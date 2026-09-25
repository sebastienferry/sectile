package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Folder states the desktop offers a Git initialization for (#481).
const (
	gitStateReady   = "ready"   // inside a checkout whose HEAD names a commit
	gitStateUnborn  = "unborn"  // inside a checkout whose HEAD names no commit yet
	gitStateFolder  = "folder"  // an existing directory outside any checkout
	gitStateMissing = "missing" // anything else
)

// gitFolderState reports what a folder is to Git, and the folder that
// initialization works in: the checkout's top level, or the folder itself
// when it belongs to none.
func gitFolderState(ctx context.Context, path string) (string, string) {
	if path == "" || !filepath.IsAbs(path) {
		return path, gitStateMissing
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return path, gitStateMissing
	}
	path = filepath.Clean(path)
	top, err := gitLocal(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil {
		// A checkout Git refuses to read (an ownership it distrusts, a broken
		// .git) is not a plain folder: initializing it would nest a second
		// repository inside the first.
		if insideGitCheckout(path) {
			return path, gitStateMissing
		}
		return path, gitStateFolder
	}
	top = filepath.Clean(top)
	if _, err := gitLocal(ctx, top, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return top, gitStateUnborn
	}
	return top, gitStateReady
}

// insideGitCheckout tells whether path or one of its parents holds a .git
// entry, whatever Git makes of it.
func insideGitCheckout(path string) bool {
	for dir := path; ; dir = filepath.Dir(dir) {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		if filepath.Dir(dir) == dir {
			return false
		}
	}
}

// desktopGitInit serves the Git initialization of a project folder (#481).
//
// GET ?path= reports the folder's state. POST {path} makes a plain folder a
// repository on main with an empty first commit, or gives an unborn one its
// first commit. Nothing of the folder is staged, no remote is added and
// nothing is pushed: the repository only exists so worktrees can start from
// it.
func (d *agentDaemon) desktopGitInit(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		top, state := gitFolderState(r.Context(), strings.TrimSpace(r.URL.Query().Get("path")))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"path": top, "state": state})
	case http.MethodPost:
		var input struct {
			Path string `json:"path"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&input) != nil {
			http.Error(w, "Folder required", 400)
			return
		}
		path := strings.TrimSpace(input.Path)
		if err := initializableFolder(path); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		d.prepareMu.Lock()
		defer d.prepareMu.Unlock()
		top, state := gitFolderState(r.Context(), filepath.Clean(path))
		if state == gitStateMissing {
			http.Error(w, fmt.Sprintf("%s is no longer a folder Sectile can initialize", path), 400)
			return
		}
		if err := initializableFolder(top); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		initialized, committed := false, false
		if state == gitStateFolder {
			// symbolic-ref rather than init -b, which needs Git 2.28.
			if _, err := gitLocal(r.Context(), top, "init", "-q"); err != nil {
				http.Error(w, err.Error(), 422)
				return
			}
			initialized = true
			if _, err := gitLocal(r.Context(), top, "symbolic-ref", "HEAD", "refs/heads/main"); err != nil {
				http.Error(w, err.Error(), 422)
				return
			}
		}
		if state != gitStateReady {
			// .tasks/ is excluded before the commit: a ready repository is never
			// touched again, so an exclusion that failed after the commit would
			// never be retried. Before it, the repository stays unborn and a
			// retry redoes both.
			if err := excludeTaskWorktrees(r.Context(), top); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			// An empty commit leaves a pre-commit hook nothing to check, and a
			// commit-message linter must not block the setup. A failure is not
			// rolled back: the repository stays unborn, and a retry commits.
			if _, err := gitLocal(r.Context(), top, "commit", "-q", "--allow-empty", "--no-verify", "-m", "Initial commit"); err != nil {
				http.Error(w, err.Error(), 422)
				return
			}
			committed = true
		}
		top, state = gitFolderState(r.Context(), top)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"path": top, "state": state, "initialized": initialized, "committed": committed})
	default:
		http.Error(w, "Method not allowed", 405)
	}
}

// initializableFolder refuses a path that is not an existing absolute folder,
// and the two folders a repository must never be created in by accident.
func initializableFolder(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("The folder to initialize must be an absolute path")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return fmt.Errorf("The folder %s does not exist", path)
	}
	home, _ := os.UserHomeDir()
	if clean := filepath.Clean(path); filepath.Dir(clean) == clean || (home != "" && sameDirectory(clean, home)) {
		return fmt.Errorf("Sectile does not initialize a Git repository in %s: choose the project's own folder", path)
	}
	return nil
}
