package models

import (
	"net/url"
	"strconv"
	"strings"
)

// RepositoryIdentity reduces a Git remote, in URL or scp-like form, to
// host/path: lowercased, without scheme, user, port, ".git" suffix or trailing
// slash. Two remotes naming the same repository over SSH and HTTPS compare
// equal, which is what the server and the agent both need when they decide
// whether a checkout or a pull request belongs to a given repository.
func RepositoryIdentity(remote string) string {
	remote = strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(remote), "/"), ".git")
	if parsed, err := url.Parse(remote); err == nil && parsed.Host != "" {
		return strings.ToLower(parsed.Hostname() + strings.TrimRight(parsed.Path, "/"))
	}
	if i := strings.Index(remote, "@"); i >= 0 && i < strings.Index(remote, ":") {
		remote = remote[i+1:]
	}
	return strings.ToLower(strings.Replace(remote, ":", "/", 1))
}

// PullRequestLink is what a pull request URL says about where it lives. It is
// an identifier only: nothing here is a credential destination or an API
// endpoint.
type PullRequestLink struct {
	// Forge is "github" or "gitlab".
	Forge string
	// Host is the lowercased web host, such as github.com or gitlab.example.org.
	Host string
	// Repository is owner/repo on GitHub, group/.../project on GitLab.
	Repository string
	Number     int
}

// Identity is the link's repository in RepositoryIdentity form.
func (l PullRequestLink) Identity() string {
	return strings.ToLower(l.Host + "/" + l.Repository)
}

// ParsePullRequestLink recognizes a GitHub pull request or a GitLab merge
// request URL. GitHub is recognized by a host naming it, or by githubHost when
// an enterprise installation uses another name; GitLab by its
// /-/merge_requests/<n> path, whatever the host, so a self-hosted instance is
// recognized too. Anything else is not a link.
func ParsePullRequestLink(raw, githubHost string) (PullRequestLink, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" {
		return PullRequestLink{}, false
	}
	host := strings.ToLower(u.Hostname())
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return PullRequestLink{}, false
		}
	}
	link := PullRequestLink{Host: host}
	var number string
	n := len(parts)
	switch {
	case n >= 5 && parts[n-3] == "-" && parts[n-2] == "merge_requests":
		link.Forge, link.Repository, number = "gitlab", strings.Join(parts[:n-3], "/"), parts[n-1]
	case n == 4 && parts[2] == "pull" && (strings.Contains(host, "github") || (githubHost != "" && strings.EqualFold(host, githubHost))):
		link.Forge, link.Repository, number = "github", strings.Join(parts[:2], "/"), parts[3]
	default:
		return PullRequestLink{}, false
	}
	if link.Number, err = strconv.Atoi(number); err != nil || link.Number <= 0 {
		return PullRequestLink{}, false
	}
	return link, true
}

// ProjectRepository is one repository a project declares: the remote as typed,
// and its identity, which is what every comparison uses.
type ProjectRepository struct {
	URL      string `json:"url"`
	Identity string `json:"identity"`
}

// NormalizeProjectRepositories orders a project's repositories: the code
// remote first when there is one, then the declared remotes in their order,
// without blanks and without a second spelling of a repository already listed.
func NormalizeProjectRepositories(codeRemote string, urls []string) []ProjectRepository {
	result := []ProjectRepository{}
	seen := map[string]bool{}
	for _, raw := range append([]string{codeRemote}, urls...) {
		raw = strings.TrimSpace(raw)
		identity := RepositoryIdentity(raw)
		if raw == "" || identity == "" || seen[identity] {
			continue
		}
		seen[identity] = true
		result = append(result, ProjectRepository{URL: raw, Identity: identity})
	}
	return result
}

// DuplicateRepository returns the first remote of urls that names a repository
// already listed before it, code remote included, or "" when there is none.
// It is how a second spelling of one remote is refused rather than dropped.
func DuplicateRepository(codeRemote string, urls []string) string {
	seen := map[string]bool{}
	if identity := RepositoryIdentity(codeRemote); strings.TrimSpace(codeRemote) != "" {
		seen[identity] = true
	}
	for _, raw := range urls {
		identity := RepositoryIdentity(raw)
		if strings.TrimSpace(raw) == "" {
			continue
		}
		if seen[identity] {
			return raw
		}
		seen[identity] = true
	}
	return ""
}

// FindProjectRepository returns the declared repository whose identity is
// that of remote, which may be given as a URL or as an identity.
func FindProjectRepository(repositories []ProjectRepository, remote string) (ProjectRepository, bool) {
	identity := RepositoryIdentity(remote)
	for _, repository := range repositories {
		if identity != "" && repository.Identity == identity {
			return repository, true
		}
	}
	return ProjectRepository{}, false
}

// Folder map roles, as the agent receives them at launch. A local folder has
// no remote: it is changed in place, with no worktree and no pull request
// (#484).
const (
	FolderRolePrimary = "primary"
	FolderRoleChanged = "changed"
	FolderRoleContext = "context"
	FolderRoleSpec    = "spec"
	FolderRoleLocal   = "local"
)

// FolderMapEntry describes one folder of a task to the agent. Path is empty
// when the repository is not mapped on the workstation; Worktree is set when
// the task has a worktree in it. Attached marks a folder attached to the
// project on the workstation (#484), whose Kind says "git", "folder", or
// "missing" when it is no longer there.
type FolderMapEntry struct {
	Remote   string `json:"remote"`
	Identity string `json:"identity"`
	Role     string `json:"role"`
	Path     string `json:"path"`
	Worktree string `json:"worktree,omitempty"`
	Kind     string `json:"kind,omitempty"`
	Attached bool   `json:"attached,omitempty"`
}

// PrimaryResolution is the outcome of ResolvePrimaryRepository.
type PrimaryResolution int

const (
	// PrimaryResolved names the repository the task runs in.
	PrimaryResolved PrimaryResolution = iota
	// PrimaryDefault is the project's own root: the code repository.
	PrimaryDefault
	// PrimaryUnmapped means the task's repository is known but has no folder
	// on this workstation.
	PrimaryUnmapped
	// PrimaryUndeclared means the task is pinned to a repository its project
	// does not declare (#737), which only the Any repository option resolves.
	PrimaryUndeclared
)

// ResolvePrimaryRepository decides which repository a task runs in, before
// anything is launched. pinned is the task's repository identity, "" when not
// pinned; mapped tells whether a repository has a folder on this workstation.
// A pin to a listed repository names it, resolved when it has a folder here
// and unmapped otherwise; a pin to another repository is undeclared (#737);
// anything else is the code repository (#484). The returned repository is set
// for every outcome but PrimaryDefault.
func ResolvePrimaryRepository(pinned string, repositories []ProjectRepository, mapped func(identity string) bool) (ProjectRepository, PrimaryResolution) {
	if pinned = strings.TrimSpace(pinned); pinned != "" {
		if found, ok := FindProjectRepository(repositories, pinned); ok {
			if mapped(found.Identity) {
				return found, PrimaryResolved
			}
			return found, PrimaryUnmapped
		}
		// A pin outside the list names a repository the project does not
		// declare (#737); what names no repository is treated as absent.
		if identity := RepositoryIdentity(pinned); IsRemoteIdentity(identity) {
			return ProjectRepository{URL: pinned, Identity: identity}, PrimaryUndeclared
		}
	}
	return ProjectRepository{}, PrimaryDefault
}

// IsRemoteIdentity reports an identity that names a repository on a host,
// host/path, as opposed to a folder, a bare name or a home path.
func IsRemoteIdentity(identity string) bool {
	host, path, ok := strings.Cut(identity, "/")
	return ok && host != "" && path != "" && !strings.ContainsAny(identity, `\~`) && !strings.HasPrefix(host, ".")
}

// LegacyRepoPath is one working directory typed before repositories existed:
// the project's repoPath or repoPaths, or a ticket's repoPath, with the
// tickets that pinned it.
type LegacyRepoPath struct {
	Path    string   `json:"path"`
	TaskIDs []string `json:"taskIds,omitempty"`
}

// LegacyRepoPaths is what a local agent converts, once per project.
type LegacyRepoPaths struct {
	Migrated bool             `json:"migrated"`
	Paths    []LegacyRepoPath `json:"paths"`
}

// ConvertedRepoPath is a legacy path whose checkout names a repository.
type ConvertedRepoPath struct {
	Path    string   `json:"path"`
	URL     string   `json:"url"`
	TaskIDs []string `json:"taskIds,omitempty"`
}

// DroppedRepoPath is a legacy path the converting workstation could not
// resolve: "not found", "not a git checkout" or "no origin".
type DroppedRepoPath struct {
	Path    string   `json:"path"`
	Reason  string   `json:"reason"`
	TaskIDs []string `json:"taskIds,omitempty"`
}

// RepositoryConversion is the report of a conversion, as the agent posts it
// and as the project keeps it once applied.
type RepositoryConversion struct {
	Converted   []ConvertedRepoPath `json:"converted"`
	Dropped     []DroppedRepoPath   `json:"dropped"`
	ConvertedAt string              `json:"convertedAt,omitempty"`
	UserID      string              `json:"userId,omitempty"`
}

// RepositoryWorktree answers a repository_worktree operation: the task's
// worktree in a secondary repository.
type RepositoryWorktree struct {
	Repository string `json:"repository"`
	Path       string `json:"path"`
	Branch     string `json:"branch"`
	// Source says where the repository's checkout was found (#737): its
	// mapping, the project checkout, an attached folder, the path the caller
	// gave, or a clone the agent made.
	Source string `json:"source,omitempty"`
	// PathChecked echoes a path the caller gave, so the server can tell an
	// agent that checked it from one that ignored it.
	PathChecked bool `json:"pathChecked,omitempty"`
	// Remembered says the checkout was written to the workstation mapping.
	Remembered bool `json:"remembered,omitempty"`
	// AddedToSession says the worktree was added to the ticket's running
	// Claude Code sessions.
	AddedToSession bool `json:"addedToSession,omitempty"`
	// Warning is what the caller should know, a failed fetch for instance.
	Warning string `json:"warning,omitempty"`
}

// Where a repository_worktree found the repository's checkout (#737).
const (
	RepositorySourceMapping  = "mapping"
	RepositorySourceProject  = "project"
	RepositorySourceAttached = "attached"
	RepositorySourcePath     = "path"
	RepositorySourceClone    = "clone"
)

// WorktreeRemoval answers a remove_workspace operation over several
// repositories.
type WorktreeRemoval struct {
	Removed []string                `json:"removed"`
	Failed  []WorktreeRemovalFailed `json:"failed,omitempty"`
}

// WorktreeRemovalFailed names a repository whose worktree could not be removed.
type WorktreeRemovalFailed struct {
	Repository string `json:"repository"`
	Error      string `json:"error"`
}

// Outcomes of one worktree in an archive_workspace answer (#755).
const (
	ArchiveRemoved  = "removed"
	ArchiveAbsent   = "absent"
	ArchiveShared   = "shared"
	ArchiveDisabled = "disabled"
	ArchiveFailed   = "failed"
)

// Outcomes of the task's local branch in an archive_workspace answer.
const (
	ArchiveBranchDeleted = "deleted"
	ArchiveBranchKept    = "kept"
)

// WorkspaceArchive answers an archive_workspace operation: what became of
// each worktree of the task, and of its local branch there.
type WorkspaceArchive struct {
	Repositories []WorkspaceArchiveEntry `json:"repositories"`
}

// WorkspaceArchiveEntry is one worktree of the task. Repository is empty for
// the checkout of a project that names no repository; Role is "code" or
// "specifications". A kept branch carries the reason it was kept and never
// fails the entry.
type WorkspaceArchiveEntry struct {
	Repository    string `json:"repository"`
	Role          string `json:"role"`
	Path          string `json:"path,omitempty"`
	Outcome       string `json:"outcome"`
	Error         string `json:"error,omitempty"`
	Branch        string `json:"branch,omitempty"`
	BranchOutcome string `json:"branchOutcome,omitempty"`
	BranchReason  string `json:"branchReason,omitempty"`
}

// Archivable says the task may be archived: no worktree of it was left
// behind. An answer with no entry comes from an agent that did not run the
// operation, and is not archivable.
func (a WorkspaceArchive) Archivable() bool {
	if len(a.Repositories) == 0 {
		return false
	}
	for _, entry := range a.Repositories {
		if entry.Outcome == ArchiveFailed {
			return false
		}
	}
	return true
}
