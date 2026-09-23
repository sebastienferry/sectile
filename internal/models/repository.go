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
