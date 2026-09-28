package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tasks/internal/db"
	"tasks/internal/models"
)

// batchLaunchServer serves the launch route and the agent socket, so a batch
// launch can be accepted end to end.
func batchLaunchServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws/agent-connect" {
			h.HandleAgentConnect(w, r)
			return
		}
		h.HandleTaskDetail(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

func batchBody(ids ...string) string {
	quoted, _ := json.Marshal(ids)
	return `{"skillId":"pickup_issues","batchTaskIds":` + string(quoted) + `}`
}

// launchBatch posts a batch launch while a fake agent acknowledges the dispatch,
// and returns the answer.
func launchBatch(t *testing.T, server *httptest.Server, cookie *http.Cookie, key string, lead string, body string) (int, string) {
	t.Helper()
	conn := connectAgentAs(t, server, key, "default")
	type answer struct {
		status int
		body   string
	}
	answers := make(chan answer, 1)
	go func() {
		status, text := call(t, server, cookie, http.MethodPost, "/api/tasks/"+lead+"/run-skill", body)
		answers <- answer{status, text}
	}()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var dispatch AgentMessage
	if err := conn.ReadJSON(&dispatch); err != nil {
		t.Fatalf("the agent heard no dispatch: %v", err)
	}
	payload, _ := json.Marshal(map[string]string{"status": "completed", "summary": "Native CLI opened"})
	if err := conn.WriteJSON(AgentMessage{MsgID: dispatch.MsgID, TaskID: lead, Type: "step_status", Payload: payload}); err != nil {
		t.Fatal(err)
	}
	select {
	case a := <-answers:
		return a.status, a.body
	case <-time.After(5 * time.Second):
		t.Fatal("the launch was never answered")
	}
	return 0, ""
}

// An accepted batch records its members in launch order on the batch run, and
// the launch answers with the lead already showing its batch.
func TestBatchLaunchRecordsItsMembers(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := batchLaunchServer(t, h)
	aliceID, alice := account(t, database, "alice@example.com")
	key, _, err := database.CreateAPIKey(aliceID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	lead, second, third := guardTask(t, database, "Lead"), guardTask(t, database, "Second"), guardTask(t, database, "Third")

	status, body := launchBatch(t, server, alice, key, lead.ID, batchBody(lead.ID, second.ID, third.ID))
	if status != http.StatusOK {
		t.Fatalf("batch launch: %d %s", status, body)
	}
	var response models.RunSkillResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatal(err)
	}
	if response.Task.Batch == nil || response.Task.Batch.Position != 1 || response.Task.Batch.Size != 3 {
		t.Fatalf("the launch answered the lead without its batch: %+v", response.Task.Batch)
	}
	for i, member := range []*models.Task{lead, second, third} {
		batch, err := database.ActiveBatchOf(member.ID)
		if err != nil || batch == nil {
			t.Fatalf("%s is not in the batch: %v", member.Key, err)
		}
		want := models.BatchMemberWaiting
		if i == 0 {
			want = models.BatchMemberProcessing
		}
		if batch.Position != i+1 || batch.State != want || batch.LeadKey != lead.Key || batch.RunID != response.Task.Batch.RunID {
			t.Fatalf("batch of %s = %+v", member.Key, batch)
		}
	}
	// Only the lead carries runs: the other tickets have nothing recorded.
	if n := activityCount(t, database, second.ID); n != 0 {
		t.Fatalf("the batch recorded %d activities on %s", n, second.Key)
	}

	// While it runs, every member is busy, and the refusal names the batch.
	for _, member := range []*models.Task{second, third} {
		status, body := call(t, server, alice, http.MethodPost, "/api/tasks/"+member.ID+"/run-skill", `{"skillId":"clarify"}`)
		want := member.Key + " is part of the batch led by " + lead.Key + ", which is still running."
		if status != http.StatusConflict || !strings.Contains(body, want) || !strings.Contains(body, `"batchLeadKey":"`+lead.Key+`"`) {
			t.Fatalf("a launch on %s: %d %s", member.Key, status, body)
		}
		if n := activityCount(t, database, member.ID); n != 0 {
			t.Fatalf("the refusal recorded %d activities on %s", n, member.Key)
		}
	}
	// A member already done is still busy.
	if _, _, err := database.MarkBatchMemberProcessing(response.Task.Batch.RunID, third.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := database.MarkBatchMemberProcessing(response.Task.Batch.RunID, second.ID); err != nil {
		t.Fatal(err)
	}
	if status, body := call(t, server, alice, http.MethodPost, "/api/tasks/"+third.ID+"/run-skill", `{"skillId":"clarify"}`); status != http.StatusConflict {
		t.Fatalf("a launch on a done member: %d %s", status, body)
	}

	// Once the batch ends, its tickets are free again. The launch itself is
	// not made: the fake agent would leave it waiting for an acknowledgement.
	if _, err := database.FinishRemoteRun(lead.ID, response.Task.Batch.RunID, "canceled", "stopped"); err != nil {
		t.Fatal(err)
	}
	for _, member := range []*models.Task{second, third} {
		if active, batch, err := database.ActiveBusyCause(member.ID); err != nil || active != nil || batch != nil {
			t.Fatalf("%s is still busy once the batch ended: %+v %+v %v", member.Key, active, batch, err)
		}
	}
}

// A batch that breaks the rules of its shape is refused before anything is
// recorded.
func TestBatchLaunchRefusesAMalformedBatch(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := runSkillServer(t, h)
	_, alice := account(t, database, "alice@example.com")
	lead, second := guardTask(t, database, "Lead"), guardTask(t, database, "Second")
	project, err := database.CreateProject(models.CreateProjectRequest{Name: "Elsewhere"})
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := database.CreateTask(models.CreateTaskRequest{Title: "Foreign", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct{ name, body string }{
		{"a single ticket", batchBody(lead.ID)},
		{"a ticket twice", batchBody(lead.ID, second.ID, lead.ID)},
		{"another project", batchBody(lead.ID, foreign.ID)},
		{"another first ticket", batchBody(second.ID, lead.ID)},
		{"an unknown ticket", batchBody(lead.ID, "nope")},
		{"another skill", `{"skillId":"clarify","batchTaskIds":["` + lead.ID + `","` + second.ID + `"]}`},
		{"launch anyway", `{"skillId":"pickup_issues","force":true,"batchTaskIds":["` + lead.ID + `","` + second.ID + `"]}`},
	} {
		status, body := call(t, server, alice, http.MethodPost, "/api/tasks/"+lead.ID+"/run-skill", c.body)
		if status != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", c.name, status, body)
		}
	}
	for _, task := range []*models.Task{lead, second, foreign} {
		if n := activityCount(t, database, task.ID); n != 0 {
			t.Fatalf("a refused batch recorded %d activities on %s", n, task.Key)
		}
		if batch, _ := database.ActiveBatchOf(task.ID); batch != nil {
			t.Fatalf("a refused batch recorded %s as a member", task.Key)
		}
	}
}

// A batch cannot take in a busy ticket: the refusal names the first one and
// what makes it busy, and nothing is recorded on any ticket.
func TestBatchLaunchRefusesABusyMember(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := runSkillServer(t, h)
	_, alice := account(t, database, "alice@example.com")
	lead, busy, other := guardTask(t, database, "Lead"), guardTask(t, database, "Busy"), guardTask(t, database, "Other")
	run, err := database.StartAgentRun(busy.ID, "implement", db.RunLaunch{})
	if err != nil {
		t.Fatal(err)
	}

	status, body := call(t, server, alice, http.MethodPost, "/api/tasks/"+lead.ID+"/run-skill", batchBody(lead.ID, busy.ID, other.ID))
	if status != http.StatusConflict || !strings.Contains(body, busy.Key+" cannot join the batch") || !strings.Contains(body, `"activeRunId":"`+run.ID+`"`) {
		t.Fatalf("a batch with a busy ticket: %d %s", status, body)
	}
	for _, task := range []*models.Task{lead, other} {
		if n := activityCount(t, database, task.ID); n != 0 {
			t.Fatalf("the refused batch recorded %d activities on %s", n, task.Key)
		}
	}
	if batch, _ := database.ActiveBatchOf(lead.ID); batch != nil {
		t.Fatal("the refused batch recorded its members")
	}

	// A ticket of another running batch is busy the same way.
	if _, err := database.FinishRemoteRun(busy.ID, run.ID, "completed", "done"); err != nil {
		t.Fatal(err)
	}
	batchRun, err := database.StartAgentRun(other.ID, "pickup_issues", db.RunLaunch{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RecordBatch(batchRun.ID, []string{other.ID, busy.ID}); err != nil {
		t.Fatal(err)
	}
	status, body = call(t, server, alice, http.MethodPost, "/api/tasks/"+lead.ID+"/run-skill", batchBody(lead.ID, busy.ID))
	if status != http.StatusConflict || !strings.Contains(body, busy.Key+" is part of the batch led by "+other.Key) {
		t.Fatalf("a batch with a ticket of another batch: %d %s", status, body)
	}
}

// "Launch anyway" on a member follows the rule of any busy ticket: the batch
// run's owner or an admin, never a third party, and the batch is untouched.
func TestLaunchAnywayOnABatchMember(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := runSkillServer(t, h)
	// The first account is the deployment's admin, the others are members.
	_, alice := account(t, database, "alice@example.com")
	bobID, bob := account(t, database, "bob@example.com")
	_, carol := account(t, database, "carol@example.com")
	lead, member := guardTask(t, database, "Lead"), guardTask(t, database, "Member")
	run, err := database.StartAgentRun(lead.ID, "pickup_issues", db.RunLaunch{UserID: bobID})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RecordBatch(run.ID, []string{lead.ID, member.ID}); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name   string
		cookie *http.Cookie
		status int
		expect string
	}{
		// No agent is connected, so getting past the guard ends on that refusal.
		{"the owner forces through", bob, http.StatusConflict, "Connect the local agent"},
		{"an admin forces through", alice, http.StatusConflict, "Connect the local agent"},
		{"a third party is refused", carol, http.StatusForbidden, msgNotOwner},
	} {
		status, body := call(t, server, c.cookie, http.MethodPost, "/api/tasks/"+member.ID+"/run-skill", `{"skillId":"clarify","force":true}`)
		if status != c.status || !strings.Contains(body, c.expect) {
			t.Fatalf("%s: %d %s", c.name, status, body)
		}
	}
	batch, err := database.ActiveBatchOf(member.ID)
	if err != nil || batch == nil || batch.State != models.BatchMemberWaiting {
		t.Fatalf("forcing disturbed the batch: %+v %v", batch, err)
	}
}
