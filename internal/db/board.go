package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"tasks/internal/trackerapi"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// Board columns are Jira-like: a column is a name plus the tracker statuses it
// groups, and the project owns them. Importing a tracker board is only a
// starting point — columns can then be created, renamed, reordered and have
// statuses reassigned freely, and each agentic workflow stage is assigned to one
// or several of them.

const boardAPITimeout = 60 * time.Second

// trackerReaderFor resolves a project's default tracker and the ticketing
// system that answers for it (#741). The caller then asks the tracker whether
// it has the notion at hand, so a project on a tracker without boards gets a
// limit named, not a failure.
func (d *DB) trackerReaderFor(projectID string) (tracker.TicketingSystem, *models.Tracker, error) {
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, nil, fmt.Errorf("project not found")
	}
	trk := d.trackerOfProjectUnsafe(proj)
	return d.trackerReaderOf(trk)
}

// trackerReaderByID is the ticketing system of the tracker with that id.
func (d *DB) trackerReaderByID(trackerID string) (tracker.TicketingSystem, *models.Tracker, error) {
	trk, err := d.GetTrackerByID(trackerID)
	if err != nil {
		return nil, nil, err
	}
	if trk == nil {
		return nil, nil, fmt.Errorf("tracker non trouvé")
	}
	return d.trackerReaderOf(trk)
}

// ListTrackerBoardsAs returns a tracker's boards, for an admin configuring it
// (#741).
func (d *DB) ListTrackerBoardsAs(ctx context.Context, trackerID string) ([]models.TrackerBoard, error) {
	ts, trk, err := d.trackerReaderByID(trackerID)
	if err != nil {
		return nil, err
	}
	if !ts.Supports(tracker.CapBoard) {
		return nil, tracker.Unsupported(ts.Name(), tracker.CapBoard)
	}
	ctx, cancel := context.WithTimeout(ctx, boardAPITimeout)
	defer cancel()
	return ts.ListBoards(ctx, tracker.BoardsRequest{Tracker: trk})
}

// ListTrackerIssueTypesAs returns the work item types a tracker exposes.
func (d *DB) ListTrackerIssueTypesAs(ctx context.Context, trackerID string) ([]string, error) {
	ts, trk, err := d.trackerReaderByID(trackerID)
	if err != nil {
		return nil, err
	}
	if !ts.Supports(tracker.CapBoard) {
		return nil, tracker.Unsupported(ts.Name(), tracker.CapBoard)
	}
	ctx, cancel := context.WithTimeout(ctx, boardAPITimeout)
	defer cancel()
	return ts.ListIssueTypes(ctx, tracker.ProjectRequest{Tracker: trk})
}

// ImportTrackerBoardColumns retains a board for a tracker, then refreshes its
// columns from it, as ImportProjectBoardColumns does for a project's default
// tracker.
func (d *DB) ImportTrackerBoardColumns(ctx context.Context, trackerID, boardID string) (*models.Tracker, error) {
	trk, err := d.GetTrackerByID(trackerID)
	if err != nil {
		return nil, err
	}
	if trk == nil {
		return nil, fmt.Errorf("tracker non trouvé")
	}
	if boardID = strings.TrimSpace(boardID); boardID == "" {
		boardID = trk.BoardID
	}
	if boardID == "" {
		return nil, fmt.Errorf("no board selected")
	}
	if boardID != trk.BoardID {
		if _, err := d.UpdateTrackerMirror(trk.ID, func(t *models.Tracker) { t.BoardID = boardID }); err != nil {
			return nil, err
		}
	}
	if _, err := d.SyncTrackerBoardColumns(ctx, trk.ID); err != nil {
		return nil, err
	}
	return d.GetTrackerByID(trk.ID)
}

// GetTrackerStatusesAs lists the statuses a tracker's workflows expose, to
// assign them to its columns. The dispatch is on the tracker: GitHub answers
// with the ProjectsV2 single-select options of its repository, any other
// tracker that has boards with its own status list, and a tracker with neither
// notion answers nothing rather than failing (#741).
func (d *DB) GetTrackerStatusesAs(ctx context.Context, trackerID string) ([]string, error) {
	ts, trk, err := d.trackerReaderByID(trackerID)
	if err != nil {
		return nil, err
	}
	if trk.Provider == "github" {
		return githubRepoStatuses(d.trackerClientAs(tracker.ActingUser(ctx), "github", trk.ID), trk.Scope), nil
	}
	if !ts.Supports(tracker.CapBoard) {
		return []string{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, boardAPITimeout)
	defer cancel()
	statuses, err := ts.ListStatuses(ctx, tracker.ProjectRequest{Tracker: trk})
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, st := range statuses {
		name := strings.TrimSpace(st.Name)
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		out = append(out, name)
	}
	return out, nil
}

// trackerReaderOf is the ticketing system of one tracker.
func (d *DB) trackerReaderOf(trk *models.Tracker) (tracker.TicketingSystem, *models.Tracker, error) {
	ts, err := d.TrackerFor(trk)
	if err != nil {
		return nil, trk, err
	}
	return ts, trk, nil
}

// trackerClientAs is trackerAs for a call addressed to a tracker rather than
// to a project: the acting person's credential where they stored one, the
// tracker's otherwise (#741). Reads only.
func (d *DB) trackerClientAs(userID, trackerName, trackerID string) *trackerapi.Client {
	client, _, err := d.trackers.ForActingUser(userID, trackerName, trackerID)
	if err != nil || client == nil {
		return d.trackers.For(trackerID)
	}
	return client
}

// githubRepoStatuses reads the ProjectsV2 single-select options of a GitHub
// repository, falling back to the two states an issue always has. It is the
// historical body of the project status list, read for a GitHub tracker since
// the board is configured on the tracker (#741).
func githubRepoStatuses(client *trackerapi.Client, githubRepo string) []string {
	seen := map[string]bool{}
	out := []string{}

	repo := models.CleanGithubRepo(githubRepo)
	if repo != "" && client != nil {

		parts := strings.Split(repo, "/")
		if len(parts) == 2 {
			gqlQuery, _ := trackerapi.GithubStatusQuery(repo)

			if output, err := client.GithubGraphQL(gqlQuery); err == nil {
				var gqlRes struct {
					Data struct {
						Repository struct {
							ProjectsV2 struct {
								Nodes []struct {
									Fields struct {
										Nodes []struct {
											Name    string `json:"name"`
											Options []struct {
												Name string `json:"name"`
											} `json:"options"`
										} `json:"nodes"`
									} `json:"fields"`
								} `json:"nodes"`
							} `json:"projectsV2"`
						} `json:"repository"`
						User struct {
							ProjectsV2 struct {
								Nodes []struct {
									Fields struct {
										Nodes []struct {
											Name    string `json:"name"`
											Options []struct {
												Name string `json:"name"`
											} `json:"options"`
										} `json:"nodes"`
									} `json:"fields"`
								} `json:"nodes"`
							} `json:"projectsV2"`
						} `json:"user"`
					} `json:"data"`
				}
				if json.Unmarshal(output, &gqlRes) == nil {
					allProjects := append(gqlRes.Data.Repository.ProjectsV2.Nodes, gqlRes.Data.User.ProjectsV2.Nodes...)
					for _, pNode := range allProjects {
						for _, fNode := range pNode.Fields.Nodes {
							if strings.EqualFold(fNode.Name, "Status") || strings.EqualFold(fNode.Name, "Statut") || len(fNode.Options) > 0 {
								for _, opt := range fNode.Options {
									name := strings.TrimSpace(opt.Name)
									if name != "" && !seen[strings.ToLower(name)] {
										seen[strings.ToLower(name)] = true
										out = append(out, name)
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Standard GitHub states if no board columns were found
	if len(out) == 0 {
		for _, s := range []string{"open", "closed"} {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}

	return out
}

// MoveTaskToTrackerStatus records the new tracker status locally and queues the
// transition. The local write comes first so the card stays in the column it was
// dropped in; the tracker call runs in the activity queue, where a refusal stays
// readable instead of being lost in an expired request. The returned task is the
// local state, the returned activity is the transition to follow.
func (d *DB) MoveTaskToTrackerStatus(ctx context.Context, taskIDOrKey string, statusName string) (*models.Task, *models.TaskActivity, error) {
	return d.MoveTaskToTrackerStatusIn(ctx, taskIDOrKey, statusName, "")
}

// MoveTaskToTrackerStatusIn is MoveTaskToTrackerStatus made from a project's
// board: the stage the status gives is read through that project's stage
// mapping when it selects the ticket's tracker (#741, stagemapping.go).
func (d *DB) MoveTaskToTrackerStatusIn(ctx context.Context, taskIDOrKey string, statusName string, projectID string) (*models.Task, *models.TaskActivity, error) {
	statusName = strings.TrimSpace(statusName)
	if statusName == "" {
		return nil, nil, fmt.Errorf("statut cible manquant")
	}

	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, nil, fmt.Errorf("tâche non trouvée")
	}
	if projectID = strings.TrimSpace(projectID); projectID != "" {
		task.ContextProjectID = projectID
	}

	// Determine workflow stage for this status/column, through the stage
	// mapping that applies to the ticket (#741, stagemapping.go).
	trk := d.stageTrackerOfTaskUnsafe(task, "")
	targetStage := StageForTrackerStatus(trk, statusName)
	if targetStage == "" {
		targetStage = GetStageLabelForStatus(models.Status(statusName))
	}
	if targetStage == "" {
		targetStage = "new"
	}

	// The labels are derived from the locked row rather than from the task read
	// above, so a label another instance added meanwhile is kept.
	d.mu.Lock()
	execErr := d.conn.WithTx(func(tx *sqlTx) error {
		locked, err := d.lockTaskUnsafe(tx, task.ID)
		if err != nil {
			return err
		}
		if locked == nil {
			return fmt.Errorf("tâche non trouvée")
		}
		newLabels := SetWorkflowLabel(locked.Labels, "#"+strings.TrimPrefix(targetStage, "#"))
		labelsJSON, _ := json.Marshal(newLabels)
		newStatus := locked.Status
		if internalSt, ok := InternalStatusForStage(targetStage); ok {
			newStatus = internalSt
		}
		_, err = tx.Exec("UPDATE tasks SET tracker_status = ?, labels = ?, status = ?, updated_at = ? WHERE id = ?", statusName, string(labelsJSON), string(newStatus), time.Now(), task.ID)
		return err
	})
	d.mu.Unlock()
	if execErr != nil {
		return nil, nil, execErr
	}

	activity, opErr := d.EnqueueTrackerOp(ctx, TrackerOp{
		Kind:         TrackerOpTransition,
		ProjectID:    task.ProjectID,
		TaskID:       task.ID,
		TaskKey:      task.Key,
		TargetStatus: statusName,
	})
	if opErr != nil {
		return nil, nil, opErr
	}

	updated, err := d.GetTaskByID(task.ID)
	if err != nil {
		return nil, activity, err
	}
	return updated, activity, nil
}

// resolveBoardID names the board that drives a tracker: the one it recorded, or
// its first scrum board, which is the one carrying columns worth mirroring, or
// failing that its first board at all. A tracker with no board is an error, not
// an empty column list.
func resolveBoardID(ctx context.Context, ts tracker.TicketingSystem, trk *models.Tracker) (string, error) {
	if boardID := strings.TrimSpace(trk.BoardID); boardID != "" {
		return boardID, nil
	}
	boards, err := ts.ListBoards(ctx, tracker.BoardsRequest{Tracker: trk})
	if err != nil {
		return "", err
	}
	for _, b := range boards {
		if strings.EqualFold(b.Type, "scrum") {
			return b.ID, nil
		}
	}
	if len(boards) > 0 {
		return boards[0].ID, nil
	}
	return "", fmt.Errorf("no board on project %s", trk.Name)
}

// SyncProjectBoardColumns refreshes the columns of a project's default tracker
// (#741). See SyncTrackerBoardColumns.
func (d *DB) SyncProjectBoardColumns(ctx context.Context, projectID string) (string, error) {
	_, trk, err := d.trackerReaderFor(projectID)
	if err != nil {
		return "", err
	}
	if trk == nil || trk.ID == "" {
		return "", fmt.Errorf("project has no tracker")
	}
	return d.SyncTrackerBoardColumns(ctx, trk.ID)
}

// SyncTrackerBoardColumns refreshes a tracker's columns from its board,
// merging rather than overwriting: the column list and their order come from the
// tracker, while the statuses a user assigned by hand to a column of the same
// name are kept — as long as the tracker does not claim them elsewhere. A status
// the tracker maps nowhere, such as a workflow status absent from the board,
// therefore stays where the user put it. The mirror lands on the tracker row,
// which every project selecting the tracker reads (#741).
//
// Called at the end of a sync on a tracker with boards, so the board follows
// the tracker without a manual import.
func (d *DB) SyncTrackerBoardColumns(ctx context.Context, trackerID string) (string, error) {
	trk, err := d.GetTrackerByID(trackerID)
	if err != nil {
		return "", err
	}
	if trk == nil {
		return "", fmt.Errorf("tracker not found")
	}
	ts, err := d.TrackerFor(trk)
	if err != nil {
		return "", err
	}
	if !ts.Supports(tracker.CapBoard) {
		return "", tracker.Unsupported(ts.Name(), tracker.CapBoard)
	}

	ctx, cancel := context.WithTimeout(ctx, boardAPITimeout)
	defer cancel()

	boardID, err := resolveBoardID(ctx, ts, trk)
	if err != nil {
		return "", err
	}

	remote, err := ts.ListBoardColumns(ctx, tracker.BoardRequest{Tracker: trk, BoardID: boardID})
	if err != nil {
		return "", err
	}

	// Statuses the tracker assigns, so a manual assignment cannot duplicate one.
	claimed := map[string]bool{}
	for _, col := range remote {
		for _, st := range col.Statuses {
			claimed[strings.ToLower(st)] = true
		}
	}

	previousByName := map[string][]string{}
	// Hiding a column is the user's display choice: it survives a re-read of
	// the board, which only knows names, order and statuses.
	previousHidden := map[string]bool{}
	for _, col := range trk.TrackerColumns {
		previousByName[strings.ToLower(col.Name)] = col.Statuses
		previousHidden[strings.ToLower(col.Name)] = col.Hidden
	}

	merged := make([]models.TrackerColumn, 0, len(remote))
	for _, col := range remote {
		statuses := append([]string{}, col.Statuses...)
		seen := map[string]bool{}
		for _, st := range statuses {
			seen[strings.ToLower(st)] = true
		}
		for _, st := range previousByName[strings.ToLower(col.Name)] {
			key := strings.ToLower(st)
			if seen[key] || claimed[key] {
				continue
			}
			statuses = append(statuses, st)
			seen[key] = true
		}
		merged = append(merged, models.TrackerColumn{Name: col.Name, Statuses: statuses, Hidden: previousHidden[strings.ToLower(col.Name)]})
	}

	// Columns the user created and the tracker does not know are kept at the end
	// rather than silently dropped: they may hold statuses nothing else claims.
	remoteNames := map[string]bool{}
	for _, col := range remote {
		remoteNames[strings.ToLower(col.Name)] = true
	}
	for _, col := range trk.TrackerColumns {
		if remoteNames[strings.ToLower(col.Name)] {
			continue
		}
		kept := []string{}
		for _, st := range col.Statuses {
			if !claimed[strings.ToLower(st)] {
				kept = append(kept, st)
			}
		}
		if len(kept) > 0 {
			merged = append(merged, models.TrackerColumn{Name: col.Name, Statuses: kept, Hidden: col.Hidden})
		}
	}

	// Stage assignments are keyed by column name: drop the ones whose column
	// disappeared, keep the rest untouched.
	names := map[string]bool{}
	for _, col := range merged {
		names[col.Name] = true
	}
	stages := map[string][]string{}
	for stage, cols := range trk.StageColumns {
		kept := []string{}
		for _, c := range cols {
			if names[c] {
				kept = append(kept, c)
			}
		}
		if len(kept) > 0 {
			stages[stage] = kept
		}
	}

	// The sprints follow at the same time: their state is what separates NOW
	// (active sprint) from NEXT (future sprint) in the roadmap.
	sprints := trk.Sprints
	sprintErr := tracker.Unsupported(ts.Name(), tracker.CapSprint)
	if ts.Supports(tracker.CapSprint) {
		var remoteSprints []models.TrackerSprint
		remoteSprints, sprintErr = ts.ListSprints(ctx, tracker.BoardRequest{Tracker: trk, BoardID: boardID})
		if sprintErr == nil {
			sprints = remoteSprints
		}
	}

	if _, err := d.UpdateTrackerMirror(trk.ID, func(t *models.Tracker) {
		t.BoardID, t.TrackerColumns, t.StageColumns, t.Sprints = boardID, merged, stages, sprints
	}); err != nil {
		return "", err
	}

	note := fmt.Sprintf("%d columns taken from board %s", len(merged), boardID)
	if sprintErr == nil {
		active := 0
		for _, sp := range sprints {
			if sp.State == "active" {
				active++
			}
		}
		note = fmt.Sprintf("%s, %d sprints (%d active)", note, len(sprints), active)
	}
	return note, nil
}

// Le mapping du projet fait le lien entre statut du tracker, colonne et étape du
// workflow agentique. Ces deux fonctions le traversent dans les deux sens, pour
// que changer l'un des deux côtés dans la fiche mette l'autre à jour.

var stageToInternalStatus = map[string]models.Status{
	"new":         models.StatusToClarify,
	"clarified":   models.StatusClarified,
	"specified":   models.StatusToImplement,
	"implemented": models.StatusToTest,
	"reviewed":    models.StatusToClose,
	"finished":    models.StatusFinished,
}

var workflowStageOrder = []string{"new", "clarified", "specified", "implemented", "reviewed", "finished"}

// StageForTrackerStatus returns the workflow stage a tracker status belongs to,
// through the column that groups it on the tracker's board (#741). Empty when
// the tracker has no mapping for it, in which case the caller keeps whatever
// it had. trk carries the stage mapping that applies (withStageMappingUnsafe):
// a project's own or the tracker's.
func StageForTrackerStatus(trk *models.Tracker, trackerStatus string) string {
	trackerStatus = strings.ToLower(strings.TrimSpace(trackerStatus))
	if trk == nil || trackerStatus == "" {
		return ""
	}
	if trackerStatus == "done" || trackerStatus == "closed" || trackerStatus == "finished" {
		return "finished"
	}
	column := ""
	for _, col := range trk.TrackerColumns {
		if strings.ToLower(col.Name) == trackerStatus {
			column = col.Name
			break
		}
		for _, st := range col.Statuses {
			if strings.ToLower(st) == trackerStatus {
				column = col.Name
				break
			}
		}
		if column != "" {
			break
		}
	}
	if strings.EqualFold(column, "done") || strings.EqualFold(column, "closed") || strings.EqualFold(column, "finished") {
		return "finished"
	}
	if column == "" {
		// Fallback: check if trackerStatus matches a stage name directly
		for _, stage := range workflowStageOrder {
			if stage == trackerStatus || "#"+stage == trackerStatus {
				return stage
			}
		}
		// Fallback to keyword matching via GetStageLabelForStatus
		return GetStageLabelForStatus(models.Status(trackerStatus))
	}
	// Plusieurs étapes sur une colonne : la moins avancée, celle qui reste à
	// faire, comme côté interface.
	for _, stage := range workflowStageOrder {
		for _, name := range trk.StageColumns[stage] {
			if strings.EqualFold(name, column) || strings.EqualFold(name, trackerStatus) {
				return stage
			}
		}
	}
	return GetStageLabelForStatus(models.Status(trackerStatus))
}

// TrackerStatusForStage returns the tracker status a workflow stage lands on:
// the first status of the first column that stage is assigned to on the
// tracker's board (#741), by the stage mapping trk carries, as for
// StageForTrackerStatus.
func TrackerStatusForStage(trk *models.Tracker, stage string) string {
	stage = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(stage), "#"))
	if trk == nil || stage == "" {
		return ""
	}
	for _, columnName := range trk.StageColumns[stage] {
		for _, col := range trk.TrackerColumns {
			if col.Name == columnName && len(col.Statuses) > 0 {
				return col.Statuses[0]
			}
		}
	}
	return ""
}

// InternalStatusForStage folds a workflow stage onto the six internal statuses,
// which the generic views and the existing filters still use.
func InternalStatusForStage(stage string) (models.Status, bool) {
	stage = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(stage), "#"))
	st, ok := stageToInternalStatus[stage]
	return st, ok
}

// Le pas suivant du workflow agentique, côté serveur. Le front a la même
// résolution, mais la chaîne autonome tourne dans le worker : elle ne peut pas
// dépendre de l'interface, qui peut être fermée.

// StageOfTask returns a task's workflow stage: explicit workflow label first,
// then the board column mapping, then fallback to internal status.
func (d *DB) StageOfTask(task *models.Task) string {
	if task == nil {
		return ""
	}

	// 1. Les labels de workflow explicites ont priorité absolue dans la vue agentique.
	// On vérifie de l'étape la plus avancée à la moins avancée.
	for i := len(workflowStageOrder) - 1; i >= 0; i-- {
		stage := workflowStageOrder[i]
		for _, label := range task.Labels {
			clean := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(label), "#"))
			if clean == "closed" || clean == "done" {
				clean = "finished"
			}
			if clean == stage {
				return stage
			}
		}
	}

	// 2. The board column of the ticket's tracker when no label says, through
	// the stage mapping that applies to the ticket (#741).
	trk := d.stageTrackerOfTaskUnsafe(task, "")
	if stage := StageForTrackerStatus(trk, task.TrackerStatus); stage != "" {
		return stage
	}

	// 3. Repli sur le statut interne, comme le fait l'interface. Sans lui, un
	// ticket clos sans label de workflow était rendu comme « new », et l'app
	// proposait de clarifier un ticket déjà terminé.
	switch task.Status {
	case models.StatusFinished, models.StatusDone:
		return "finished"
	case models.StatusToClose:
		return "reviewed"
	case models.StatusToTest, models.StatusToValidate:
		return "implemented"
	case models.StatusToImplement, models.StatusInProgress:
		return "specified"
	case models.StatusClarified:
		return "clarified"
	case models.StatusSpecified:
		return "specified"
	}
	return "new"
}

// StageStep describes what advancing one step from a stage means: which skill
// runs, under which label. A step carries no mode of its own: a step is the pair
// (stage, skill), and the skill is where the execution mode is configured.
type StageStep struct {
	SkillID string
	Label   string
}

var stageSteps = map[string]StageStep{
	"new":         {SkillID: "clarify", Label: "Clarifier les exigences"},
	"clarified":   {SkillID: "specify", Label: "Spécifier la solution (SDD)"},
	"specified":   {SkillID: "implement", Label: "Implémenter le code et tests"},
	"implemented": {SkillID: "adjust", Label: "Adjust the existing PR/MR; merge remains manual"},
	"reviewed":    {SkillID: "handoff", Label: "Handoff et nettoyage local"},
}

// NextStep returns the step to run from a stage, and whether there is one.
func NextStep(stage string) (StageStep, bool) {
	step, ok := stageSteps[strings.ToLower(strings.TrimSpace(stage))]
	return step, ok
}

// stageRank places a stage in workflowStageOrder, which is what lets a full
// chain run tell "already past the stop stage" from "not there yet". A stage
// absent from the list ranks -1, so an unknown stage never reads as finished
// work.
func stageRank(stage string) int {
	stage = strings.ToLower(strings.TrimSpace(stage))
	for i, known := range workflowStageOrder {
		if known == stage {
			return i
		}
	}
	return -1
}

// stageAtOrPast says whether a task has already reached a stage. It is false for
// an unknown stage, which leaves the chain free to start rather than refusing on
// a value it cannot place.
func stageAtOrPast(stage, target string) bool {
	current, want := stageRank(stage), stageRank(target)
	return current >= 0 && want >= 0 && current >= want
}

// DefaultFullChainStopStage is where a full chain run stops when the project
// stores nothing: the PR is opened and waiting for the user to review and merge.
// Merging is strictly reserved for the human user.
const DefaultFullChainStopStage = models.FullChainStopReviewed
