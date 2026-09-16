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
	if item.Number < 1 {
		return nil, fmt.Errorf("GitHub returned an invalid issue identity")
	}
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
