package db

import (
	"testing"

	"tasks/internal/models"
)

func TestEnsureDefaultBookmark(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	// Default project exists on init
	projects, err := database.GetProjects()
	if err != nil {
		t.Fatalf("GetProjects failed: %v", err)
	}
	if len(projects) == 0 {
		t.Fatalf("Expected at least one project")
	}
	defaultProjID := projects[0].ID

	user1 := "user-1"
	// Before accessing bookmarks, user has none
	bookmarks, err := database.GetUserProjectBookmarks(user1)
	if err != nil {
		t.Fatalf("GetUserProjectBookmarks failed: %v", err)
	}
	if len(bookmarks) != 1 || bookmarks[0] != defaultProjID {
		t.Fatalf("Expected default bookmark [%s], got %v", defaultProjID, bookmarks)
	}

	// Calling EnsureDefaultBookmark again is idempotent
	if err := database.EnsureDefaultBookmark(user1); err != nil {
		t.Fatalf("EnsureDefaultBookmark failed: %v", err)
	}
	bookmarks, _ = database.GetUserProjectBookmarks(user1)
	if len(bookmarks) != 1 {
		t.Fatalf("Expected exactly 1 bookmark, got %d", len(bookmarks))
	}
}

func TestBookmarkIsolationAndToggle(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	p2, err := database.CreateProject(models.CreateProjectRequest{
		Name: "Project Beta",
		Slug: "proj-beta",
	})
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	userA := "user-a"
	userB := "user-b"

	// userA bookmarks p2
	if err := database.BookmarkProject(userA, p2.ID); err != nil {
		t.Fatalf("BookmarkProject userA failed: %v", err)
	}

	// userB accesses bookmarks -> gets default project only
	bookmarksB, err := database.GetUserProjectBookmarks(userB)
	if err != nil {
		t.Fatalf("GetUserProjectBookmarks userB failed: %v", err)
	}
	if len(bookmarksB) != 1 || bookmarksB[0] == p2.ID {
		t.Fatalf("userB should not have p2 bookmarked, got %v", bookmarksB)
	}

	// userA toggles p2 -> should be unbookmarked
	bookmarked, err := database.ToggleProjectBookmark(userA, p2.ID)
	if err != nil {
		t.Fatalf("ToggleProjectBookmark failed: %v", err)
	}
	if bookmarked {
		t.Fatalf("Expected p2 to be unbookmarked after toggle")
	}

	// userA toggles p2 again -> should be bookmarked
	bookmarked, err = database.ToggleProjectBookmark(userA, p2.ID)
	if err != nil {
		t.Fatalf("ToggleProjectBookmark failed: %v", err)
	}
	if !bookmarked {
		t.Fatalf("Expected p2 to be bookmarked after toggle")
	}

	// Test UnbookmarkProject
	if err := database.UnbookmarkProject(userA, p2.ID); err != nil {
		t.Fatalf("UnbookmarkProject failed: %v", err)
	}
}

func TestGetProjectsForUser(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	p2, err := database.CreateProject(models.CreateProjectRequest{
		Name: "Project Gamma",
		Slug: "proj-gamma",
	})
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	userX := "user-x"
	// Before bookmarking p2, only default project is bookmarked
	projects, err := database.GetProjectsForUser(userX)
	if err != nil {
		t.Fatalf("GetProjectsForUser failed: %v", err)
	}
	for _, p := range projects {
		if p.ID == p2.ID && p.Bookmarked {
			t.Fatalf("p2 should not be bookmarked for userX")
		}
		if p.IsDefault && !p.Bookmarked {
			t.Fatalf("default project should be bookmarked for userX")
		}
	}

	// Now bookmark p2
	_ = database.BookmarkProject(userX, p2.ID)
	projects, err = database.GetProjectsForUser(userX)
	if err != nil {
		t.Fatalf("GetProjectsForUser failed: %v", err)
	}
	for _, p := range projects {
		if p.ID == p2.ID && !p.Bookmarked {
			t.Fatalf("p2 should now be bookmarked for userX")
		}
	}
}

func TestDeleteProjectCascadesBookmarks(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	p, err := database.CreateProject(models.CreateProjectRequest{
		Name: "Temporary Project",
		Slug: "temp-proj",
	})
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	user := "user-del"
	if err := database.BookmarkProject(user, p.ID); err != nil {
		t.Fatalf("BookmarkProject failed: %v", err)
	}

	// Delete project
	if err := database.DeleteProject(p.ID); err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}

	// Verify bookmark was deleted
	var count int
	_ = database.conn.QueryRow("SELECT COUNT(*) FROM user_project_bookmarks WHERE project_id = ?", p.ID).Scan(&count)
	if count != 0 {
		t.Fatalf("Expected 0 bookmarks for deleted project, got %d", count)
	}
}

func TestUserBookmarkTaskAndFacetScoping(t *testing.T) {
	database, cleanup := setupTestDB(t)
	defer cleanup()

	// Default project has id "default"
	p2, err := database.CreateProject(models.CreateProjectRequest{
		Name: "Project Two",
		Slug: "proj-two",
	})
	if err != nil {
		t.Fatalf("CreateProject failed: %v", err)
	}

	// Create a task in default project
	t1, err := database.CreateTask(models.CreateTaskRequest{
		Title:     "Task in default",
		ProjectID: "default",
		Source:    "local",
	})
	if err != nil {
		t.Fatalf("CreateTask 1 failed: %v", err)
	}

	// Create a task in Project Two
	t2, err := database.CreateTask(models.CreateTaskRequest{
		Title:     "Task in proj two",
		ProjectID: p2.ID,
		Source:    "local",
	})
	if err != nil {
		t.Fatalf("CreateTask 2 failed: %v", err)
	}

	user := "user-scope"
	// User has default project bookmarked (seeded automatically). Does NOT have p2 bookmarked yet.
	tasks, err := database.GetTasksForUser(user, "", "", "", "", "all", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatalf("GetTasksForUser failed: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != t1.ID {
		t.Fatalf("Expected only t1 for user without p2 bookmark, got %d tasks", len(tasks))
	}

	facets, err := database.GetTaskFacetsForUser(user, "all")
	if err != nil {
		t.Fatalf("GetTaskFacetsForUser failed: %v", err)
	}
	if facets.Total != 1 {
		t.Fatalf("Expected facets.Total == 1, got %d", facets.Total)
	}

	// Now bookmark p2
	_ = database.BookmarkProject(user, p2.ID)

	tasks, err = database.GetTasksForUser(user, "", "", "", "", "all", "", "", "", "", nil, nil, false)
	if err != nil {
		t.Fatalf("GetTasksForUser failed: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("Expected 2 tasks after bookmarking p2, got %d", len(tasks))
	}

	facets, err = database.GetTaskFacetsForUser(user, "all")
	if err != nil {
		t.Fatalf("GetTaskFacetsForUser failed: %v", err)
	}
	if facets.Total != 2 {
		t.Fatalf("Expected facets.Total == 2, got %d", facets.Total)
	}

	_ = t2
}
