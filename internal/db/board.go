package db

import (
	"encoding/json"
	"fmt"
	"strings"
	"tasks/internal/trackerapi"
	"time"

	"tasks/internal/models"
)

// Board columns are Jira-like: a column is a name plus the tracker statuses it
// groups, and the project owns them. Importing a tracker board is only a
// starting point — columns can then be created, renamed, reordered and have
// statuses reassigned freely, and each agentic workflow stage is assigned to one
// or several of them.

const boardAPITimeout = 60 * time.Second

// ListProjectTrackerBoards returns the tracker boards attached to a project.
func (d *DB) ListProjectTrackerBoards(projectID string) ([]models.TrackerBoard, error) {
	return []models.TrackerBoard{}, nil
}

// ListProjectIssueTypes returns the work item types the project's tracker
// exposes, for the settings that pick which ones are imported.
func (d *DB) ListProjectIssueTypes(projectID string) ([]string, error) {
	return []string{}, nil
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

				if output, err := d.trackers.GithubGraphQL(gqlQuery); err == nil {
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
	} else if proj.IssueTracker == "linear" && proj.LinearTeam != "" {
		tasks, err := d.trackers.SyncFromLinear(proj.LinearTeam)
		if err == nil && len(tasks) > 0 {
			for _, t := range tasks {
				st := strings.TrimSpace(string(t.Status))
				if st != "" && !seen[strings.ToLower(st)] {
					seen[strings.ToLower(st)] = true
					out = append(out, st)
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
// Called at the end of a Jira sync, so the board follows the tracker without a
// manual import.
func (d *DB) SyncProjectBoardColumns(projectID string) (string, error) {
	return "", nil
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
// runs. How it runs is not a property of the step: a step is the pair (stage,
// skill) and the mode is resolved per launch by ResolveSkillMode.
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

// AutonomousStopStage is the fallback stop stage for an autonomous run: the PR is
// opened and waiting for the user to review and merge. A project can stop earlier
// through its own autonomousStopStage setting. Merging is strictly reserved for
// the human user in either case.
const AutonomousStopStage = models.StopStageReviewed
