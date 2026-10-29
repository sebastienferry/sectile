package db

import (
	"errors"
	"fmt"
	"strings"

	"tasks/internal/models"
)

// ActiveRunsCountForProject returns the number of tasks with active runs or in-flight launches for a given project.
func (d *DB) ActiveRunsCountForProject(projectID string) (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	project, err := d.getProjectByIDUnsafe(projectID)
	if err != nil || project == nil {
		return 0, fmt.Errorf("project not found: %s", projectID)
	}

	return activeRunsCountForProjectOn(d.conn, project)
}

func activeRunsCountForProjectOn(q rowQuerier, project *models.Project) (int, error) {
	// A run counts through activeRunPredicate, which the catalog test keeps in
	// step with the skills. A launch still being handed to an agent is recorded
	// as agent_launch meanwhile, and holds its slot too.
	//
	// Only the orchestrator's own work counts (ADR 0063): its prompt prefix
	// travels with the queued launch, the agent_launch row, the remote run and
	// every chained step. A session somebody started by hand takes no slot; it
	// still keeps the orchestrator off its task, which is busy.
	query := `
  SELECT COUNT(DISTINCT a.task_id)
  FROM task_activities a
  WHERE (a.run_project_id IN (?, ?) OR (COALESCE(a.run_project_id, '') = '' AND a.task_id IN (SELECT id FROM tasks WHERE project_id IN (?, ?))))
    AND COALESCE(a.prompt, '') LIKE ?
    AND ((` + activeRunPredicate() + `) OR (skill_id = 'agent_launch' AND status IN (` + sqlStringList(activeRunStatuses) + `)))`
	var count int
	err := q.QueryRow(query, project.ID, project.Slug, project.ID, project.Slug, orchestratorPromptPrefix+"%").Scan(&count)
	return count, err
}

var (
	errOrchestratorCapacity = errors.New("max concurrency limit reached")
	// ErrOrchestratorProjectNotFound, ErrOrchestratorDisabled and
	// ErrOrchestratorGuidelinesTooLarge let a caller answer with the right status.
	ErrOrchestratorProjectNotFound    = errors.New("project not found")
	ErrOrchestratorDisabled           = errors.New("orchestrator is disabled for this project")
	ErrOrchestratorGuidelinesTooLarge = fmt.Errorf("orchestration guidelines exceed %d KiB", models.OrchestratorGuidelinesMaxBytes/1024)
)

// admitOrchestratedActivity checks capacity and inserts the queued reservation in
// one transaction. The project row lock serializes PostgreSQL server instances;
// SQLite takes its write lock at Begin. The cap is read from that locked row,
// never from the caller, so every orchestrator shares the project's own limit
// and a settings change applies to the next admission. The caller holds d.mu,
// and only queues the job after this transaction commits.
func (d *DB) admitOrchestratedActivity(act models.TaskActivity) error {
	return d.conn.WithTx(func(tx *sqlTx) error {
		project := &models.Project{}
		var maxConcurrent int
		if err := tx.QueryRow("SELECT id, slug, orchestrator_max_concurrent FROM projects WHERE id = ?"+d.dialect.ForUpdate(), act.RunProjectID).Scan(&project.ID, &project.Slug, &maxConcurrent); err != nil {
			return err
		}
		count, err := activeRunsCountForProjectOn(tx, project)
		if err != nil {
			return err
		}
		if count >= models.NormalizeOrchestratorMaxConcurrent(maxConcurrent) {
			return errOrchestratorCapacity
		}
		return insertTaskActivity(tx, d.instanceID, act)
	})
}

// orchestratorPromptPrefix opens every orchestrated prompt. The chain hand-back
// reads it to tell an orchestrated chain, whose context reaches every step, from
// a manual one, whose launch prompt belongs to its first step only. ADR 0063
// counts orchestrator slots by it.
const orchestratorPromptPrefix = "Backlog orchestration:"

// isOrchestratedPrompt says whether a run was launched by the orchestrator.
func isOrchestratedPrompt(prompt string) bool {
	return strings.HasPrefix(prompt, orchestratorPromptPrefix)
}
