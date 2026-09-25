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

// Folder map roles, as the agent receives them at launch.
const (
	FolderRolePrimary = "primary"
	FolderRoleChanged = "changed"
	FolderRoleContext = "context"
	FolderRoleSpec    = "spec"
)

// FolderMapEntry describes one folder of a task to the agent. Path is empty
// when the repository is not mapped on the workstation; Worktree is set when
// the task has a worktree in it.
type FolderMapEntry struct {
	Remote   string `json:"remote"`
	Identity string `json:"identity"`
	Role     string `json:"role"`
	Path     string `json:"path"`
	Worktree string `json:"worktree,omitempty"`
}

// PrimaryResolution is the outcome of ResolvePrimaryRepository.
type PrimaryResolution int

const (
	// PrimaryResolved names the repository the task runs in.
	PrimaryResolved PrimaryResolution = iota
	// PrimaryDefault keeps today's behaviour: the project's own root.
	PrimaryDefault
	// PrimaryUnmapped means the task's repository is known but has no folder
	// on this workstation.
	PrimaryUnmapped
	// PrimaryAmbiguous means several mapped repositories could be the task's.
	PrimaryAmbiguous
)

// ResolvePrimaryRepository decides which repository a task runs in, before
// anything is launched. pinned is the task's repository identity, "" when not
// pinned; mapped tells whether a repository has a folder on this workstation.
// The order is: the pin, the only repository, the code remote of a mono-repo
// project, the only mapped repository (which the caller should pin), else
// unmapped when nothing is mapped and ambiguous otherwise. The returned
// repository is set for PrimaryResolved and PrimaryUnmapped; pin is true when
// the choice came from the workstation and should be recorded on the task.
func ResolvePrimaryRepository(pinned string, repositories []ProjectRepository, monoRepo bool, mapped func(identity string) bool) (repository ProjectRepository, outcome PrimaryResolution, pin bool) {
	if pinned = strings.TrimSpace(pinned); pinned != "" {
		if found, ok := FindProjectRepository(repositories, pinned); ok {
			if mapped(found.Identity) {
				return found, PrimaryResolved, false
			}
			return found, PrimaryUnmapped, false
		}
		// A pin outside the list cannot be stored (the server refuses it); an
		// old one is treated as absent rather than trusted.
	}
	if len(repositories) <= 1 || monoRepo {
		return ProjectRepository{}, PrimaryDefault, false
	}
	var candidates []ProjectRepository
	for _, repository := range repositories {
		if mapped(repository.Identity) {
			candidates = append(candidates, repository)
		}
	}
	switch len(candidates) {
	case 0:
		return ProjectRepository{}, PrimaryUnmapped, false
	case 1:
		return candidates[0], PrimaryResolved, true
	}
	return ProjectRepository{}, PrimaryAmbiguous, false
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
}

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
