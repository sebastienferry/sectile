package db

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

// batchDB opens a local board whose macro M-1 carries the given slicing.
func batchDB(t *testing.T, todos []models.MacroTodo) (*DB, *models.Project) {
	t.Helper()
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "test.db"), NewDB)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	proj, err := database.CreateProject(models.CreateProjectRequest{Name: "Board", Slug: "board", IssueTracker: "local"})
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	if _, err := database.SaveMacroMeta(proj.ID, "M-1", nil, nil, nil, &todos); err != nil {
		t.Fatalf("save: %v", err)
	}
	return database, proj
}

func storedTodos(t *testing.T, database *DB, projectID string) map[string]models.MacroTodo {
	t.Helper()
	meta, err := database.findMacroMeta(projectID, "M-1")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]models.MacroTodo{}
	for _, todo := range meta.Todos {
		byID[todo.ID] = todo
	}
	return byID
}

func outcomeStatuses(batch *MacroStoryBatch) []string {
	out := make([]string, 0, len(batch.Results))
	for _, r := range batch.Results {
		out = append(out, r.TodoID+":"+r.Status)
	}
	return out
}

func TestBatchCreatesEveryLineInSlicingOrder(t *testing.T) {
	database, proj := batchDB(t, []models.MacroTodo{{ID: "a", Text: "First"}, {ID: "b", Text: "Second"}, {ID: "c", Text: "Third"}})

	batch, err := database.CreateStoriesFromMacroTodos(context.Background(), proj.ID, "M-1", []string{"c", "a", "b", "a"})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if got := strings.Join(outcomeStatuses(batch), ","); got != "a:created,b:created,c:created" {
		t.Fatalf("outcomes = %s", got)
	}
	if batch.Created != 3 || batch.Skipped != 0 || batch.Failed != 0 {
		t.Fatalf("counts = %d/%d/%d", batch.Created, batch.Skipped, batch.Failed)
	}
	stored := storedTodos(t, database, proj.ID)
	for _, r := range batch.Results {
		if r.Task == nil || r.StoryKey == "" || stored[r.TodoID].StoryKey != r.StoryKey {
			t.Errorf("line %s: outcome %+v, stored %+v", r.TodoID, r, stored[r.TodoID])
		}
		if r.Task != nil && (r.Task.ParentKey != "M-1" || r.Task.Title != stored[r.TodoID].Text) {
			t.Errorf("line %s: story %q under %q", r.TodoID, r.Task.Title, r.Task.ParentKey)
		}
	}
	for _, todo := range batch.Macro.Todos {
		if todo.StoryKey == "" {
			t.Errorf("the returned macro must carry every key, got %+v", todo)
		}
	}
}

func TestBatchSkipsAttachedLinesAndReportsFailuresWithoutStopping(t *testing.T) {
	database, proj := batchDB(t, []models.MacroTodo{
		{ID: "a", Text: "First"},
		{ID: "done", Text: "Already", StoryKey: "LOC-99"},
		{ID: "gone", Text: "Lost target", TargetProjectID: "gone-project"},
		{ID: "c", Text: "Last"},
	})

	batch, err := database.CreateStoriesFromMacroTodos(context.Background(), proj.ID, "M-1", []string{"a", "done", "gone", "c", "unknown"})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if got := strings.Join(outcomeStatuses(batch), ","); got != "a:created,done:skipped,gone:failed,c:created,unknown:failed" {
		t.Fatalf("outcomes = %s", got)
	}
	if batch.Created != 2 || batch.Skipped != 1 || batch.Failed != 2 {
		t.Fatalf("counts = %d/%d/%d", batch.Created, batch.Skipped, batch.Failed)
	}
	byID := map[string]MacroStoryOutcome{}
	for _, r := range batch.Results {
		byID[r.TodoID] = r
	}
	if byID["done"].StoryKey != "LOC-99" {
		t.Errorf("a skipped line names its key, got %+v", byID["done"])
	}
	if !strings.Contains(byID["gone"].Error, "le projet cible gone-project n'existe plus") {
		t.Errorf("a failed line gives the single-line reason, got %q", byID["gone"].Error)
	}
	if byID["unknown"].Error != "ligne de TODO introuvable" {
		t.Errorf("an unknown id is reported, got %q", byID["unknown"].Error)
	}
	stored := storedTodos(t, database, proj.ID)
	if stored["gone"].StoryKey != "" || stored["done"].StoryKey != "LOC-99" {
		t.Errorf("failed and skipped lines keep their key: %+v %+v", stored["gone"], stored["done"])
	}
	tasks, _ := database.GetTasks("", "", "", "", proj.ID, "", "", "", "", nil, nil, false)
	if len(tasks) != 2 {
		t.Errorf("two stories expected, got %d", len(tasks))
	}
}

func TestBatchSkipsARoadmapProjectLine(t *testing.T) {
	database, proj := batchDB(t, []models.MacroTodo{{ID: "a", Text: "Read only", StoryKey: "ROAD-1"}})
	declared := []string{"ROAD"}
	if _, err := database.UpdateProject(proj.ID, models.UpdateProjectRequest{RoadmapProjects: &declared}); err != nil {
		t.Fatal(err)
	}
	batch, err := database.CreateStoriesFromMacroTodos(context.Background(), proj.ID, "M-1", []string{"a"})
	if err != nil || batch.Skipped != 1 || batch.Results[0].StoryKey != "ROAD-1" {
		t.Fatalf("a roadmap project's line is skipped: %+v %v", batch, err)
	}
	// Alone, the same line is still refused as read-only.
	if _, _, _, err := database.CreateStoryFromMacroTodo(context.Background(), proj.ID, "M-1", "a"); err == nil || !strings.Contains(err.Error(), "projet de roadmap") {
		t.Fatalf("got %v", err)
	}
}

func TestBatchRelaunchSkipsEveryLine(t *testing.T) {
	database, proj := batchDB(t, []models.MacroTodo{{ID: "a", Text: "First"}, {ID: "b", Text: "Second"}})
	if _, err := database.CreateStoriesFromMacroTodos(context.Background(), proj.ID, "M-1", []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	batch, err := database.CreateStoriesFromMacroTodos(context.Background(), proj.ID, "M-1", []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Created != 0 || batch.Skipped != 2 {
		t.Fatalf("a relaunch creates nothing, got %+v", outcomeStatuses(batch))
	}
	tasks, _ := database.GetTasks("", "", "", "", proj.ID, "", "", "", "", nil, nil, false)
	if len(tasks) != 2 {
		t.Fatalf("two stories expected, got %d", len(tasks))
	}
}

func TestBatchWholeRefusalsCreateNothing(t *testing.T) {
	database, proj := batchDB(t, []models.MacroTodo{{ID: "a", Text: "First"}})
	cases := []struct {
		name, project, macro string
		ids                  []string
		want                 string
	}{
		{"empty selection", proj.ID, "M-1", []string{" ", ""}, "aucune ligne sélectionnée"},
		{"unknown project", "nope", "M-1", []string{"a"}, "projet non trouvé"},
		{"macro without shaping", proj.ID, "M-404", []string{"a"}, "sans cadrage enregistré"},
	}
	for _, c := range cases {
		batch, err := database.CreateStoriesFromMacroTodos(context.Background(), c.project, c.macro, c.ids)
		if err == nil || batch != nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %+v %v", c.name, batch, err)
		}
	}
	tasks, _ := database.GetTasks("", "", "", "", proj.ID, "", "", "", "", nil, nil, false)
	if len(tasks) != 0 {
		t.Fatalf("nothing may be created, got %d", len(tasks))
	}
}

// Two batches racing on one server create each line once.
func TestConcurrentBatchesCreateEachLineOnce(t *testing.T) {
	database, proj := batchDB(t, []models.MacroTodo{{ID: "a", Text: "First"}, {ID: "b", Text: "Second"}, {ID: "c", Text: "Third"}})
	var wg sync.WaitGroup
	created := make([]int, 2)
	for i := range created {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			batch, err := database.CreateStoriesFromMacroTodos(context.Background(), proj.ID, "M-1", []string{"a", "b", "c"})
			if err != nil {
				t.Errorf("batch %d: %v", i, err)
				return
			}
			created[i] = batch.Created
		}(i)
	}
	wg.Wait()
	if created[0]+created[1] != 3 {
		t.Fatalf("each line must be created once, got %v", created)
	}
	tasks, _ := database.GetTasks("", "", "", "", proj.ID, "", "", "", "", nil, nil, false)
	if len(tasks) != 3 {
		t.Fatalf("three stories expected, got %d", len(tasks))
	}
}

// A slicing saved by a tab loaded before the batch does not clear the keys the
// batch recorded, and a line it omits is still removed (FR7b).
func TestStaleSlicingSaveKeepsStoryKeys(t *testing.T) {
	stale := []models.MacroTodo{{ID: "a", Text: "First"}, {ID: "b", Text: "Second"}, {ID: "c", Text: "Third"}}
	database, proj := batchDB(t, stale)
	if _, err := database.CreateStoriesFromMacroTodos(context.Background(), proj.ID, "M-1", []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	keys := storedTodos(t, database, proj.ID)

	edited := []models.MacroTodo{{ID: "a", Text: "First, reworded"}, {ID: "c", Text: "Third"}}
	if _, err := database.SaveMacroMeta(proj.ID, "M-1", nil, nil, nil, &edited); err != nil {
		t.Fatal(err)
	}
	stored := storedTodos(t, database, proj.ID)
	if stored["a"].StoryKey != keys["a"].StoryKey || stored["a"].Text != "First, reworded" {
		t.Errorf("the edit and the key must both be kept, got %+v", stored["a"])
	}
	if _, ok := stored["b"]; ok {
		t.Errorf("a line the save omits is removed, got %+v", stored["b"])
	}
	if stored["c"].StoryKey != "" {
		t.Errorf("an unattached line stays unattached, got %+v", stored["c"])
	}
}

// Recording one key keeps an edit of another line saved in the meantime (FR7).
func TestRecordLineStoryKeyKeepsAConcurrentEdit(t *testing.T) {
	database, proj := batchDB(t, []models.MacroTodo{{ID: "a", Text: "First"}, {ID: "b", Text: "Second"}})
	edited := []models.MacroTodo{{ID: "a", Text: "First"}, {ID: "b", Text: "Second, edited", TargetProjectID: "other"}, {ID: "n", Text: "New"}}
	if _, err := database.SaveMacroMeta(proj.ID, "M-1", nil, nil, nil, &edited); err != nil {
		t.Fatal(err)
	}
	saved, found, err := database.recordLineStoryKey(proj.ID, "M-1", "a", "LOC-1")
	if err != nil || !found {
		t.Fatalf("record: %v %v", found, err)
	}
	if len(saved.Todos) != 3 || saved.Todos[0].StoryKey != "LOC-1" || saved.Todos[1].Text != "Second, edited" || saved.Todos[1].TargetProjectID != "other" || saved.Todos[1].StoryKey != "" {
		t.Fatalf("only line a's key may change, got %+v", saved.Todos)
	}
	if _, found, err := database.recordLineStoryKey(proj.ID, "M-1", "removed", "LOC-2"); err != nil || found {
		t.Fatalf("a removed line is reported gone: %v %v", found, err)
	}
}

// The single-line action keeps its refusal wording on an attached line.
func TestSingleLineRefusalOnAnAttachedLineIsUnchanged(t *testing.T) {
	database, proj := batchDB(t, []models.MacroTodo{{ID: "a", Text: "First", StoryKey: "LOC-7"}})
	_, _, _, err := database.CreateStoryFromMacroTodo(context.Background(), proj.ID, "M-1", "a")
	if err == nil || err.Error() != "cette ligne a déjà produit LOC-7" {
		t.Fatalf("got %v", err)
	}
}
