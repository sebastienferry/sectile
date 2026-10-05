package agent

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"tasks/internal/agentconfig"
	"tasks/internal/agentexec"
	"tasks/internal/models"
)

// repositoryRequest is what a repository_worktree operation asks for: the
// repository as the caller named it, and, with the project's Any repository
// option (#737), the URL to clone it from and a checkout the caller found.
type repositoryRequest struct {
	// Repository is a remote URL, a host/path identity, or a folder.
	Repository string
	// URL is the remote as the caller typed it, "" from an older server.
	URL string
	// Path is a local checkout of the repository the caller found.
	Path string
	// Device names this workstation in a refusal.
	Device string
	// SettingsRoot is where the workstation settings are read and written,
	// to remember a checkout found or cloned.
	SettingsRoot string
}

// locateRepository is repositoryFolder with where the folder came from: its
// mapping, the project checkout, or an attached folder.
func locateRepository(ctx context.Context, overrides agentconfig.Settings, projectID, projectRoot, code, identity string) (folder, source string, ok bool) {
	if root, ok := repositoryRoot(overrides, projectRoot, code, identity); ok {
		if strings.TrimSpace(overrides.Repositories[identity]) != "" {
			return root, models.RepositorySourceMapping, true
		}
		return root, models.RepositorySourceProject, true
	}
	if identity == "" {
		return "", "", false
	}
	for _, folder := range attachedFolders(ctx, overrides, projectID) {
		if folder.Kind == folderKindGit && folder.Identity == identity {
			return folder.Path, models.RepositorySourceAttached, true
		}
	}
	return "", "", false
}

// unknownRepositoryFolder finds a folder for a repository the workstation
// does not know yet, with the Any repository option on: the checkout the
// caller gave, else a clone in the project's clones folder.
func unknownRepositoryFolder(ctx context.Context, config agentconfig.Config, overrides agentconfig.Settings, projectRoot, identity string, request repositoryRequest) (folder, source string, err error) {
	if strings.TrimSpace(request.Path) != "" {
		folder, err := checkRepositoryPath(ctx, request.Path, identity)
		return folder, models.RepositorySourcePath, err
	}
	clones := overrides.ClonesPath(config.ProjectID, projectRoot)
	if clones == "" {
		return "", "", fmt.Errorf("no clones folder for %s: set the project's local repository or its clones folder in the desktop project settings", identity)
	}
	folder, err = cloneRepository(ctx, clones, cloneURL(request.URL, identity, config.GitRemoteURL), identity)
	return folder, models.RepositorySourceClone, err
}

// checkRepositoryPath accepts path as the checkout of identity: an absolute
// folder, the top level of a Git checkout, whose origin is identity.
func checkRepositoryPath(ctx context.Context, path, identity string) (string, error) {
	path = strings.TrimSpace(path)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s is not an absolute path", path)
	}
	folder := describeFolder(ctx, filepath.Clean(path))
	switch {
	case folder.Kind == folderKindMissing:
		return "", fmt.Errorf("%s does not exist or is not a folder", path)
	case folder.Err != "":
		return "", fmt.Errorf("%s could not be read: %s", path, folder.Err)
	case folder.Kind != folderKindGit:
		return "", fmt.Errorf("%s is not a Git checkout", path)
	case !sameDirectory(folder.Path, path):
		return "", fmt.Errorf("%s is inside the checkout %s: give its top level", path, folder.Path)
	case folder.Identity == "":
		return "", fmt.Errorf("%s has no origin remote", path)
	case folder.Identity != identity:
		return "", fmt.Errorf("the origin of %s is %s, not %s", path, folder.Remote, identity)
	}
	return folder.Path, nil
}

// cloneURL is the URL a repository is cloned from: the one the caller typed
// when it is a URL, else one built from the identity in the scheme of the
// project's code remote, SSH when that remote is SSH, HTTPS otherwise.
func cloneURL(requested, identity, codeRemote string) string {
	requested = strings.TrimSpace(requested)
	if parsed, err := url.Parse(requested); err == nil && parsed.Scheme != "" && (parsed.Host != "" || parsed.Scheme == "file") {
		return requested
	}
	if scpLike(requested) {
		return requested
	}
	host, repository, _ := strings.Cut(identity, "/")
	codeRemote = strings.TrimSpace(codeRemote)
	if scpLike(codeRemote) || strings.HasPrefix(codeRemote, "ssh://") {
		return "git@" + host + ":" + repository + ".git"
	}
	return "https://" + host + "/" + repository + ".git"
}

// scpLike reports a remote written git@host:path.
func scpLike(remote string) bool {
	at, colon := strings.Index(remote, "@"), strings.Index(remote, ":")
	return at > 0 && colon > at && !strings.Contains(remote[:colon], "/")
}

// cloneRepository clones remote into clones/<repository name> and answers
// that folder. A folder already there is used when it is a checkout of the
// same repository and refused otherwise: nothing is ever overwritten. The
// clone is made in a hidden sibling renamed once complete, so a failed clone
// leaves nothing under the repository's name.
func cloneRepository(ctx context.Context, clones, remote, identity string) (string, error) {
	name := path.Base(identity)
	if name == "" || name == "." || name == "/" {
		return "", fmt.Errorf("%s names no repository to clone", identity)
	}
	target := filepath.Join(clones, name)
	if _, err := os.Lstat(target); err == nil {
		folder := describeFolder(ctx, target)
		if folder.Kind == folderKindGit && folder.Identity == identity && sameDirectory(folder.Path, target) {
			return folder.Path, nil
		}
		return "", fmt.Errorf("%s already exists and is not a checkout of %s: move it, or set another clones folder", target, identity)
	}
	if err := os.MkdirAll(clones, 0o755); err != nil {
		return "", err
	}
	partial := filepath.Join(clones, "."+name+".cloning")
	if err := os.RemoveAll(partial); err != nil {
		return "", err
	}
	if raw, err := cloneCommand(ctx, remote, partial).CombinedOutput(); err != nil {
		_ = os.RemoveAll(partial)
		return "", fmt.Errorf("git clone %s: %w: %s", remote, err, strings.TrimSpace(string(raw)))
	}
	if err := os.Rename(partial, target); err != nil {
		_ = os.RemoveAll(partial)
		return "", err
	}
	return target, nil
}

// cloneCommand is the git clone of remote into dest. It never prompts: a
// credential the workstation's git cannot supply fails the clone instead of
// waiting on a terminal nobody watches.
func cloneCommand(ctx context.Context, remote, dest string) *exec.Cmd {
	cmd := agentexec.Hidden(exec.CommandContext(ctx, "git", "clone", "--quiet", remote, dest))
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}

// rememberRepositoryFolder maps identity to folder in the workstation
// settings, unless it is mapped already, so the next ticket of any project
// finds it without a path and handoff can remove its worktree. It answers
// whether it wrote the mapping.
func rememberRepositoryFolder(settingsRoot, identity, folder string) (bool, error) {
	unlock := agentconfig.LockSettings()
	defer unlock()
	settings, err := agentconfig.ReadSettings(settingsRoot)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(settings.Repositories[identity]) != "" {
		return false, nil
	}
	if settings.Repositories == nil {
		settings.Repositories = map[string]string{}
	}
	settings.Repositories[identity] = filepath.Clean(folder)
	if err := agentconfig.WriteSettings(settings); err != nil {
		return false, err
	}
	return true, nil
}

// errPathWithoutOption refuses a checkout given while the option is off.
var errPathWithoutOption = errors.New("the project's Any repository option is off on this workstation: a path is only accepted when it is on")
