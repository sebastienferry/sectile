package db

import (
	"errors"
	"fmt"
	"strings"

	"tasks/internal/models"
)

// RunProjectCandidate is one project a run on a ticket could work for.
type RunProjectCandidate struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ErrRunProjectAmbiguous refuses a run on a ticket of several projects that
// names none of them (#741). An interactive launch asks its user which one;
// an unattended one (autonomous, a batch, a pickup) is refused outright, the
// candidates listed.
type ErrRunProjectAmbiguous struct {
	Candidates []RunProjectCandidate
	Unattended bool
}

func (e *ErrRunProjectAmbiguous) Error() string {
	if e.Unattended {
		return fmt.Sprintf("lancement automatique refusé : ce ticket appartient à plusieurs projets (%s) ; lancez-le depuis le board de l'un d'eux", e.CandidateNames())
	}
	return fmt.Sprintf("ce ticket appartient à plusieurs projets (%s) : choisissez celui pour lequel le lancer", e.CandidateNames())
}

// CandidateNames lists the candidates as "name (id)", comma separated.
func (e *ErrRunProjectAmbiguous) CandidateNames() string {
	names := make([]string, 0, len(e.Candidates))
	for _, c := range e.Candidates {
		names = append(names, fmt.Sprintf("%s (%s)", c.Name, c.ID))
	}
	return strings.Join(names, ", ")
}

// ErrTaskInNoProject refuses a run on a ticket no project shows: it has no
// repositories to work in until it is labelled into one.
var ErrTaskInNoProject = errors.New("ce ticket n'appartient à aucun projet : ajoutez-lui le label d'un projet avant de le lancer")

// ErrRunProjectNotMember refuses a run for a project the ticket is not in.
var ErrRunProjectNotMember = errors.New("ce ticket n'appartient pas à ce projet")

// ResolveRunProject chooses the project a run on the ticket works for (#741):
// the one requested, which must be one of the ticket's projects; else the
// ticket's only project; else, for a ticket of several projects, an
// *ErrRunProjectAmbiguous. A ticket in no project cannot be run.
func (d *DB) ResolveRunProject(task *models.Task, requested string, unattended bool) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.resolveRunProjectUnsafe(task, requested, unattended)
}

func (d *DB) resolveRunProjectUnsafe(task *models.Task, requested string, unattended bool) (string, error) {
	if task == nil {
		return "", fmt.Errorf("task not found")
	}
	candidates, err := d.memberProjectsUnsafe(task)
	if err != nil {
		return "", err
	}
	if requested = strings.TrimSpace(requested); requested != "" {
		for _, p := range candidates {
			if p.ID == requested || (p.Slug != "" && p.Slug == requested) {
				return p.ID, nil
			}
		}
		return "", ErrRunProjectNotMember
	}
	switch len(candidates) {
	case 0:
		return "", ErrTaskInNoProject
	case 1:
		return candidates[0].ID, nil
	}
	ambiguous := &ErrRunProjectAmbiguous{Unattended: unattended}
	for _, p := range candidates {
		ambiguous.Candidates = append(ambiguous.Candidates, RunProjectCandidate{ID: p.ID, Name: p.Name})
	}
	return "", ambiguous
}

// RunProjectOfTask is the project the ticket's run works for when it is one of
// the ticket's projects, else the ticket's computed project: what an
// operation made from inside a run works for (#741).
func (d *DB) RunProjectOfTask(task *models.Task) string {
	if task == nil {
		return ""
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if run := d.runProjectOfTaskUnsafe(task.ID); run != "" {
		for _, id := range task.ProjectIDs {
			if id == run {
				return run
			}
		}
	}
	return task.ProjectID
}

// runProjectOfTaskUnsafe is the project of the ticket's latest run that
// recorded one and is still open, else of its latest run that recorded one:
// what a call made from inside a run, with no project of its own, works for.
// Empty when no run recorded a project.
func (d *DB) runProjectOfTaskUnsafe(taskID string) string {
	var projectID string
	_ = d.conn.QueryRow(`SELECT run_project_id FROM task_activities
		WHERE task_id = ? AND COALESCE(run_project_id, '') <> ''
		ORDER BY CASE WHEN status IN ('queued', 'pending', 'running') THEN 0 ELSE 1 END, created_at DESC LIMIT 1`, taskID).Scan(&projectID)
	return projectID
}
