package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

func bookmarksTestHandler(t *testing.T) (*Handler, *db.DB, *http.Cookie, *http.Cookie) {
	t.Helper()
	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	u1, err := database.SignInLocal("user1@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token1, _, err := database.CreateWebSession(u1.ID)
	if err != nil {
		t.Fatal(err)
	}

	u2, err := database.SignInLocal("user2@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token2, _, err := database.CreateWebSession(u2.ID)
	if err != nil {
		t.Fatal(err)
	}

	h := NewHandler(database)
	return h, database, &http.Cookie{Name: sessionCookie, Value: token1}, &http.Cookie{Name: sessionCookie, Value: token2}
}

func TestUserBookmarksRequireSession(t *testing.T) {
	h, _, _, _ := bookmarksTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/me/project-bookmarks", nil)
	w := httptest.NewRecorder()
	h.HandleUserProjectBookmarks(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauthenticated request, got %d", w.Code)
	}
}

func TestUserBookmarksCRUDAndToggle(t *testing.T) {
	h, database, user1Cookie, user2Cookie := bookmarksTestHandler(t)

	// Create a new project
	p2, err := database.CreateProject(models.CreateProjectRequest{
		Name: "Project Beta",
		Slug: "proj-beta",
	})
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	// 1. GET /api/me/project-bookmarks for user 1 (seeds default project)
	w := httptest.NewRecorder()
	h.HandleUserProjectBookmarks(w, signedRequest(user1Cookie, http.MethodGet, "/api/me/project-bookmarks", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/me/project-bookmarks: %d %s", w.Code, w.Body)
	}
	var list []string
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 default bookmark, got %v", list)
	}

	// 2. PUT /api/me/project-bookmarks/{id}
	w = httptest.NewRecorder()
	h.HandleUserProjectBookmarks(w, signedRequest(user1Cookie, http.MethodPut, "/api/me/project-bookmarks/"+p2.ID, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("PUT /api/me/project-bookmarks/%s: %d %s", p2.ID, w.Code, w.Body)
	}

	// Verify user 1 has 2 bookmarks now
	w = httptest.NewRecorder()
	h.HandleUserProjectBookmarks(w, signedRequest(user1Cookie, http.MethodGet, "/api/me/project-bookmarks", nil))
	json.NewDecoder(w.Body).Decode(&list)
	if len(list) != 2 {
		t.Fatalf("expected 2 bookmarks for user1, got %d", len(list))
	}

	// Verify user 2 is isolated and does not have p2 bookmarked
	w = httptest.NewRecorder()
	h.HandleUserProjectBookmarks(w, signedRequest(user2Cookie, http.MethodGet, "/api/me/project-bookmarks", nil))
	var list2 []string
	json.NewDecoder(w.Body).Decode(&list2)
	if len(list2) != 1 || list2[0] == p2.ID {
		t.Fatalf("user 2 should not have p2 bookmarked, got %v", list2)
	}

	// 3. POST /api/me/project-bookmarks/{id}/toggle
	w = httptest.NewRecorder()
	h.HandleUserProjectBookmarks(w, signedRequest(user1Cookie, http.MethodPost, "/api/me/project-bookmarks/"+p2.ID+"/toggle", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("POST toggle: %d %s", w.Code, w.Body)
	}
	var toggleRes map[string]bool
	json.NewDecoder(w.Body).Decode(&toggleRes)
	if toggleRes["bookmarked"] != false {
		t.Fatalf("expected bookmarked=false after toggle, got %v", toggleRes)
	}

	// Toggle back to true
	w = httptest.NewRecorder()
	h.HandleUserProjectBookmarks(w, signedRequest(user1Cookie, http.MethodPost, "/api/me/project-bookmarks/"+p2.ID+"/toggle", nil))
	json.NewDecoder(w.Body).Decode(&toggleRes)
	if toggleRes["bookmarked"] != true {
		t.Fatalf("expected bookmarked=true after second toggle, got %v", toggleRes)
	}

	// 4. DELETE /api/me/project-bookmarks/{id}
	w = httptest.NewRecorder()
	h.HandleUserProjectBookmarks(w, signedRequest(user1Cookie, http.MethodDelete, "/api/me/project-bookmarks/"+p2.ID, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("DELETE: %d %s", w.Code, w.Body)
	}
	json.NewDecoder(w.Body).Decode(&toggleRes)
	if toggleRes["bookmarked"] != false {
		t.Fatalf("expected bookmarked=false after delete, got %v", toggleRes)
	}
}
