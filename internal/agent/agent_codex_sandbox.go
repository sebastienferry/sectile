package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// worktreeGitWritableRoots scopes built-in Codex launches to a linked worktree's
// metadata, shared object/ref/log directories and packed-reference files. Explicit
// file roots override Codex's protected common .git directory without opening
// hooks or configuration. Include absent lock/temporary files: Git creates them
// during reference cleanup and atomically replaces packed-refs when packing.
func worktreeGitWritableRoots(dir string) []string {
	if strings.TrimSpace(dir) == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := gitLocal(ctx, dir, "rev-parse", "--git-dir", "--git-common-dir")
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return nil
	}
	resolve := func(p string) string {
		p = strings.TrimSpace(p)
		if p == "" {
			return ""
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		p = filepath.Clean(p)
		if real, evalErr := filepath.EvalSymlinks(p); evalErr == nil {
			return real
		}
		return p
	}
	gitDir := resolve(lines[0])
	commonDir := resolve(lines[1])
	if gitDir == "" || commonDir == "" || gitDir == commonDir {
		return nil
	}
	var roots []string
	for _, candidate := range []string{
		gitDir,
		filepath.Join(commonDir, "objects"),
		filepath.Join(commonDir, "refs"),
		filepath.Join(commonDir, "logs"),
	} {
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			roots = append(roots, candidate)
		}
	}
	for _, name := range []string{"packed-refs", "packed-refs.lock", "packed-refs.new"} {
		candidate := filepath.Join(commonDir, name)
		// Never turn a pre-existing symlink or directory into a file grant.
		info, err := os.Lstat(candidate)
		if os.IsNotExist(err) || (err == nil && info.Mode().IsRegular()) {
			roots = append(roots, candidate)
		}
	}
	return roots
}
