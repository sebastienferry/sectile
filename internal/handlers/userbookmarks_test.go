package handlers

import (
	"bytes"
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

func TestProjectCreationAutoBookmark(t *testing.T) {
	h, _, user1Cookie, user2Cookie := bookmarksTestHandler(t)

	reqBody, _ := json.Marshal(models.CreateProjectRequest{
		Name: "Created Project",
		Slug: "created-proj",
	})
	w := httptest.NewRecorder()
	h.HandleProjects(w, signedRequest(user1Cookie, http.MethodPost, "/api/projects", bytes.NewReader(reqBody)))
	if w.Code != http.StatusCreated {
		t.Fatalf("POST /api/projects expected 201 Created, got %d: %s", w.Code, w.Body)
	}

	var created models.Project
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("decode created project: %v", err)
	}
	if !created.Bookmarked {
		t.Fatalf("expected created project to have bookmarked=true for creator")
	}

	// Verify user 1 has created project in bookmarks
	w = httptest.NewRecorder()
	h.HandleUserProjectBookmarks(w, signedRequest(user1Cookie, http.MethodGet, "/api/me/project-bookmarks", nil))
	var user1Bookmarks []string
	_ = json.NewDecoder(w.Body).Decode(&user1Bookmarks)
	found := false
	for _, b := range user1Bookmarks {
		if b == created.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected created project %s in user1 bookmarks %v", created.ID, user1Bookmarks)
	}

	// Verify user 2 does NOT have it bookmarked
	w = httptest.NewRecorder()
	h.HandleProjects(w, signedRequest(user2Cookie, http.MethodGet, "/api/projects", nil))
	var user2Projects []models.Project
	_ = json.NewDecoder(w.Body).Decode(&user2Projects)
	for _, p := range user2Projects {
		if p.ID == created.ID && p.Bookmarked {
			t.Fatalf("user 2 should not have created project bookmarked")
		}
	}
}

func TestTaskAndFacetsScopingHTTP(t *testing.T) {
	h, database, user1Cookie, _ := bookmarksTestHandler(t)

	pOther, err := database.CreateProject(models.CreateProjectRequest{
		Name: "Other Unbookmarked Project",
		Slug: "other-unbookmarked",
	})
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	t1, err := database.CreateTask(models.CreateTaskRequest{
		Title:     "Task in default",
		ProjectID: "default",
		Source:    "local",
	})
	if err != nil {
		t.Fatalf("CreateTask 1 failed: %v", err)
	}

	_, err = database.CreateTask(models.CreateTaskRequest{
		Title:     "Task in unbookmarked project",
		ProjectID: pOther.ID,
		Source:    "local",
	})
	if err != nil {
		t.Fatalf("CreateTask 2 failed: %v", err)
	}

	// User 1 requests tasks with projectId=all
	w := httptest.NewRecorder()
	h.HandleTasks(w, signedRequest(user1Cookie, http.MethodGet, "/api/tasks?projectId=all", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("HandleTasks failed: %d %s", w.Code, w.Body)
	}
	var tasks []models.Task
	_ = json.NewDecoder(w.Body).Decode(&tasks)
	if len(tasks) != 1 || tasks[0].ID != t1.ID {
		t.Fatalf("expected only task t1 for user1, got %d tasks", len(tasks))
	}

	// User 1 requests task facets with projectId=all
	w = httptest.NewRecorder()
	h.HandleTaskFacets(w, signedRequest(user1Cookie, http.MethodGet, "/api/tasks/facets?projectId=all", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("HandleTaskFacets failed: %d %s", w.Code, w.Body)
	}
	var facets db.TaskFacets
	_ = json.NewDecoder(w.Body).Decode(&facets)
	if facets.Total != 1 {
		t.Fatalf("expected facets.Total=1 for user1, got %d", facets.Total)
	}
}

