package models

import (
	"strings"
	"time"
)

// Tracker is one server-side source of tickets: one Jira space, one GitHub
// repository, one GitLab project, or the local board of one project (#741). It
// is synchronised in full whatever projects exist, and a project selects which
// of its tickets it shows.
//
// It holds the board mirror a project held before: the board, its columns,
// how those columns map onto the workflow stages, its sprints and the issue
// types imported, plus the background synchronisation settings. The
// status→stage mapping is therefore one per tracker.
type Tracker struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Provider is "jira", "github", "gitlab" or "local".
	Provider string `json:"provider"`
	// Site is the tracker's own address when it overrides the deployment's
	// one (a Jira site, a GitHub or GitLab API URL), empty when it uses the
	// deployment's. A local tracker has none.
	Site string `json:"site"`
	// Scope names the source on its site: a Jira project key, a GitHub
	// owner/repo, a GitLab project path, or the project id of a local board.
	Scope string `json:"scope"`
	// Identity is derived from the provider, the resolved site and the scope
	// (TrackerIdentity). It is stored so that two trackers naming the same
	// source cannot exist.
	Identity            string              `json:"identity"`
	BoardID             string              `json:"boardId,omitempty"`
	TrackerColumns      []TrackerColumn     `json:"trackerColumns,omitempty"`
	StageColumns        map[string][]string `json:"stageColumns,omitempty"`
	Sprints             []TrackerSprint     `json:"sprints,omitempty"`
	IssueTypes          []string            `json:"issueTypes,omitempty"`
	AutoSyncEnabled     bool                `json:"autoSyncEnabled"`
	AutoSyncIntervalMin int                 `json:"autoSyncIntervalMin"`
	CreatedAt           time.Time           `json:"createdAt"`
	UpdatedAt           time.Time           `json:"updatedAt"`
}

// ProjectTracker is one tracker a project selects its tickets from, by id and
// by identity, on the pattern of ProjectRepository.
type ProjectTracker struct {
	TrackerID string `json:"trackerId"`
	Identity  string `json:"identity"`
}

// TrackerAddress reduces a tracker URL to what tells two sites apart: no
// scheme, no trailing slash, the host lower-cased.
func TrackerAddress(raw string) string {
	address := strings.TrimSpace(raw)
	if i := strings.Index(address, "://"); i >= 0 {
		address = address[i+3:]
	}
	address = strings.TrimRight(address, "/")
	if host, path, found := strings.Cut(address, "/"); found {
		return strings.ToLower(host) + "/" + path
	}
	return strings.ToLower(address)
}

// TrackerScope normalises a scope the way its provider compares it: a Jira key
// upper-cased, a GitHub owner/repo or a GitLab path lower-cased without
// surrounding slashes, a local project id as it is.
func TrackerScope(provider, scope string) string {
	scope = strings.TrimSpace(scope)
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "jira":
		return strings.ToUpper(scope)
	case "github", "gitlab":
		return strings.ToLower(strings.TrimSuffix(strings.Trim(scope, "/"), ".git"))
	}
	return scope
}

// TrackerIdentity reduces a tracker to what tells two of them apart, as
// RepositoryIdentity does for a remote: the provider lower-cased, the site as
// TrackerAddress reduces it, and the scope as TrackerScope normalises it. Two
// Jira spaces of one site are two identities; one Jira space spelled with two
// cases of its URL is one.
func TrackerIdentity(provider, site, scope string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	return provider + "|" + TrackerAddress(site) + "|" + TrackerScope(provider, scope)
}

// NormalizeProjectTrackers drops the blank entries and the second occurrence of
// an identity already listed, keeping the order, which is the trackers'
// position on the project.
func NormalizeProjectTrackers(list []ProjectTracker) []ProjectTracker {
	result := []ProjectTracker{}
	seen := map[string]bool{}
	for _, entry := range list {
		entry.TrackerID, entry.Identity = strings.TrimSpace(entry.TrackerID), strings.TrimSpace(entry.Identity)
		key := entry.Identity
		if key == "" {
			key = "id:" + entry.TrackerID
		}
		if (entry.TrackerID == "" && entry.Identity == "") || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, entry)
	}
	return result
}
