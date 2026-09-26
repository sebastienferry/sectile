package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
)

// openEditorDaemon is a daemon with one running and one exited run, both in
// worktree, and a launcher that records what it was asked to open.
func openEditorDaemon(t *testing.T, editor string) (*agentDaemon, string, *[]string) {
	t.Helper()
	testhome.Temp(t)
	if editor != "" {
		if err := agentconfig.WriteSettings(agentconfig.Settings{Defaults: agentconfig.Defaults{EditorCommand: editor}}); err != nil {
			t.Fatal(err)
		}
	}
	worktree := t.TempDir()
	exited := make(chan struct{})
	close(exited)
	launched := []string{}
	d := &agentDaemon{
		repoRoot: t.TempDir(),
		loopback: loopbackServer{desktopToken: "private"},
		queue: runQueue{runs: map[string]*controlledRun{
			"running":   {desktop: desktopRun{ID: "running", Directory: worktree, Status: "running"}, exited: make(chan struct{})},
			"exited":    {desktop: desktopRun{ID: "exited", Directory: worktree, Status: "completed"}, exited: exited},
			"no-dir":    {desktop: desktopRun{ID: "no-dir", Status: "running"}, exited: make(chan struct{})},
			"removed":   {desktop: desktopRun{ID: "removed", Directory: filepath.Join(worktree, "gone"), Status: "completed"}, exited: exited},
			"not-a-dir": {desktop: desktopRun{ID: "not-a-dir", Directory: filepath.Join(worktree, "file"), Status: "running"}, exited: make(chan struct{})},
		}},
		openEditorFn: func(editor, directory string) error {
			launched = append(launched, editor+" "+directory)
			return nil
		},
	}
	if err := os.WriteFile(filepath.Join(worktree, "file"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	return d, worktree, &launched
}

func openEditorRequest(d *agentDaemon, method, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/desktop/open-editor", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer private")
	w := httptest.NewRecorder()
	d.desktopHandler(w, req)
	return w
}

func TestDesktopOpenEditorOpensTheRunDirectory(t *testing.T) {
	d, worktree, launched := openEditorDaemon(t, "cursor -n")
	for _, id := range []string{"running", "exited"} {
		w := openEditorRequest(d, http.MethodPost, `{"runId":"`+id+`"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", id, w.Code, w.Body.String())
		}
		var out map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out["editor"] != "cursor -n" || out["directory"] != worktree {
			t.Fatalf("%s: unexpected answer %v", id, out)
		}
	}
	want := []string{"cursor -n " + worktree, "cursor -n " + worktree}
	if !slices.Equal(*launched, want) {
		t.Fatalf("launched %v, want %v", *launched, want)
	}
}

func TestDesktopOpenEditorRefusals(t *testing.T) {
	d, worktree, launched := openEditorDaemon(t, "zed")
	for _, tc := range []struct {
		name, method, body string
		code               int
		message            string
	}{
		{"method", http.MethodGet, "", http.StatusMethodNotAllowed, "Method not allowed"},
		{"no run", http.MethodPost, `{}`, http.StatusBadRequest, "Run ID required"},
		{"bad body", http.MethodPost, `{`, http.StatusBadRequest, "Run ID required"},
		{"unknown run", http.MethodPost, `{"runId":"unknown"}`, http.StatusNotFound, "Run not found"},
		{"no directory", http.MethodPost, `{"runId":"no-dir"}`, http.StatusConflict, "This execution has no worktree"},
		{"removed worktree", http.MethodPost, `{"runId":"removed"}`, http.StatusGone, "The worktree no longer exists: " + filepath.Join(worktree, "gone")},
		{"not a directory", http.MethodPost, `{"runId":"not-a-dir"}`, http.StatusGone, "The worktree no longer exists: " + filepath.Join(worktree, "file")},
	} {
		w := openEditorRequest(d, tc.method, tc.body)
		if w.Code != tc.code || strings.TrimSpace(w.Body.String()) != tc.message {
			t.Errorf("%s: got %d %q, want %d %q", tc.name, w.Code, strings.TrimSpace(w.Body.String()), tc.code, tc.message)
		}
	}
	if len(*launched) != 0 {
		t.Fatalf("a refusal launched %v", *launched)
	}
}

// With no editor chosen the route refuses instead of falling back to `code`,
// as the server's open_editor operation does.
func TestDesktopOpenEditorNeedsAConfiguredEditor(t *testing.T) {
	d, _, launched := openEditorDaemon(t, "")
	w := openEditorRequest(d, http.MethodPost, `{"runId":"running"}`)
	if w.Code != http.StatusConflict || strings.TrimSpace(w.Body.String()) != "No editor is configured in Settings" {
		t.Fatalf("got %d %q", w.Code, w.Body.String())
	}
	if len(*launched) != 0 {
		t.Fatalf("launched %v without an editor", *launched)
	}
}

func TestDesktopOpenEditorReportsALaunchFailure(t *testing.T) {
	d, _, _ := openEditorDaemon(t, "missing-editor")
	d.openEditorFn = func(string, string) error { return errors.New("failed to open in 'missing-editor': not found") }
	w := openEditorRequest(d, http.MethodPost, `{"runId":"running"}`)
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "missing-editor") {
		t.Fatalf("got %d %q", w.Code, w.Body.String())
	}
}

func TestDesktopStatusAnnouncesOpenEditor(t *testing.T) {
	d, _, _ := openEditorDaemon(t, "")
	req := httptest.NewRequest(http.MethodGet, "/desktop/status", nil)
	req.Header.Set("Authorization", "Bearer private")
	w := httptest.NewRecorder()
	d.desktopHandler(w, req)
	var status struct {
		Capabilities []string `json:"capabilities"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(status.Capabilities, openEditorCapability) {
		t.Fatalf("capabilities %v lack %s", status.Capabilities, openEditorCapability)
	}
}
