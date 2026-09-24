package trackerapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// The Jira adapter manages the sprints of its boards.
var _ tracker.SprintManager = (*JiraAdapter)(nil)

// jiraSprint is a sprint as the Agile API writes it back.
type jiraSprint struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	State     string `json:"state"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

func (sp jiraSprint) model() models.TrackerSprint {
	return models.TrackerSprint{ID: fmt.Sprint(sp.ID), Name: sp.Name, State: strings.ToLower(sp.State), StartDate: sp.StartDate, EndDate: sp.EndDate}
}

// jiraSprintDate writes a date the way the Agile API takes it, RFC3339. A bare
// day given as an end date means the end of that day, so the sprint covers it.
func jiraSprintDate(raw string, end bool) (string, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.Format(time.RFC3339), nil
	}
	day, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return "", fmt.Errorf("date invalide %q : attendu AAAA-MM-JJ", raw)
	}
	if end {
		day = day.Add(24*time.Hour - time.Second)
	}
	return day.Format(time.RFC3339), nil
}

// CreateSprint creates one sprint on the project's board.
func (j *JiraAdapter) CreateSprint(ctx context.Context, req tracker.SprintCreateRequest) (models.TrackerSprint, error) {
	boardID := strings.TrimSpace(req.BoardID)
	if boardID == "" && req.Project != nil {
		boardID = strings.TrimSpace(req.Project.BoardID)
	}
	if boardID == "" {
		return models.TrackerSprint{}, fmt.Errorf("choisissez d'abord un board dans les options du projet")
	}
	var board int
	if _, err := fmt.Sscanf(boardID, "%d", &board); err != nil {
		return models.TrackerSprint{}, fmt.Errorf("board Jira invalide %q", boardID)
	}
	c, err := j.forProject(ctx, req.Project)
	if err != nil {
		return models.TrackerSprint{}, err
	}
	var created jiraSprint
	err = c.jira(ctx, http.MethodPost, "/rest/agile/1.0/sprint", nil, map[string]any{
		"name":          strings.TrimSpace(req.Name),
		"originBoardId": board,
		"startDate":     req.Start.Format(time.RFC3339),
		"endDate":       req.End.Format(time.RFC3339),
	}, &created)
	if err != nil {
		return models.TrackerSprint{}, err
	}
	return created.model(), nil
}

// UpdateSprint sends only the fields the patch carries. A state Jira refuses,
// such as closing a sprint that never started, comes back as Jira says it.
func (j *JiraAdapter) UpdateSprint(ctx context.Context, project *models.Project, sprintID string, patch models.SprintPatch) (models.TrackerSprint, error) {
	id := strings.TrimSpace(sprintID)
	if id == "" {
		return models.TrackerSprint{}, fmt.Errorf("sprint manquant")
	}
	body := map[string]any{}
	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" {
			return models.TrackerSprint{}, fmt.Errorf("un sprint doit garder un nom")
		}
		body["name"] = name
	}
	if patch.Start != nil {
		start, err := jiraSprintDate(*patch.Start, false)
		if err != nil {
			return models.TrackerSprint{}, err
		}
		body["startDate"] = start
	}
	if patch.End != nil {
		end, err := jiraSprintDate(*patch.End, true)
		if err != nil {
			return models.TrackerSprint{}, err
		}
		body["endDate"] = end
	}
	if patch.State != nil {
		state := strings.ToLower(strings.TrimSpace(*patch.State))
		if state != "active" && state != "future" && state != "closed" {
			return models.TrackerSprint{}, fmt.Errorf("état de sprint invalide %q", *patch.State)
		}
		body["state"] = state
	}
	if len(body) == 0 {
		return models.TrackerSprint{}, fmt.Errorf("rien à modifier sur ce sprint")
	}
	c, err := j.forProject(ctx, project)
	if err != nil {
		return models.TrackerSprint{}, err
	}
	var updated jiraSprint
	// POST is the partial update; PUT would replace the sprint whole.
	if err := c.jira(ctx, http.MethodPost, "/rest/agile/1.0/sprint/"+url.PathEscape(id), nil, body, &updated); err != nil {
		return models.TrackerSprint{}, err
	}
	return updated.model(), nil
}

// DeleteSprint deletes a sprint. A 404 is a sprint already gone, which is
// what the caller asked for.
func (j *JiraAdapter) DeleteSprint(ctx context.Context, project *models.Project, sprintID string) error {
	id := strings.TrimSpace(sprintID)
	if id == "" {
		return fmt.Errorf("sprint manquant")
	}
	c, err := j.forProject(ctx, project)
	if err != nil {
		return err
	}
	err = c.jira(ctx, http.MethodDelete, "/rest/agile/1.0/sprint/"+url.PathEscape(id), nil, nil, nil)
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.Status == http.StatusNotFound {
		return nil
	}
	return err
}
