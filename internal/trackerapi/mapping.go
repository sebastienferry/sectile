package trackerapi

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"tasks/internal/models"
	"time"
)

type GithubIssueItem struct {
	PullRequest json.RawMessage `json:"pull_request"`
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	URL         string          `json:"url"`
	HTMLURL     string          `json:"html_url"`
	State       string          `json:"state"`
	Milestone   *struct {
		Title  string `json:"title"`
		Number int    `json:"number"`
	} `json:"milestone"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Assignees []struct {
		Login string `json:"login"`
	} `json:"assignees"`
}

func CleanGithubRepo(repo string) string { return models.CleanGithubRepo(repo) }

func githubTask(repo string, item GithubIssueItem) (*models.Task, error) {
	var labels []string
	for _, l := range item.Labels {
		labels = append(labels, l.Name)
	}

	var status models.Status = models.StatusToClarify
	if strings.EqualFold(item.State, "closed") {
		status = models.StatusFinished
	} else {
		for _, l := range labels {
			clean := strings.ToLower(strings.TrimPrefix(l, "#"))
			switch clean {
			case "new", "untouched":
				status = models.StatusToClarify
			case "clarified":
				status = models.StatusClarified
			case "specified":
				status = models.StatusToImplement
			case "implemented":
				status = models.StatusToTest
			case "reviewed":
				status = models.StatusToClose
			case "finished", "closed", "done":
				status = models.StatusFinished
			}
		}
	}

	assignee := ""
	if len(item.Assignees) > 0 {
		assignee = item.Assignees[0].Login
	}

	extURL := item.HTMLURL
	if extURL == "" {
		extURL = item.URL
	}
	if extURL == "" || strings.HasPrefix(extURL, "https://api.github.com/") {
		if repo != "" {
			extURL = fmt.Sprintf("https://github.com/%s/issues/%d", repo, item.Number)
		}
	}

	sprint := ""
	parentTitle := ""
	parentKey := ""
	if item.Milestone != nil && item.Milestone.Title != "" {
		mTitle := item.Milestone.Title
		if strings.HasPrefix(strings.ToLower(mTitle), "sprint") || strings.HasPrefix(strings.ToLower(mTitle), "s-") {
			sprint = mTitle
		} else {
			parentTitle = mTitle
			parentKey = fmt.Sprintf("M-%d", item.Milestone.Number)
		}
	}

	for _, l := range labels {
		low := strings.ToLower(l)
		if strings.HasPrefix(low, "macro:") {
			parentTitle = strings.TrimSpace(l[6:])
		} else if strings.HasPrefix(low, "parent:") {
			parentKey = strings.TrimSpace(l[7:])
		}
	}

	task := &models.Task{
		ID:          fmt.Sprintf("gh-%d", item.Number),
		Key:         fmt.Sprintf("#%d", item.Number),
		Title:       item.Title,
		Description: item.Body,
		Status:      status,
		Priority:    models.PriorityMedium,
		Labels:      labels,
		Assignee:    assignee,
		Sprint:      sprint,
		ParentKey:   parentKey,
		ParentTitle: parentTitle,
		ParentType:  "macro",
		Source:      "github",
		ExternalURL: &extURL,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	return task, nil
}

func cleanGithubIssueNum(keyOrNumber string) (int, error) {
	s := strings.TrimSpace(keyOrNumber)
	s = strings.TrimPrefix(s, "GH-#")
	s = strings.TrimPrefix(s, "gh-")
	s = strings.TrimPrefix(s, "GH-")
	s = strings.TrimPrefix(s, "#")
	return strconv.Atoi(s)
}

func isFinishedStatus(status *models.Status, labels []string) (bool, bool) {
	// returns (isClosed, hasExplicitState)
	if status != nil {
		s := strings.ToLower(string(*status))
		if s == "finished" || s == "done" || s == "closed" || s == "completed" {
			return true, true
		}
		for _, l := range labels {
			clean := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(l), "#"))
			if clean == "finished" || clean == "closed" || clean == "done" {
				return true, true
			}
		}
		return false, true
	}
	for _, l := range labels {
		clean := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(l), "#"))
		if clean == "finished" || clean == "closed" || clean == "done" {
			return true, true
		}
	}
	return false, false
}

type LinearIssueNode struct {
	ID          string `json:"id"`
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Priority    int    `json:"priority"`
	State       struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Type  string `json:"type"`
		Color string `json:"color"`
	} `json:"state"`
	Assignee *struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
		AvatarUrl   string `json:"avatarUrl"`
	} `json:"assignee"`
	Labels *struct {
		Nodes []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type LinearQueryResponse struct {
	Nodes []LinearIssueNode `json:"nodes"`
}

func linearTasks(resp LinearQueryResponse) ([]models.Task, error) {
	var tasks []models.Task
	for i, node := range resp.Nodes {
		var priority models.Priority
		switch node.Priority {
		case 1:
			priority = models.PriorityUrgent
		case 2:
			priority = models.PriorityHigh
		case 3:
			priority = models.PriorityMedium
		default:
			priority = models.PriorityLow
		}

		var status models.Status
		stateName := strings.ToLower(node.State.Name)
		switch {
		case strings.Contains(stateName, "clarif") || strings.Contains(stateName, "triage") || strings.Contains(stateName, "backlog"):
			status = models.StatusToClarify
		case strings.Contains(stateName, "specif") || strings.Contains(stateName, "todo") || strings.Contains(stateName, "unstarted"):
			status = models.StatusClarified
		case strings.Contains(stateName, "progress") || strings.Contains(stateName, "started") || strings.Contains(stateName, "implem"):
			status = models.StatusToImplement
		case strings.Contains(stateName, "test") || strings.Contains(stateName, "valid") || strings.Contains(stateName, "review"):
			status = models.StatusToTest
		case strings.Contains(stateName, "done") || strings.Contains(stateName, "completed") || strings.Contains(stateName, "close") || strings.Contains(stateName, "cancel"):
			status = models.StatusToClose
		default:
			status = models.StatusToClarify
		}

		var labels []string
		if node.Labels != nil {
			for _, l := range node.Labels.Nodes {
				labels = append(labels, l.Name)
			}
		}

		assignee := ""
		if node.Assignee != nil {
			assignee = node.Assignee.Name
			if assignee == "" {
				assignee = node.Assignee.DisplayName
			}
		}

		cTime, _ := time.Parse(time.RFC3339, node.CreatedAt)
		uTime, _ := time.Parse(time.RFC3339, node.UpdatedAt)
		if cTime.IsZero() {
			cTime = time.Now()
		}
		if uTime.IsZero() {
			uTime = time.Now()
		}

		extURL := node.URL

		tasks = append(tasks, models.Task{
			ID:          node.ID,
			Key:         node.Identifier,
			Title:       node.Title,
			Description: node.Description,
			Status:      status,
			Priority:    priority,
			Labels:      labels,
			Assignee:    assignee,
			Position:    i,
			Source:      "linear",
			ExternalURL: &extURL,
			CreatedAt:   cTime,
			UpdatedAt:   uTime,
		})
	}

	return tasks, nil
}

func mapStatusToLinearState(status models.Status) string {
	switch status {
	case models.StatusToClarify, models.StatusBacklog:
		return "Backlog"
	case models.StatusClarified:
		return "Todo"
	case models.StatusToImplement, models.StatusInProgress:
		return "In Progress"
	case models.StatusToTest, models.StatusToValidate:
		return "In Review"
	case models.StatusToClose:
		return "In Review"
	case models.StatusDone, models.StatusFinished:
		return "Done"
	default:
		s := strings.ToLower(string(status))
		if s == "finished" || s == "done" || s == "closed" || s == "completed" {
			return "Done"
		}
		return "Backlog"
	}
}
