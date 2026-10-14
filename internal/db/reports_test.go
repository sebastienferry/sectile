package db

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"tasks/internal/testsqlite"
	"testing"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

type reportRecorder struct {
	tracker.BaseTicketingSystem
	mu                       sync.Mutex
	uploads, comments, finds int
	uploadErr, commentErr    error
	findErr                  error
	attachment               *models.ReportAttachment
	summary                  string
	onUpload                 func()
}

func (r *reportRecorder) UploadReport(ctx context.Context, req tracker.AddCommentRequest, name string) (*models.ReportAttachment, error) {
	if r.onUpload != nil {
		r.onUpload()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.uploads++
	if r.uploadErr != nil {
		return nil, r.uploadErr
	}
	r.attachment = &models.ReportAttachment{ID: "attachment", Name: name, URL: "https://jira.example/attachment"}
	return r.attachment, nil
}
func (r *reportRecorder) FindReportAttachment(ctx context.Context, req tracker.AddCommentRequest, name string) (*models.ReportAttachment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.finds++
	return r.attachment, r.findErr
}
func (r *reportRecorder) AddReportSummary(ctx context.Context, req tracker.AddCommentRequest, marker string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.comments++
	r.summary = req.Body
	return "comment", r.commentErr
}

func reportFixture(t *testing.T, d *DB) (*models.Task, *reportRecorder) {
	t.Helper()
	p, err := d.CreateProject(models.CreateProjectRequest{Name: "Reports", IssueTracker: "local"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: p.ID, Title: "Report"})
	if err != nil {
		t.Fatal(err)
	}
	recorder := &reportRecorder{BaseTicketingSystem: tracker.BaseTicketingSystem{TrackerName: "local", Capabilities: []tracker.Capability{tracker.CapComment}}}
	d.TrackerRegistry().Register("local", recorder)
	return task, recorder
}

func checkReportRetentionAndRetry(t *testing.T, d *DB) {
	task, remote := reportFixture(t, d)
	body := "\r\n **bold `code`**\r\n 日本語  \n"
	meta := models.ReportMetadata{PublicationID: "round-one", Stage: "clarified", Round: 1, Summary: "Three decisions settled. One open question."}
	remote.commentErr = fmt.Errorf("comment refused")
	remote.onUpload = func() {
		reports, err := d.GetTaskReports(task.ID)
		if err != nil || len(reports) != 1 || reports[0].Body != body {
			t.Errorf("report not retained before upload: %#v %v", reports, err)
		}
	}
	receipt, err := d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, body, meta, nil, body)
	if err == nil || receipt.Attachment == nil || receipt.CommentError == "" || remote.uploads != 1 {
		t.Fatalf("comment failure: %#v %v", receipt, err)
	}
	remote.commentErr = nil
	remote.onUpload = nil
	// Clear the in-memory adapter and reuse a fresh one against durable receipts.
	next := &reportRecorder{BaseTicketingSystem: remote.BaseTicketingSystem}
	d.TrackerRegistry().Register("local", next)
	receipt, err = d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, body, meta, nil, body)
	if err != nil || next.uploads != 0 || next.comments != 1 || receipt.CommentStatus != "completed" || receipt.CommentError != "" {
		t.Fatalf("retry duplicated upload: %#v %v", receipt, err)
	}
	if !strings.Contains(next.summary, receipt.Attachment.Name) {
		t.Fatal("summary does not identify attachment")
	}
	if _, err = d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, body, meta, nil, body); err != nil || next.comments != 1 {
		t.Fatalf("completed retry: %v", err)
	}
	if _, err = d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, body+"changed", meta, nil, body); err == nil {
		t.Fatal("identity collision accepted")
	}
	meta.PublicationID = "round-two"
	meta.Round = 2
	if _, err = d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, body, meta, nil, body); err != nil {
		t.Fatal(err)
	}
	reports, err := d.GetTaskReports(task.ID)
	if err != nil || len(reports) != 2 || reports[0].Round != 1 || reports[1].Round != 2 {
		t.Fatalf("chronological rounds: %#v %v", reports, err)
	}
}
func TestReportRetentionAndRetrySQLite(t *testing.T) { checkReportRetentionAndRetry(t, testDB(t)) }
func TestReportRetentionAndRetryPostgres(t *testing.T) {
	checkReportRetentionAndRetry(t, openPostgres(t))
}

func TestReportUploadFailureIsWarning(t *testing.T) {
	for _, uploadErr := range []error{&trackerapi.HTTPError{Status: 403}, &trackerapi.HTTPError{Status: 413}, fmt.Errorf("timeout")} {
		t.Run(uploadErr.Error(), func(t *testing.T) {
			d := testDB(t)
			task, remote := reportFixture(t, d)
			remote.uploadErr = uploadErr
			meta := models.ReportMetadata{PublicationID: "spec", Stage: "specified", Summary: "Specification ready."}
			receipt, err := d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, "full **`report`**", meta, []string{"https://github.com/acme/app/pull/1"}, "")
			if err != nil || receipt.Warning == "" || receipt.CommentStatus != "completed" || remote.comments != 1 || !strings.Contains(remote.summary, "could not be attached") || !strings.Contains(remote.summary, "/pull/1") {
				t.Fatalf("fallback: %#v %v %s", receipt, err, remote.summary)
			}
			if _, err := d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, "full **`report`**", meta, []string{"https://github.com/acme/app/pull/1"}, ""); err != nil || remote.uploads != 1 {
				t.Fatal("warning retry repeated upload")
			}
		})
	}
}

func TestReportClaimAndUncertainReconciliation(t *testing.T) {
	d := testDB(t)
	task, remote := reportFixture(t, d)
	meta := models.ReportMetadata{PublicationID: "concurrent", Stage: "implemented", Summary: "Done."}
	entered := make(chan struct{})
	release := make(chan struct{})
	remote.onUpload = func() { close(entered); <-release }
	done := make(chan error, 1)
	go func() {
		_, err := d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, "body", meta, nil, "")
		done <- err
	}()
	<-entered
	if _, err := d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, "body", meta, nil, ""); err == nil {
		t.Fatal("concurrent publisher acquired active claim")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	reports, _ := d.GetTaskReports(task.ID)
	r := reports[0]
	r.Attachment = nil
	r.UploadStatus = "uploading"
	r.CommentStatus = "pending"
	raw, _ := json.Marshal(r)
	if _, err := d.conn.Exec("UPDATE task_reports SET payload=?,claim='expired',claim_until=? WHERE id=?", string(raw), time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano), r.ID); err != nil {
		t.Fatal(err)
	}
	remote.onUpload = nil
	receipt, err := d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, "body", meta, nil, "")
	if err != nil || receipt.Attachment == nil || remote.finds != 1 || remote.uploads != 1 {
		t.Fatalf("uncertain reconciliation: %#v %v", receipt, err)
	}
}

func TestReportStorageFailureStopsPublication(t *testing.T) {
	d := testDB(t)
	task, remote := reportFixture(t, d)
	if _, err := d.conn.Exec("DROP TABLE task_reports"); err != nil {
		t.Fatal(err)
	}
	_, err := d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, "body", models.ReportMetadata{PublicationID: "p", Stage: "specified", Summary: "Ready."}, nil, "")
	if err == nil || remote.uploads != 0 || remote.comments != 0 {
		t.Fatal("remote write attempted without retention")
	}
}

func TestStageReportIsRetainedAtomicallyWithOriginalBody(t *testing.T) {
	d := testDB(t)
	task, remote := reportFixture(t, d)
	body := "\n report **`code`** \r\n"
	meta := &models.ReportMetadata{PublicationID: "stage", Stage: "clarified", Round: 3, Summary: "Scope accepted."}
	updated, activity, err := d.TransitionTaskStage(task.ID, "clarified", body, "", "", meta)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != models.StatusClarified || activity == nil {
		t.Fatal("stage not recorded")
	}
	reports, err := d.GetTaskReports(task.ID)
	if err != nil || len(reports) != 1 || reports[0].Body != body || reports[0].Round != 3 {
		t.Fatalf("stage retention: %#v %v", reports, err)
	}
	if _, _, err := d.TransitionTaskStage(task.ID, "clarified", "changed", "", "", meta); err == nil {
		t.Fatal("conflicting stage publication accepted")
	}
	_ = remote
}

func TestManagedStageGuardRemainsWithReportMetadata(t *testing.T) {
	d := testDB(t)
	task, _ := reportFixture(t, d)
	if err := d.AddTaskActivity(models.TaskActivity{ID: "managed", TaskID: task.ID, SkillID: "clarify", Action: "Managed stage", Status: "running", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	_, _, err := d.TransitionTaskStage(task.ID, "clarified", "report", "", "", &models.ReportMetadata{PublicationID: "managed", Stage: "clarified", Summary: "Done."})
	if err == nil {
		t.Fatal("report metadata bypassed managed ownership guard")
	}
	reports, _ := d.GetTaskReports(task.ID)
	if len(reports) != 0 {
		t.Fatal("unvalidated managed report retained")
	}
}

func TestReportRetryAfterDatabaseRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart.db")
	d, err := testsqlite.New(t, path, NewDB)
	if err != nil {
		t.Fatal(err)
	}
	task, remote := reportFixture(t, d)
	meta := models.ReportMetadata{PublicationID: "restart", Stage: "specified", Summary: "Specification ready."}
	remote.commentErr = fmt.Errorf("comment refused")
	receipt, err := d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, "report", meta, nil, "")
	if err == nil || receipt.Attachment == nil {
		t.Fatalf("setup: %#v %v", receipt, err)
	}
	d.Close()
	reopened, err := NewDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	next := &reportRecorder{BaseTicketingSystem: remote.BaseTicketingSystem}
	reopened.TrackerRegistry().Register("local", next)
	receipt, err = reopened.PublishTaskReportAs(context.Background(), Actor{}, task.ID, "report", meta, nil, "")
	if err != nil || next.uploads != 0 || next.comments != 1 || receipt.Attachment.ID != "attachment" {
		t.Fatalf("restart duplicated upload: %#v %v", receipt, err)
	}
}

func TestUncertainAttachmentReadFailureStillPublishesSummary(t *testing.T) {
	d := testDB(t)
	task, remote := reportFixture(t, d)
	meta := models.ReportMetadata{PublicationID: "uncertain", Stage: "specified", Summary: "Ready."}
	r := models.TaskReport{ID: "retained", TaskID: task.ID, ReportMetadata: meta, Body: "body", CreatedAt: time.Now().UTC(), UploadStatus: "uploading", CommentStatus: "pending"}
	if trk := d.trackerOfTaskUnsafe(task); trk != nil {
		r.TrackerID = trk.ID
	}
	if _, err := retainReport(d.conn, r); err != nil {
		t.Fatal(err)
	}
	remote.findErr = fmt.Errorf("Jira metadata unavailable")
	receipt, err := d.PublishTaskReportAs(context.Background(), Actor{}, task.ID, "body", meta, nil, "")
	if err != nil || remote.uploads != 0 || remote.comments != 1 || receipt.Warning == "" || receipt.UploadStatus != "uncertain" {
		t.Fatalf("uncertain fallback: %#v %v", receipt, err)
	}
}
