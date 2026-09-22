package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// The roadmap horizon of a macro is a local decision, and this file is what
// makes it leave Sectile: it writes that decision on the tracker's own epic as
// a "roadmap:<horizon>" label, and reads those labels back so a classification
// made on the tracker is not silently outranked by ours.
//
// A label is the medium on purpose. No tracker has a field for "NOW / NEXT /
// LATER", every team that needs one ends up with a label, and a label is
// already readable by a filter, a board and a JQL query. Sectile therefore owns
// exactly one axis, "roadmap:", writes every value of it exclusively, and
// touches nothing else on the epic.

const (
	// macroWriteTimeout covers one label write on one epic.
	macroWriteTimeout = 60 * time.Second
	// macroReadTimeout covers reading a project's epics, which is one paginated
	// request on the trackers that answer it.
	macroReadTimeout = 90 * time.Second
)

// isMilestoneKey tells a macro that exists only as a GitHub milestone, or only
// locally, from one that is a real work item on the tracker.
//
// CreateMacro numbers both of those "M-<n>", and neither can carry a label: a
// milestone has none, and a local key names nothing remote. Pushing them by
// number would edit whatever issue happens to carry that number, which is the
// one outcome worse than not pushing at all.
func isMilestoneKey(key string) bool {
	clean := strings.ToUpper(strings.TrimSpace(key))
	rest, ok := strings.CutPrefix(clean, "M-")
	if !ok || rest == "" {
		return false
	}
	for _, r := range rest {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// belongsToProject tells a macro whose key carries the project's tracker prefix
// from one that arrived attached to an epic of another project.
//
// Only our own epics are ever pushed. A foreign epic belongs to another team's
// board, and writing our axis on it would classify their work from a roadmap
// that is not theirs. It would also never leave the pending list: the read that
// checks the result is scoped to this project and cannot see it.
func belongsToProject(key string, proj *models.Project) bool {
	prefix := strings.ToUpper(strings.TrimSpace(proj.JiraProject))
	if prefix == "" {
		return true
	}
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(key)), prefix+"-")
}

// removedRoadmapLabels lists the labels of the axis to strip for a target: all
// of them but the one being written. An empty target removes the whole axis,
// which is what un-classifying a macro means.
func removedRoadmapLabels(target string) []string {
	out := make([]string, 0, len(AllRoadmapLabels()))
	for _, label := range AllRoadmapLabels() {
		if label != target {
			out = append(out, label)
		}
	}
	return out
}

// macroTracker resolves the tracker that carries a project's epics and refuses
// early everything it cannot label.
//
// Every refusal is a sentence rather than a silence. The classification stays
// local whether the push was impossible or merely failed, and the difference is
// exactly what the user needs to read in the activity.
func (d *DB) macroTracker(projectID string, macroKey string) (tracker.TicketingSystem, *models.Project, error) {
	projectID = strings.TrimSpace(projectID)
	macroKey = strings.TrimSpace(macroKey)
	if projectID == "" || macroKey == "" {
		return nil, nil, fmt.Errorf("projet et clé de macro obligatoires")
	}
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, nil, fmt.Errorf("projet non trouvé")
	}
	if isMilestoneKey(macroKey) {
		return nil, nil, fmt.Errorf("%s est un jalon : un jalon ne porte pas de label, la classification reste locale", macroKey)
	}
	if !belongsToProject(macroKey, proj) {
		return nil, nil, fmt.Errorf("%s appartient à un autre projet que %s : la classification reste locale", macroKey, proj.JiraProject)
	}
	ts, err := d.TrackerForProject(proj)
	if err != nil {
		return nil, nil, err
	}
	if !ts.Supports(tracker.CapUpdate) || !ts.Supports(tracker.CapLabels) {
		return nil, nil, tracker.Unsupported(ts.Name(), tracker.CapLabels)
	}
	return ts, proj, nil
}

// PushMacroHorizonLabel mirrors the classification onto the tracker's epic: it
// adds the label of the chosen horizon and removes the other three. An empty
// horizon removes the axis altogether.
//
// It performs the tracker call itself, so it is only ever run from a queued
// activity (TrackerOpEpicHorizon or TrackerOpPushHorizons): a click must not
// wait on it, and its failure has to stay readable in the activity rather than
// vanish behind an HTTP timeout.
func (d *DB) PushMacroHorizonLabel(ctx context.Context, projectID string, macroKey string, horizon string) (string, error) {
	ts, proj, err := d.macroTracker(projectID, macroKey)
	if err != nil {
		return "", err
	}
	macroKey = strings.TrimSpace(macroKey)

	target := RoadmapLabel(horizon)
	var added []string
	if target != "" {
		added = []string{target}
	}

	ctx, cancel := context.WithTimeout(ctx, macroWriteTimeout)
	defer cancel()
	// Only the labels travel. No status, no title, no description: an axis write
	// that also carried the rest would push back whatever the local copy held,
	// which on an epic Sectile never imported is nothing at all.
	if err := ts.UpdateIssue(ctx, tracker.UpdateIssueRequest{
		Project:       proj,
		Key:           macroKey,
		Labels:        added,
		RemovedLabels: removedRoadmapLabels(target),
	}); err != nil {
		return "", err
	}

	if target == "" {
		return fmt.Sprintf("labels roadmap retirés de %s", macroKey), nil
	}
	return fmt.Sprintf("« %s » posé sur %s", target, macroKey), nil
}

// PushEpicHorizonLabel is the epic-named alias kept for the callers that speak
// of epics rather than macros.
func (d *DB) PushEpicHorizonLabel(ctx context.Context, projectID string, epicKey string, horizon string) (string, error) {
	return d.PushMacroHorizonLabel(ctx, projectID, epicKey, horizon)
}

// remoteMacros reads a project's epics from its tracker, keyed by epic key.
//
// The read goes through the interface rather than naming a tracker: a tracker
// with no notion of epics says so through its capabilities, instead of failing
// with an authentication error belonging to another product.
func (d *DB) remoteMacros(ctx context.Context, proj *models.Project) (map[string]models.Task, error) {
	if proj == nil {
		return nil, fmt.Errorf("projet non trouvé")
	}
	ts, err := d.TrackerForProject(proj)
	if err != nil {
		return nil, err
	}
	if !ts.Supports(tracker.CapEpic) {
		return nil, tracker.Unsupported(ts.Name(), tracker.CapEpic)
	}

	ctx, cancel := context.WithTimeout(ctx, macroReadTimeout)
	defer cancel()
	epics, err := ts.ListEpics(ctx, tracker.ProjectRequest{Project: proj})
	if err != nil {
		return nil, err
	}

	out := make(map[string]models.Task, len(epics))
	for _, epic := range epics {
		key := strings.TrimSpace(epic.Key)
		if key == "" {
			continue
		}
		out[key] = epic
	}
	return out, nil
}

// ImportMacroHorizons reads the roadmap labels of a project's epics and records
// them locally, along with what the local roadmap has no other way of learning:
// the epic's own title and whether it is closed, since the sync imports stories
// and never the epic itself.
//
// The tracker wins when an epic carries a label, that being the shared source.
// An epic without one keeps whatever was decided locally, and that decision
// will be pushed the next time it is touched.
func (d *DB) ImportMacroHorizons(ctx context.Context, projectID string) (string, error) {
	proj, err := d.GetProjectByID(strings.TrimSpace(projectID))
	if err != nil || proj == nil {
		return "", fmt.Errorf("projet non trouvé")
	}
	found, err := d.remoteMacros(ctx, proj)
	if err != nil {
		return "", err
	}

	classified, closed := 0, 0
	for key, epic := range found {
		horizon := HorizonFromLabels(epic.Labels)
		var horizonPtr *string
		if horizon != "" {
			horizonPtr = &horizon
			classified++
		}
		title := epic.Title
		status := strings.TrimSpace(epic.TrackerStatus)
		if status == "" {
			status = string(epic.Status)
		}
		isClosed := epic.Status == models.StatusFinished || epic.Status == models.StatusDone
		if isClosed {
			closed++
		}
		if _, err := d.saveMacroMetaFull(proj.ID, key, horizonPtr, nil, nil, nil, &title, &status, &isClosed); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%d macro(s) lue(s) (%d classée(s), %d terminée(s))", len(found), classified, closed), nil
}

// ImportEpicHorizons is the epic-named alias of ImportMacroHorizons.
func (d *DB) ImportEpicHorizons(ctx context.Context, projectID string) (string, error) {
	return d.ImportMacroHorizons(ctx, projectID)
}

// PendingHorizonPushes lists the macros classified locally whose epic does not
// carry the matching roadmap label yet. That covers anything classified before
// the mirroring existed, anything classified while the tracker was unreachable,
// and every failed push.
//
// Macros that can never be pushed — milestones, local keys, epics of another
// project — are left out rather than listed as late: a list that only grows is
// one nobody acts on.
func (d *DB) PendingHorizonPushes(ctx context.Context, projectID string) ([]models.MacroMeta, error) {
	projectID = strings.TrimSpace(projectID)
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, fmt.Errorf("projet non trouvé")
	}
	metas, err := d.GetProjectMacros(projectID)
	if err != nil {
		return nil, err
	}

	classified := make([]models.MacroMeta, 0, len(metas))
	for _, meta := range metas {
		if meta.Horizon == "" || isMilestoneKey(meta.Key) || !belongsToProject(meta.Key, proj) {
			continue
		}
		classified = append(classified, meta)
	}
	// The tracker is not read at all when nothing is classified: the answer is
	// already known, and asking anyway would make an empty roadmap fail on a
	// project whose credentials are not set up yet.
	if len(classified) == 0 {
		return []models.MacroMeta{}, nil
	}

	remote, err := d.remoteMacros(ctx, proj)
	if err != nil {
		return nil, err
	}

	pending := []models.MacroMeta{}
	for _, meta := range classified {
		epic, known := remote[meta.Key]
		if !known || HorizonFromLabels(epic.Labels) != meta.Horizon {
			pending = append(pending, meta)
		}
	}
	return pending, nil
}

// PushPendingHorizons mirrors every locally classified macro whose label is
// missing or stale. Explicit rather than automatic: it edits one ticket per
// macro, which is not something a sync gets to decide on its own.
//
// One failure does not stop the others, and each is named: a run that stopped
// at the first refusal would leave the rest silently unpushed.
func (d *DB) PushPendingHorizons(ctx context.Context, projectID string) (int, []string, error) {
	pending, err := d.PendingHorizonPushes(ctx, projectID)
	if err != nil {
		return 0, nil, err
	}

	pushed := 0
	failures := []string{}
	for _, meta := range pending {
		if _, err := d.PushMacroHorizonLabel(ctx, projectID, meta.Key, meta.Horizon); err != nil {
			failures = append(failures, fmt.Sprintf("%s : %v", meta.Key, err))
			continue
		}
		pushed++
	}
	return pushed, failures, nil
}
