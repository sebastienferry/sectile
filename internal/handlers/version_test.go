package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/version"
)

// The footer of the interface shows whatever this route says, so it has to say
// the build's own version rather than anything derived from the request.
func TestVersionRouteReportsTheBuild(t *testing.T) {
	restore := version.Version
	t.Cleanup(func() { version.Version = restore })
	version.Version = "v9.9.9"

	h := &Handler{}
	w := httptest.NewRecorder()
	h.HandleVersion(w, httptest.NewRequest(http.MethodGet, VersionPath, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var got version.Info
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if got.Version != "v9.9.9" {
		t.Errorf("version = %q, want v9.9.9", got.Version)
	}
}

func TestVersionRouteRefusesAWrite(t *testing.T) {
	h := &Handler{}
	w := httptest.NewRecorder()
	h.HandleVersion(w, httptest.NewRequest(http.MethodPost, VersionPath, nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}

// The changelog is served verbatim: what the release manager proofread before
// tagging is what the reader sees.
func TestChangelogRouteServesTheEmbeddedFile(t *testing.T) {
	h := &Handler{}
	w := httptest.NewRecorder()
	h.HandleChangelog(w, httptest.NewRequest(http.MethodGet, ChangelogPath, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", w.Code, w.Body.String())
	}
	var body struct {
		Markdown string `json:"markdown"`
		Version  string `json:"version"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(body.Markdown, "# Changelog") {
		t.Errorf("changelog does not start with its own heading: %.40q", body.Markdown)
	}
	// Keep a Changelog is the format the release rule in AGENTS.md writes; a
	// changelog without a version heading is one no release ever landed in.
	if !strings.Contains(body.Markdown, "## [") {
		t.Error("changelog carries no version heading")
	}
	if body.Version == "" {
		t.Error("changelog answer carries no version")
	}
}
