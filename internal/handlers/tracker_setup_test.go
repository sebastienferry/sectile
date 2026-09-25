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
)

// The setup screen checks a credential against the instance, and saves no
// token without an admin behind it (#464).
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

	// A token in a save is a server credential, an admin's to set: without a
	// session it is refused before any check, and nothing changes. The admin's
	// path is covered by TestAnAdminSetsChecksAndClearsTheServerCredential.
	rr := post("/api/setup/tracker", "good-token")
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("a token saved without a session must be refused: %d %s", rr.Code, rr.Body.String())
	}
	settings, err := database.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.GithubTokenSet || settings.GithubApiUrl != "" {
		t.Fatalf("the configuration changed on a refused save: %+v", settings)
	}
}
