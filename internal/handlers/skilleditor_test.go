package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/models"
)

func skillEditorRequest(h *Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.HandleProjectDetail(rec, req)
	return rec
}

// The skill editor carries the override kind (#732): a save that names none
// defaults, an explicit one is kept, a work-only override that does not parse is
// refused with the parser's reason, and the list says which skills take one.
func TestSkillEditorSavesTheOverrideKind(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Editor"})
	if err != nil {
		t.Fatal(err)
	}
	base := "/api/projects/" + project.ID + "/skill-editor"

	rec := skillEditorRequest(h, http.MethodPut, base+"/clarify", `{"content":"## Steps\nProject steps."}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("saving without a kind: %d %s", rec.Code, rec.Body.String())
	}
	var entry models.SkillEditorEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.OverrideKind != models.SkillOverrideWork || !entry.IsCustom {
		t.Fatalf("a new override without a kind: %+v", entry)
	}

	rec = skillEditorRequest(h, http.MethodPut, base+"/clarify", `{"content":"---\nname: clarify-issue\n---\nWhole skill.","overrideKind":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("switching to a full replacement: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry.OverrideKind != models.SkillOverrideFull {
		t.Fatalf("an explicit full save: %+v", entry)
	}

	rec = skillEditorRequest(h, http.MethodPut, base+"/specify", `{"content":"Intro.\n## Steps\nSteps.","overrideKind":"work"}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "surcharge du travail invalide") || !strings.Contains(rec.Body.String(), "text before the first section") {
		t.Fatalf("an invalid work-only override: %d %s", rec.Code, rec.Body.String())
	}
	rec = skillEditorRequest(h, http.MethodPut, base+"/specify", `{"content":"## Steps\nSteps.","overrideKind":"partial"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("an unknown kind: %d %s", rec.Code, rec.Body.String())
	}

	rec = skillEditorRequest(h, http.MethodGet, base, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("listing: %d %s", rec.Code, rec.Body.String())
	}
	var entries []models.SkillEditorEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	listed := map[string]models.SkillEditorEntry{}
	for _, e := range entries {
		listed[e.ID] = e
	}
	if e := listed["specify"]; !e.Overridable || !strings.Contains(e.DefaultWorkContent, "## Steps") {
		t.Fatalf("specify takes a work-only override: %+v", e)
	}
	if e := listed["refine_macro"]; e.Overridable || e.DefaultWorkContent != "" {
		t.Fatalf("a macro skill is replaced whole: %+v", e)
	}
}
