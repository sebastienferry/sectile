package db

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"tasks/internal/models"
	"tasks/internal/tracker"
)

// These tests run what #407 is about: two server processes sharing one
// PostgreSQL database. Each opens two stores on the same DSN, so two instance
// ids and two connection pools, and races calls across both. DB.mu serialises
// nothing between them; only the database does.

// openPostgresPair opens a cleared store and a second one on the same database.
func openPostgresPair(t *testing.T) (*DB, *DB) {
	t.Helper()
	first := openPostgres(t)
	second, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatalf("opening a second store: %v", err)
	}
	t.Cleanup(func() { second.Close() })
	return first, second
}

// race runs n calls split across both stores, released together.
func race(n int, a, b *DB, call func(i int, d *DB)) {
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		d := a
		if i%2 == 1 {
			d = b
		}
		wg.Add(1)
		go func(i int, d *DB) {
			defer wg.Done()
			<-start
			call(i, d)
		}(i, d)
	}
	close(start)
	wg.Wait()
}

func seedRemoteRun(t *testing.T, d *DB, id string) {
	t.Helper()
	if _, err := d.conn.Exec(`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status, output, concurrent)
		VALUES (?, 't1', 'remote_run', 'clarify', ?, 'running', '', 1)`, id, RunActionAgent); err != nil {
		t.Fatalf("seeding run %s: %v", id, err)
	}
}

func TestPostgresConcurrentOutputAppendsKeepEveryChunk(t *testing.T) {
	a, b := openPostgresPair(t)
	seedProjectAndUser(t, a)
	seedTask(t, a)
	seedRemoteRun(t, a, "r1")

	const n = 40
	var failures atomic.Int32
	race(n, a, b, func(i int, d *DB) {
		if err := d.AppendRemoteRunOutput("t1", "r1", fmt.Sprintf("<chunk-%02d>", i)); err != nil {
			failures.Add(1)
		}
	})
	if failures.Load() != 0 {
		t.Fatalf("%d appends failed", failures.Load())
	}
	run, _ := a.GetActivityByID("r1")
	for i := 0; i < n; i++ {
		if c := strings.Count(run.Output, fmt.Sprintf("<chunk-%02d>", i)); c != 1 {
			t.Fatalf("chunk %d appears %d times in %q", i, c, run.Output)
		}
	}
}

func TestPostgresConcurrentOutputAppendsTruncateOnce(t *testing.T) {
	a, b := openPostgresPair(t)
	seedProjectAndUser(t, a)
	seedTask(t, a)
	seedRemoteRun(t, a, "r1")

	chunk := strings.Repeat("é", RemoteRunOutputLimit/10)
	race(24, a, b, func(i int, d *DB) {
		if err := d.AppendRemoteRunOutput("t1", "r1", chunk); err != nil {
			t.Errorf("append %d: %v", i, err)
		}
	})
	run, _ := a.GetActivityByID("r1")
	if c := strings.Count(run.Output, remoteRunOutputTruncated); c != 1 {
		t.Fatalf("the truncation notice appears %d times", c)
	}
	if got, want := len([]rune(run.Output)), RemoteRunOutputLimit+len([]rune(remoteRunOutputTruncated)); got != want {
		t.Fatalf("output holds %d characters, want %d", got, want)
	}
}

func TestPostgresConcurrentTransitionsKeepEveryLink(t *testing.T) {
	a, b := openPostgresPair(t)
	seedProjectAndUser(t, a)
	seedTask(t, a)

	const n = 12
	race(n, a, b, func(i int, d *DB) {
		url := fmt.Sprintf("https://github.com/o/r/pull/%d", i+1)
		if _, _, err := d.TransitionTaskStage("t1", "clarified", "", url, "feat/1"); err != nil {
			t.Errorf("transition %d: %v", i, err)
		}
	})
	task, _ := a.GetTaskByID("t1")
	if len(task.PrLinks) != n {
		t.Fatalf("the task keeps %d links, want %d: %v", len(task.PrLinks), n, task.PrLinks)
	}
	if d := a.StageOfTask(task); d != "clarified" {
		t.Fatalf("stage = %q, labels = %v", d, task.Labels)
	}
}

func TestPostgresTransitionsHonourTheRunningStageOnBothInstances(t *testing.T) {
	a, b := openPostgresPair(t)
	seedProjectAndUser(t, a)
	seedTask(t, a)
	if _, err := a.conn.Exec(`INSERT INTO task_activities (id, task_id, skill_id, skill_name, action, status)
		VALUES ('managed', 't1', 'clarify', 'clarify', 'Managed clarification', 'running')`); err != nil {
		t.Fatal(err)
	}
	var applied atomic.Int32
	race(8, a, b, func(i int, d *DB) {
		if _, _, err := d.TransitionTaskStage("t1", "clarified", "", "", ""); err == nil {
			applied.Add(1)
		}
	})
	if applied.Load() != 0 {
		t.Fatalf("%d transitions bypassed the running stage", applied.Load())
	}
}

func TestPostgresDiscoveryRacingTransitionsKeepsEveryLink(t *testing.T) {
	a, b := openPostgresPair(t)
	seedProjectAndUser(t, a)
	seedTask(t, a)
	if _, _, err := a.TransitionTaskStage("t1", "clarified", "", "https://github.com/o/r/pull/100", "feat/1"); err != nil {
		t.Fatal(err)
	}

	const n = 12
	race(n, a, b, func(i int, d *DB) {
		url := fmt.Sprintf("https://github.com/o/r/pull/%d", i+1)
		if i%4 < 2 {
			task, _ := d.GetTaskByID("t1")
			if _, _, err := d.applyDiscoveredPullRequests(task, []models.TaskPullRequest{{URL: url, Branch: "feat/1"}}); err != nil {
				t.Errorf("discovery %d: %v", i, err)
			}
			return
		}
		if _, _, err := d.TransitionTaskStage("t1", "clarified", "", url, "feat/1"); err != nil {
			t.Errorf("transition %d: %v", i, err)
		}
	})
	task, _ := a.GetTaskByID("t1")
	if len(task.PrLinks) != n+1 {
		t.Fatalf("the task keeps %d links, want %d: %v", len(task.PrLinks), n+1, task.PrLinks)
	}
}

func TestPostgresOneOrdinaryActiveRunPerTask(t *testing.T) {
	a, b := openPostgresPair(t)
	seedProjectAndUser(t, a)
	seedTask(t, a)

	var inserted, refused, concurrent atomic.Int32
	race(20, a, b, func(i int, d *DB) {
		act := models.TaskActivity{ID: uuid.NewString(), TaskID: "t1", SkillID: "clarify", Status: "queued", CreatedAt: time.Now(), Concurrent: i%4 == 3}
		err := d.AddTaskActivity(act)
		switch {
		case err == nil && act.Concurrent:
			concurrent.Add(1)
		case err == nil:
			inserted.Add(1)
		case isUniqueViolation(err, activeRunIndex):
			refused.Add(1)
		default:
			t.Errorf("insert %d: %v", i, err)
		}
	})
	if inserted.Load() != 1 || refused.Load() != 14 || concurrent.Load() != 5 {
		t.Fatalf("ordinary %d, refused %d, concurrent %d; want 1, 14, 5", inserted.Load(), refused.Load(), concurrent.Load())
	}
}

func TestPostgresProjectWorkerLockSpansInstances(t *testing.T) {
	a, b := openPostgresPair(t)
	previous := projectWorkerRetry
	projectWorkerRetry = 20 * time.Millisecond
	t.Cleanup(func() { projectWorkerRetry = previous })

	release, err := a.dialect.AcquireProjectWorker(a.conn, "p1")
	if err != nil {
		t.Fatal(err)
	}
	acquired := make(chan func(), 1)
	go func() {
		other, err := b.dialect.AcquireProjectWorker(b.conn, "p1")
		if err != nil {
			t.Error(err)
			return
		}
		acquired <- other
	}()

	// Another project never waits on this one.
	otherProject, err := b.dialect.AcquireProjectWorker(b.conn, "p2")
	if err != nil {
		t.Fatal(err)
	}
	otherProject()

	select {
	case <-acquired:
		t.Fatal("the second instance ran a job of the project while the first held it")
	case <-time.After(300 * time.Millisecond):
	}
	release()
	select {
	case other := <-acquired:
		other()
	case <-time.After(5 * time.Second):
		t.Fatal("the second instance never got the project after the release")
	}
}

func TestPostgresFinishedRunIsNotRevivedByAnotherInstance(t *testing.T) {
	a, b := openPostgresPair(t)
	seedProjectAndUser(t, a)
	seedTask(t, a)
	seedRemoteRun(t, a, "r1")
	if _, err := a.FinishRemoteRun("t1", "r1", "canceled", "stopped by the owner"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := b.SyncRemoteRunStatus("r1", "t1", "p1", "#1", "clarify", "running", "late report", &now); err != nil {
		t.Fatal(err)
	}
	run, _ := a.GetActivityByID("r1")
	if run.Status != "canceled" {
		t.Fatalf("the canceled run was revived: %q", run.Status)
	}
}

// convertTracker creates one issue per call, slowly enough for a second
// conversion to arrive while the first is still waiting on it.
type convertTracker struct {
	tracker.BaseTicketingSystem
	created atomic.Int32
}

func (c *convertTracker) CreateIssue(ctx context.Context, req tracker.CreateIssueRequest) (*models.Task, error) {
	n := c.created.Add(1)
	time.Sleep(150 * time.Millisecond)
	return &models.Task{Key: fmt.Sprintf("REM-%d", n)}, nil
}

func TestPostgresConcurrentConversionsCreateOneIssue(t *testing.T) {
	a, b := openPostgresPair(t)
	seedProjectAndUser(t, a)
	if _, err := a.conn.Exec(`INSERT INTO tasks (id, project_id, key, title, status, priority, source) VALUES ('t2','p1','P-1','T','backlog','medium','local')`); err != nil {
		t.Fatal(err)
	}
	fake := &convertTracker{BaseTicketingSystem: tracker.BaseTicketingSystem{TrackerName: "remote", Capabilities: []tracker.Capability{tracker.CapCreate}}}
	a.TrackerRegistry().Register("remote", fake)
	b.TrackerRegistry().Register("remote", fake)

	var converted atomic.Int32
	race(6, a, b, func(i int, d *DB) {
		if _, err := d.ConvertTaskToRemote(context.Background(), "t2", "remote"); err == nil {
			converted.Add(1)
		}
	})
	if fake.created.Load() != 1 || converted.Load() != 1 {
		t.Fatalf("issues created %d, conversions %d; want 1 and 1", fake.created.Load(), converted.Load())
	}
	task, _ := a.GetTaskByID("t2")
	if task.Key != "REM-1" || task.Source != "remote" {
		t.Fatalf("task = %s / %s", task.Key, task.Source)
	}
}

func TestPostgresConcurrentLocalTasksGetDistinctKeys(t *testing.T) {
	a, b := openPostgresPair(t)
	seedProjectAndUser(t, a)
	if _, err := a.conn.Exec(`UPDATE projects SET issue_tracker = 'local' WHERE id = 'p1'`); err != nil {
		t.Fatal(err)
	}
	const n = 16
	keys := make(chan string, n)
	race(n, a, b, func(i int, d *DB) {
		task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: "p1", Title: fmt.Sprintf("T%d", i), Source: "local"})
		if err != nil {
			t.Errorf("creation %d: %v", i, err)
			return
		}
		keys <- task.Key
	})
	close(keys)
	seen := map[string]bool{}
	for k := range keys {
		if seen[k] {
			t.Fatalf("key %s given twice", k)
		}
		seen[k] = true
	}
	if len(seen) != n {
		t.Fatalf("%d tasks created, want %d", len(seen), n)
	}
}

func TestPostgresConcurrentDefaultProjectsLeaveOne(t *testing.T) {
	a, b := openPostgresPair(t)
	race(8, a, b, func(i int, d *DB) {
		if _, err := d.CreateProject(models.CreateProjectRequest{Name: fmt.Sprintf("Default %d", i), IsDefault: true}); err != nil {
			t.Errorf("creation %d: %v", i, err)
		}
	})
	var defaults int
	if err := a.conn.QueryRow("SELECT COUNT(*) FROM projects WHERE is_default = 1").Scan(&defaults); err != nil {
		t.Fatal(err)
	}
	if defaults != 1 {
		t.Fatalf("%d default projects", defaults)
	}
}

func TestPostgresConcurrentProjectEditsKeepBothFields(t *testing.T) {
	a, b := openPostgresPair(t)
	seedProjectAndUser(t, a)
	for round := 0; round < 10; round++ {
		description, color := fmt.Sprintf("description %d", round), fmt.Sprintf("color-%d", round)
		race(2, a, b, func(i int, d *DB) {
			req := models.UpdateProjectRequest{Description: &description}
			if i == 1 {
				req = models.UpdateProjectRequest{Color: &color}
			}
			if _, err := d.UpdateProject("p1", req); err != nil {
				t.Errorf("round %d edit %d: %v", round, i, err)
			}
		})
		p, _ := a.GetProjectByID("p1")
		if p.Description != description || p.Color != color {
			t.Fatalf("round %d kept %q / %q", round, p.Description, p.Color)
		}
	}
}
