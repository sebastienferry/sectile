package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
)

// A macro skill run is a remote run that belongs to a macro rather than to a
// task. It is stored as a project activity (project_id set, task_id NULL) and
// names its macro in macro_key: a pseudo task identifier would bring back the
// overloading #310 removed from task_id, and break its foreign key on
// PostgreSQL. It has no stage, so it neither checks nor moves one, and its end
// hands nothing back to a workflow chain.

// macroRunningIndex is the partial unique index that allows one running run
// per macro (migration 11), across server instances.
const macroRunningIndex = "idx_task_activities_macro_running"

// ErrMacroRunBusy refuses a second launch on a macro that is already running
// one. Two realignments of one specification would write in the same
// worktree at the same time.
var ErrMacroRunBusy = fmt.Errorf("une exécution est déjà en cours sur cette macro")

// macroOf returns the project and the stored macro a key names, or an error
// that says which of the two is missing.
func (d *DB) macroOf(projectID, macroKey string) (*models.Project, *models.MacroMeta, error) {
	project, err := d.GetProjectByID(strings.TrimSpace(projectID))
	if err != nil {
		return nil, nil, err
	}
	if project == nil {
		return nil, nil, fmt.Errorf("project not found")
	}
	macros, err := d.GetProjectMacros(project.ID)
	if err != nil {
		return nil, nil, err
	}
	key := strings.TrimSpace(macroKey)
	for i := range macros {
		if strings.EqualFold(macros[i].Key, key) {
			return project, &macros[i], nil
		}
	}
	return nil, nil, fmt.Errorf("macro %s not found in project %s", key, project.Name)
}

// StartMacroRun records a macro skill run an agent is being dispatched for.
func (d *DB) StartMacroRun(projectID, macroKey, skill string, launch RunLaunch) (*models.TaskActivity, error) {
	return d.startMacroRun(projectID, macroKey, skill, "", true, launch)
}

// StartMacroRunBy records a macro skill run a connected client reports, or
// returns the run it reuses through runID.
func (d *DB) StartMacroRunBy(userID, projectID, macroKey, skill, runID string) (*models.TaskActivity, error) {
	return d.startMacroRun(projectID, macroKey, skill, runID, false, RunLaunch{UserID: userID})
}

func (d *DB) startMacroRun(projectID, macroKey, skill, runID string, agentOwned bool, launch RunLaunch) (*models.TaskActivity, error) {
	project, macro, err := d.macroOf(projectID, macroKey)
	if err != nil {
		return nil, err
	}
	if runID = strings.TrimSpace(runID); runID != "" {
		activity, err := d.GetActivityByID(runID)
		if err != nil {
			return nil, err
		}
		if activity == nil || activity.ProjectID != project.ID || activity.TaskID != "" || activity.SkillID != "remote_run" ||
			!strings.EqualFold(d.macroKeyOfActivity(runID), macro.Key) {
			return nil, adoptionRefusal(nil, "macro")
		}
		adopted, err := d.adoptRun(activity, "macro")
		if err != nil {
			return nil, err
		}
		adopted.MacroKey = macro.Key
		return adopted, nil
	}
	if strings.TrimSpace(skill) == "" {
		return nil, fmt.Errorf("skill is required")
	}
	now := time.Now()
	activity := &models.TaskActivity{ID: uuid.NewString(), ProjectID: project.ID,
		SkillID: "remote_run", SkillName: skill, Action: RunActionClient,
		Status: "running", Summary: "Execution reported by a local agent or native client",
		CreatedAt: now, StartedAt: &now, Steps: []string{}, UserID: strings.TrimSpace(launch.UserID),
		Provider: strings.TrimSpace(launch.Provider), Model: strings.TrimSpace(launch.Model), MacroKey: macro.Key}
	if agentOwned {
		activity.Action = RunActionAgent
	}
	// The busy check and the record are one step: two launches in the same
	// instant would otherwise both find the macro free and write in the same
	// worktree. Across server instances the partial unique index of migration 11
	// refuses the second one. The insert names the columns every activity has,
	// the macro and the mode follow in the same transaction, so a run is never
	// left without the macro that finds it.
	d.mu.Lock()
	defer d.mu.Unlock()
	var busy int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM task_activities
		WHERE project_id = ? AND task_id IS NULL AND skill_id = 'remote_run' AND status = 'running' AND UPPER(macro_key) = UPPER(?)`,
		project.ID, macro.Key).Scan(&busy); err != nil {
		return nil, err
	}
	if busy > 0 {
		return nil, ErrMacroRunBusy
	}
	tx, err := d.conn.Begin()
	if err != nil {
		return nil, err
	}
	if err := insertTaskActivity(tx, d.instanceID, *activity); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if _, err := tx.Exec("UPDATE task_activities SET macro_key=?, run_mode=? WHERE id=?",
		macro.Key, models.NormalizeSkillMode(launch.Mode), activity.ID); err != nil {
		_ = tx.Rollback()
		if isUniqueViolation(err, macroRunningIndex) {
			return nil, ErrMacroRunBusy
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return activity, nil
}

// macroKeyOfActivity reads the macro an activity belongs to, "" for any other.
func (d *DB) macroKeyOfActivity(id string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var key string
	_ = d.conn.QueryRow("SELECT macro_key FROM task_activities WHERE id = ?", id).Scan(&key)
	return key
}

// ActiveRunOnMacro reports the run that makes a macro busy, or nil.
func (d *DB) ActiveRunOnMacro(projectID, macroKey string) (*models.TaskActivity, error) {
	runs, err := d.macroRuns(projectID, macroKey, true, 1)
	if err != nil || len(runs) == 0 {
		return nil, err
	}
	return &runs[0], nil
}

// MacroRuns lists a macro's skill runs, most recent first.
func (d *DB) MacroRuns(projectID, macroKey string, limit int) ([]models.TaskActivity, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	return d.macroRuns(projectID, macroKey, false, limit)
}

func (d *DB) macroRuns(projectID, macroKey string, runningOnly bool, limit int) ([]models.TaskActivity, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	query := `SELECT id, project_id, macro_key, skill_name, action, status, summary, created_at, started_at, completed_at, user_id, run_provider, run_model
		FROM task_activities
		WHERE project_id = ? AND task_id IS NULL AND skill_id = 'remote_run' AND macro_key <> '' AND UPPER(macro_key) = UPPER(?)`
	if runningOnly {
		query += " AND status = 'running'"
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	rows, err := d.conn.Query(query, projectID, strings.TrimSpace(macroKey), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []models.TaskActivity{}
	for rows.Next() {
		var a models.TaskActivity
		var startedAt, completedAt sql.NullTime
		var provider, model sql.NullString
		if err := rows.Scan(&a.ID, &a.ProjectID, &a.MacroKey, &a.SkillName, &a.Action, &a.Status, &a.Summary, &a.CreatedAt,
			&startedAt, &completedAt, &a.UserID, &provider, &model); err != nil {
			return nil, err
		}
		a.SkillID, a.Steps = "remote_run", []string{}
		if startedAt.Valid {
			a.StartedAt = &startedAt.Time
		}
		if completedAt.Valid {
			a.CompletedAt = &completedAt.Time
		}
		a.Provider, a.Model = provider.String, model.String
		runs = append(runs, a)
	}
	return runs, rows.Err()
}

// FinishMacroRunAs closes a macro skill run, under the same ownership rule as
// a task run: its owner, an admin, or anyone for a run with no owner.
func (d *DB) FinishMacroRunAs(caller Actor, admin bool, projectID, macroKey, runID, status, note string) (*models.TaskActivity, error) {
	if status != "completed" && status != "failed" && status != "canceled" {
		return nil, fmt.Errorf("status must be completed, failed or canceled")
	}
	project, macro, err := d.macroOf(projectID, macroKey)
	if err != nil {
		return nil, err
	}
	existing, err := d.GetActivityByID(runID)
	if err != nil {
		return nil, err
	}
	if existing == nil || existing.ProjectID != project.ID || existing.TaskID != "" || existing.SkillID != "remote_run" ||
		!strings.EqualFold(d.macroKeyOfActivity(runID), macro.Key) {
		return nil, fmt.Errorf("remote run not found on this macro")
	}
	if !admin && existing.UserID != "" && existing.UserID != caller.ID {
		return nil, ErrRunNotYours
	}
	summary := note
	if strings.Contains(existing.Summary, models.RunSilencePrefix) {
		summary = existing.Summary + " - " + note
	}
	// As for a task run, a cancellation the server decided on a disconnection
	// stays correctable by the identified owner, and only by them: the
	// server's own closure names nobody and must never rewrite an outcome it
	// just recorded.
	// A queued run is closable as well, as for a task run (#499).
	closable := "status IN ('running', 'queued')"
	args := []any{status, summary, time.Now(), runID, project.ID}
	if strings.TrimSpace(caller.ID) != "" {
		closable = "(status IN ('running', 'queued') OR (status='canceled' AND summary LIKE ?))"
		args = append(args, "%"+models.RunDisconnectNote+"%")
	}
	d.mu.Lock()
	result, err := d.conn.Exec(`UPDATE task_activities SET status=?, summary=?, completed_at=?, waiting_since=NULL, waiting_session=''
		WHERE id=? AND project_id=? AND task_id IS NULL AND skill_id='remote_run' AND `+closable, args...)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	activity, err := d.GetActivityByID(runID)
	if err != nil {
		return nil, err
	}
	if count == 0 && (activity == nil || activity.Status != status) {
		return nil, fmt.Errorf("remote run not found or already finished with another status")
	}
	activity.MacroKey = macro.Key
	return activity, nil
}

// PrepareMacroWorktree asks the caller's local agent to prepare a macro's
// specification checkout. The worktree lives on the agent's machine, next to
// the checkouts it maps; the server only knows which macro to name.
func (d *DB) PrepareMacroWorktree(ctx context.Context, userID, projectID, macroKey string) (*models.MacroWorkspace, error) {
	project, macro, err := d.macroOf(projectID, macroKey)
	if err != nil {
		return nil, err
	}
	var workspace models.MacroWorkspace
	err = d.callAgentContext(ctx, agentprotocol.Operation{UserID: strings.TrimSpace(userID), ProjectID: project.ID,
		Action: "macro_worktree", MacroKey: macro.Key, MacroTitle: macro.Title}, &workspace)
	if err != nil {
		return nil, err
	}
	// The slicing travels with the checkout, so a skill invoked by hand, which
	// holds no API token, reads its input from the same answer.
	workspace.ProjectID, workspace.MacroKey, workspace.Todos = project.ID, macro.Key, macro.Todos
	return &workspace, nil
}
