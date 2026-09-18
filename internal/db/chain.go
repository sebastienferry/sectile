package db

import (
	"fmt"
	"log"

	"tasks/internal/models"
)

// A run is the only thing that knows whether the work it was launched for
// actually happened, and it says so by moving the task's stage before it ends.
// Nothing used to read that back: an autonomous run that ended having moved
// nothing closed exactly like one that did the work, so a board that stopped
// advancing gave no reason anywhere, and a full chain run stopped after its
// first step even though its own contract is to run until the stop stage.
//
// handBackRun is where a finished run is compared to the stage it started from.
// It never invents a transition: the server has no way to tell correct work from
// a CLI that printed a refusal and exited zero, so what is missing is recorded
// rather than assumed.

// runOutcome is what was recorded about a run at launch, plus the skill it ran.
type runOutcome struct {
	RunLaunch
	Skill string
}

// runOutcomeOf reads back what the launcher recorded. A run older than these
// columns reads as empty, which disables every judgement below.
func (d *DB) runOutcomeOf(runID string) runOutcome {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out runOutcome
	_ = d.conn.QueryRow("SELECT skill_name, run_mode, launch_stage, chain_stop_stage FROM task_activities WHERE id = ?", runID).
		Scan(&out.Skill, &out.Mode, &out.Stage, &out.ChainStop)
	return out
}

// noteRun appends one sentence to a run's summary. Appending rather than
// replacing keeps the reason the process gave for ending, which is the other
// half of the story.
func (d *DB) noteRun(runID, note string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, _ = d.conn.Exec("UPDATE task_activities SET summary = CASE WHEN summary = '' THEN ? ELSE summary || ' — ' || ? END WHERE id = ?", note, note, runID)
}

// skillOwnsAStage says whether a skill is one whose whole point is to leave the
// task on the next stage. A skill that legitimately moves nothing, a story
// rewrite or a macro refinement, must not be reported as having failed to.
func skillOwnsAStage(skill string) bool {
	stageSkill, ok := StageSkillByID(skill)
	return ok && stageRank(stageSkill.ToStage) >= 0
}

// handBackRun records what a finished autonomous run handed back, and continues
// a full chain when there is something left to run. It is called once, when the
// run actually moves from running to finished.
func (d *DB) handBackRun(taskID, runID, status string) {
	out := d.runOutcomeOf(runID)
	// An interactive run is handed back by the user closing the session, and a
	// run whose launch predates these columns cannot be judged at all.
	if models.NormalizeSkillMode(out.Mode) != models.SkillModeAutonomous {
		return
	}
	task, err := d.GetTaskByID(taskID)
	if err != nil || task == nil {
		return
	}
	stage := d.StageOfTask(task)
	chained := out.ChainStop != ""

	if status != "completed" {
		if chained {
			d.noteRun(runID, fmt.Sprintf("the full chain stops here: the run ended as %s", status))
		}
		return
	}
	if out.Stage != "" && stage == out.Stage && skillOwnsAStage(out.Skill) {
		note := fmt.Sprintf("the run ended without moving the task, still at %q", stage)
		if chained {
			note += "; the full chain stops here"
		}
		d.noteRun(runID, note)
		return
	}
	if !chained {
		return
	}
	if stage == "finished" || stageAtOrPast(stage, out.ChainStop) {
		d.noteRun(runID, fmt.Sprintf("the full chain reached its stop stage %q", out.ChainStop))
		return
	}
	step, ok := NextStep(stage)
	if !ok {
		d.noteRun(runID, fmt.Sprintf("the full chain stops: no step follows stage %q", stage))
		return
	}
	if _, _, err := d.enqueueChainStep(task.ID, step.SkillID, out.ChainStop); err != nil {
		log.Printf("[chain] %s: could not enqueue %s from stage %s: %v", task.Key, step.SkillID, stage, err)
		d.noteRun(runID, "the full chain could not continue: "+err.Error())
	}
}

// enqueueChainStep runs the next step of a chain the way the first one was
// started: headless, whatever mode a single launch of that skill would resolve
// to, and still carrying the stage the chain stops at.
func (d *DB) enqueueChainStep(taskID, skillID, stopStage string) (*models.Task, *models.TaskActivity, error) {
	// A chained step carries no model override: the chain is launched once and
	// each step resolves the configured model, as it resolves its own stage.
	return d.enqueueSkillOnTask(taskID, skillID, "", true, models.SkillModeAutonomous, "", stopStage)
}
