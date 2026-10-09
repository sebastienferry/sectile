package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/skills"
	"tasks/internal/testhome"
)

// directProjectsServer serves several projects, each with its own skills, for
// the direct setup to compose from. A failing project answers HTTP 500.
type directProjectsServer struct {
	mu      sync.Mutex
	order   []string
	skills  map[string][]agentconfig.Skill
	failing map[string]bool
	fetched map[string]int
}

func newDirectProjectsServer(t *testing.T, ids ...string) (*directProjectsServer, *httptest.Server) {
	t.Helper()
	p := &directProjectsServer{order: ids, skills: map[string][]agentconfig.Skill{}, failing: map[string]bool{}, fetched: map[string]int{}}
	for _, id := range ids {
		p.skills[id] = builtinServerSkills()
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/agent/projects":
			listed := agentconfig.Projects{SchemaVersion: agentconfig.Version}
			for _, id := range p.order {
				listed.Projects = append(listed.Projects, agentconfig.Project{ID: id, Name: id})
			}
			_ = json.NewEncoder(w).Encode(listed)
		case "/api/v1/agent/config":
			id := r.URL.Query().Get("projectId")
			p.fetched[id]++
			list, ok := p.skills[id]
			if !ok || p.failing[id] {
				http.Error(w, `{"error":"unavailable"}`, http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(agentconfig.Config{SchemaVersion: agentconfig.Version, ProjectID: id, ProjectName: id, AIProvider: "codex", Skills: list})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return p, srv
}

// override gives a project a work-only override of one skill, as the server
// sends it.
func (p *directProjectsServer) override(project, id, work string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	list := append([]agentconfig.Skill{}, p.skills[project]...)
	for i := range list {
		if list[i].ID == id {
			list[i].Custom, list[i].OverrideKind, list[i].WorkContent = true, models.SkillOverrideWork, work
		}
	}
	p.skills[project] = list
}

func (p *directProjectsServer) reset(project string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.skills[project] = builtinServerSkills()
}

func (p *directProjectsServer) config(project string) agentconfig.Config {
	p.mu.Lock()
	defer p.mu.Unlock()
	return agentconfig.Config{SchemaVersion: agentconfig.Version, ProjectID: project, ProjectName: project, AIProvider: "codex", Skills: append([]agentconfig.Skill{}, p.skills[project]...)}
}

// builtinServerSkills are the catalogue skills as a server without overrides
// sends them.
func builtinServerSkills() []agentconfig.Skill {
	var out []agentconfig.Skill
	for _, id := range []string{"clarify", "specify", "pickup"} {
		stage, _ := skills.StageSkillByID(id)
		out = append(out, agentconfig.Skill{
			ID: stage.ID, Directory: stage.DirName, Command: stage.Command,
			Content: skills.RenderSkillContent(stage, ""), CommandContent: skills.RenderSkillCommand(stage, ""),
			DirectContent: skills.RenderDirectSkillContent(stage), DirectCommandContent: skills.RenderDirectSkillCommand(stage),
		})
	}
	return out
}

func skillByID(t *testing.T, config agentconfig.Config, id string) agentconfig.Skill {
	t.Helper()
	for _, skill := range config.Skills {
		if skill.ID == id {
			return skill
		}
	}
	t.Fatalf("skill %s missing from %+v", id, config.Skills)
	return agentconfig.Skill{}
}

// The direct copy is shared by every project of the workstation (#732): it
// carries one variant per project that overrides a section, the workstation's
// own work override as the fallback, and nothing from a disconnected project.
// A skill nobody overrides keeps the server's built-in.
func TestDirectSetupConfigCarriesEveryProjectsVariant(t *testing.T) {
	testhome.Temp(t)
	projects, srv := newDirectProjectsServer(t, "alpha", "beta", "gamma")
	projects.override("alpha", "clarify", "## Steps\nAlpha steps.")
	projects.override("beta", "clarify", "## Steps\nBeta steps.")
	projects.override("gamma", "clarify", "## Steps\nGamma steps.")
	projects.failing["gamma"] = true
	d := &agentDaemon{repoRoot: t.TempDir(), link: serverLink{serverURL: srv.URL, token: "token", projectID: "alpha"}}
	if _, err := agentconfig.UpdateSettings(d.localSettingsRoot(), func(s *agentconfig.Settings) error {
		s.Skills = map[string]agentconfig.SkillOverride{"clarify": {Kind: models.SkillOverrideWork, Content: "## Goal\nWorkstation goal."}}
		s.DisconnectedProjects = map[string]bool{"gamma": true}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	config, warnings, err := d.directSetupConfig(context.Background(), projects.config("alpha"))
	if err != nil || len(warnings) != 0 {
		t.Fatal(err, warnings)
	}
	clarify := skillByID(t, config, "clarify")
	for _, want := range []string{`projectId "alpha"`, "Alpha steps.", `projectId "beta"`, "Beta steps.", "Otherwise", "Workstation goal."} {
		if !strings.Contains(clarify.DirectContent, want) || !strings.Contains(clarify.DirectCommandContent, want) {
			t.Fatalf("the direct clarify lacks %q:\n%s", want, clarify.DirectContent)
		}
	}
	if strings.Contains(clarify.DirectContent, "Gamma") || projects.fetched["gamma"] != 0 {
		t.Fatalf("a disconnected project was read (%d fetches):\n%s", projects.fetched["gamma"], clarify.DirectContent)
	}
	if pickup := skillByID(t, config, "pickup"); !strings.Contains(pickup.DirectContent, `projectId "beta"`) || !strings.Contains(pickup.DirectContent, "Beta steps.") {
		t.Fatalf("pickup does not inline the overridden stage:\n%s", pickup.DirectContent)
	}
	stage, _ := skills.StageSkillByID("specify")
	if specify := skillByID(t, config, "specify"); specify.DirectContent != skills.RenderDirectSkillContent(stage) {
		t.Fatalf("a skill nobody overrides changed:\n%s", specify.DirectContent)
	}
}

// A project whose configuration cannot be read is left out of the copies,
// with a warning naming it: the caller decides whether to write them.
func TestDirectSetupConfigWarnsOnAFailedFetch(t *testing.T) {
	testhome.Temp(t)
	projects, srv := newDirectProjectsServer(t, "alpha", "beta")
	projects.override("alpha", "clarify", "## Steps\nAlpha steps.")
	projects.override("beta", "clarify", "## Steps\nBeta steps.")
	projects.failing["beta"] = true
	d := &agentDaemon{repoRoot: t.TempDir(), link: serverLink{serverURL: srv.URL, token: "token", projectID: "alpha"}}
	config, warnings, err := d.directSetupConfig(context.Background(), projects.config("alpha"))
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], `project "beta" configuration not read`) {
		t.Fatalf("warnings = %q, want one naming beta", warnings)
	}
	clarify := skillByID(t, config, "clarify")
	if !strings.Contains(clarify.DirectContent, `projectId "alpha"`) || !strings.Contains(clarify.DirectContent, "Alpha steps.") {
		t.Fatalf("the direct clarify lacks alpha's variant:\n%s", clarify.DirectContent)
	}
	if strings.Contains(clarify.DirectContent, `projectId "beta"`) || strings.Contains(clarify.DirectContent, "Beta steps.") {
		t.Fatalf("the direct clarify carries the unreadable beta:\n%s", clarify.DirectContent)
	}
}

// init writes the direct copies without an unreachable project's variant, as
// it did before work overrides existed, and its message names the project.
func TestInitWarnsAboutAnUnreachableProject(t *testing.T) {
	home := testhome.Temp(t)
	projects, srv := newDirectProjectsServer(t, "alpha", "beta")
	projects.override("alpha", "clarify", "## Steps\nAlpha steps.")
	projects.failing["beta"] = true
	out, err := InitContext(context.Background(), []string{"--provider", "codex", "--url", srv.URL, "--token", "token", "--project", "alpha", "--repo", t.TempDir()})
	if err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "\nWarning: project \"beta\" configuration not read") {
		t.Fatalf("the message does not name beta:\n%s", out)
	}
	if content := codexClarify(t, home); !strings.Contains(content, "Alpha steps.") {
		t.Fatalf("the copy lacks alpha's variant:\n%s", content)
	}
}

// refreshFixture is a workstation mapped to project alpha, its checkout a Git
// repository, with codex set up by hand when setUp is true.
func refreshFixture(t *testing.T, setUp bool) (home, root string, projects *directProjectsServer, d *agentDaemon) {
	t.Helper()
	home = testhome.Temp(t)
	root = t.TempDir()
	if _, err := gitLocal(context.Background(), root, "init"); err != nil {
		t.Fatal(err)
	}
	projects, srv := newDirectProjectsServer(t, "alpha", "beta")
	d = &agentDaemon{repoRoot: root, link: serverLink{serverURL: srv.URL, token: "token", projectID: "alpha"}}
	if setUp {
		if _, err := agentconfig.ScaffoldProvider(root, projects.config("alpha"), "codex"); err != nil {
			t.Fatal(err)
		}
	}
	return home, root, projects, d
}

func refreshSkills(t *testing.T, d *agentDaemon) map[string]any {
	t.Helper()
	value, err := d.executeOperation(context.Background(), agentprotocol.Operation{ProjectID: "alpha", Action: "refresh_skills"})
	if err != nil {
		t.Fatal(err)
	}
	return value.(map[string]any)
}

func codexClarify(t *testing.T, home string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, ".agents", "skills", "clarify-issue", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// A workstation that never set up a direct copy gets none from a refresh.
func TestRefreshSkillsWritesNothingWithoutADirectSetup(t *testing.T) {
	home, _, projects, d := refreshFixture(t, false)
	projects.override("alpha", "clarify", "## Steps\nAlpha steps.")
	if value := refreshSkills(t, d); value["written"] != 0 {
		t.Fatalf("written = %v", value["written"])
	}
	for _, dir := range []string{".agents", ".claude", ".gemini", ".claude.json", ".codex"} {
		if _, err := os.Stat(filepath.Join(home, dir)); !os.IsNotExist(err) {
			t.Fatalf("%s was written: %v", dir, err)
		}
	}
}

// The refresh rewrites the providers the manifest names, and installs no other
// provider nor any MCP registration.
func TestRefreshSkillsRewritesOnlyManagedProviders(t *testing.T) {
	home, _, projects, d := refreshFixture(t, true)
	projects.override("beta", "clarify", "## Steps\nBeta steps.")
	if value := refreshSkills(t, d); value["written"] == 0 {
		t.Fatalf("nothing written: %v", value)
	}
	if content := codexClarify(t, home); !strings.Contains(content, `projectId "beta"`) || !strings.Contains(content, "Beta steps.") {
		t.Fatalf("the codex copy lacks beta's variant:\n%s", content)
	}
	for _, dir := range []string{".claude", ".gemini", ".claude.json", ".codex"} {
		if _, err := os.Stat(filepath.Join(home, dir)); !os.IsNotExist(err) {
			t.Fatalf("%s was written: %v", dir, err)
		}
	}
}

// A copy edited by hand is backed up in the checkout before it is replaced.
func TestRefreshSkillsBacksUpAHandEdit(t *testing.T) {
	home, root, projects, d := refreshFixture(t, true)
	projects.override("alpha", "clarify", "## Steps\nAlpha steps.")
	path := filepath.Join(home, ".agents", "skills", "clarify-issue", "SKILL.md")
	if err := os.WriteFile(path, []byte("My own clarify"), 0600); err != nil {
		t.Fatal(err)
	}
	refreshSkills(t, d)
	if content := codexClarify(t, home); !strings.Contains(content, "Alpha steps.") {
		t.Fatalf("the copy was not refreshed:\n%s", content)
	}
	found := false
	_ = filepath.WalkDir(filepath.Join(root, ".taskflow", "skill-backups"), func(p string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			if raw, readErr := os.ReadFile(p); readErr == nil && string(raw) == "My own clarify" {
				found = true
			}
		}
		return nil
	})
	if !found {
		t.Fatal("the hand edit was not backed up")
	}
}

// A refresh that cannot read a project still writes the copies with the other
// projects' variants, and names the skipped one in its warnings: one unreadable
// project does not keep every other project's variant stale.
func TestRefreshSkillsWarnsAndWritesWhenAProjectFails(t *testing.T) {
	home, _, projects, d := refreshFixture(t, true)
	projects.override("alpha", "clarify", "## Steps\nAlpha steps.")
	projects.failing["beta"] = true
	value := refreshSkills(t, d)
	assertWarnsAbout(t, value, "beta")
	if content := codexClarify(t, home); !strings.Contains(content, "Alpha steps.") {
		t.Fatalf("the copy lacks alpha's variant:\n%s", content)
	}
}

// sync_config, like refresh_skills, writes the copies without the unreadable
// project and names it in its warnings.
func TestSyncConfigWarnsAndWritesWhenAProjectFails(t *testing.T) {
	home, _, projects, d := refreshFixture(t, true)
	projects.override("alpha", "clarify", "## Steps\nAlpha steps.")
	projects.failing["beta"] = true
	value, err := d.executeOperation(context.Background(), agentprotocol.Operation{ProjectID: "alpha", Action: "sync_config", Provider: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	assertWarnsAbout(t, value.(map[string]any), "beta")
	if content := codexClarify(t, home); !strings.Contains(content, "Alpha steps.") {
		t.Fatalf("the copy lacks alpha's variant:\n%s", content)
	}
}

func assertWarnsAbout(t *testing.T, value map[string]any, project string) {
	t.Helper()
	warnings, _ := value["warnings"].([]string)
	if len(warnings) != 1 || !strings.Contains(warnings[0], `"`+project+`"`) {
		t.Fatalf("warnings = %v, want one naming %s", value["warnings"], project)
	}
	if value["written"] == 0 {
		t.Fatalf("nothing written: %v", value)
	}
}

// An agent serving every project refreshes the user-level copies of a project
// it has no local mapping for.
func TestRefreshSkillsWithoutAProjectMapping(t *testing.T) {
	home, _, projects, d := refreshFixture(t, true)
	d.link.projectID = "all"
	if _, _, err := d.localProjectRoot(context.Background(), projects.config("alpha")); err == nil {
		t.Fatal("the fixture maps alpha")
	}
	projects.override("alpha", "clarify", "## Steps\nAlpha steps.")
	if value := refreshSkills(t, d); value["written"] == 0 {
		t.Fatalf("nothing written: %v", value)
	}
	if content := codexClarify(t, home); !strings.Contains(content, "Alpha steps.") {
		t.Fatalf("the copy was not refreshed:\n%s", content)
	}
}

// An agent serving every project leaves its copies as they are when a project
// it disconnected changes: that project has no variant in them.
func TestRefreshSkillsIgnoresADisconnectedProject(t *testing.T) {
	home, _, projects, d := refreshFixture(t, true)
	d.link.projectID = "all"
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		t.Fatal(err)
	}
	settings.DisconnectedProjects = map[string]bool{"alpha": true}
	if err := agentconfig.WriteSettings(settings); err != nil {
		t.Fatal(err)
	}
	before := codexClarify(t, home)
	projects.override("alpha", "clarify", "## Steps\nAlpha steps.")
	if value := refreshSkills(t, d); value["written"] != 0 {
		t.Fatalf("written = %v", value["written"])
	}
	if content := codexClarify(t, home); content != before {
		t.Fatalf("the copy changed:\n%s", content)
	}
}

// Two saves of a project close together: the refresh that fetched the
// project before the second save holds the direct copy lock from that fetch,
// so the refresh sent for the second save cannot fetch, nor write, before it,
// and the copy written last carries the second save.
func TestRefreshSkillsWritesTheLaterSaveLast(t *testing.T) {
	home, _, projects, d := refreshFixture(t, true)
	projects.override("alpha", "clarify", "## Steps\nFirst save.")
	target, err := url.Parse(d.link.serverURL)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	var mu sync.Mutex
	alphaFetches := 0
	fetched, release := make(chan struct{}), make(chan struct{})
	gate := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/config" || r.URL.Query().Get("projectId") != "alpha" {
			proxy.ServeHTTP(w, r)
			return
		}
		mu.Lock()
		alphaFetches++
		first := alphaFetches == 1
		mu.Unlock()
		// The first fetch reads the server before the second save, then is
		// held until the test releases it.
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, r)
		if first {
			close(fetched)
			<-release
		}
		for key, values := range rec.Header() {
			w.Header()[key] = values
		}
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
	}))
	t.Cleanup(gate.Close)
	d.link.serverURL = gate.URL

	refresh := func(done chan<- error) {
		_, err := d.executeOperation(context.Background(), agentprotocol.Operation{ProjectID: "alpha", Action: "refresh_skills"})
		done <- err
	}
	first, second := make(chan error, 1), make(chan error, 1)
	go refresh(first)
	<-fetched
	projects.override("alpha", "clarify", "## Steps\nSecond save.")
	go refresh(second)
	select {
	case err := <-second:
		close(release)
		t.Fatalf("the second refresh finished while the first held its fetch: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	mu.Lock()
	started := alphaFetches
	mu.Unlock()
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
	if started != 1 {
		t.Fatalf("%d fetches of alpha started while the first refresh held its fetch, want 1", started)
	}
	if content := codexClarify(t, home); !strings.Contains(content, "Second save.") {
		t.Fatalf("the copy written last lacks the second save:\n%s", content)
	}
}

// Once the override is gone, the next refresh puts the built-in copy back.
func TestRefreshSkillsRestoresTheBuiltIn(t *testing.T) {
	home, _, projects, d := refreshFixture(t, true)
	projects.override("alpha", "clarify", "## Steps\nAlpha steps.")
	refreshSkills(t, d)
	if content := codexClarify(t, home); !strings.Contains(content, "Alpha steps.") {
		t.Fatalf("the override did not reach the copy:\n%s", content)
	}
	projects.reset("alpha")
	refreshSkills(t, d)
	stage, _ := skills.StageSkillByID("clarify")
	if content := codexClarify(t, home); content != skills.RenderDirectSkillContent(stage) {
		t.Fatalf("the built-in did not come back:\n%s", content)
	}
}
