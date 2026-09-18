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

// trackerReaderFor resolves a project and the tracker that answers for it. The
// caller then asks the tracker whether it has the notion at hand, so a project
// on a tracker without boards gets a limit named, not a failure.
func (d *DB) trackerReaderFor(projectID string) (tracker.TicketingSystem, *models.Project, error) {
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, nil, fmt.Errorf("projet non trouvé")
	}
	ts, err := d.TrackerForProject(proj)
	if err != nil {
		return nil, proj, err
	}
	return ts, proj, nil
}

// ListProjectTrackerBoards returns the tracker boards attached to a project.
func (d *DB) ListProjectTrackerBoards(projectID string) ([]models.TrackerBoard, error) {
	ts, proj, err := d.trackerReaderFor(projectID)
	if err != nil {
		return nil, err
	}
	if !ts.Supports(tracker.CapBoard) {
		return nil, tracker.Unsupported(ts.Name(), tracker.CapBoard)
	}
	ctx, cancel := context.WithTimeout(context.Background(), boardAPITimeout)
	defer cancel()
	return ts.ListBoards(ctx, tracker.BoardsRequest{Project: proj})
}

// ListProjectIssueTypes returns the work item types the project's tracker
// exposes, for the settings that pick which ones are imported.
func (d *DB) ListProjectIssueTypes(projectID string) ([]string, error) {
	ts, proj, err := d.trackerReaderFor(projectID)
	if err != nil {
		return nil, err
	}
	if !ts.Supports(tracker.CapBoard) {
		return nil, tracker.Unsupported(ts.Name(), tracker.CapBoard)
	}
	ctx, cancel := context.WithTimeout(context.Background(), boardAPITimeout)
	defer cancel()
	return ts.ListIssueTypes(ctx, tracker.ProjectRequest{Project: proj})
}

// ImportProjectBoardColumns retains a board for the project then refreshes from
// it. Le bouton « Détecter » et la synchro doivent donner le même résultat, donc
// les deux passent par la même fusion : elle préserve les statuts affectés à la
// main et les colonnes masquées, et ramène les sprints avec leur état.
func (d *DB) ImportProjectBoardColumns(projectID string, boardID string) (*models.Project, error) {
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, fmt.Errorf("projet non trouvé")
	}

	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		boardID = proj.BoardID
	}
	if boardID == "" {
		return nil, fmt.Errorf("aucun board sélectionné")
	}

	if boardID != proj.BoardID {
		if _, err := d.UpdateProject(proj.ID, models.UpdateProjectRequest{BoardID: &boardID}); err != nil {
			return nil, err
		}
	}

	if _, err := d.SyncProjectBoardColumns(proj.ID); err != nil {
		return nil, err
	}
	return d.GetProjectByID(proj.ID)
}

// GetProjectTrackerStatuses lists the tracker statuses actually seen on the
// project's tickets, plus those already assigned to a column. The point is to
// offer real values in the assignment UI rather than the instance's full status
// list, most of which never appears on this project.
func (d *DB) GetProjectTrackerStatuses(projectID string) ([]string, error) {
	proj, err := d.GetProjectByID(projectID)
	if err != nil || proj == nil {
		return nil, fmt.Errorf("projet non trouvé")
	}

	seen := map[string]bool{}
	out := []string{}

	// If GitHub tracker, query GitHub ProjectsV2 columns / SingleSelectField options via GraphQL API
	if proj.IssueTracker == "github" {
		repo := models.CleanGithubRepo(proj.GithubRepo)
		if repo != "" {

			parts := strings.Split(repo, "/")
			if len(parts) == 2 {
				gqlQuery, _ := trackerapi.GithubStatusQuery(repo)

				if output, err := d.tracker(proj.ID).GithubGraphQL(gqlQuery); err == nil {
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
	}

	return out, nil
}

// MoveTaskToTrackerStatus records the new tracker status locally and queues the
// transition. The local write comes first so the card stays in the column it was
// dropped in; the tracker call runs in the activity queue, where a refusal stays
// readable instead of being lost in an expired request. The returned task is the
// local state, the returned activity is the transition to follow.
func (d *DB) MoveTaskToTrackerStatus(taskIDOrKey string, statusName string) (*models.Task, *models.TaskActivity, error) {
	statusName = strings.TrimSpace(statusName)
	if statusName == "" {
		return nil, nil, fmt.Errorf("statut cible manquant")
	}

	task, err := d.GetTaskByID(taskIDOrKey)
	if err != nil || task == nil {
		return nil, nil, fmt.Errorf("tâche non trouvée")
	}

	proj, _ := d.GetProjectByID(task.ProjectID)

	// Determine workflow stage for this status/column
	targetStage := ""
	if proj != nil {
		targetStage = StageForTrackerStatus(proj, statusName)
	}
	if targetStage == "" {
		targetStage = GetStageLabelForStatus(models.Status(statusName))
	}
	if targetStage == "" {
		targetStage = "new"
	}

	newLabels := SetWorkflowLabel(task.Labels, "#"+strings.TrimPrefix(targetStage, "#"))
	labelsJSON, _ := json.Marshal(newLabels)

	newStatus := task.Status
	if internalSt, ok := InternalStatusForStage(targetStage); ok {
		newStatus = internalSt
	}

	d.mu.Lock()
	_, execErr := d.conn.Exec("UPDATE tasks SET tracker_status = ?, labels = ?, status = ?, updated_at = ? WHERE id = ?", statusName, string(labelsJSON), string(newStatus), time.Now(), task.ID)
	d.mu.Unlock()
	if execErr != nil {
		return nil, nil, execErr
	}

	activity, opErr := d.EnqueueTrackerOp(TrackerOp{
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

// SyncProjectBoardColumns refreshes a project's columns from its tracker board,
// merging rather than overwriting: the column list and their order come from the
// tracker, while the statuses a user assigned by hand to a column of the same
// name are kept — as long as the tracker does not claim them elsewhere. A status
// the tracker maps nowhere, such as a workflow status absent from the board,
// therefore stays where the user put it.
//
// Called at the end of a sync on a tracker with boards, so the board follows
// the tracker without a manual import.
func (d *DB) SyncProjectBoardColumns(projectID string) (string, error) {
	ts, proj, err := d.trackerReaderFor(projectID)
	if err != nil {
		return "", err
	}
	if !ts.Supports(tracker.CapBoard) {
		return "", tracker.Unsupported(ts.Name(), tracker.CapBoard)
	}

	ctx, cancel := context.WithTimeout(context.Background(), boardAPITimeout)
	defer cancel()

	boardID := strings.TrimSpace(proj.BoardID)
	if boardID == "" {
		// No board chosen yet: the first scrum board of the project is the one
		// that carries columns worth mirroring.
		boards, err := ts.ListBoards(ctx, tracker.BoardsRequest{Project: proj})
		if err != nil {
			return "", err
		}
		for _, b := range boards {
			if strings.EqualFold(b.Type, "scrum") {
				boardID = b.ID
				break
			}
		}
		if boardID == "" && len(boards) > 0 {
			boardID = boards[0].ID
		}
		if boardID == "" {
			return "", fmt.Errorf("aucun board sur le projet %s", proj.Name)
		}
	}

	remote, err := ts.ListBoardColumns(ctx, tracker.BoardRequest{Project: proj, BoardID: boardID})
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
	for _, col := range proj.TrackerColumns {
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
	for _, col := range proj.TrackerColumns {
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
	for stage, cols := range proj.StageColumns {
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
	sprints := proj.Sprints
	sprintErr := tracker.Unsupported(ts.Name(), tracker.CapSprint)
	if ts.Supports(tracker.CapSprint) {
		var remoteSprints []models.TrackerSprint
		remoteSprints, sprintErr = ts.ListSprints(ctx, tracker.BoardRequest{Project: proj, BoardID: boardID})
		if sprintErr == nil {
			sprints = remoteSprints
		}
	}

	if _, err := d.UpdateProject(proj.ID, models.UpdateProjectRequest{
		BoardID:        &boardID,
		TrackerColumns: &merged,
		StageColumns:   &stages,
		Sprints:        &sprints,
	}); err != nil {
		return "", err
	}

	note := fmt.Sprintf("%d colonnes du board %s reprises", len(merged), boardID)
	if sprintErr == nil {
		active := 0
		for _, sp := range sprints {
			if sp.State == "active" {
				active++
			}
		}
		note = fmt.Sprintf("%s, %d sprints (%d actifs)", note, len(sprints), active)
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
// through the column that groups it. Empty when the project has no mapping for
// it, in which case the caller keeps whatever it had.
func StageForTrackerStatus(proj *models.Project, trackerStatus string) string {
	trackerStatus = strings.ToLower(strings.TrimSpace(trackerStatus))
	if proj == nil || trackerStatus == "" {
		return ""
	}
	if trackerStatus == "done" || trackerStatus == "closed" || trackerStatus == "finished" {
		return "finished"
	}
	column := ""
	for _, col := range proj.TrackerColumns {
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
		for _, name := range proj.StageColumns[stage] {
			if strings.EqualFold(name, column) || strings.EqualFold(name, trackerStatus) {
				return stage
			}
		}
	}
	return GetStageLabelForStatus(models.Status(trackerStatus))
}

// TrackerStatusForStage returns the tracker status a workflow stage lands on:
// the first status of the first column that stage is assigned to.
func TrackerStatusForStage(proj *models.Project, stage string) string {
	stage = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(stage), "#"))
	if proj == nil || stage == "" {
		return ""
	}
	for _, columnName := range proj.StageColumns[stage] {
		for _, col := range proj.TrackerColumns {
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

	// 2. Colonne du board configurée pour le projet si pas de label explicite
	if task.ProjectID != "" {
		if proj, _ := d.GetProjectByID(task.ProjectID); proj != nil {
			if stage := StageForTrackerStatus(proj, task.TrackerStatus); stage != "" {
				return stage
			}
		}
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
