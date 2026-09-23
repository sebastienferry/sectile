package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

// TestProjectEpicColorsOverHTTP covers the contract the settings screen talks
// to. Switching the colour off is the case worth pinning: false has to reach
// the database as "off", not as "the caller said nothing".
func TestProjectEpicColorsOverHTTP(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Colours"})
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandler(database)
	patch := func(body string) (models.Project, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPatch, "/api/projects/"+project.ID, strings.NewReader(body))
		rec := httptest.NewRecorder()
		h.HandleProjectDetail(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH %s returned %d: %s", body, rec.Code, rec.Body.String())
		}
		var out models.Project
		var raw map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("decoding the response to %s: %v", body, err)
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &raw)
		return out, raw
	}

	// The field is always present, so the interface can tell "off" from an
	// older server that does not know the setting.
	if got, raw := patch(`{"description":"first"}`); got.EpicColors || raw["epicColors"] != false {
		t.Fatalf("a new project reports epicColors=%v, want an explicit false", raw["epicColors"])
	}
	if got, _ := patch(`{"epicColors":true}`); !got.EpicColors {
		t.Fatal("after enabling, the project does not show epic colours")
	}
	if got, _ := patch(`{"description":"unrelated"}`); !got.EpicColors {
		t.Fatal("an unrelated change turned epic colours off")
	}
	if got, _ := patch(`{"epicColors":false}`); got.EpicColors {
		t.Fatal("after disabling, the project still shows epic colours")
	}
}
