package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
)

// The setup screen saves nothing until the instance has accepted the
// parameters, which is what keeps a stale token from reaching the
// configuration and failing later in a background synchronisation.
func TestHandleTrackerSetupChecksBeforeSaving(t *testing.T) {
	instance := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer good-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"login": "octocat"})
	}))
	defer instance.Close()

	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	h := handlers.NewHandler(database)

	post := func(path, token string) *httptest.ResponseRecorder {
		body := `{"tracker":"github","siteUrl":"` + instance.URL + `","token":"` + token + `"}`
		rr := httptest.NewRecorder()
		h.HandleTrackerSetup(rr, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
		return rr
	}

	if rr := post("/api/setup/tracker/check", "good-token"); rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "octocat") {
		t.Fatalf("check: %d %s", rr.Code, rr.Body.String())
	}

	rr := post("/api/setup/tracker", "wrong-token")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("a refused credential must not be saved: %d %s", rr.Code, rr.Body.String())
	}
	settings, err := database.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.GithubTokenSet || settings.GithubApiUrl != "" {
		t.Fatalf("the configuration changed on a failed check: %+v", settings)
	}

	if rr = post("/api/setup/tracker", "good-token"); rr.Code != http.StatusOK {
		t.Fatalf("save: %d %s", rr.Code, rr.Body.String())
	}
	var saved models.Settings
	if err = json.Unmarshal(rr.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.GithubTokenSet || saved.GithubApiUrl != instance.URL {
		t.Fatalf("parameters not persisted: %+v", saved)
	}
	if strings.Contains(rr.Body.String(), "good-token") {
		t.Fatalf("the response carried the token back: %s", rr.Body.String())
	}
}
