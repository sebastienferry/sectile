package db

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"tasks/internal/models"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

var reportFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func reportFilename(taskKey string, r models.TaskReport) string {
	stage := map[string]string{"clarified": "clarification", "specified": "specification", "implemented": "implementation", "reviewed": "review", "finished": "handoff"}[r.Stage]
	if r.Round > 0 {
		stage += fmt.Sprintf("-round-%d", r.Round)
	}
	suffix := fmt.Sprintf("%x", sha256.Sum256([]byte(r.ID)))[:16]
	return strings.Trim(reportFilenameChars.ReplaceAllString(taskKey, "-"), "-") + "-" + stage + "-" + suffix + ".md"
}

// GetTaskReports reads retained reports independently of tracker comments.
// Callers use the same task access boundary as the task detail API.
func (d *DB) GetTaskReports(taskID string) ([]models.TaskReport, error) {
	task, err := d.GetTaskByID(taskID)
	if err != nil || task == nil {
		return nil, fmt.Errorf("tâche non trouvée")
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	rows, err := d.conn.Query("SELECT payload FROM task_reports WHERE task_id = ? ORDER BY created_at, id", task.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reports := []models.TaskReport{}
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var r models.TaskReport
		if err := json.Unmarshal([]byte(payload), &r); err != nil {
			return nil, err
		}
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

func sameReport(a, b models.TaskReport) bool {
	return a.TaskID == b.TaskID && a.TrackerID == b.TrackerID && a.ActorID == b.ActorID && a.ReportMetadata == b.ReportMetadata && a.Body == b.Body && strings.Join(a.PRURLs, "\n") == strings.Join(b.PRURLs, "\n")
}

type reportStore interface {
	rowQuerier
	activityExecutor
}

func retainReport(store reportStore, r models.TaskReport) (models.TaskReport, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	_, err = store.Exec(`INSERT INTO task_reports(id,task_id,tracker_id,publication_id,payload,created_at) VALUES(?,?,?,?,?,?) ON CONFLICT(task_id,tracker_id,publication_id) DO NOTHING`, r.ID, r.TaskID, r.TrackerID, r.PublicationID, string(raw), r.CreatedAt.Format(time.RFC3339Nano))
	if err != nil {
		return r, err
	}
	var saved string
	if err = store.QueryRow("SELECT payload FROM task_reports WHERE task_id=? AND tracker_id=? AND publication_id=?", r.TaskID, r.TrackerID, r.PublicationID).Scan(&saved); err != nil {
		return r, err
	}
	var retained models.TaskReport
	if err = json.Unmarshal([]byte(saved), &retained); err != nil {
		return r, err
	}
	if !sameReport(r, retained) {
		return r, fmt.Errorf("identifiant de publication déjà utilisé pour un autre rapport")
	}
	return retained, nil
}

// PublishTaskReportAs retains the exact body before any remote write. A claim
// serializes writers across processes; expired claims reconcile uncertain steps.
func (d *DB) PublishTaskReportAs(ctx context.Context, actor Actor, taskID, body string, meta models.ReportMetadata, prURLs []string, markdownBody string) (*models.TaskReport, error) {
	ctx, cancel := context.WithTimeout(ctx, commentsTimeout)
	defer cancel()
	if err := meta.Validate(""); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body) == "" {
		return nil, fmt.Errorf("rapport vide")
	}
	task, err := d.GetTaskByID(taskID)
	if err != nil || task == nil {
		return nil, fmt.Errorf("tâche non trouvée")
	}
	trk := d.trackerOfTaskUnsafe(task)
	trackerID := ""
	if trk != nil {
		trackerID = trk.ID
	}
	r := models.TaskReport{ID: uuid.NewString(), TaskID: task.ID, TrackerID: trackerID, ActorID: actor.ID, ReportMetadata: meta, Body: body, PRURLs: prURLs, CreatedAt: time.Now().UTC(), UploadStatus: "pending", CommentStatus: "pending"}
	d.mu.Lock()
	r, err = retainReport(d.conn, r)
	if err != nil {
		d.mu.Unlock()
		return nil, err
	}
	if r.CommentStatus == "completed" {
		d.mu.Unlock()
		return &r, nil
	}
	claim := uuid.NewString()
	now := time.Now().UTC()
	res, err := d.conn.Exec("UPDATE task_reports SET claim=?,claim_until=? WHERE id=? AND (claim='' OR claim_until < ?)", claim, now.Add(5*time.Minute).Format(time.RFC3339Nano), r.ID, now.Format(time.RFC3339Nano))
	d.mu.Unlock()
	if err != nil {
		return &r, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return &r, err
	}
	if n != 1 {
		return &r, fmt.Errorf("publication du rapport en cours ; réessayez après sa fin")
	}
	defer func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		_, _ = d.conn.Exec("UPDATE task_reports SET claim='',claim_until='' WHERE id=? AND claim=?", r.ID, claim)
	}()
	// The payload may have changed between the first read and acquiring the claim.
	d.mu.RLock()
	var fresh string
	err = d.conn.QueryRow("SELECT payload FROM task_reports WHERE id=?", r.ID).Scan(&fresh)
	d.mu.RUnlock()
	if err != nil {
		return &r, err
	}
	if err = json.Unmarshal([]byte(fresh), &r); err != nil {
		return &r, err
	}
	if r.CommentStatus == "completed" {
		return &r, nil
	}
	save := func() error {
		payload, err := json.Marshal(r)
		if err != nil {
			return err
		}
		d.mu.Lock()
		defer d.mu.Unlock()
		res, err := d.conn.Exec("UPDATE task_reports SET payload=? WHERE id=? AND claim=?", string(payload), r.ID, claim)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("publication du rapport non détenue")
		}
		return nil
	}
	ctx = tracker.WithTracker(tracker.WithActingUser(ctx, actor.ID), trk)
	ts, err := d.TrackerForTask(task)
	if err != nil {
		return &r, err
	}
	req := tracker.AddCommentRequest{Tracker: trk, Key: task.Key, Body: body}
	if writer, ok := ts.(tracker.ReportWriter); ok {
		filename := reportFilename(task.Key, r)
		if r.UploadStatus == "uploading" || r.UploadStatus == "uncertain" {
			a, findErr := writer.FindReportAttachment(ctx, req, filename)
			if findErr != nil {
				r.UploadStatus = "uncertain"
				r.Warning = fmt.Sprintf("Résultat d'envoi incertain : %v", findErr)
				if err := save(); err != nil {
					return &r, err
				}
			}
			if a == nil && findErr == nil {
				r.UploadStatus = "uncertain"
				r.Warning = "Résultat de l'envoi incertain ; vérifiez les pièces jointes Jira avant une nouvelle publication."
				if err := save(); err != nil {
					return &r, err
				}
			}
			if a != nil {
				r.Attachment = a
				r.UploadStatus = "completed"
				r.Warning = ""
				if err := save(); err != nil {
					return &r, err
				}
			}
		}
		if r.UploadStatus == "pending" {
			r.UploadStatus = "uploading"
			if err := save(); err != nil {
				return &r, err
			}
			a, uploadErr := writer.UploadReport(ctx, req, filename)
			if uploadErr != nil {
				var httpErr *trackerapi.HTTPError
				if errors.As(uploadErr, &httpErr) && httpErr.Status >= 400 && httpErr.Status < 500 {
					r.UploadStatus = "failed"
				} else {
					r.UploadStatus = "uncertain"
				}
				r.Warning = uploadErr.Error()
			} else {
				r.Attachment = a
				r.UploadStatus = "completed"
			}
			if err := save(); err != nil {
				return &r, err
			}
		}
		summary := meta.Summary
		if r.Attachment != nil {
			summary += "\n\nRapport complet : " + r.Attachment.Name + "\n" + r.Attachment.URL
		} else {
			summary += "\n\nLe rapport complet n'a pas pu être joint. Il reste disponible dans l'historique Sectile."
		}
		for _, link := range prURLs {
			if link != "" {
				summary += "\n" + link
			}
		}
		req.Body = summary
		r.CommentStatus = "publishing"
		if err := save(); err != nil {
			return &r, err
		}
		r.CommentID, err = writer.AddReportSummary(ctx, req, "sectile.report."+r.ID)
	} else {
		req.Body = markdownBody
		if req.Body == "" {
			req.Body = body
		}
		err = ts.AddComment(ctx, req)
		r.UploadStatus = "unsupported"
	}
	if err != nil {
		r.CommentStatus = "failed"
		saveErr := save()
		if saveErr != nil {
			return &r, saveErr
		}
		return &r, err
	}
	r.CommentStatus = "completed"
	if err := save(); err != nil {
		return &r, err
	}
	return &r, nil
}
