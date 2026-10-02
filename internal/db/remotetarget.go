package db

import (
	"context"
	"fmt"
	"strings"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// createStoryInRoadmapProject creates a slicing line's story in one of the
// project's roadmap projects, under the macro's epic (#632).
//
// The story is another team's work item, so it is created on Jira and nowhere
// else: no task is recorded, nothing enters the board or the counters, and the
// line keeps its key. That is the rule of the roadmap projects, whose work items
// are never imported, rather than an exception to it.
//
// The parent is written after the creation, as createStoryUnder does, so that a
// parent Jira refuses does not lose a story that exists by then: the notice
// says so and the key is kept.
func (d *DB) createStoryInRoadmapProject(ctx context.Context, proj *models.Project, key string, macroKey string, title string) (*models.Task, string, error) {
	view := roadmapProjectView(proj, key)
	ts, err := d.TrackerForProject(view)
	if err != nil {
		return nil, "", err
	}
	if !ts.Supports(tracker.CapCreate) {
		return nil, "", tracker.Unsupported(ts.Name(), tracker.CapCreate)
	}
	ctx = tracker.WithProject(ctx, proj.ID)
	created, err := ts.CreateIssue(ctx, tracker.CreateIssueRequest{
		Project:  view,
		Title:    strings.TrimSpace(title),
		Priority: models.PriorityMedium,
	})
	if err != nil {
		return nil, "", err
	}
	if created == nil || strings.TrimSpace(created.Key) == "" {
		return nil, "", fmt.Errorf("Jira n'a pas confirmé la story créée dans %s", view.JiraProject)
	}
	// A task nobody imported has no local identity: the empty id is what tells
	// the caller not to open it.
	created.ID = ""
	created.ProjectID = ""
	created.ParentKey = macroKey
	created.ParentType = "macro"

	// The notice is what went wrong, as createStoryUnder's is: that the story
	// stays in Jira is told by the answer itself, a task without an id.
	notice := ""
	if macroKey = strings.TrimSpace(macroKey); macroKey != "" {
		if err := ts.SetParent(ctx, created.Key, macroKey); err != nil {
			notice = fmt.Sprintf("épic %s non posé comme parent de %s sur Jira : %v ; la story reste dans le projet %s", macroKey, created.Key, err, view.JiraProject)
		}
	}
	return created, notice, nil
}
