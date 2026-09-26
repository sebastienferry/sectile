package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tasks/internal/models"
)

// macroWorkspace is the answer the server relays, shared through models.
type macroWorkspace = models.MacroWorkspace

// macroFetchTimeout bounds the fetch that brings the default branch up to date.
const macroFetchTimeout = 30 * time.Second

// macroWorktreeLocks serialises the preparations of one specifications
// repository: two launches for the same macro must land in the same tree, and
// git refuses two concurrent "worktree add" on one repository anyway.
var macroWorktreeLocks sync.Map

func lockMacroRepository(repo string) func() {
	value, _ := macroWorktreeLocks.LoadOrStore(filepath.Clean(repo), &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// ensureMacroWorktree prepares the checkout a macro's specification is written
// in, inside the specifications repository.
//
// Two macros specified in one checkout share its untracked files, which Git
// carries across a branch switch without a word: each macro therefore gets its
// own worktree at .tasks/worktrees/<KEY>, on its own branch. The branch is the
// existing one named after the key when there is one, else a new
// "<KEY>-<slug>" started from the remote default branch, fetched first. An
// existing tree is reused as it is, uncommitted work included: it is never
// reset. With worktrees off, the checkout itself is returned with the macro
// branch it should be on, and nothing is created. A folder outside any Git
// checkout is returned as it is, with no branch and a warning saying so.
func ensureMacroWorktree(ctx context.Context, specRepo, macroKey, title string, useWorktrees bool) (macroWorkspace, error) {
	specRepo = strings.TrimSpace(specRepo)
	key := strings.ToUpper(strings.TrimSpace(macroKey))
	if specRepo == "" {
		return macroWorkspace{}, fmt.Errorf("aucun dossier des spécifications configuré pour ce projet")
	}
	if key == "" || key == "." || key == ".." || strings.ContainsAny(key, "/\\") {
		return macroWorkspace{}, fmt.Errorf("clé de macro invalide : %q", macroKey)
	}
	if info, err := os.Stat(specRepo); err != nil || !info.IsDir() {
		return macroWorkspace{}, fmt.Errorf("le dossier des spécifications %s est introuvable", specRepo)
	}
	if _, err := gitLocal(ctx, specRepo, "rev-parse", "--git-dir"); err != nil {
		// A plain folder is written in place: there is no branch to be on and
		// nothing to commit, and saying so keeps the skill from trying.
		return macroWorkspace{Path: specRepo, Branch: "", Worktree: false,
			Warning: specRepo + " n'est pas un dépôt Git : la spécification est écrite directement dans ce dossier, sans branche, sans commit ni push"}, nil
	}
	unlock := lockMacroRepository(specRepo)
	defer unlock()

	var warnings []string
	hasOrigin := false
	if remotes, err := gitLocal(ctx, specRepo, "remote"); err == nil {
		for _, remote := range strings.Fields(remotes) {
			hasOrigin = hasOrigin || remote == "origin"
		}
	}
	if !hasOrigin {
		warnings = append(warnings, "pas de distant origin : la base est la branche par défaut locale")
	} else {
		// A remote that hangs (a VPN down, a credential prompt) must not hold
		// the launch: past the bound, the base is what is known locally.
		fetchCtx, cancel := context.WithTimeout(ctx, macroFetchTimeout)
		_, err := gitLocal(fetchCtx, specRepo, "fetch", "--quiet", "--prune", "origin")
		cancel()
		if err != nil {
			warnings = append(warnings, "fetch impossible, la base peut être périmée : "+err.Error())
		}
	}

	defaultBranch, base := macroBaseBranch(ctx, specRepo)
	branch, err := existingMacroBranch(ctx, specRepo, key, defaultBranch)
	if err != nil {
		return macroWorkspace{}, err
	}
	if branch == "" {
		branch = models.MacroBranchName(key, title)
	}
	if branch == defaultBranch {
		return macroWorkspace{}, fmt.Errorf("la branche de la macro %s ne peut pas être la branche par défaut %s", key, defaultBranch)
	}
	if _, err := gitLocal(ctx, specRepo, "check-ref-format", "--branch", branch); err != nil {
		return macroWorkspace{}, err
	}
	warning := strings.Join(warnings, " ; ")

	if !useWorktrees {
		note := "worktrees désactivés : le checkout est utilisé directement"
		if warning != "" {
			note = warning + " ; " + note
		}
		return macroWorkspace{Path: specRepo, Branch: branch, Worktree: false, Warning: note}, nil
	}

	// A branch already checked out, in the main checkout or in any worktree,
	// is reused where it is: a second tree on it is what git refuses, and the
	// checkout that holds it may carry uncommitted work.
	existing, err := worktreeForBranch(ctx, specRepo, branch)
	if err != nil {
		return macroWorkspace{}, err
	}
	if existing != "" {
		if sameDirectory(existing, specRepo) {
			existing = specRepo
		}
		return macroWorkspace{Path: existing, Branch: branch, Worktree: !sameDirectory(existing, specRepo), Warning: warning}, nil
	}

	if err := excludeTaskWorktrees(ctx, specRepo); err != nil {
		return macroWorkspace{}, err
	}
	target := filepath.Join(specRepo, ".tasks", "worktrees", key)
	if err := clearStaleMacroPath(ctx, specRepo, target); err != nil {
		return macroWorkspace{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return macroWorkspace{}, err
	}
	args := []string{"worktree", "add", target, branch}
	if _, err := gitLocal(ctx, specRepo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
		start := base
		if _, err := gitLocal(ctx, specRepo, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch); err == nil {
			start = "refs/remotes/origin/" + branch
		}
		if start == "" {
			return macroWorkspace{}, fmt.Errorf("aucune branche par défaut dans %s pour démarrer la branche %s", specRepo, branch)
		}
		args = []string{"worktree", "add", "-b", branch, target, start}
	}
	if _, err := gitLocal(ctx, specRepo, args...); err != nil {
		return macroWorkspace{}, err
	}
	return macroWorkspace{Path: target, Branch: branch, Worktree: true, Warning: warning}, nil
}

// macroBaseBranch names the repository's default branch and the ref a new
// macro branch starts from: the remote default branch when the remote says
// which one it is, else a local main or master.
func macroBaseBranch(ctx context.Context, repo string) (name, base string) {
	if head, err := gitLocal(ctx, repo, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil && strings.HasPrefix(head, "origin/") {
		return strings.TrimPrefix(head, "origin/"), "refs/remotes/" + head
	}
	for _, candidate := range []string{"main", "master"} {
		if _, err := gitLocal(ctx, repo, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+candidate); err == nil {
			return candidate, "refs/remotes/origin/" + candidate
		}
		if _, err := gitLocal(ctx, repo, "show-ref", "--verify", "--quiet", "refs/heads/"+candidate); err == nil {
			return candidate, "refs/heads/" + candidate
		}
	}
	// A clone whose origin/HEAD was never recorded (a remote added by hand)
	// and whose default branch has another name: the remote says which.
	lsCtx, cancel := context.WithTimeout(ctx, macroFetchTimeout)
	defer cancel()
	if out, err := gitLocal(lsCtx, repo, "ls-remote", "--symref", "origin", "HEAD"); err == nil {
		for _, line := range strings.Split(out, "\n") {
			if rest, ok := strings.CutPrefix(line, "ref: refs/heads/"); ok {
				name := strings.TrimSpace(strings.SplitN(rest, "\t", 2)[0])
				if _, err := gitLocal(ctx, repo, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+name); err == nil {
					return name, "refs/remotes/origin/" + name
				}
			}
		}
	}
	return "", ""
}

// existingMacroBranch finds the branch already named after the macro key,
// local branches first, then the remote ones under their local name. The
// default branch never qualifies, whatever its name.
func existingMacroBranch(ctx context.Context, repo, key, defaultBranch string) (string, error) {
	for _, scope := range []struct{ refs, strip string }{{"refs/heads", ""}, {"refs/remotes/origin", "origin/"}} {
		out, err := gitLocal(ctx, repo, "for-each-ref", "--format=%(refname:short)", scope.refs)
		if err != nil {
			return "", err
		}
		for _, line := range strings.Split(out, "\n") {
			name := strings.TrimPrefix(strings.TrimSpace(line), scope.strip)
			if name == "" || name == "HEAD" || name == defaultBranch {
				continue
			}
			if models.MacroBranchMatches(name, key) {
				return name, nil
			}
		}
	}
	return "", nil
}

// clearStaleMacroPath makes room at the worktree path. Git no longer knowing a
// tree there is pruned; an empty leftover directory is removed. A directory
// that still holds something is never deleted: it may be somebody's work, and
// the refusal names it instead.
func clearStaleMacroPath(ctx context.Context, repo, target string) error {
	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s existe et n'est pas un dossier : déplacez-le pour créer le worktree de la macro", target)
	}
	if _, err := gitLocal(ctx, repo, "worktree", "prune"); err != nil {
		return err
	}
	if top, err := gitLocal(ctx, target, "rev-parse", "--show-toplevel"); err == nil && sameDirectory(top, target) {
		// A valid worktree on another branch: leave it, and say so.
		occupant, _ := gitLocal(ctx, target, "branch", "--show-current")
		return fmt.Errorf("%s est déjà un worktree, sur la branche %q : retirez-le pour y placer la macro", target, occupant)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s n'est pas un worktree valide et n'est pas vide : déplacez son contenu pour recréer le worktree de la macro", target)
	}
	return os.Remove(target)
}

// excludeTaskWorktrees keeps .tasks/ out of the repository's status through its
// info/exclude file, so the repository's own .gitignore is never edited.
func excludeTaskWorktrees(ctx context.Context, repo string) error {
	path, err := excludeFilePath(ctx, repo)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		switch strings.TrimSpace(line) {
		case ".tasks", ".tasks/", "/.tasks", "/.tasks/":
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := string(raw)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return os.WriteFile(path, []byte(content+"/.tasks/\n"), 0o644)
}
