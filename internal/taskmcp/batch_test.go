package taskmcp

import (
	"path/filepath"
	"testing"

	"tasks/internal/db"
	"tasks/internal/models"
)

// A batch agent reports each ticket it starts on with start_run and the batch
// runId (#522): the batch run comes back, the ticket takes the processing mark,
// and finish_run on the lead ends the batch for every ticket.
func TestABatchRunIsReportedOnEachMemberOverMCP(t *testing.T) {
	database, err := db.NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var tasks []*models.Task
	for _, title := range []string{"Lead", "Second"} {
		task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: title, Source: "local"})
		if err != nil {
			t.Fatal(err)
		}
		tasks = append(tasks, task)
	}
	lead, second := tasks[0], tasks[1]
	run, err := database.StartAgentRun(lead.ID, "pickup_issues", db.RunLaunch{UserID: tester.UserID})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RecordBatch(run.ID, []string{lead.ID, second.ID}); err != nil {
		t.Fatal(err)
	}

	started, err := call(t, database, "start_run", map[string]any{"taskKey": second.Key, "skill": "pickup_issues", "runId": run.ID})
	if err != nil {
		t.Fatalf("start_run on the second ticket with the batch runId: %v", err)
	}
	if started["id"] != run.ID {
		t.Fatalf("start_run returned run %v, want the batch run %s", started["id"], run.ID)
	}
	if batch, _ := database.ActiveBatchOf(second.ID); batch == nil || batch.State != models.BatchMemberProcessing {
		t.Fatalf("the second ticket did not take the mark: %+v", batch)
	}
	if activities, _ := database.GetTaskActivities(second.ID); len(activities) != 0 {
		t.Fatalf("start_run recorded %d activities on the second ticket", len(activities))
	}

	if _, err := call(t, database, "finish_run", map[string]any{"taskKey": lead.Key, "runId": run.ID, "status": "completed", "note": "batch done"}); err != nil {
		t.Fatalf("finish_run on the lead: %v", err)
	}
	for _, task := range tasks {
		if batch, _ := database.ActiveBatchOf(task.ID); batch != nil {
			t.Fatalf("%s still shows the batch once it ended: %+v", task.Key, batch)
		}
	}
}
