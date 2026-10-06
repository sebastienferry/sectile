package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/testhome"
)

// sandboxRegressionDaemon is an agent whose settings live in a temporary home,
// with one project "p" mapped to a Git checkout.
func sandboxRegressionDaemon(t *testing.T) (*agentDaemon, string) {
	t.Helper()
	root := t.TempDir()
	testhome.Set(t, root)
	for _, args := range [][]string{{"init"}, {"remote", "add", "origin", "https://example.test/project.git"}} {
		if _, err := gitLocal(context.Background(), root, args...); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/agent/config") {
			_ = json.NewEncoder(w).Encode(agentconfig.Config{SchemaVersion: agentconfig.Version, ProjectID: "p", GitRemoteURL: "https://example.test/project.git"})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return &agentDaemon{repoRoot: root, loopback: loopbackServer{desktopToken: "private"}, link: serverLink{serverURL: srv.URL, projectID: "p"}}, root
}

func desktopCall(t *testing.T, d *agentDaemon, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r.Header.Set("Authorization", "Bearer private")
	w := httptest.NewRecorder()
	d.desktopHandler(w, r)
	return w
}

// foldedSettings writes a layout 3 file whose projects hold Sandbox rules and
// runs the start-up migration on it, as the #730 upgrade did.
func foldedSettings(t *testing.T, root string) {
	t.Helper()
	path, _ := agentconfig.SettingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	raw := `{"layout":3,"projectSettings":{` +
		`"p":{"path":` + strconv.Quote(root) + `,"claudeSandbox":{"allow":["Read","Bash(go test:*)"]}},` +
		`"q":{"path":"/q","claudeSandbox":{"allow":["Grep"],"deny":["Bash(rm:*)"]}}}}`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if migrated, report, err := agentconfig.MigrateSettingsReport(root); !migrated || !report.SandboxFolded || err != nil {
		t.Fatalf("fold: %v %+v %v", migrated, report, err)
	}
}

// The rules the #730 upgrade folded into the workstation survive every save
// Desktop makes, with the payload each dialog sends (#744).
func TestFoldedSandboxRulesSurviveEveryDesktopSave(t *testing.T) {
	d, root := sandboxRegressionDaemon(t)
	foldedSettings(t, root)
	folded := []string{"Read", "Bash(go test:*)", "Grep"}
	check := func(step string) {
		t.Helper()
		settings, err := agentconfig.ReadSettings(root)
		if err != nil {
			t.Fatalf("%s: %v", step, err)
		}
		global := settings.Defaults.ClaudeSandbox
		if global == nil {
			t.Fatalf("%s: the workstation Sandbox values are gone", step)
		}
		for _, rule := range folded {
			if !slices.Contains(global.Allow, rule) {
				t.Fatalf("%s: folded rule %q lost: %v", step, rule, global.Allow)
			}
		}
		if !slices.Contains(global.Deny, "Bash(rm:*)") {
			t.Fatalf("%s: folded deny rule lost: %v", step, global.Deny)
		}
	}
	check("fold")
	opened := func() map[string]any {
		t.Helper()
		w := desktopCall(t, d, "GET", "/desktop/workstation/sandbox", nil)
		var view struct {
			ClaudeSandbox map[string]any `json:"claudeSandbox"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &view) != nil {
			t.Fatalf("GET workstation sandbox: %d %s", w.Code, w.Body.String())
		}
		return view.ClaudeSandbox
	}
	expect := func(step string, w *httptest.ResponseRecorder, code int) {
		t.Helper()
		if w.Code != code {
			t.Fatalf("%s: %d %s", step, w.Code, w.Body.String())
		}
		check(step)
	}

	expect("workstation defaults", desktopCall(t, d, "PUT", "/desktop/workstation", map[string]any{"editorCommand": "vim"}), 204)

	read := opened()
	expect("Sandbox category, base read", desktopCall(t, d, "PUT", "/desktop/workstation/sandbox",
		map[string]any{"claudeSandbox": read, "claudeSandboxBase": read, "projects": []string{}}), 200)

	expect("Sandbox category, empty base", desktopCall(t, d, "PUT", "/desktop/workstation/sandbox",
		map[string]any{"claudeSandbox": map[string]any{"allow": []string{"Glob"}}, "claudeSandboxBase": map[string]any{}, "projects": []string{}}), 200)

	expect("Sandbox category, stale base", desktopCall(t, d, "PUT", "/desktop/workstation/sandbox",
		map[string]any{"claudeSandbox": read, "claudeSandboxBase": read, "projects": []string{}}), 200)

	expect("Sandbox category, no base", desktopCall(t, d, "PUT", "/desktop/workstation/sandbox",
		map[string]any{"claudeSandbox": opened(), "projects": []string{}}), 200)

	expect("project settings", desktopCall(t, d, "POST", "/desktop/projects", map[string]any{"projectId": "p", "path": root,
		"claudeSandbox": map[string]any{"allow": []string{"Bash(make:*)"}}, "claudeSandboxBase": map[string]any{}}), 204)

	if _, err := d.addProjectAllowRules("p", []string{"WebFetch(domain:pypi.org)"}); err != nil {
		t.Fatal(err)
	}
	check("Always allow")

	expect("Move to global", desktopCall(t, d, "POST", "/desktop/project/sandbox/promote",
		map[string]any{"projectId": "p", "rule": "WebFetch(domain:pypi.org)"}), 200)
}

// After an agent that predates #730 rewrote the file, a rule "Always allow"
// adds stays on its project: nothing folds it into the emptied workstation
// level, and the rewrite is kept as a backup (#744).
func TestAnOlderAgentRewriteIsNotFoldedAgain(t *testing.T) {
	d, root := sandboxRegressionDaemon(t)
	foldedSettings(t, root)
	path, _ := agentconfig.SettingsPath()
	raw, _ := os.ReadFile(path)
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	// What the older agent writes: its own layout, defaults without the field
	// it does not know, every key it does not own kept.
	delete(fields["defaults"].(map[string]any), "claudeSandbox")
	fields["layout"] = 3
	raw, _ = json.Marshal(fields)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := d.addProjectAllowRules("p", []string{"WebFetch(domain:pypi.org)"}); err != nil {
		t.Fatal(err)
	}
	settings, err := agentconfig.ReadSettings(root)
	if err != nil || settings.Defaults.ClaudeSandbox != nil {
		t.Fatalf("the project rule was folded: %+v %v", settings.Defaults.ClaudeSandbox, err)
	}
	if p := settings.Project("p").ClaudeSandbox; p == nil || !slices.Contains(p.Allow, "WebFetch(domain:pypi.org)") {
		t.Fatalf("the rule left its project: %+v", p)
	}
	if backups, _ := filepath.Glob(path + ".bak-layout3-*"); len(backups) != 1 {
		t.Fatalf("the older agent's file must be kept once: %v", backups)
	}
}
