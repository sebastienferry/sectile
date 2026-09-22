package handlers

import (
	"net/http"
	"strings"

	sectile "tasks"
	"tasks/internal/version"
)

// VersionPath and ChangelogPath are the two routes that answer "what am I
// running, and what changed". They are named here rather than spelled out at
// the registration site so the route, the guard and the tests cannot drift.
const (
	VersionPath   = "/api/version"
	ChangelogPath = "/api/changelog"
)

// HandleVersion reports the release this server was built from.
//
// It is behind the session guard on purpose. A version number is a small thing
// to leak, but it is exactly the small thing an unauthenticated scan is looking
// for, and nothing needs it before sign-in: /api/health already answers the
// load balancer, and the interface only asks for a version once somebody is
// looking at the footer.
func (h *Handler) HandleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, version.Current())
}

// HandleChangelog serves CHANGELOG.md as it is written.
//
// The file travels as Markdown rather than as a parsed structure: both
// interfaces already render Markdown, and a release note that has been through
// a parser is a release note whose author cannot tell what readers will see.
func (h *Handler) HandleChangelog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	markdown := sectile.ChangelogMarkdown()
	// An empty changelog means the build lost its embedded file, which is a
	// server fault and not an empty release history. Saying so beats serving a
	// blank panel the reader would read as "nothing ever changed".
	if strings.TrimSpace(markdown) == "" {
		writeError(w, http.StatusInternalServerError, "This build embeds no changelog")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"markdown": markdown, "version": version.Current().Version})
}
