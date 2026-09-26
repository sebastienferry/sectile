package trackerapi

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"tasks/internal/models"
)

// GitlabIssueItem is an issue as the REST API v4 serves it, reduced to what a
// task carries. The iteration is only present on a Premium instance.
type GitlabIssueItem struct {
	ID          int64              `json:"id"`
	IID         int                `json:"iid"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	State       string             `json:"state"`
	WebURL      string             `json:"web_url"`
	Labels      []string           `json:"labels"`
	Author      *gitlabUser        `json:"author"`
	Assignees   []gitlabUser       `json:"assignees"`
	Milestone   *gitlabMilestone   `json:"milestone"`
	Iteration   *gitlabRESTIterate `json:"iteration"`
	IssueType   string             `json:"issue_type"`
	CreatedAt   *time.Time         `json:"created_at"`
	UpdatedAt   *time.Time         `json:"updated_at"`
}

type gitlabUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	State     string `json:"state"`
	AvatarURL string `json:"avatar_url"`
}

type gitlabMilestone struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	State     string `json:"state"`
	StartDate string `json:"start_date"`
	DueDate   string `json:"due_date"`
}

// gitlabRESTIterate is the iteration of an issue as REST writes it; its title
// is null on the iterations an automatic cadence generates.
type gitlabRESTIterate struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	StartDate string `json:"start_date"`
	DueDate   string `json:"due_date"`
}

// gitlabStageLabels are the stage labels a stage write keeps exactly one of.
var gitlabStageLabels = []string{"#new", "#clarified", "#specified", "#implemented", "#reviewed", "#finished"}

func isStageLabel(label string) bool {
	for _, s := range gitlabStageLabels {
		if strings.EqualFold(strings.TrimSpace(label), s) {
			return true
		}
	}
	return false
}

// gitlabTeamPrefix marks the scoped label carrying the team, the only scoped
// label Sectile writes.
const gitlabTeamPrefix = "team::"

// gitlabIterationName is how an iteration is called on a card and in the
// sprint list alike: its title, else its dates, since the iterations an
// automatic cadence generates often have no title.
func gitlabIterationName(title, start, due string) string {
	if t := strings.TrimSpace(title); t != "" {
		return t
	}
	return fmt.Sprintf("Itération %s au %s", start, due)
}

// gitlabTask turns an issue into a task. listLabels is the set of labels the
// project's boards use as lists, read once per synchronisation; nil means the
// caller did not read it, and the tracker status is then only opened or
// closed.
func gitlabTask(item GitlabIssueItem, listLabels map[string]bool) (*models.Task, error) {
	if item.IID < 1 {
		return nil, fmt.Errorf("GitLab a renvoyé un ticket sans numéro")
	}
	labels := append([]string{}, item.Labels...)
	closed := strings.EqualFold(item.State, "closed")

	status := models.StatusFinished
	trackerStatus := "closed"
	if !closed {
		status = statusFromStageLabels(labels)
		trackerStatus = "opened"
		for _, l := range labels {
			if listLabels[l] {
				trackerStatus = l
				break
			}
		}
	}

	// The macro lives in labels only: on GitLab a milestone is a sprint.
	parentKey, parentTitle := "", ""
	team := ""
	for _, l := range labels {
		low := strings.ToLower(l)
		switch {
		case strings.HasPrefix(low, "macro:"):
			parentTitle = strings.TrimSpace(l[len("macro:"):])
		case strings.HasPrefix(low, "parent:"):
			parentKey = strings.TrimSpace(l[len("parent:"):])
		case team == "" && strings.HasPrefix(low, gitlabTeamPrefix):
			team = strings.TrimSpace(l[len(gitlabTeamPrefix):])
		}
	}

	sprint := ""
	if item.Iteration != nil && item.Iteration.ID > 0 {
		sprint = gitlabIterationName(item.Iteration.Title, item.Iteration.StartDate, item.Iteration.DueDate)
	} else if item.Milestone != nil {
		sprint = strings.TrimSpace(item.Milestone.Title)
	}

	assignee := ""
	if len(item.Assignees) > 0 {
		assignee = item.Assignees[0].Username
	}
	creator, creatorAvatar := "", ""
	if item.Author != nil {
		creator, creatorAvatar = item.Author.Username, item.Author.AvatarURL
	}
	var extURL *string
	if u := strings.TrimSpace(item.WebURL); u != "" {
		extURL = &u
	}
	parentType := ""
	if parentKey != "" || parentTitle != "" {
		parentType = "macro"
	}
	now := time.Now()
	return &models.Task{
		ID:               "gl-" + strconv.Itoa(item.IID),
		Key:              "#" + strconv.Itoa(item.IID),
		Title:            item.Title,
		Description:      item.Description,
		Status:           status,
		Priority:         models.PriorityMedium,
		Labels:           labels,
		Assignee:         assignee,
		Creator:          creator,
		CreatorAvatar:    creatorAvatar,
		TrackerStatus:    trackerStatus,
		Sprint:           sprint,
		Team:             team,
		TeamID:           team,
		Source:           "gitlab",
		ExternalURL:      extURL,
		IssueType:        item.IssueType,
		ParentKey:        parentKey,
		ParentTitle:      parentTitle,
		ParentType:       parentType,
		TrackerCreatedAt: item.CreatedAt,
		TrackerUpdatedAt: item.UpdatedAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

// gitlabIssueIID reads the issue number out of a key: "#12", "12" or a local
// id such as "gl-p1-12".
func gitlabIssueIID(key string) (int, error) {
	s := strings.TrimSpace(key)
	if i := strings.LastIndex(s, "-"); i >= 0 && strings.HasPrefix(strings.ToLower(s), "gl-") {
		s = s[i+1:]
	}
	s = strings.TrimPrefix(s, "#")
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("numéro de ticket GitLab invalide : %q", key)
	}
	return n, nil
}
