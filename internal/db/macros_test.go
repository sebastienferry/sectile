package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

func TestProjectCompatibility(t *testing.T) {
	p1 := &models.Project{ID: "p1", IssueTracker: "github", GithubRepo: "owner/repo1"}
	p2 := &models.Project{ID: "p2", IssueTracker: "github", GithubRepo: "owner/repo2"}
	pLocal := &models.Project{ID: "p3", IssueTracker: "local"}
	pJira := &models.Project{ID: "p4", IssueTracker: "jira"}

	if !IsProjectCompatible(p1, p2) {
		t.Errorf("expected GitHub projects to be compatible")
	}
	if !IsProjectCompatible(p1, pLocal) {
		t.Errorf("expected GitHub and local project to be compatible")
	}
	if IsProjectCompatible(p1, pJira) {
		t.Errorf("expected GitHub and Jira project not to be compatible")
	}
}

func TestMigrateMacroAndTasks(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := testsqlite.New(t, dbPath, NewDB)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Create two local projects
	p1, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Project A",
		Slug:         "project-a",
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatalf("Failed to create project A: %v", err)
	}

	p2, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Project B",
		Slug:         "project-b",
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatalf("Failed to create project B: %v", err)
	}

	// Create a macro in project A
	h := "now"
	desc := "Macro in Project A"
	todos := []models.MacroTodo{{ID: "t1", Text: "Do something", Done: false}}
	macroA, err := database.SaveMacroMeta(p1.ID, "M-1", &h, &desc, nil, &todos)
	if err != nil {
		t.Fatalf("Failed to save macro: %v", err)
	}
	if macroA.Key != "M-1" {
		t.Fatalf("Expected macro key M-1, got %s", macroA.Key)
	}

	shaping := "shaping"
	if _, err := database.SaveMacroAxes(p1.ID, "M-1", nil, nil, &shaping); err != nil {
		t.Fatalf("Failed to save readiness: %v", err)
	}

	// Create task under macro in project A
	taskA, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID: p1.ID,
		Title:     "Task under M-1",
		Status:    models.StatusToClarify,
		Source:    "local",
	})
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Set parent on task
	err = database.writeTaskParentLocally(taskA, "M-1")
	if err != nil {
		t.Fatalf("Failed to update parent: %v", err)
	}

	// Migrate macro and attached tasks from p1 to p2
	migratedMacro, taskCount, err := database.MigrateMacro(context.Background(), p1.ID, "M-1", p2.ID, true)
	if err != nil {
		t.Fatalf("MigrateMacro failed: %v", err)
	}
	if migratedMacro.ProjectID != p2.ID {
		t.Errorf("Expected migrated macro project ID %s, got %s", p2.ID, migratedMacro.ProjectID)
	}
	if taskCount != 1 {
		t.Errorf("Expected 1 migrated task, got %d", taskCount)
	}
	movedMacros, _ := database.GetProjectMacros(p2.ID)
	if len(movedMacros) != 1 || movedMacros[0].Readiness != "shaping" {
		t.Errorf("the moved macro should keep its readiness: %+v", movedMacros)
	}

	// Verify task in DB is now in p2
	updatedTask, err := database.GetTaskByID(taskA.ID)
	if err != nil || updatedTask == nil {
		t.Fatalf("Failed to get updated task: %v", err)
	}
	if updatedTask.ProjectID != p2.ID {
		t.Errorf("Expected task project ID %s, got %s", p2.ID, updatedTask.ProjectID)
	}
}

func TestMigrateSingleTask(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := testsqlite.New(t, dbPath, NewDB)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	p1, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Project 1",
		Slug:         "project-1",
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatalf("Failed to create project 1: %v", err)
	}
	p2, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Project 2",
		Slug:         "project-2",
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatalf("Failed to create project 2: %v", err)
	}

	task, err := database.CreateTask(models.CreateTaskRequest{
		ProjectID: p1.ID,
		Title:     "Standalone Task",
		Status:    models.StatusToClarify,
		Source:    "local",
	})
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	count, err := database.MigrateTasks(context.Background(), []string{task.ID}, p2.ID)
	if err != nil {
		t.Fatalf("MigrateTasks failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 task migrated, got %d", count)
	}

	migratedTask, err := database.GetTaskByID(task.ID)
	if err != nil || migratedTask == nil {
		t.Fatalf("Failed to get migrated task: %v", err)
	}
	if migratedTask.ProjectID != p2.ID {
		t.Errorf("Expected task project ID to be %s, got %s", p2.ID, migratedTask.ProjectID)
	}
}

// L'origine d'une ligne de découpe fait l'aller-retour, et son absence reste
// lisible : les lignes enregistrées avant ces champs valent « saisie à la
// main », et rien dans la relecture ne doit les distinguer d'un choix.
func TestMacroTodoOriginRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	database, err := testsqlite.New(t, filepath.Join(tempDir, "test.db"), NewDB)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	proj, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Origin Project",
		Slug:         "origin-project",
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	todos := []models.MacroTodo{
		{
			ID:              "imported",
			Text:            "Import the slicing",
			SourceKind:      models.MacroTodoFromTasks,
			SourceEntry:     "## Group 3 - Import the slicing [P1]",
			TargetProjectID: "other-project",
		},
		{ID: "by-hand", Text: "Ask the PM about the quarter"},
	}
	if _, err := database.SaveMacroMeta(proj.ID, "M-1", nil, nil, nil, &todos); err != nil {
		t.Fatalf("SaveMacroMeta failed: %v", err)
	}

	metas, err := database.GetProjectMacros(proj.ID)
	if err != nil {
		t.Fatalf("GetProjectMacros failed: %v", err)
	}
	if len(metas) != 1 || len(metas[0].Todos) != 2 {
		t.Fatalf("expected one macro carrying two todos, got %d macro(s)", len(metas))
	}

	imported := metas[0].Todos[0]
	if imported.SourceKind != models.MacroTodoFromTasks {
		t.Errorf("expected source kind %q, got %q", models.MacroTodoFromTasks, imported.SourceKind)
	}
	if imported.SourceEntry != "## Group 3 - Import the slicing [P1]" {
		t.Errorf("expected the raw entry title to survive, got %q", imported.SourceEntry)
	}
	if imported.TargetProjectID != "other-project" {
		t.Errorf("expected target project %q, got %q", "other-project", imported.TargetProjectID)
	}

	byHand := metas[0].Todos[1]
	if byHand.SourceKind != "" || byHand.SourceEntry != "" || byHand.TargetProjectID != "" {
		t.Errorf("a hand-typed line must carry no origin, got %+v", byHand)
	}
}

// Une ligne écrite avant ces champs se relit sans origine, et non en erreur :
// la colonne porte du JSON, et c'est cette absence de clé qui tient lieu de
// migration.
func TestMacroTodoLegacyRowReadsWithoutOrigin(t *testing.T) {
	legacy := `[{"id":"old","text":"Written before the origin fields","done":true,"storyKey":"PE-42"}]`
	todos := parseMacroTodos(legacy)
	if len(todos) != 1 {
		t.Fatalf("expected one todo, got %d", len(todos))
	}
	if todos[0].Text != "Written before the origin fields" || !todos[0].Done || todos[0].StoryKey != "PE-42" {
		t.Errorf("the known fields must be unchanged, got %+v", todos[0])
	}
	if todos[0].SourceKind != "" || todos[0].SourceEntry != "" || todos[0].TargetProjectID != "" {
		t.Errorf("a legacy line must read as hand-typed, got %+v", todos[0])
	}
}

// Une origine faite de blancs est une absence d'origine ; une origine inconnue
// est gardée telle quelle, pour qu'une version antérieure ne vide pas les
// lignes qu'une version ultérieure a écrites.
func TestMacroTodoOriginIsTrimmedAndUnknownKindKept(t *testing.T) {
	tempDir := t.TempDir()
	database, err := testsqlite.New(t, filepath.Join(tempDir, "test.db"), NewDB)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	proj, err := database.CreateProject(models.CreateProjectRequest{
		Name:         "Trim Project",
		Slug:         "trim-project",
		IssueTracker: "local",
	})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	todos := []models.MacroTodo{
		{ID: "blank", Text: "Blank origin", SourceKind: "   ", SourceEntry: "\t", TargetProjectID: " "},
		{ID: "future", Text: "Origin from a later version", SourceKind: " design "},
		// The scenarios source was removed (#426); a line an older build saved
		// with it is an unknown origin like any other, never dropped.
		{ID: "retired", Text: "Origin this version no longer reads", SourceKind: "scenarios"},
	}
	if _, err := database.SaveMacroMeta(proj.ID, "M-2", nil, nil, nil, &todos); err != nil {
		t.Fatalf("SaveMacroMeta failed: %v", err)
	}

	metas, err := database.GetProjectMacros(proj.ID)
	if err != nil {
		t.Fatalf("GetProjectMacros failed: %v", err)
	}
	if len(metas) != 1 || len(metas[0].Todos) != 3 {
		t.Fatalf("expected one macro carrying three todos, got %d macro(s)", len(metas))
	}

	blank := metas[0].Todos[0]
	if blank.SourceKind != "" || blank.SourceEntry != "" || blank.TargetProjectID != "" {
		t.Errorf("a blank origin must read as none, got %+v", blank)
	}
	if kind := metas[0].Todos[1].SourceKind; kind != "design" {
		t.Errorf("an unknown source kind must be kept as written, got %q", kind)
	}
	if kind := metas[0].Todos[2].SourceKind; kind != "scenarios" {
		t.Errorf("a retired source kind must be kept as written, got %q", kind)
	}
}

// The Roadmap links its "story created" toast to the new ticket, so turning a
// todo line into a story has to hand the created task back, not only its key.
func TestCreateStoryFromMacroTodoReturnsTheTask(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), NewDB)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	proj, err := database.CreateProject(models.CreateProjectRequest{Name: "Todo story", Slug: "todo-story", IssueTracker: "local"})
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	todos := []models.MacroTodo{{ID: "t1", Text: "Link the toast"}}
	if _, err := database.SaveMacroMeta(proj.ID, "M-40", nil, nil, nil, &todos); err != nil {
		t.Fatalf("Failed to save macro: %v", err)
	}

	meta, task, _, err := database.CreateStoryFromMacroTodo(context.Background(), proj.ID, "M-40", "t1")
	if err != nil {
		t.Fatalf("CreateStoryFromMacroTodo: %v", err)
	}
	if task == nil || task.ID == "" || task.Key == "" {
		t.Fatalf("expected the created task, got %+v", task)
	}
	if task.Title != "Link the toast" || task.ParentKey != "M-40" {
		t.Errorf("task = %q under %q, want %q under M-40", task.Title, task.ParentKey, "Link the toast")
	}
	if meta == nil || len(meta.Todos) != 1 || meta.Todos[0].StoryKey != task.Key {
		t.Errorf("the todo line should carry the story key %q, got %+v", task.Key, meta)
	}
}

// A ticket imported since the adoption (#741) names its tracker rather than a
// project; renaming its macro still renames its parent. A Jira epic is shared
// by every project selecting its tracker, so the tickets of each follow.
func TestRenamingAMacroRenamesTheParentOfItsTrackerTickets(t *testing.T) {
	d := testDB(t)
	delivery := spaceProject(t, d, "Delivery", "delivery-admin")
	bidder := spaceProject(t, d, "Bidder", "bidder")
	if delivery.DefaultTrackerID != bidder.DefaultTrackerID {
		t.Fatalf("the two projects select trackers %s and %s, want GODE's alone", delivery.DefaultTrackerID, bidder.DefaultTrackerID)
	}
	notes, err := d.CreateProject(models.CreateProjectRequest{Name: "Notes"})
	if err != nil {
		t.Fatal(err)
	}
	ticket := func(key string, labels []string) models.Task {
		return models.Task{Key: key, Title: key, Status: models.StatusToClarify, Priority: models.PriorityMedium, Labels: labels, Source: "jira",
			ParentKey: "GODE-100", ParentTitle: "Old epic", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	}
	if err := d.ImportOrUpdateTasks(delivery.DefaultTrackerID, []models.Task{ticket("GODE-1", []string{"delivery-admin"}), ticket("GODE-2", []string{"bidder"})}); err != nil {
		t.Fatal(err)
	}
	local := ticket("N-1", []string{})
	local.Source, local.ParentKey, local.ParentTitle = "local", "M-1", "Old local"
	if err := d.ImportOrUpdateTasks(notes.DefaultTrackerID, []models.Task{local}); err != nil {
		t.Fatal(err)
	}
	var trackerRows int
	_ = d.conn.QueryRow(`SELECT COUNT(*) FROM tasks WHERE project_id = ?`, trackerSentinel(delivery.DefaultTrackerID)).Scan(&trackerRows)
	if trackerRows != 2 {
		t.Fatalf("%d tickets name GODE's tracker, want both imported ones", trackerRows)
	}

	renamed := "New epic"
	if _, err := d.UpdateMacro(context.Background(), delivery.ID, "GODE-100", &renamed, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	renamedLocal := "New local"
	if _, err := d.UpdateMacro(context.Background(), notes.ID, "M-1", &renamedLocal, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"GODE-1": "New epic", "GODE-2": "New epic", "N-1": "New local"} {
		var parentTitle string
		if err := d.conn.QueryRow(`SELECT parent_title FROM tasks WHERE key = ?`, key).Scan(&parentTitle); err != nil {
			t.Fatal(err)
		}
		if parentTitle != want {
			t.Fatalf("%s has parent title %q after the rename, want %q", key, parentTitle, want)
		}
	}
}

// Two projects showing every ticket of one Jira space each define a local M-3
// (#741). A ticket of the space under M-3 names no one of the two macros for
// sure: renaming Alpha's M-3 leaves its parent title alone, while Alpha's own
// local ticket under M-3 follows, and so does a shared ticket under a key only
// Alpha defines.
func TestRenamingALocalMacroLeavesTheTicketsAnotherProjectsMacroOfThatKeyMayHold(t *testing.T) {
	d := testDB(t)
	alpha := spaceProject(t, d, "Alpha", "")
	beta := spaceProject(t, d, "Beta", "")
	if alpha.DefaultTrackerID != beta.DefaultTrackerID {
		t.Fatalf("the two projects select trackers %s and %s, want GODE's alone", alpha.DefaultTrackerID, beta.DefaultTrackerID)
	}
	ticket := func(key, parent, title string) models.Task {
		return models.Task{Key: key, Title: key, Status: models.StatusToClarify, Priority: models.PriorityMedium, Source: "jira",
			ParentKey: parent, ParentTitle: title, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	}
	if err := d.ImportOrUpdateTasks(alpha.DefaultTrackerID, []models.Task{ticket("GODE-1", "M-3", "Beta's M-3"), ticket("GODE-2", "M-4", "Alpha's M-4")}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec(`INSERT INTO tasks (id, project_id, key, title, status, priority, labels, source, parent_key, parent_title, created_at, updated_at)
		VALUES ('alpha-local', ?, 'L-1', 'Local', 'to_clarify', 'medium', '[]', 'local', 'M-3', 'Alpha''s M-3', ?, ?)`, alpha.ID, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	horizon := "now"
	for _, p := range []*models.Project{alpha, beta} {
		if _, err := d.SaveMacroMeta(p.ID, "M-3", &horizon, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.SaveMacroMeta(alpha.ID, "M-4", &horizon, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"M-3", "M-4"} {
		renamed := "Alpha's renamed " + key
		if _, err := d.UpdateMacro(context.Background(), alpha.ID, key, &renamed, nil, nil, nil, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	for key, want := range map[string]string{"GODE-1": "Beta's M-3", "L-1": "Alpha's renamed M-3", "GODE-2": "Alpha's renamed M-4"} {
		var parentTitle string
		if err := d.conn.QueryRow(`SELECT parent_title FROM tasks WHERE key = ?`, key).Scan(&parentTitle); err != nil {
			t.Fatal(err)
		}
		if parentTitle != want {
			t.Errorf("%s has parent title %q after renaming Alpha's macros, want %q", key, parentTitle, want)
		}
	}

	// Deleting Alpha's M-3 detaches the tickets the rename reached, alone.
	if err := d.DeleteMacro(context.Background(), alpha.ID, "M-3"); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"GODE-1": "M-3", "L-1": ""} {
		var parentKey string
		if err := d.conn.QueryRow(`SELECT parent_key FROM tasks WHERE key = ?`, key).Scan(&parentKey); err != nil {
			t.Fatal(err)
		}
		if parentKey != want {
			t.Errorf("%s has parent %q after deleting Alpha's M-3, want %q", key, parentKey, want)
		}
	}
}

// A macro row stored under its project's slug is the project's own (#741):
// renaming it still renames the parent of the tickets the project shows,
// rather than leaving them out as if another project held a macro of that key.
func TestRenamingAMacroStoredUnderItsProjectSlugRenamesTheParentOfItsTickets(t *testing.T) {
	d := testDB(t)
	alpha, err := d.CreateProject(models.CreateProjectRequest{Name: "Alpha", Slug: "alpha", IssueTracker: "jira", JiraProject: "GODE"})
	if err != nil {
		t.Fatal(err)
	}
	if alpha.Slug == "" || alpha.Slug == alpha.ID {
		t.Fatalf("the project needs a slug apart from its id: %+v", alpha)
	}
	if err := d.ImportOrUpdateTasks(alpha.DefaultTrackerID, []models.Task{{Key: "GODE-1", Title: "GODE-1", Status: models.StatusToClarify, Priority: models.PriorityMedium,
		Source: "jira", ParentKey: "M-5", ParentTitle: "Old", CreatedAt: time.Now(), UpdatedAt: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	horizon := "now"
	if _, err := d.SaveMacroMeta(alpha.ID, "M-5", &horizon, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec(`UPDATE macros SET project_id = ? WHERE project_id = ? AND key = 'M-5'`, alpha.Slug, alpha.ID); err != nil {
		t.Fatal(err)
	}

	renamed := "New"
	if _, err := d.UpdateMacro(context.Background(), alpha.ID, "M-5", &renamed, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	var parentTitle string
	if err := d.conn.QueryRow(`SELECT parent_title FROM tasks WHERE key = 'GODE-1'`).Scan(&parentTitle); err != nil {
		t.Fatal(err)
	}
	if parentTitle != "New" {
		t.Fatalf("GODE-1 has parent title %q after renaming its project's M-5, want %q", parentTitle, "New")
	}
}
