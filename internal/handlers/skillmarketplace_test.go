package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/models"
)

const packSkillBody = "---\nname: clarify-issue\ndescription: acme\n---\n\n# Acme clarify\n\nAsk the three Acme questions.\n"

// marketplaceServer serves the registry and the project routes behind the same
// guard main.go installs, so the tests exercise the real authorization chain.
func marketplaceServer(t *testing.T) (*httptest.Server, *db.DB, *models.Project) {
	t.Helper()
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	stub := func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		switch op.Action {
		case "marketplace_catalog":
			return json.Marshal(models.MarketplaceCatalog{Name: "acme", Owner: "Acme", Commit: strings.Repeat("a", 40),
				Plugins: []models.MarketplacePlugin{{Name: "acme-flow", Source: "./p", Version: "2.1.0", Skills: []string{"clarify-issue"}}}})
		case "marketplace_pack":
			return json.Marshal(models.SkillPack{Marketplace: "acme", Plugin: "acme-flow", Version: "2.1.0",
				Commit: strings.Repeat("a", 40), Bodies: map[string]string{"clarify-issue": packSkillBody}})
		}
		return json.RawMessage(`{}`), nil
	}

	no := false
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Packs", RepoPath: "/not-mounted", IssueTracker: "local", UseWorktrees: &no})
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandler(database)
	// NewHandler routes local operations to the agent dispatcher; the tests
	// answer for the workstation instead.
	database.SetAgentOperations(stub)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/skill-marketplaces", h.HandleSkillMarketplaces)
	mux.HandleFunc("/api/skill-marketplaces/", h.HandleSkillMarketplaces)
	mux.HandleFunc("/api/projects/", h.HandleProjectDetail)
	server := httptest.NewServer(h.EnableCORS(h.RequireSession(mux)))
	t.Cleanup(server.Close)
	return server, database, project
}

// The registry is a deployment setting: a member reads it and cannot write it.
func TestSkillMarketplaceRegistryIsAdminOnlyToWrite(t *testing.T) {
	server, database, _ := marketplaceServer(t)
	_, admin := account(t, database, "admin@example.com")
	_, member := account(t, database, "member@example.com")

	body := `{"name":"acme","kind":"path","locator":"/srv/marketplaces/acme"}`
	if status, text := call(t, server, member, http.MethodPost, "/api/skill-marketplaces", body); status != http.StatusForbidden {
		t.Fatalf("a member registered a marketplace: %d %s", status, text)
	}
	if status, text := call(t, server, admin, http.MethodPost, "/api/skill-marketplaces", body); status != http.StatusCreated {
		t.Fatalf("registration refused: %d %s", status, text)
	}
	if status, text := call(t, server, member, http.MethodGet, "/api/skill-marketplaces", ""); status != http.StatusOK || !strings.Contains(text, "acme") {
		t.Fatalf("a member cannot read the registry: %d %s", status, text)
	}
	if status, _ := call(t, server, member, http.MethodDelete, "/api/skill-marketplaces/acme", ""); status != http.StatusForbidden {
		t.Fatal("a member unregistered a marketplace")
	}
	if status, text := call(t, server, admin, http.MethodGet, "/api/skill-marketplaces/acme/catalog", ""); status != http.StatusOK || !strings.Contains(text, "acme-flow") {
		t.Fatalf("catalog: %d %s", status, text)
	}
}

func TestUnknownMarketplaceIsNotFound(t *testing.T) {
	server, database, _ := marketplaceServer(t)
	_, admin := account(t, database, "admin@example.com")

	if status, _ := call(t, server, admin, http.MethodGet, "/api/skill-marketplaces/ghost", ""); status != http.StatusNotFound {
		t.Fatal("an unregistered marketplace was found")
	}
	if status, _ := call(t, server, admin, http.MethodDelete, "/api/skill-marketplaces/ghost", ""); status != http.StatusNotFound {
		t.Fatal("removing an unregistered marketplace succeeded")
	}
}

// Previewing answers the diff and leaves the project exactly as it was;
// applying is the call that writes, and it answers the new pin.
func TestSkillPackPreviewIsReadOnlyAndApplyPins(t *testing.T) {
	server, database, project := marketplaceServer(t)
	_, admin := account(t, database, "admin@example.com")

	if status, text := call(t, server, admin, http.MethodPost, "/api/skill-marketplaces", `{"name":"acme","kind":"path","locator":"/srv/acme"}`); status != http.StatusCreated {
		t.Fatalf("registration: %d %s", status, text)
	}
	base := "/api/projects/" + project.ID + "/skill-pack"

	status, text := call(t, server, admin, http.MethodPost, base+"/preview", `{"marketplace":"acme","plugin":"acme-flow"}`)
	if status != http.StatusOK || !strings.Contains(text, "Ask the three Acme questions") {
		t.Fatalf("preview: %d %s", status, text)
	}
	if status, text := call(t, server, admin, http.MethodGet, base, ""); status != http.StatusOK || strings.TrimSpace(text) != "null" {
		t.Fatalf("the preview left a pin: %d %s", status, text)
	}

	status, text = call(t, server, admin, http.MethodPost, base, `{"marketplace":"acme","plugin":"acme-flow"}`)
	if status != http.StatusOK {
		t.Fatalf("apply: %d %s", status, text)
	}
	var pin models.SkillPackPin
	if err := json.Unmarshal([]byte(text), &pin); err != nil {
		t.Fatal(err)
	}
	if pin.Marketplace != "acme" || pin.Plugin != "acme-flow" || pin.Version != "2.1.0" {
		t.Fatalf("pin: %+v", pin)
	}

	if status, text := call(t, server, admin, http.MethodDelete, base, ""); status != http.StatusOK {
		t.Fatalf("unpin: %d %s", status, text)
	}
	if status, text := call(t, server, admin, http.MethodGet, base, ""); status != http.StatusOK || strings.TrimSpace(text) != "null" {
		t.Fatalf("the pin survived unpinning: %d %s", status, text)
	}
}

// A member may read what a pack would change, and may not make it effective.
func TestApplyingASkillPackIsAnAdminAction(t *testing.T) {
	server, database, project := marketplaceServer(t)
	_, admin := account(t, database, "admin@example.com")
	_, member := account(t, database, "member@example.com")
	if status, _ := call(t, server, admin, http.MethodPost, "/api/skill-marketplaces", `{"name":"acme","kind":"path","locator":"/srv/acme"}`); status != http.StatusCreated {
		t.Fatal("registration refused")
	}
	base := "/api/projects/" + project.ID + "/skill-pack"

	if status, text := call(t, server, member, http.MethodPost, base+"/preview", `{"marketplace":"acme","plugin":"acme-flow"}`); status != http.StatusOK {
		t.Fatalf("a member cannot preview: %d %s", status, text)
	}
	if status, _ := call(t, server, member, http.MethodPost, base, `{"marketplace":"acme","plugin":"acme-flow"}`); status != http.StatusForbidden {
		t.Fatal("a member applied a pack")
	}
}
