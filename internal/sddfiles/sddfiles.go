// Package sddfiles finds and reads a macro's specification files in a
// specifications folder.
//
// It is a leaf package on purpose: the local agent reads the files for the
// server, and neither side may import the other's filesystem helpers. The
// parsing of what is read stays with the server.
package sddfiles

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"tasks/internal/models"
)

// settingHint names where the specifications folder is set, so every refusal
// points the reader at the one place they can fix it.
const settingHint = "le « Specifications folder » se règle dans les réglages du projet de l'app desktop"

// SearchRoots returns the directories a macro's folder is looked for in,
// according to the project's SDD framework.
//
// OpenSpec's archive is left out: an archived change is finished work, and
// slicing it would reproduce work already delivered.
func SearchRoots(specFramework string) []string {
	if strings.EqualFold(strings.TrimSpace(specFramework), "openspec") {
		// Written with a forward slash on purpose: this root is also passed to
		// git as "<branch>:openspec/changes", and git never matches a path
		// with a backslash. Disk reads go through filepath.Join, which
		// normalises the separator on Windows.
		return []string{"openspec/changes"}
	}
	return []string{"specs"}
}

// FindMacroDir looks for a macro's specification folder under a
// specifications folder, by the prefix of its key.
//
// The match ignores case: an OpenSpec change folder carries the key in lower
// case where the branch carries it in upper case. The slug after the key is
// not known to Sectile, hence a prefix search rather than a built path.
//
// Two folders for one key, an abandoned one and a resumed one, are not
// impossible: the most recently modified wins.
func FindMacroDir(folder, specFramework, macroKey string) (string, error) {
	folder = strings.TrimSpace(folder)
	key := strings.ToLower(strings.TrimSpace(macroKey))
	if folder == "" {
		return "", fmt.Errorf("aucun dossier des spécifications pour ce projet (%s)", settingHint)
	}
	if key == "" {
		return "", fmt.Errorf("clé de macro manquante")
	}

	type candidate struct {
		path    string
		modTime int64
	}
	var found []candidate
	for _, root := range SearchRoots(specFramework) {
		entries, err := os.ReadDir(filepath.Join(folder, root))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := strings.ToLower(entry.Name())
			if name != key && !strings.HasPrefix(name, key+"-") {
				continue
			}
			var mod int64
			if info, statErr := entry.Info(); statErr == nil {
				mod = info.ModTime().Unix()
			}
			found = append(found, candidate{path: filepath.Join(folder, root, entry.Name()), modTime: mod})
		}
	}
	if len(found) == 0 {
		return "", notFound(folder, specFramework, macroKey, false)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].modTime > found[j].modTime })
	return found[0].path, nil
}

// notFound says that no folder carries the macro. The folder is named, not
// only the roots searched: the first cause of this refusal is looking in the
// wrong folder. The macro branch is suggested only where there can be one.
func notFound(folder, specFramework, macroKey string, git bool) error {
	cause := "ce dossier n'est pas celui qui les porte"
	if git {
		cause = "soit la spécification est sur la branche de la macro, non fusionnée, soit " + cause
	}
	return fmt.Errorf("aucun dossier de spécification pour %s dans %s (cherché sous %s) : %s (%s)",
		strings.ToUpper(strings.TrimSpace(macroKey)), folder, strings.Join(SearchRoots(specFramework), ", "), cause, settingHint)
}

// Read returns the content of a macro's specification file and where it was
// read: a path in the working tree, or "<branch>:<path>".
//
// The working tree comes first. When the macro's folder is not there and the
// specifications folder is a Git checkout, the file is read from the macro's
// branch: a macro's specification is written on its own branch, and slicing
// must work before that branch is merged. A plain folder has no branch to
// fall back on.
func Read(ctx context.Context, folder, specFramework, macroKey, fileName string) (string, string, error) {
	folder = strings.TrimSpace(folder)
	fileName = strings.TrimSpace(fileName)
	if fileName != "tasks.md" && fileName != "spec.md" {
		return "", "", fmt.Errorf("fichier de spécification inconnu : %q", fileName)
	}

	dir, dirErr := FindMacroDir(folder, specFramework, macroKey)
	if dirErr == nil {
		path := filepath.Join(dir, fileName)
		raw, readErr := os.ReadFile(path)
		if readErr == nil {
			return string(raw), path, nil
		}
		// The folder is there but not this file: the chosen source is missing,
		// not the specification, and the message must tell them apart.
		return "", "", fmt.Errorf("%s est absent de %s : cette source n'a rien à lire, essayez l'autre", fileName, dir)
	}
	if folder == "" || !isGitCheckout(ctx, folder) {
		return "", "", dirErr
	}
	if content, path, err := readFromBranch(ctx, folder, specFramework, macroKey, fileName); err == nil {
		return content, path, nil
	}
	return "", "", notFound(folder, specFramework, macroKey, true)
}

// isGitCheckout reports whether the folder is inside a Git checkout.
func isGitCheckout(ctx context.Context, folder string) bool {
	_, err := gitOutput(ctx, folder, "rev-parse", "--git-dir")
	return err == nil
}

// readFromBranch reads the file on the macro's branch, without touching the
// working tree. The branch is looked for by the prefix of the key, like the
// folder: it is the convention the specification skills follow.
func readFromBranch(ctx context.Context, repo, specFramework, macroKey, fileName string) (string, string, error) {
	branch, err := findMacroBranch(ctx, repo, macroKey)
	if err != nil {
		return "", "", err
	}

	key := strings.ToLower(strings.TrimSpace(macroKey))
	for _, root := range SearchRoots(specFramework) {
		// The folder is listed on the branch, its slug not being known.
		out, listErr := gitOutput(ctx, repo, "ls-tree", "--name-only", branch+":"+root)
		if listErr != nil {
			continue
		}
		for _, entry := range strings.Split(out, "\n") {
			entry = strings.TrimSuffix(strings.TrimSpace(entry), "/")
			name := strings.ToLower(entry)
			if name == "" || (name != key && !strings.HasPrefix(name, key+"-")) {
				continue
			}
			path := root + "/" + entry + "/" + fileName
			content, showErr := gitOutput(ctx, repo, "show", branch+":"+path)
			if showErr != nil {
				continue
			}
			return content, branch + ":" + path, nil
		}
	}
	return "", "", fmt.Errorf("rien à lire sur la branche %s", branch)
}

// findMacroBranch looks for a macro's branch by the prefix of its key.
func findMacroBranch(ctx context.Context, repo, macroKey string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(macroKey))
	if key == "" {
		return "", fmt.Errorf("clé de macro manquante")
	}
	out, err := gitOutput(ctx, repo, "for-each-ref", "--format=%(refname:short)", "refs/heads", "refs/remotes")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		ref := strings.TrimSpace(line)
		if ref == "" {
			continue
		}
		// The agent's macro worktree uses the same rule, so both name the
		// same branch.
		if models.MacroBranchMatches(ref, key) {
			return ref, nil
		}
	}
	return "", fmt.Errorf("aucune branche pour %s", strings.ToUpper(macroKey))
}

func gitOutput(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
