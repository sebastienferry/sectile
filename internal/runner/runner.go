package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"tasks/internal/models"
)

type Runner struct{}

func NewRunner() *Runner {
	return &Runner{}
}

func GetDynamicCustomPath() string {
	homeDir, _ := os.UserHomeDir()
	var parts []string
	if homeDir != "" {
		parts = append(parts, filepath.Join(homeDir, ".local", "bin"))
	}
	parts = append(parts, "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin")
	return strings.Join(parts, ":")
}

func FindCliTool(tool string) (string, error) {
	if p, err := exec.LookPath(tool); err == nil {
		return p, nil
	}
	homeDir, _ := os.UserHomeDir()
	var candidates []string
	if homeDir != "" {
		candidates = append(candidates, filepath.Join(homeDir, ".local", "bin", tool))
	}
	candidates = append(candidates, "/opt/homebrew/bin/"+tool, "/usr/local/bin/"+tool, "/usr/bin/"+tool)
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return tool, fmt.Errorf("tool %s not found", tool)
}

// Helper to execute command with timeout and full environment
func (r *Runner) runCommand(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	// Inherit and extend PATH dynamically to include ~/.local/bin and Homebrew paths
	env := os.Environ()
	customPath := GetDynamicCustomPath()
	foundPath := false
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = "PATH=" + customPath + ":" + strings.TrimPrefix(e, "PATH=")
			foundPath = true
			break
		}
	}
	if !foundPath {
		env = append(env, "PATH="+customPath)
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	errOutput := stderr.String()

	if err != nil {
		combined := strings.TrimSpace(output + "\n" + errOutput)
		if combined == "" {
			combined = err.Error()
		}
		return combined, fmt.Errorf("command '%s %s' failed: %w (output: %s)", name, strings.Join(args, " "), err, combined)
	}

	if output == "" && errOutput != "" {
		return errOutput, nil
	}
	return output, nil
}

func (r *Runner) CheckCliTools(repoPath string) []models.CliStatus {
	tools := []string{"git", "gh", "linear", "agy", "vibe", "claude", "gemini", "codex", "uv", "specify", "openspec"}
	var results []models.CliStatus

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tool := range tools {
		path, err := FindCliTool(tool)

		status := models.CliStatus{
			Tool:      tool,
			Available: err == nil,
			Path:      path,
		}

		if status.Available {
			switch tool {
			case "gh":
				out, aErr := r.runCommand(ctx, repoPath, path, "auth", "status")
				if aErr == nil || strings.Contains(out, "Logged in to") {
					status.AuthStatus = "Authenticated"
					status.Details = "GitHub CLI connected"
				} else {
					status.AuthStatus = "Not Authenticated"
					status.Details = "Run 'gh auth login'"
				}
			case "linear":
				out, aErr := r.runCommand(ctx, repoPath, path, "auth", "whoami")
				if aErr == nil && strings.Contains(out, "Workspace:") {
					status.AuthStatus = "Authenticated"
					status.Details = "Linear connected"
				} else {
					status.AuthStatus = "Not Authenticated"
					status.Details = "Run 'linear auth login'"
				}
			case "uv":
				status.AuthStatus = "Ready"
				status.Details = "uv available (required to install GitHub Spec Kit)"
			case "specify":
				status.AuthStatus = "Ready"
				status.Details = "GitHub Spec Kit CLI installed"
			case "openspec":
				status.AuthStatus = "Ready"
				status.Details = "OpenSpec CLI installed"
			case "git":
				status.AuthStatus = "Ready"
				status.Details = "Git available"
			case "agy":
				status.AuthStatus = "Ready"
				status.Details = "Antigravity CLI Agent ready"
			case "vibe":
				status.AuthStatus = "Ready"
				status.Details = "Mistral Vibe CLI Agent ready"
			case "claude":
				status.AuthStatus = "Ready"
				status.Details = "Claude Code CLI Agent ready"
			case "gemini":
				status.AuthStatus = "Ready"
				status.Details = "Gemini CLI Agent ready"
			case "codex":
				status.AuthStatus = "Ready"
				status.Details = "Codex CLI Agent ready"
			}
		} else {
			status.AuthStatus = "Not Installed"
			switch tool {
			case "uv":
				status.Details = "uv missing — curl -LsSf https://astral.sh/uv/install.sh | sh"
			case "specify":
				status.Details = "GitHub Spec Kit missing — install it from a project (Spec Kit / OpenSpec panel)"
			case "openspec":
				status.Details = "OpenSpec missing — install it from a project (Spec Kit / OpenSpec panel)"
			default:
				status.Details = fmt.Sprintf("Tool '%s' not found in PATH", tool)
			}
		}

		results = append(results, status)
	}

	return results
}

// Linear API Data Models
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

func (r *Runner) SyncFromLinear(teamKey string) ([]models.Task, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	linearPath, _ := FindCliTool("linear")
	if linearPath == "" {
		linearPath = "linear"
	}

	var args []string
	if teamKey != "" {
		args = []string{"issue", "query", "--team", teamKey, "--json"}
	} else {
		args = []string{"issue", "query", "--json"}
	}

	output, err := r.runCommand(ctx, "", linearPath, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query Linear: %w (output: %s)", err, output)
	}

	var resp LinearQueryResponse
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Linear JSON response: %w", err)
	}

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
			status = models.StatusToSpecify
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

type LinearCreateOutput struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	URL        string `json:"url"`
	Title      string `json:"title"`
}

var validLinearLabels = map[string]string{
	"new":          "New",
	"to-clarify":   "to-clarify",
	"toclarify":    "to-clarify",
	"to_clarify":   "to-clarify",
	"clarified":    "clarified",
	"clarify":      "clarified",
	"specified":    "specified",
	"specify":      "specified",
	"to-specify":   "specified",
	"to_specify":   "specified",
	"implemented":  "Implemented",
	"implement":    "Implemented",
	"to-implement": "Implemented",
	"to_implement": "Implemented",
	"handoff":      "finished",
	"finished":     "finished",
	"reviewed":     "Reviewed",
	"review":       "Reviewed",
	"to-review":    "Reviewed",
	"to_review":    "Reviewed",
	"to-test":      "Reviewed",
	"to_test":      "Reviewed",
	"to-close":     "Reviewed",
	"to_close":     "Reviewed",
	"validate":     "validate",
	"validated":    "validated",
	"design":       "design",
	"enhancement":  "enhancement",
	"migrated":     "Migrated",
	"milestone":    "milestone",
	"improvement":  "Improvement",
	"feature":      "Feature",
	"bug":          "Bug",
	"pinned":       "pinned",
	"Pinned":       "pinned",
}

func filterLinearLabels(labels []string) []string {
	var valid []string
	seen := make(map[string]bool)
	for _, l := range labels {
		cleaned := strings.ToLower(strings.TrimSpace(l))
		if target, ok := validLinearLabels[cleaned]; ok {
			if !seen[target] {
				seen[target] = true
				valid = append(valid, target)
			}
		}
	}
	return valid
}

func (r *Runner) CreateLinearIssue(teamKey string, title string, description string, priority models.Priority, labels []string) (*models.Task, error) {
	if teamKey == "" {
		teamKey = "FRE"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	linearPath, err := exec.LookPath("linear")
	if err != nil {
		linearPath = "/opt/homebrew/bin/linear"
	}

	priorityNum := "3"
	switch priority {
	case models.PriorityUrgent:
		priorityNum = "1"
	case models.PriorityHigh:
		priorityNum = "2"
	case models.PriorityMedium:
		priorityNum = "3"
	case models.PriorityLow:
		priorityNum = "4"
	}

	filteredLabels := filterLinearLabels(labels)

	args := []string{"issue", "create", "--team", teamKey, "--title", title, "-p", priorityNum, "--no-interactive"}
	if description != "" {
		args = append(args, "-d", description)
	}
	for _, l := range filteredLabels {
		if l != "" {
			args = append(args, "-l", l)
		}
	}

	output, err := r.runCommand(ctx, "", linearPath, args...)
	if err != nil {
		// Fallback retry without labels if a label doesn't exist on Linear workspace
		retryArgs := []string{"issue", "create", "--team", teamKey, "--title", title, "-p", priorityNum, "--no-interactive"}
		if description != "" {
			retryArgs = append(retryArgs, "-d", description)
		}
		retryOutput, retryErr := r.runCommand(ctx, "", linearPath, retryArgs...)
		if retryErr != nil {
			return nil, fmt.Errorf("linear issue create failed: %w (output: %s)", err, output)
		}
		output = retryOutput
	}

	urlRe := regexp.MustCompile(`https?://linear\.app/\S+`)
	urlMatch := urlRe.FindString(output)

	keyRe := regexp.MustCompile(`([A-Z0-9]+-\d+)`)
	keyMatch := keyRe.FindString(output)
	if keyMatch == "" {
		keyMatch = fmt.Sprintf("%s-new", teamKey)
	}

	var extURL *string
	if urlMatch != "" {
		cleanURL := strings.TrimSpace(urlMatch)
		extURL = &cleanURL
	}

	now := time.Now()
	id := uuid.New().String()

	return &models.Task{
		ID:          id,
		Key:         keyMatch,
		Title:       title,
		Description: description,
		Status:      models.StatusToClarify,
		Priority:    priority,
		Labels:      labels,
		Source:      "linear",
		ExternalURL: extURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func mapStatusToLinearState(status models.Status) string {
	switch status {
	case models.StatusToClarify, models.StatusBacklog:
		return "Backlog"
	case models.StatusToSpecify, models.StatusSpecified:
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

func (r *Runner) UpdateLinearIssueState(issueKey string, status models.Status) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	linearPath, err := exec.LookPath("linear")
	if err != nil {
		linearPath = "/opt/homebrew/bin/linear"
	}

	stateName := mapStatusToLinearState(status)
	output, err := r.runCommand(ctx, "", linearPath, "issue", "update", issueKey, "--state", stateName)
	if err != nil {
		log.Printf("[CLI] linear issue update %s --state '%s' failed: %v (output: %s)", issueKey, stateName, err, output)
	}
	return err
}

func (r *Runner) UpdateLinearIssue(issueKey string, title *string, description *string, priority *models.Priority, status *models.Status, labels []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	linearPath, err := exec.LookPath("linear")
	if err != nil {
		linearPath = "/opt/homebrew/bin/linear"
	}

	filteredLabels := filterLinearLabels(labels)

	buildArgs := func(withLabels bool) []string {
		args := []string{"issue", "update", issueKey}
		if title != nil && *title != "" {
			args = append(args, "--title", *title)
		}
		if description != nil {
			args = append(args, "--description", *description)
		}
		if priority != nil {
			switch *priority {
			case models.PriorityUrgent:
				args = append(args, "--priority", "1")
			case models.PriorityHigh:
				args = append(args, "--priority", "2")
			case models.PriorityMedium:
				args = append(args, "--priority", "3")
			case models.PriorityLow:
				args = append(args, "--priority", "4")
			}
		}
		if status != nil {
			args = append(args, "--state", mapStatusToLinearState(*status))
		}
		if withLabels {
			for _, l := range filteredLabels {
				if l != "" {
					args = append(args, "-l", l)
				}
			}
		}
		return args
	}

	args := buildArgs(len(filteredLabels) > 0)
	output, err := r.runCommand(ctx, "", linearPath, args...)
	if err != nil {
		log.Printf("[CLI] linear issue update %s with labels failed: %v (output: %s), retrying without labels...", issueKey, err, output)
		retryArgs := buildArgs(false)
		output, err = r.runCommand(ctx, "", linearPath, retryArgs...)
		if err != nil {
			log.Printf("[CLI] linear issue update %s without labels also failed: %v (output: %s)", issueKey, err, output)
		}
	}
	return err
}

// GitHub structures
type GithubIssueItem struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	URL     string `json:"url"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Milestone *struct {
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

// CleanGithubRepo extracts the owner/repo string from a Git URL or plain repo identifier.
func CleanGithubRepo(repo string) string {
	raw := strings.TrimSpace(repo)
	raw = strings.TrimSuffix(raw, ".git")
	if strings.Contains(raw, "github.com/") {
		parts := strings.Split(raw, "github.com/")
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
	} else if strings.Contains(raw, "github.com:") {
		parts := strings.Split(raw, "github.com:")
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
	}
	if strings.Contains(raw, "/") && !strings.Contains(raw, ":") && !strings.Contains(raw, " ") {
		return raw
	}
	return raw
}

// ResolveGithubRepo attempts to determine the target repository and path dynamically.
func ResolveGithubRepo(repo string, repoPath string) (string, string) {
	if repoPath == "" {
		repoPath = "."
	}
	if repo != "" {
		return CleanGithubRepo(repo), repoPath
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "config", "--get", "remote.origin.url")
	if out, err := cmd.Output(); err == nil {
		raw := strings.TrimSpace(string(out))
		cleaned := CleanGithubRepo(raw)
		if cleaned != "" {
			return cleaned, repoPath
		}
	}
	return "", repoPath
}

func (r *Runner) SyncFromGithub(repo string, repoPath string) ([]models.Task, error) {
	repo, repoPath = ResolveGithubRepo(repo, repoPath)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ghPath, _ := FindCliTool("gh")
	if ghPath == "" {
		ghPath = "gh"
	}

	endpoint := "repos/" + repo + "/issues?per_page=100&state=all"
	output, err := r.runCommand(ctx, repoPath, ghPath, "api", "--paginate", endpoint)
	if err != nil {
		// Fallback to gh issue list if gh api repos/repo/issues fails
		var fallbackArgs []string
		if repo != "" {
			fallbackArgs = []string{"issue", "list", "-R", repo, "--limit", "1000", "--state", "all", "--json", "number,title,labels,body,url,state,assignees"}
		} else {
			fallbackArgs = []string{"issue", "list", "--limit", "1000", "--state", "all", "--json", "number,title,labels,body,url,state,assignees"}
		}
		fallbackOutput, fallbackErr := r.runCommand(ctx, repoPath, ghPath, fallbackArgs...)
		if fallbackErr != nil {
			return nil, fmt.Errorf("failed to query GitHub issues via API or CLI: %w (output: %s)", err, output)
		}
		output = fallbackOutput
	}

	var items []GithubIssueItem
	// Stream decoding for multiple paginated JSON arrays or single array
	dec := json.NewDecoder(strings.NewReader(output))
	for dec.More() {
		var pageItems []GithubIssueItem
		if decErr := dec.Decode(&pageItems); decErr != nil {
			break
		}
		items = append(items, pageItems...)
	}

	if len(items) == 0 && len(output) > 0 {
		if unmarshalErr := json.Unmarshal([]byte(output), &items); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse GitHub JSON: %w", unmarshalErr)
		}
	}

	var tasks []models.Task
	for i, item := range items {
		// Skip pull requests returned by /repos/{owner}/{repo}/issues API endpoint
		if strings.Contains(item.URL, "/pull/") {
			continue
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
					status = models.StatusToSpecify
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

		tasks = append(tasks, models.Task{
			ID:          fmt.Sprintf("gh-%d", item.Number),
			Key:         fmt.Sprintf("#%d", item.Number),
			Title:       item.Title,
			Description: item.Body,
			Status:      status,
			Priority:    models.PriorityMedium,
			Labels:      labels,
			Assignee:    assignee,
			Position:    i,
			Sprint:      sprint,
			ParentKey:   parentKey,
			ParentTitle: parentTitle,
			ParentType:  "macro",
			Source:      "github",
			ExternalURL: &extURL,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		})
	}

	return tasks, nil
}

func (r *Runner) FetchSingleGithubIssue(repo string, repoPath string, issueNumber int) (*models.Task, error) {
	repo, repoPath = ResolveGithubRepo(repo, repoPath)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	ghPath, _ := FindCliTool("gh")
	if ghPath == "" {
		ghPath = "gh"
	}

	endpoint := fmt.Sprintf("repos/%s/issues/%d", repo, issueNumber)
	output, err := r.runCommand(ctx, repoPath, ghPath, "api", endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch GitHub issue #%d: %w", issueNumber, err)
	}

	var item GithubIssueItem
	if err := json.Unmarshal([]byte(output), &item); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub issue JSON: %w", err)
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
				status = models.StatusToSpecify
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

func (r *Runner) CreateGithubIssue(repo string, repoPath string, title string, description string, labels []string) (*models.Task, error) {
	repo, repoPath = ResolveGithubRepo(repo, repoPath)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	ghPath, _ := FindCliTool("gh")
	if ghPath == "" {
		ghPath = "gh"
	}

	// Filter internal workflow labels (like "New", "untouched")
	var filteredLabels []string
	for _, l := range labels {
		l = strings.TrimSpace(l)
		if l != "" && !strings.EqualFold(l, "new") && !strings.EqualFold(l, "untouched") {
			filteredLabels = append(filteredLabels, l)
		}
	}

	payload := map[string]interface{}{
		"title": title,
		"body":  description,
	}
	if len(filteredLabels) > 0 {
		payload["labels"] = filteredLabels
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode GitHub issue payload: %w", err)
	}

	tmpFile, tmpErr := os.CreateTemp("", "gh-create-*.json")
	if tmpErr != nil {
		return nil, fmt.Errorf("failed to create temporary payload file: %w", tmpErr)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	_, _ = tmpFile.Write(payloadBytes)
	_ = tmpFile.Close()

	endpoint := "repos/" + repo + "/issues"
	output, err := r.runCommand(ctx, repoPath, ghPath, "api", "-X", "POST", endpoint, "--input", tmpPath)
	if err != nil {
		// Fallback to CLI issue create
		var fallbackArgs []string
		if repo != "" {
			fallbackArgs = []string{"issue", "create", "-R", repo, "--title", title, "--body", description}
		} else {
			fallbackArgs = []string{"issue", "create", "--title", title, "--body", description}
		}
		for _, l := range filteredLabels {
			fallbackArgs = append(fallbackArgs, "--label", l)
		}
		fallbackOutput, fallbackErr := r.runCommand(ctx, repoPath, ghPath, fallbackArgs...)
		if fallbackErr != nil {
			return nil, fmt.Errorf("gh issue create API failed: %w (output: %s)", err, output)
		}
		output = fallbackOutput
	}

	var res struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
		URL     string `json:"url"`
	}
	_ = json.Unmarshal([]byte(output), &res)

	issueNum := strconv.Itoa(res.Number)
	issueURL := res.HTMLURL
	if issueURL == "" {
		issueURL = res.URL
	}
	if issueNum == "0" {
		re := regexp.MustCompile(`https://github\.com/[^/\s]+/[^/\s]+/issues/(\d+)`)
		matches := re.FindStringSubmatch(output)
		if len(matches) > 1 {
			issueURL = matches[0]
			issueNum = matches[1]
		}
	}

	key := fmt.Sprintf("#%s", issueNum)
	id := fmt.Sprintf("gh-%s", issueNum)

	now := time.Now()
	return &models.Task{
		ID:          id,
		Key:         key,
		Title:       title,
		Description: description,
		Status:      models.StatusToClarify,
		Priority:    models.PriorityMedium,
		Labels:      labels,
		Source:      "github",
		ExternalURL: &issueURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
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

func (r *Runner) UpdateGithubIssueState(repo string, repoPath string, keyOrNumber string, status models.Status) error {
	repo, repoPath = ResolveGithubRepo(repo, repoPath)

	num, err := cleanGithubIssueNum(keyOrNumber)
	if err != nil {
		return fmt.Errorf("invalid github issue number: %s", keyOrNumber)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ghPath, _ := FindCliTool("gh")
	if ghPath == "" {
		ghPath = "gh"
	}

	state := "open"
	isClosed, _ := isFinishedStatus(&status, nil)
	if isClosed {
		state = "closed"
	}

	endpoint := fmt.Sprintf("repos/%s/issues/%d", repo, num)
	payloadBytes, _ := json.Marshal(map[string]interface{}{
		"state": state,
	})

	tmpFile, tmpErr := os.CreateTemp("", "gh-state-*.json")
	if tmpErr == nil {
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)
		_, _ = tmpFile.Write(payloadBytes)
		_ = tmpFile.Close()

		_, err = r.runCommand(ctx, repoPath, ghPath, "api", "-X", "PATCH", endpoint, "--input", tmpPath)
		if err == nil {
			return nil
		}
	}

	// Fallback to CLI
	var args []string
	if state == "closed" {
		args = []string{"issue", "close", strconv.Itoa(num)}
	} else {
		args = []string{"issue", "reopen", strconv.Itoa(num)}
	}
	if repo != "" {
		args = append(args, "-R", repo)
	}
	_, err = r.runCommand(ctx, repoPath, ghPath, args...)
	return err
}

func (r *Runner) UpdateGithubIssue(repo string, repoPath string, keyOrNumber string, title *string, description *string, status *models.Status, labels []string, removedLabels []string) error {
	repo, repoPath = ResolveGithubRepo(repo, repoPath)

	num, err := cleanGithubIssueNum(keyOrNumber)
	if err != nil {
		return fmt.Errorf("invalid github issue number: %s", keyOrNumber)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ghPath, _ := FindCliTool("gh")
	if ghPath == "" {
		ghPath = "gh"
	}

	isClosed, hasState := isFinishedStatus(status, labels)
	state := "open"
	if isClosed {
		state = "closed"
	}

	// 1. Update issue via REST API (PATCH repos/{owner}/{repo}/issues/{issue_number})
	payload := make(map[string]interface{})
	if hasState {
		payload["state"] = state
	}
	if title != nil && *title != "" {
		payload["title"] = *title
	}
	if description != nil {
		payload["body"] = *description
	}
	if len(labels) > 0 {
		var filtered []string
		for _, l := range labels {
			cleanL := strings.TrimPrefix(strings.TrimSpace(l), "#")
			if cleanL != "" {
				filtered = append(filtered, cleanL)
			}
		}
		if len(filtered) > 0 {
			payload["labels"] = filtered
		}
	}

	if len(payload) > 0 {
		payloadBytes, _ := json.Marshal(payload)
		tmpFile, tmpErr := os.CreateTemp("", "gh-edit-*.json")
		if tmpErr == nil {
			tmpPath := tmpFile.Name()
			defer os.Remove(tmpPath)
			_, _ = tmpFile.Write(payloadBytes)
			_ = tmpFile.Close()

			endpoint := fmt.Sprintf("repos/%s/issues/%d", repo, num)
			_, err := r.runCommand(ctx, repoPath, ghPath, "api", "-X", "PATCH", endpoint, "--input", tmpPath)
			if err == nil {
				return nil
			}
			log.Printf("[API] gh api PATCH %s failed: %v, falling back to CLI", endpoint, err)
		}
	}

	// Fallback gh issue edit CLI call
	var args []string
	if repo != "" {
		args = []string{"issue", "edit", strconv.Itoa(num), "-R", repo}
	} else {
		args = []string{"issue", "edit", strconv.Itoa(num)}
	}
	baseArgsLen := len(args)

	if title != nil && *title != "" {
		args = append(args, "--title", *title)
	}
	if description != nil {
		args = append(args, "--body", *description)
	}
	for _, l := range labels {
		cleanL := strings.TrimLeft(strings.TrimSpace(l), "#")
		if cleanL != "" {
			args = append(args, "--add-label", cleanL)
		}
	}
	for _, rl := range removedLabels {
		cleanRL := strings.TrimLeft(strings.TrimSpace(rl), "#")
		if cleanRL != "" {
			args = append(args, "--remove-label", cleanRL)
		}
	}

	// Only invoke gh issue edit if there are actual fields/flags to edit
	if len(args) > baseArgsLen {
		_, _ = r.runCommand(ctx, repoPath, ghPath, args...)
	}

	// Also close or reopen via CLI fallback if state changed
	if hasState {
		var stateArgs []string
		if state == "closed" {
			stateArgs = []string{"issue", "close", strconv.Itoa(num)}
		} else {
			stateArgs = []string{"issue", "reopen", strconv.Itoa(num)}
		}
		if repo != "" {
			stateArgs = append(stateArgs, "-R", repo)
		}
		_, _ = r.runCommand(ctx, repoPath, ghPath, stateArgs...)
	}

	return nil
}

// -------------------------------------------------------------
// GITHUB MILESTONES (MACROS) INTEGRATION
// -------------------------------------------------------------

type GithubMilestoneItem struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       string `json:"state"`
}

func (r *Runner) CreateGithubMilestone(repo string, repoPath string, title string, description string) (int, error) {
	repo, repoPath = ResolveGithubRepo(repo, repoPath)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	ghPath, _ := FindCliTool("gh")
	if ghPath == "" {
		ghPath = "gh"
	}

	endpoint := fmt.Sprintf("repos/%s/milestones", repo)
	output, err := r.runCommand(ctx, repoPath, ghPath, "api", endpoint, "-X", "POST", "-f", "title="+title, "-f", "description="+description)
	if err != nil {
		return 0, fmt.Errorf("failed to create GitHub milestone: %w (output: %s)", err, output)
	}

	var res struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal([]byte(output), &res); err != nil {
		return 0, nil
	}
	return res.Number, nil
}

func (r *Runner) DeleteGithubMilestone(repo string, repoPath string, milestoneNumber int) error {
	repo, repoPath = ResolveGithubRepo(repo, repoPath)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	ghPath, _ := FindCliTool("gh")
	if ghPath == "" {
		ghPath = "gh"
	}

	endpoint := fmt.Sprintf("repos/%s/milestones/%d", repo, milestoneNumber)
	output, err := r.runCommand(ctx, repoPath, ghPath, "api", endpoint, "-X", "DELETE")
	if err != nil {
		return fmt.Errorf("failed to delete GitHub milestone: %w (output: %s)", err, output)
	}
	return nil
}

func (r *Runner) UpdateGithubMilestone(repo string, repoPath string, milestoneNumber int, title string, description string, state string) error {
	repo, repoPath = ResolveGithubRepo(repo, repoPath)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	ghPath, _ := FindCliTool("gh")
	if ghPath == "" {
		ghPath = "gh"
	}

	endpoint := fmt.Sprintf("repos/%s/milestones/%d", repo, milestoneNumber)
	args := []string{"api", endpoint, "-X", "PATCH"}
	if title != "" {
		args = append(args, "-f", "title="+title)
	}
	if description != "" {
		args = append(args, "-f", "description="+description)
	}
	if state != "" {
		args = append(args, "-f", "state="+state)
	}

	output, err := r.runCommand(ctx, repoPath, ghPath, args...)
	if err != nil {
		return fmt.Errorf("failed to update GitHub milestone: %w (output: %s)", err, output)
	}
	return nil
}

func (r *Runner) ListGithubMilestones(repo string, repoPath string) ([]GithubMilestoneItem, error) {
	repo, repoPath = ResolveGithubRepo(repo, repoPath)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	ghPath, _ := FindCliTool("gh")
	if ghPath == "" {
		ghPath = "gh"
	}

	endpoint := fmt.Sprintf("repos/%s/milestones?state=all&per_page=100", repo)
	output, err := r.runCommand(ctx, repoPath, ghPath, "api", endpoint, "--paginate")
	if err != nil {
		return nil, fmt.Errorf("failed to list GitHub milestones: %w (output: %s)", err, output)
	}

	var items []GithubMilestoneItem
	dec := json.NewDecoder(strings.NewReader(output))
	for dec.More() {
		var page []GithubMilestoneItem
		if err := dec.Decode(&page); err != nil {
			break
		}
		items = append(items, page...)
	}
	if len(items) == 0 && len(output) > 0 {
		_ = json.Unmarshal([]byte(output), &items)
	}
	return items, nil
}

func (r *Runner) SetGithubIssueMilestone(repo string, repoPath string, issueNumber int, milestoneTitleOrNumber string) error {
	repo, repoPath = ResolveGithubRepo(repo, repoPath)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	ghPath, _ := FindCliTool("gh")
	if ghPath == "" {
		ghPath = "gh"
	}

	target := strings.TrimSpace(milestoneTitleOrNumber)
	var args []string
	if target == "" || target == "0" || strings.EqualFold(target, "null") || strings.EqualFold(target, "none") {
		args = []string{"issue", "edit", fmt.Sprintf("%d", issueNumber), "-R", repo, "--remove-milestone"}
	} else {
		args = []string{"issue", "edit", fmt.Sprintf("%d", issueNumber), "-R", repo, "--milestone", target}
	}
	output, err := r.runCommand(ctx, repoPath, ghPath, args...)
	if err != nil {
		// Fallback: If gh issue edit fails, try direct REST API
		if target == "" || target == "0" || strings.EqualFold(target, "null") || strings.EqualFold(target, "none") {
			apiArgs := []string{"api", fmt.Sprintf("repos/%s/issues/%d", repo, issueNumber), "-X", "PATCH", "-F", "milestone=null"}
			if _, apiErr := r.runCommand(ctx, repoPath, ghPath, apiArgs...); apiErr == nil {
				return nil
			}
		}
		return fmt.Errorf("failed to set GitHub issue milestone: %w (output: %s)", err, output)
	}
	return nil
}

func (r *Runner) TransferGithubIssue(sourceRepo string, sourceRepoPath string, issueNumber int, targetRepo string, targetRepoPath string) (int, string, error) {
	sourceRepo, sourceRepoPath = ResolveGithubRepo(sourceRepo, sourceRepoPath)
	targetRepo, targetRepoPath = ResolveGithubRepo(targetRepo, targetRepoPath)

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	ghPath, _ := FindCliTool("gh")
	if ghPath == "" {
		ghPath = "gh"
	}

	output, err := r.runCommand(ctx, sourceRepoPath, ghPath, "issue", "transfer", fmt.Sprintf("%d", issueNumber), targetRepo, "-R", sourceRepo)
	if err != nil {
		return 0, "", fmt.Errorf("erreur transfert issue GitHub #%d vers %s: %w (output: %s)", issueNumber, targetRepo, err, output)
	}

	newUrl := strings.TrimSpace(output)
	newNum := issueNumber
	if idx := strings.LastIndex(newUrl, "/"); idx != -1 {
		if n, parseErr := strconv.Atoi(strings.TrimSpace(newUrl[idx+1:])); parseErr == nil && n > 0 {
			newNum = n
		}
	}
	return newNum, newUrl, nil
}

// -------------------------------------------------------------
// JIRA CLI (acli) INTEGRATION
// -------------------------------------------------------------

// jiraSearchFields is the exact set of fields acli accepts for
// 'jira workitem search --fields'. Notably 'created' and 'updated' are
// rejected by the CLI, so task timestamps fall back to the import time.
const jiraSearchFields = "key,summary,description,status,priority,assignee,labels,issuetype"

// NormalizeIssueTypes cleans a configured list of work item types.
func NormalizeIssueTypes(types []string) []string {
	out := make([]string, 0, len(types))
	seen := map[string]bool{}
	for _, t := range types {
		t = strings.TrimSpace(t)
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
	}
	if len(out) == 0 {
		return []string{"Task", "Story"}
	}
	return out
}

func (r *Runner) AddIssueComment(source string, repo string, repoPath string, key string, body string) error {
	if body == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if source == "linear" {
		linPath, _ := FindCliTool("linear")
		if linPath == "" {
			linPath = "linear"
		}
		output, err := r.runCommand(ctx, repoPath, linPath, "issue", "comment", "add", key, "--body", body)
		if err != nil {
			log.Printf("[CLI] linear issue comment add %s failed: %v (output: %s)", key, err, output)
		}
		return err

	} else if source == "github" || strings.HasPrefix(key, "#") || strings.HasPrefix(key, "GH-#") || strings.HasPrefix(key, "gh-") {
		repo, repoPath = ResolveGithubRepo(repo, repoPath)

		ghPath, _ := FindCliTool("gh")
		if ghPath == "" {
			ghPath = "gh"
		}

		num, err := cleanGithubIssueNum(key)
		if err != nil {
			return fmt.Errorf("invalid github issue number: %s", key)
		}

		payloadBytes, _ := json.Marshal(map[string]string{
			"body": body,
		})

		tmpFile, tmpErr := os.CreateTemp("", "gh-comment-*.json")
		if tmpErr == nil {
			tmpPath := tmpFile.Name()
			defer os.Remove(tmpPath)
			_, _ = tmpFile.Write(payloadBytes)
			_ = tmpFile.Close()

			endpoint := fmt.Sprintf("repos/%s/issues/%d/comments", repo, num)
			output, err := r.runCommand(ctx, repoPath, ghPath, "api", "-X", "POST", endpoint, "--input", tmpPath)
			if err == nil {
				log.Printf("[API] Comment added to GitHub %s via REST API: %s", key, strings.TrimSpace(output))
				return nil
			}
			log.Printf("[API] gh api comment failed: %v (output: %s), falling back to CLI...", err, output)
		}

		cleanNum := strconv.Itoa(num)
		var args []string
		if repo != "" {
			args = []string{"issue", "comment", cleanNum, "-R", repo, "--body", body}
		} else {
			args = []string{"issue", "comment", cleanNum, "--body", body}
		}
		output, err := r.runCommand(ctx, repoPath, ghPath, args...)
		if err != nil {
			log.Printf("[CLI] gh issue comment %s failed: %v (output: %s)", key, err, output)
		}
		return err
	}
	return nil
}

func (r *Runner) GetGithubIssueComments(repo string, repoPath string, key string) ([]models.TaskComment, error) {
	repo, repoPath = ResolveGithubRepo(repo, repoPath)
	num, err := cleanGithubIssueNum(key)
	if err != nil {
		return nil, fmt.Errorf("invalid github issue number: %s", key)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ghPath, _ := FindCliTool("gh")
	if ghPath == "" {
		ghPath = "gh"
	}

	cleanNum := strconv.Itoa(num)
	var args []string
	if repo != "" {
		args = []string{"issue", "view", cleanNum, "-R", repo, "--json", "comments"}
	} else {
		args = []string{"issue", "view", cleanNum, "--json", "comments"}
	}

	output, err := r.runCommand(ctx, repoPath, ghPath, args...)
	if err != nil {
		// Fallback to gh api
		endpoint := fmt.Sprintf("repos/%s/issues/%d/comments", repo, num)
		var apiArgs []string
		if repo != "" {
			apiArgs = []string{"api", endpoint, "-R", repo}
		} else {
			apiArgs = []string{"api", endpoint}
		}
		output, err = r.runCommand(ctx, repoPath, ghPath, apiArgs...)
		if err != nil {
			return nil, fmt.Errorf("gh comments failed: %w (output: %s)", err, output)
		}
		var apiComments []struct {
			ID        int64  `json:"id"`
			NodeID    string `json:"node_id"`
			Body      string `json:"body"`
			CreatedAt string `json:"created_at"`
			User      struct {
				Login string `json:"login"`
			} `json:"user"`
		}
		if jsonErr := json.Unmarshal([]byte(output), &apiComments); jsonErr == nil {
			var comments []models.TaskComment
			for _, c := range apiComments {
				var t *time.Time
				if parsed, pErr := time.Parse(time.RFC3339, c.CreatedAt); pErr == nil {
					t = &parsed
				}
				idStr := strconv.FormatInt(c.ID, 10)
				if c.NodeID != "" {
					idStr = c.NodeID
				}
				comments = append(comments, models.TaskComment{
					ID:        idStr,
					Author:    c.User.Login,
					Body:      c.Body,
					CreatedAt: t,
					Source:    "github",
				})
			}
			return comments, nil
		}
		return nil, err
	}

	var resp struct {
		Comments []struct {
			ID        string `json:"id"`
			Body      string `json:"body"`
			CreatedAt string `json:"createdAt"`
			Author    struct {
				Login string `json:"login"`
			} `json:"author"`
		} `json:"comments"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal github comments failed: %w", err)
	}

	var comments []models.TaskComment
	for _, c := range resp.Comments {
		var t *time.Time
		if parsed, pErr := time.Parse(time.RFC3339, c.CreatedAt); pErr == nil {
			t = &parsed
		}
		comments = append(comments, models.TaskComment{
			ID:        c.ID,
			Author:    c.Author.Login,
			Body:      c.Body,
			CreatedAt: t,
			Source:    "github",
		})
	}
	return comments, nil
}

func (r *Runner) GetLinearIssueComments(repoPath string, key string) ([]models.TaskComment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	linPath, _ := FindCliTool("linear")
	if linPath == "" {
		linPath = "linear"
	}

	output, err := r.runCommand(ctx, repoPath, linPath, "issue", "view", key, "--json")
	if err != nil {
		return nil, fmt.Errorf("linear issue view failed: %w", err)
	}

	var resp struct {
		Comments struct {
			Nodes []struct {
				ID        string `json:"id"`
				Body      string `json:"body"`
				CreatedAt string `json:"createdAt"`
				User      struct {
					Name        string `json:"name"`
					DisplayName string `json:"displayName"`
				} `json:"user"`
			} `json:"nodes"`
		} `json:"comments"`
		CommentsList []struct {
			ID        string `json:"id"`
			Body      string `json:"body"`
			CreatedAt string `json:"createdAt"`
			User      struct {
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
			} `json:"user"`
		} `json:"commentsList"`
	}
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		return nil, fmt.Errorf("parse linear comments failed: %w", err)
	}

	var comments []models.TaskComment
	nodes := resp.Comments.Nodes
	if len(nodes) == 0 && len(resp.CommentsList) > 0 {
		nodes = resp.CommentsList
	}
	for _, n := range nodes {
		author := n.User.DisplayName
		if author == "" {
			author = n.User.Name
		}
		var t *time.Time
		if parsed, pErr := time.Parse(time.RFC3339, n.CreatedAt); pErr == nil {
			t = &parsed
		}
		comments = append(comments, models.TaskComment{
			ID:        n.ID,
			Author:    author,
			Body:      n.Body,
			CreatedAt: t,
			Source:    "linear",
		})
	}
	return comments, nil
}

// installedSkillPath returns the SKILL.md of a workflow skill inside a checkout,
// whichever agent directory holds it. Empty when the skill is not installed.
func installedSkillPath(repoDir, skillID string) string {
	dirName := models.SkillDirNames[skillID]
	if dirName == "" || repoDir == "" {
		return ""
	}

	// La commande slash d'abord : c'est elle qui rend « /clarify-issue »
	// invocable. Une skill seule est choisie par le modèle, jamais appelée par
	// son nom, et le prompt se contentait alors d'être recopié.
	cmdPath := filepath.Join(repoDir, ".claude", "commands", dirName+".md")
	if fi, err := os.Stat(cmdPath); err == nil && !fi.IsDir() {
		return cmdPath
	}

	for _, agent := range models.SkillAgentDirs {
		var p string
		if agent == "" {
			p = filepath.Join(repoDir, ".skills", dirName, "SKILL.md")
		} else {
			p = filepath.Join(repoDir, agent, "skills", dirName, "SKILL.md")
		}
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// skillSlashPrompt is the unified invocation: the agent runs the skill installed
// in the repository, and the ticket context follows. The instructions then live
// in one place, the SKILL.md rendered from the in-app editor, instead of being
// written twice: once in a file, once in a prompt here.
func skillSlashPrompt(skillID string) string {
	dirName := models.SkillDirNames[skillID]
	return "/" + dirName + ` {issueKey}

Contexte du ticket
Clé : {issueKey}
Titre : {issueTitle}
Description : {issueDesc}
Branche Git : {branchName}
Dossier du projet : {repoPath}
Tracker : {tracker}`
}

// AIInvocation is everything needed to run one workflow step: where, with which
// engine, and with which prompt. Splitting it out of RunAI is what lets the same
// invocation run either headless or inside a PTY session the user can open.
type AIInvocation struct {
	RepoDir  string
	Provider string
	Template string // modèle de commande du projet, placeholders déjà résolus
	Prompt   string
	Steps    []string
}

// RunAI keeps the headless path: pipes, no terminal to attach to. It stays the
// fallback for when no session is available.
func (r *Runner) RunAI(settings *models.Settings, skillID string, task *models.Task, customPrompt string) (string, []string, error) {
	inv, err := r.PrepareAI(settings, skillID, task, customPrompt)
	if err != nil {
		return "", nil, err
	}
	steps := inv.Steps

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	output, execSteps, execErr := r.execAgentCommand(ctx, inv.RepoDir, inv.Provider, inv.Template, inv.Prompt)
	steps = append(steps, execSteps...)

	if execErr != nil {
		steps = append(steps, fmt.Sprintf("⚠️ Erreur d'exécution : %v", execErr))
		return fmt.Sprintf("### ⚠️ Erreur lors de l'exécution de la commande IA (%s)\n\n```text\n%s\n```\n\n*Vérifiez que le binaire '%s' est bien accessible et authentifié.*", inv.Provider, output, inv.Provider), steps, execErr
	}

	steps = append(steps, "✅ Réponse générée par le modèle IA avec succès")
	return output, steps, nil
}

// PrepareAI resolves the working directory, the engine and the prompt of one
// workflow step, without running anything.
func (r *Runner) PrepareAI(settings *models.Settings, skillID string, task *models.Task, customPrompt string) (*AIInvocation, error) {
	repoDir := ""
	if task != nil && task.WorktreePath != nil && *task.WorktreePath != "" {
		repoDir = *task.WorktreePath
	} else if settings != nil && settings.RepoPath != "" {
		repoDir = strings.TrimSpace(settings.RepoPath)
	}
	if repoDir == "" {
		repoDir = "."
	}
	repoDir = filepath.Clean(repoDir)
	if abs, err := filepath.Abs(repoDir); err == nil {
		repoDir = abs
	}

	var steps []string
	if stat, err := os.Stat(repoDir); err != nil || !stat.IsDir() {
		cwd, _ := os.Getwd()
		steps = append(steps, fmt.Sprintf("⚠️ Project directory '%s' not found, falling back to: %s", repoDir, cwd))
		repoDir = cwd
	} else {
		steps = append(steps, fmt.Sprintf("📁 Project working directory (CWD): %s", repoDir))
	}

	trackerName := "github"
	if task != nil && task.Source != "" {
		trackerName = strings.ToLower(task.Source)
	} else if settings != nil && settings.IssueTracker != "" {
		trackerName = strings.ToLower(settings.IssueTracker)
	}

	repoName := filepath.Base(repoDir)
	if settings != nil && settings.GithubRepo != "" {
		repoName = settings.GithubRepo
	}

	// La skill installée dans le dépôt fait référence quand elle est là : c'est
	// le fichier que l'éditeur de Taskacao produit. Les prompts ci-dessous ne
	// servent plus que de filet quand rien n'est installé.
	installedSkill := installedSkillPath(repoDir, skillID)
	if installedSkill != "" {
		steps = append(steps, fmt.Sprintf("📄 Skill du dépôt utilisée : %s", installedSkill))
	}

	var promptTemplate string
	switch skillID {
	case "clarify":
		promptTemplate = settings.PromptClarify
		if promptTemplate == "" {
			promptTemplate = "/clarify-issue {issueKey} tracked on {tracker} in {repo}"
		}
	case "specify":
		promptTemplate = settings.PromptSpecify
		if promptTemplate == "" {
			if NormalizeSpecFramework(settings.SpecFramework) == "openspec" {
				promptTemplate = `Tu es le Lead Architecte pour TaskFlow. Rédige une proposition de changement OpenSpec complète pour la tâche :
Clé : {issueKey}
Titre : {issueTitle}
Description : {issueDesc}
Branche Git cible : {branchName}
Dossier du projet : {repoPath}

Contenu attendu (Framework Spec-Driven Design : OpenSpec). Écris les fichiers dans
openspec/changes/{issueKey}-<titre-slug>/ :
1. proposal.md : problème, valeur, périmètre inclus et exclu.
2. design.md : décisions techniques et alternatives écartées.
3. tasks.md : checklist ordonnée et vérifiable de mise en œuvre.
4. specs/<capability>/spec.md : deltas de comportement en sections ## ADDED / ## MODIFIED / ## REMOVED, avec des scénarios Given / When / Then.
5. Valide avec 'openspec validate <change-id> --strict' et corrige les erreurs signalées.

Si le répertoire openspec/ est absent, signale-le au lieu de deviner la structure.`
			} else {
				promptTemplate = `Tu es le Product Owner & Architecte technique pour TaskFlow. Rédige une spécification GitHub Spec Kit complète pour la tâche :
Clé : {issueKey}
Titre : {issueTitle}
Description : {issueDesc}
Branche Git cible : {branchName}
Dossier du projet : {repoPath}

Contenu attendu (Framework Spec-Driven Design : GitHub Spec Kit). Écris les fichiers dans
specs/{issueKey}-<titre-slug>/ :
1. spec.md : contexte, user stories priorisées, périmètre exclu, exigences fonctionnelles numérotées et critères d'acceptation Given / When / Then. Pas de choix d'implémentation ici.
2. plan.md : pile technique, architecture, composants et fichiers cibles, diagrammes de flux Mermaid.
3. tasks.md : checklist ordonnée et vérifiable de mise en œuvre, plus le plan de tests.

Respecte .specify/memory/constitution.md s'il existe. Marque explicitement les points à
clarifier au lieu de les deviner.`
			}
		}
	case "implement":
		promptTemplate = settings.PromptImplement
		if promptTemplate == "" {
			promptTemplate = `Tu es le développeur senior autonome pour TaskFlow. Tu dois IMPLÉMENTER ET ÉCRIRE DIRECTEMENT les modifications de code dans le projet ({repoPath}) pour accomplir cette tâche.

Contexte de la tâche :
Clé : {issueKey}
Titre : {issueTitle}
Description : {issueDesc}
Branche Git : {branchName}
Dossier du projet : {repoPath}

INSTRUCTIONS D'EXÉCUTION OBLIGATOIRES :
1. Vérifie le code existant et assure-toi d'être sur la branche Git '{branchName}'.
2. Écris et modifie concrètement les fichiers nécessaires dans le projet pour implémenter complètement la fonctionnalité ou résoudre le bug.
3. Exécute les commandes de test et de build du projet (ex: npm run build ou go test ./... selon la stack) pour vérifier que le code compile et fonctionne parfaitement sans régression.
4. Fournis un compte-rendu clair des fichiers modifiés/créés et des résultats des validations.`
		}
	case "create_pr":
		promptTemplate = settings.PromptCreatePR
		if promptTemplate == "" {
			promptTemplate = `Tu es l'ingénieur DevOps & Release pour TaskFlow. Tu dois finaliser la tâche, commiter et créer la Pull Request, puis laisser la fusion à l’utilisateur :
Clé : {issueKey}
Titre : {issueTitle}
Description : {issueDesc}
Branche Git : {branchName}
Dossier du projet : {repoPath}

INSTRUCTIONS D'EXÉCUTION OBLIGATOIRES :
1. Vérifie l'état Git dans '{repoPath}' ('git status' et 'git remote').
2. Assure-toi que toutes les modifications sur la branche '{branchName}' sont commitées proprement avec un message conventionnel (ex: 'feat({issueKey}): {issueTitle}').
3. CAS A : Si un dépôt distant (remote 'origin' ou GitHub/GitLab) est configuré :
   - Pousse la branche vers le remote : 'git push -u origin {branchName}'
   - Crée la Pull Request via 'gh pr create' ou 'glab mr create' si disponible.
4. Si aucun remote n'est configuré, signale le blocage et conserve la branche et le worktree.
5. Fournis l'URL réelle de la Pull Request et les résultats des vérifications. Ne fusionne jamais localement ou à distance.`
		}
	case "handoff":
		promptTemplate = `Tu es responsable de la clôture propre de la tâche pour TaskFlow. Le code a été revu et fusionné : il reste à documenter le handoff et à nettoyer.

Clé : {issueKey}
Titre : {issueTitle}
Description : {issueDesc}
Branche Git : {branchName}
Dossier du projet : {repoPath}

INSTRUCTIONS D'EXÉCUTION OBLIGATOIRES :
1. Vérifie que la branche '{branchName}' est bien fusionnée dans la branche principale ('git log --oneline main..{branchName}' doit être vide). Si ce n'est pas le cas, ARRÊTE-TOI et dis-le, sans rien nettoyer.
2. Rédige le compte-rendu de handoff : ce qui a été livré, ce qui a été laissé de côté, ce qu'un lecteur doit savoir pour reprendre. Mets à jour la documentation du dépôt si le changement l'exige (README, CHANGELOG, docs).
3. Nettoie l'espace de travail local : retire le worktree de la tâche s'il existe et supprime la branche locale fusionnée.
4. Termine par un compte-rendu court : ce qui a été documenté, ce qui a été nettoyé, ce qui reste à faire côté humain.`

	case "pick":
		promptTemplate = settings.PromptPick
		if promptTemplate == "" {
			promptTemplate = "Tu es le routeur d'orchestration pour TaskFlow. Analyse l'état de la tâche {issueKey} ({issueTitle}) et détermine la prochaine action requise dans le cycle SDLC."
		}
	}

	// Ordre de priorité : le prompt surchargé dans les réglages, puis la skill
	// installée, puis le filet codé au-dessus.
	if installedSkill != "" && !settingsPromptOverridden(settings, skillID) {
		promptTemplate = skillSlashPrompt(skillID)
	}

	if customPrompt != "" {
		promptTemplate += "\n\nInstructions supplémentaires fournies par l'utilisateur :\n" + customPrompt
	}

	branchName := ""
	if task.BranchName != nil {
		branchName = *task.BranchName
	} else {
		cleanTitle := strings.ToLower(task.Title)
		cleanTitle = strings.ReplaceAll(cleanTitle, " ", "-")
		cleanTitle = strings.ReplaceAll(cleanTitle, "'", "-")
		if len(cleanTitle) > 30 {
			cleanTitle = cleanTitle[:30]
		}
		branchName = fmt.Sprintf("%s-%s", task.Key, cleanTitle)
	}

	finalPrompt := promptTemplate
	finalPrompt = strings.ReplaceAll(finalPrompt, "{issueKey}", task.Key)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{issueTitle}", task.Title)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{issueDesc}", task.Description)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{branchName}", branchName)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{repoPath}", repoDir)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{tracker}", trackerName)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{repo}", repoName)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{prompt}", customPrompt)

	provider := strings.ToLower(settings.AIProvider)
	if provider == "" {
		provider = "agy"
	}

	steps = append(steps, fmt.Sprintf("🤖 Moteur IA : %s", strings.ToUpper(provider)))

	// The custom-template branch substitutes the task placeholders first, then
	// hands the resolved template to the shared dispatcher.
	resolvedTemplate := settings.AICommandTemplate
	if resolvedTemplate != "" {
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{issueKey}", task.Key)
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{issueTitle}", task.Title)
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{issueDesc}", task.Description)
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{branchName}", branchName)
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{repoPath}", repoDir)
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{tracker}", trackerName)
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{repo}", repoName)
	}

	return &AIInvocation{
		RepoDir:  repoDir,
		Provider: provider,
		Template: resolvedTemplate,
		Prompt:   finalPrompt,
		Steps:    steps,
	}, nil
}

// escapeForDoubleQuotes makes a string safe to interpolate inside a
// double-quoted shell word. Inside double quotes sh only treats \, ", $ and `
// specially, so backslash-escaping exactly those four yields the literal text.
//
// This matters for security, not just for quoting: the AI command template is
// executed through 'sh -c', and the prompt embeds task titles and descriptions
// that come straight from Jira, GitHub or Linear. Without escaping, a ticket
// titled `"; rm -rf ~ #` would run as a shell command.
func escapeForDoubleQuotes(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch r {
		case '\\', '"', '$', '`':
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// execAgentCommand runs the configured AI CLI for an already-resolved prompt.
// cmdTemplate must have every placeholder other than {prompt} already
// substituted by the caller; it is used when it is non-empty and either the
// provider is "custom" or the template carries a {prompt} slot.
func (r *Runner) execAgentCommand(ctx context.Context, repoDir string, provider string, cmdTemplate string, finalPrompt string) (string, []string, error) {
	var steps []string

	if cmdTemplate != "" && (provider == "custom" || strings.Contains(cmdTemplate, "{prompt}")) {
		cmdToRun := strings.ReplaceAll(cmdTemplate, "{prompt}", escapeForDoubleQuotes(finalPrompt))
		steps = append(steps, fmt.Sprintf("Exécution de la commande personnalisée : %s dans %s", cmdToRun, filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, "sh", "-c", cmdToRun)
		return out, steps, err
	}

	switch provider {
	case "agy":
		agyPath, _ := FindCliTool("agy")
		steps = append(steps, fmt.Sprintf("Exécution de : agy -p \"...\" dans %s", filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, agyPath, "-p", finalPrompt, "--dangerously-skip-permissions")
		return out, steps, err

	case "vibe":
		vibePath, _ := FindCliTool("vibe")
		steps = append(steps, fmt.Sprintf("Exécution de : vibe -p \"...\" dans %s", filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, vibePath, "-p", finalPrompt, "--auto-approve")
		return out, steps, err

	case "claude":
		claudePath, _ := FindCliTool("claude")
		steps = append(steps, fmt.Sprintf("Exécution de : claude -p \"...\" dans %s", filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, claudePath, "-p", finalPrompt)
		return out, steps, err

	case "gemini":
		geminiPath, _ := FindCliTool("gemini")
		steps = append(steps, fmt.Sprintf("Exécution de : gemini -p \"...\" dans %s", filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, geminiPath, "-p", finalPrompt)
		return out, steps, err

	case "cursor":
		cursorPath, _ := FindCliTool("cursor")
		steps = append(steps, fmt.Sprintf("Exécution de : cursor agent -p \"...\" dans %s", filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, cursorPath, "agent", "-p", finalPrompt)
		return out, steps, err

	default:
		cmdStr := cmdTemplate
		if cmdStr == "" {
			cmdStr = "agy -p \"{prompt}\""
		}
		cmdToRun := strings.ReplaceAll(cmdStr, "{prompt}", escapeForDoubleQuotes(finalPrompt))
		steps = append(steps, fmt.Sprintf("Exécution de la commande personnalisée dans %s", filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, "sh", "-c", cmdToRun)
		return out, steps, err
	}
}

// RunAgentPrompt executes the configured AI CLI on a free-form prompt with no
// task context. Used by features that are not tied to a single work item, such
// as the daily digest agenda.
func (r *Runner) RunAgentPrompt(ctx context.Context, settings *models.Settings, prompt string) (string, []string, error) {
	if settings == nil {
		return "", nil, fmt.Errorf("réglages IA indisponibles")
	}

	provider := settings.AIProvider
	if provider == "" {
		provider = "agy"
	}

	repoDir := strings.TrimSpace(settings.RepoPath)
	if repoDir == "" {
		repoDir = "."
	}
	if abs, err := filepath.Abs(repoDir); err == nil {
		repoDir = abs
	}
	if stat, err := os.Stat(repoDir); err != nil || !stat.IsDir() {
		cwd, _ := os.Getwd()
		repoDir = cwd
	}

	out, steps, err := r.execAgentCommand(ctx, repoDir, provider, settings.AICommandTemplate, prompt)
	if err != nil {
		return out, steps, fmt.Errorf("exécution de l'agent %s impossible: %w", provider, err)
	}
	return out, steps, nil
}

// GetGitDiff computes git diff for a task branch or working directory
func (r *Runner) GetGitDiff(repoDir string, branchName string, taskKey string, prURL *string) (*models.GitDiffResult, error) {
	if repoDir == "" {
		return nil, fmt.Errorf("répertoire de projet non configuré")
	}

	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return &models.GitDiffResult{
			TaskKey:  taskKey,
			Branch:   branchName,
			RepoPath: repoDir,
			PrURL:    prURL,
			Error:    fmt.Sprintf("Le répertoire '%s' n'existe pas", repoDir),
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Verify it's a git repo
	if _, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return &models.GitDiffResult{
			TaskKey:  taskKey,
			Branch:   branchName,
			RepoPath: repoDir,
			PrURL:    prURL,
			Error:    fmt.Sprintf("Le répertoire '%s' n'est pas un dépôt Git valide", repoDir),
		}, nil
	}

	// Detect base branch (main or master)
	baseBranch := "main"
	if _, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--verify", "main"); err != nil {
		if _, errMaster := r.runCommand(ctx, repoDir, "git", "rev-parse", "--verify", "master"); errMaster == nil {
			baseBranch = "master"
		} else {
			baseBranch = "HEAD~1"
		}
	}

	// Get current active branch
	currBranch, _ := r.runCommand(ctx, repoDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	currBranch = strings.TrimSpace(currBranch)

	targetBranch := strings.TrimSpace(branchName)
	if targetBranch == "" {
		targetBranch = currBranch
	}

	// Try diffing between baseBranch and targetBranch
	var rawDiff string
	var numstat string
	var diffErr error

	// Check if targetBranch exists
	hasBranch := false
	if _, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--verify", targetBranch); err == nil {
		hasBranch = true
	}

	if hasBranch && targetBranch != baseBranch {
		// Three-dot diff against base
		rawDiff, diffErr = r.runCommand(ctx, repoDir, "git", "diff", fmt.Sprintf("%s...%s", baseBranch, targetBranch))
		numstat, _ = r.runCommand(ctx, repoDir, "git", "diff", "--numstat", fmt.Sprintf("%s...%s", baseBranch, targetBranch))
		if strings.TrimSpace(rawDiff) == "" {
			// Fallback to two-dot diff
			rawDiff, diffErr = r.runCommand(ctx, repoDir, "git", "diff", fmt.Sprintf("%s..%s", baseBranch, targetBranch))
			numstat, _ = r.runCommand(ctx, repoDir, "git", "diff", "--numstat", fmt.Sprintf("%s..%s", baseBranch, targetBranch))
		}
	}

	// If current branch is target branch and there are uncommitted working tree changes, include them
	if currBranch == targetBranch || strings.TrimSpace(rawDiff) == "" {
		workDiff, _ := r.runCommand(ctx, repoDir, "git", "diff", "HEAD")
		workNumstat, _ := r.runCommand(ctx, repoDir, "git", "diff", "--numstat", "HEAD")
		if strings.TrimSpace(workDiff) != "" {
			if rawDiff != "" {
				rawDiff = rawDiff + "\n" + workDiff
				numstat = numstat + "\n" + workNumstat
			} else {
				rawDiff = workDiff
				numstat = workNumstat
			}
		}
	}

	// If still empty and on base branch or no commits between, try diff of last commit
	if strings.TrimSpace(rawDiff) == "" {
		lastCommitDiff, _ := r.runCommand(ctx, repoDir, "git", "diff", "HEAD~1..HEAD")
		lastCommitNumstat, _ := r.runCommand(ctx, repoDir, "git", "diff", "--numstat", "HEAD~1..HEAD")
		if strings.TrimSpace(lastCommitDiff) != "" {
			rawDiff = lastCommitDiff
			numstat = lastCommitNumstat
		}
	}

	if diffErr != nil && strings.TrimSpace(rawDiff) == "" {
		return &models.GitDiffResult{
			TaskKey:    taskKey,
			Branch:     targetBranch,
			BaseBranch: baseBranch,
			RepoPath:   repoDir,
			PrURL:      prURL,
			Error:      fmt.Sprintf("Erreur lors de la récupération du diff : %v", diffErr),
		}, nil
	}

	// Parse numstat and split rawDiff into file blocks
	fileMap := make(map[string]*models.GitDiffFile)
	totalInsertions := 0
	totalDeletions := 0

	for _, line := range strings.Split(numstat, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			adds, _ := strconv.Atoi(parts[0])
			dels, _ := strconv.Atoi(parts[1])
			path := parts[2]
			totalInsertions += adds
			totalDeletions += dels

			fileMap[path] = &models.GitDiffFile{
				Path:      path,
				Status:    "modified",
				Additions: adds,
				Deletions: dels,
			}
		}
	}

	// Parse rawDiff chunks by file
	diffChunks := strings.Split(rawDiff, "diff --git ")
	var files []models.GitDiffFile

	for _, chunk := range diffChunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		fullChunk := "diff --git " + chunk
		firstLine := strings.SplitN(chunk, "\n", 2)[0]
		parts := strings.Fields(firstLine)

		filePath := ""
		if len(parts) >= 2 {
			filePath = strings.TrimPrefix(parts[1], "b/")
		}

		status := "modified"
		if strings.Contains(chunk, "new file mode") {
			status = "added"
		} else if strings.Contains(chunk, "deleted file mode") {
			status = "deleted"
		} else if strings.Contains(chunk, "similarity index") || strings.Contains(chunk, "rename from") {
			status = "renamed"
		}

		adds := 0
		dels := 0
		if existingFile, ok := fileMap[filePath]; ok {
			adds = existingFile.Additions
			dels = existingFile.Deletions
			existingFile.Diff = fullChunk
			existingFile.Status = status
			files = append(files, *existingFile)
			delete(fileMap, filePath)
		} else {
			// Count manually
			for _, l := range strings.Split(chunk, "\n") {
				if strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++") {
					adds++
				} else if strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---") {
					dels++
				}
			}
			totalInsertions += adds
			totalDeletions += dels
			files = append(files, models.GitDiffFile{
				Path:      filePath,
				Status:    status,
				Additions: adds,
				Deletions: dels,
				Diff:      fullChunk,
			})
		}
	}

	return &models.GitDiffResult{
		TaskKey:      taskKey,
		Branch:       targetBranch,
		BaseBranch:   baseBranch,
		RepoPath:     repoDir,
		IsClean:      len(files) == 0,
		FilesChanged: len(files),
		Insertions:   totalInsertions,
		Deletions:    totalDeletions,
		Files:        files,
		RawDiff:      rawDiff,
		PrURL:        prURL,
	}, nil
}

// GetCwdGitStatus returns git status, active branch, and repo state for a directory
func (r *Runner) GetCwdGitStatus(repoDir string) (*models.GitStatusInfo, error) {
	if repoDir == "" {
		cwd, err := os.Getwd()
		if err == nil && cwd != "" {
			repoDir = cwd
		}
	}

	if repoDir == "" {
		return &models.GitStatusInfo{
			Error: "Répertoire de travail non spécifié",
		}, nil
	}

	if fi, err := os.Stat(repoDir); err != nil || !fi.IsDir() {
		return &models.GitStatusInfo{
			RepoPath:  repoDir,
			IsGitRepo: false,
			Error:     fmt.Sprintf("Le répertoire '%s' n'existe pas", repoDir),
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if git repo
	if _, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return &models.GitStatusInfo{
			RepoPath:  repoDir,
			IsGitRepo: false,
			Error:     "Ce dossier n'est pas un dépôt Git",
		}, nil
	}

	// Real repo top level path
	topLevel, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--show-toplevel")
	if err == nil && strings.TrimSpace(topLevel) != "" {
		repoDir = strings.TrimSpace(topLevel)
	}

	// Current active branch
	branch, _ := r.runCommand(ctx, repoDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	branch = strings.TrimSpace(branch)
	if branch == "HEAD" || branch == "" {
		// Detached HEAD or tag
		if tag, err := r.runCommand(ctx, repoDir, "git", "describe", "--tags", "--always"); err == nil && strings.TrimSpace(tag) != "" {
			branch = strings.TrimSpace(tag)
		} else if shortHash, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--short", "HEAD"); err == nil {
			branch = strings.TrimSpace(shortHash)
		}
	}

	// Base branch detection
	baseBranch := "main"
	if _, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--verify", "main"); err != nil {
		if _, errMaster := r.runCommand(ctx, repoDir, "git", "rev-parse", "--verify", "master"); errMaster == nil {
			baseBranch = "master"
		}
	}

	// Status porcelain
	porcelainOut, _ := r.runCommand(ctx, repoDir, "git", "status", "--porcelain")
	modifiedCount := 0
	untrackedCount := 0
	for _, line := range strings.Split(porcelainOut, "\n") {
		line = strings.TrimRight(line, "\r\n")
		if len(line) < 2 {
			continue
		}
		if strings.HasPrefix(line, "??") {
			untrackedCount++
		} else {
			modifiedCount++
		}
	}

	isClean := (modifiedCount == 0 && untrackedCount == 0)

	// Remote info
	remoteName, _ := r.runCommand(ctx, repoDir, "git", "remote")
	remoteName = strings.TrimSpace(strings.Split(remoteName, "\n")[0])
	remoteURL := ""
	if remoteName != "" {
		if rURL, err := r.runCommand(ctx, repoDir, "git", "remote", "get-url", remoteName); err == nil {
			remoteURL = strings.TrimSpace(rURL)
		}
	}

	// Ahead / Behind counts against upstream if configured
	ahead := 0
	behind := 0
	if countsOut, err := r.runCommand(ctx, repoDir, "git", "rev-list", "--left-right", "--count", "HEAD...@{u}"); err == nil {
		parts := strings.Fields(countsOut)
		if len(parts) >= 2 {
			ahead, _ = strconv.Atoi(parts[0])
			behind, _ = strconv.Atoi(parts[1])
		}
	}

	// Latest commit message & short hash
	latestCommit, _ := r.runCommand(ctx, repoDir, "git", "log", "-1", "--format=%h %s (%cr)")
	latestCommit = strings.TrimSpace(latestCommit)

	return &models.GitStatusInfo{
		RepoPath:       repoDir,
		IsGitRepo:      true,
		Branch:         branch,
		BaseBranch:     baseBranch,
		IsClean:        isClean,
		ModifiedCount:  modifiedCount,
		UntrackedCount: untrackedCount,
		Ahead:          ahead,
		Behind:         behind,
		RemoteName:     remoteName,
		RemoteURL:      remoteURL,
		LatestCommit:   latestCommit,
	}, nil
}

// FindEditorBinary locates the binary or execution command for a given code editor on the host OS
func FindEditorBinary(editor string) (string, []string) {
	editor = strings.TrimSpace(editor)
	base := strings.ToLower(editor)

	// 1. Direct LookPath or standard CLI path
	if p, err := FindCliTool(editor); err == nil && p != "" {
		return p, nil
	}

	homeDir, _ := os.UserHomeDir()

	// 2. Known application paths & fallbacks on macOS
	if runtime.GOOS == "darwin" {
		switch base {
		case "code", "vscode":
			macPaths := []string{
				"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
				"/Applications/Visual Studio Code - Insiders.app/Contents/Resources/app/bin/code",
				filepath.Join(homeDir, "Applications/Visual Studio Code.app/Contents/Resources/app/bin/code"),
			}
			for _, mp := range macPaths {
				if _, err := os.Stat(mp); err == nil {
					return mp, nil
				}
			}
			return "open", []string{"-a", "Visual Studio Code"}

		case "cursor":
			macPaths := []string{
				"/Applications/Cursor.app/Contents/Resources/app/bin/cursor",
				filepath.Join(homeDir, "Applications/Cursor.app/Contents/Resources/app/bin/cursor"),
			}
			for _, mp := range macPaths {
				if _, err := os.Stat(mp); err == nil {
					return mp, nil
				}
			}
			return "open", []string{"-a", "Cursor"}

		case "zed":
			macPaths := []string{
				"/Applications/Zed.app/Contents/MacOS/cli",
				filepath.Join(homeDir, "Applications/Zed.app/Contents/MacOS/cli"),
			}
			for _, mp := range macPaths {
				if _, err := os.Stat(mp); err == nil {
					return mp, nil
				}
			}
			return "open", []string{"-a", "Zed"}

		case "subl", "sublime":
			macPaths := []string{
				"/Applications/Sublime Text.app/Contents/SharedSupport/bin/subl",
				filepath.Join(homeDir, "Applications/Sublime Text.app/Contents/SharedSupport/bin/subl"),
			}
			for _, mp := range macPaths {
				if _, err := os.Stat(mp); err == nil {
					return mp, nil
				}
			}
			return "open", []string{"-a", "Sublime Text"}

		case "idea", "intellij":
			return "open", []string{"-a", "IntelliJ IDEA"}

		case "webstorm":
			return "open", []string{"-a", "WebStorm"}
		}
	}

	return editor, nil
}

// OpenInEditor opens the specified directory or file in a code editor (defaults to 'code' for VS Code)
func (r *Runner) OpenInEditor(editorCmd string, targetPath string) error {
	if strings.TrimSpace(editorCmd) == "" {
		editorCmd = "code"
	}
	editorCmd = strings.TrimSpace(editorCmd)

	if targetPath == "" {
		targetPath = "."
	}
	targetPath = filepath.Clean(targetPath)
	if abs, err := filepath.Abs(targetPath); err == nil {
		targetPath = abs
	}

	parts := strings.Fields(editorCmd)
	if len(parts) == 0 {
		parts = []string{"code"}
	}

	bin, prefixArgs := FindEditorBinary(parts[0])
	var args []string
	if len(prefixArgs) > 0 {
		args = append(args, prefixArgs...)
		args = append(args, parts[1:]...)
		args = append(args, targetPath)
	} else {
		args = append(args, parts[1:]...)
		args = append(args, targetPath)
	}

	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "PATH="+GetDynamicCustomPath()+":"+os.Getenv("PATH"))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open in '%s': %w", editorCmd, err)
	}
	return nil
}

// OpenExternalTerminal opens a native host terminal window in the given directory,
// with contextual environment variables and an optional initial command line.
func (r *Runner) OpenExternalTerminal(customTermCmd string, targetPath string, initialCommand string, envVars map[string]string) error {
	if targetPath == "" {
		targetPath = "."
	}
	targetPath = filepath.Clean(targetPath)
	if abs, err := filepath.Abs(targetPath); err == nil {
		targetPath = abs
	}

	customTermCmd = strings.TrimSpace(customTermCmd)

	// Create a temporary launcher script
	tmpFile, err := os.CreateTemp("", "taskflow-term-*.command")
	if err != nil {
		return fmt.Errorf("failed to create temporary terminal script: %w", err)
	}
	scriptPath := tmpFile.Name()

	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("# TaskFlow External Terminal Session\n\n")

	customPath := GetDynamicCustomPath()
	if customPath != "" {
		sb.WriteString(fmt.Sprintf("export PATH=%q:$PATH\n", customPath))
	}

	for k, v := range envVars {
		if strings.TrimSpace(k) != "" {
			sb.WriteString(fmt.Sprintf("export %s=%q\n", k, v))
		}
	}

	sb.WriteString(fmt.Sprintf("cd %q || exit 1\n\n", targetPath))
	sb.WriteString("echo -e \"\\033[1;36m┌──────────────────────────────────────────────────┐\\033[0m\"\n")
	sb.WriteString("echo -e \"\\033[1;36m│          TaskFlow External Terminal             │\\033[0m\"\n")
	sb.WriteString("echo -e \"\\033[1;36m└──────────────────────────────────────────────────┘\\033[0m\"\n")
	sb.WriteString(fmt.Sprintf("echo -e \"\\033[0;32m📁 Dossier :\\033[0m %s\"\n", targetPath))
	if taskKey, ok := envVars["TASKFLOW_TASK_KEY"]; ok && taskKey != "" {
		sb.WriteString(fmt.Sprintf("echo -e \"\\033[0;35m🎯 Tâche   :\\033[0m %s\"\n", taskKey))
	}
	sb.WriteString("echo \"\"\n\n")

	initialCommand = strings.TrimSpace(initialCommand)
	if initialCommand != "" {
		sb.WriteString(fmt.Sprintf("echo -e \"\\033[0;33m▶ Exécution :\\033[0m %s\"\n", initialCommand))
		sb.WriteString(fmt.Sprintf("%s\n\n", initialCommand))
	}

	sb.WriteString("exec \"${SHELL:-/bin/zsh}\" -l\n")

	if _, err := tmpFile.WriteString(sb.String()); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write terminal script: %w", err)
	}
	tmpFile.Close()

	if err := os.Chmod(scriptPath, 0755); err != nil {
		return fmt.Errorf("failed to make terminal script executable: %w", err)
	}

	var cmd *exec.Cmd
	termTrimmed := strings.TrimSpace(customTermCmd)
	termLower := strings.ToLower(termTrimmed)

	switch runtime.GOOS {
	case "darwin":
		if strings.Contains(termTrimmed, "{script}") || strings.Contains(termTrimmed, "{cmd}") {
			rendered := strings.ReplaceAll(termTrimmed, "{script}", shellQuote(scriptPath))
			rendered = strings.ReplaceAll(rendered, "{cmd}", shellQuote(scriptPath))
			cmd = exec.Command("sh", "-c", rendered)
		} else if strings.Contains(termLower, "ghostty") {
			// Ghostty on macOS runs specific commands via `open -na Ghostty.app --args -e <script>`
			if _, statErr := os.Stat("/Applications/Ghostty.app"); statErr == nil {
				cmd = exec.Command("open", "-na", "/Applications/Ghostty.app", "--args", "-e", scriptPath)
			} else {
				cmd = exec.Command("open", "-na", "Ghostty", "--args", "-e", scriptPath)
			}
		} else if strings.Contains(termLower, "alacritty") {
			if bin, err := exec.LookPath("alacritty"); err == nil {
				cmd = exec.Command(bin, "-e", scriptPath)
			} else {
				cmd = exec.Command("open", "-a", "Alacritty", scriptPath)
			}
		} else if strings.Contains(termLower, "kitty") {
			if bin, err := exec.LookPath("kitty"); err == nil {
				cmd = exec.Command(bin, scriptPath)
			} else {
				cmd = exec.Command("open", "-a", "kitty", scriptPath)
			}
		} else if strings.Contains(termLower, "wezterm") {
			if bin, err := exec.LookPath("wezterm"); err == nil {
				cmd = exec.Command(bin, "start", "--", scriptPath)
			} else {
				cmd = exec.Command("open", "-a", "WezTerm", scriptPath)
			}
		} else if termTrimmed != "" {
			parts := strings.Fields(termTrimmed)
			if len(parts) == 1 && !strings.Contains(parts[0], "/") {
				cmd = exec.Command("open", "-a", parts[0], scriptPath)
			} else {
				args := append(parts[1:], scriptPath)
				cmd = exec.Command(parts[0], args...)
			}
		} else {
			cmd = exec.Command("open", scriptPath)
		}
	case "windows":
		if strings.Contains(termTrimmed, "{script}") || strings.Contains(termTrimmed, "{cmd}") {
			rendered := strings.ReplaceAll(termTrimmed, "{script}", scriptPath)
			rendered = strings.ReplaceAll(rendered, "{cmd}", scriptPath)
			cmd = exec.Command("cmd.exe", "/c", rendered)
		} else if strings.Contains(termLower, "wt") || strings.Contains(termLower, "windowsterminal") {
			cmd = exec.Command("wt.exe", "new-tab", "cmd.exe", "/k", scriptPath)
		} else {
			cmd = exec.Command("cmd.exe", "/c", "start", scriptPath)
		}
	default: // linux / unix
		if strings.Contains(termTrimmed, "{script}") || strings.Contains(termTrimmed, "{cmd}") {
			rendered := strings.ReplaceAll(termTrimmed, "{script}", shellQuote(scriptPath))
			rendered = strings.ReplaceAll(rendered, "{cmd}", shellQuote(scriptPath))
			cmd = exec.Command("sh", "-c", rendered)
		} else if strings.Contains(termLower, "ghostty") {
			cmd = exec.Command("ghostty", "-e", scriptPath)
		} else if strings.Contains(termLower, "alacritty") {
			cmd = exec.Command("alacritty", "-e", scriptPath)
		} else if strings.Contains(termLower, "kitty") {
			cmd = exec.Command("kitty", scriptPath)
		} else if strings.Contains(termLower, "wezterm") {
			cmd = exec.Command("wezterm", "start", "--", scriptPath)
		} else if termTrimmed != "" {
			parts := strings.Fields(termTrimmed)
			args := append(parts[1:], scriptPath)
			cmd = exec.Command(parts[0], args...)
		} else if _, err := exec.LookPath("x-terminal-emulator"); err == nil {
			cmd = exec.Command("x-terminal-emulator", "-e", scriptPath)
		} else if _, err := exec.LookPath("gnome-terminal"); err == nil {
			cmd = exec.Command("gnome-terminal", "--", scriptPath)
		} else if _, err := exec.LookPath("konsole"); err == nil {
			cmd = exec.Command("konsole", "-e", scriptPath)
		} else if _, err := exec.LookPath("xterm"); err == nil {
			cmd = exec.Command("xterm", "-e", scriptPath)
		} else {
			cmd = exec.Command("sh", scriptPath)
		}
	}

	if cmd == nil {
		return fmt.Errorf("unable to determine terminal launcher for OS %s", runtime.GOOS)
	}

	cmd.Env = append(os.Environ(), "PATH="+GetDynamicCustomPath()+":"+os.Getenv("PATH"))
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start external terminal: %w", err)
	}
	return nil
}

// runCommandForTest runs a shell snippet and returns its raw stdout. It exists
// so the escaping tests can assert what the shell actually parses.
func (r *Runner) runCommandForTest(script string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return r.runCommand(ctx, "", "sh", "-c", script)
}

// settingsPromptOverridden says whether the user deliberately wrote their own
// prompt for a step in the settings. That choice keeps priority over the skill
// installed in the repository.
func settingsPromptOverridden(settings *models.Settings, skillID string) bool {
	if settings == nil {
		return false
	}
	switch skillID {
	case "clarify":
		return strings.TrimSpace(settings.PromptClarify) != ""
	case "specify":
		return strings.TrimSpace(settings.PromptSpecify) != ""
	case "implement":
		return strings.TrimSpace(settings.PromptImplement) != ""
	case "create_pr", "review":
		return strings.TrimSpace(settings.PromptCreatePR) != ""
	}
	return false
}

// SessionCommandLine turns an invocation into a shell command line that can be
// injected into a PTY session, plus the cleanup of the temporary file it uses.
//
// The prompt goes through a file rather than the command line on purpose: it is
// several thousand characters of markdown carrying ticket text, and pushing that
// through a terminal line editor would truncate it and mangle the quoting. The
// file is read with a command substitution, so the agent still receives it as a
// single argument.
func (r *Runner) SessionCommandLine(inv *AIInvocation) (string, func(), error) {
	if inv == nil {
		return "", func() {}, fmt.Errorf("invocation vide")
	}

	f, err := os.CreateTemp("", "taskflow-prompt-*.md")
	if err != nil {
		return "", func() {}, err
	}
	if _, err := f.WriteString(inv.Prompt); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", func() {}, err
	}
	_ = f.Close()
	promptFile := f.Name()
	cleanup := func() { _ = os.Remove(promptFile) }

	// $(cat 'fichier') : le chemin est un temporaire que nous fabriquons, donc
	// sans apostrophe, et le prompt n'est jamais relu par le shell.
	promptRef := fmt.Sprintf(`"$(cat '%s')"`, promptFile)

	if inv.Template != "" && (inv.Provider == "custom" || strings.Contains(inv.Template, "{prompt}")) {
		return strings.ReplaceAll(inv.Template, "{prompt}", "$(cat '"+promptFile+"')"), cleanup, nil
	}

	switch inv.Provider {
	case "agy":
		bin, _ := FindCliTool("agy")
		return fmt.Sprintf("%s -p %s --dangerously-skip-permissions", shellQuote(bin), promptRef), cleanup, nil
	case "vibe":
		bin, _ := FindCliTool("vibe")
		return fmt.Sprintf("%s -p %s --auto-approve", shellQuote(bin), promptRef), cleanup, nil
	case "claude":
		bin, _ := FindCliTool("claude")
		return fmt.Sprintf("%s -p %s --dangerously-skip-permissions", shellQuote(bin), promptRef), cleanup, nil
	case "gemini":
		bin, _ := FindCliTool("gemini")
		return fmt.Sprintf("%s -p %s", shellQuote(bin), promptRef), cleanup, nil
	case "cursor":
		bin, _ := FindCliTool("cursor")
		return fmt.Sprintf("%s agent -p %s", shellQuote(bin), promptRef), cleanup, nil
	}

	template := inv.Template
	if template == "" {
		template = `agy -p "{prompt}"`
	}
	return strings.ReplaceAll(template, "{prompt}", "$(cat '"+promptFile+"')"), cleanup, nil
}

// shellQuote wraps a path in single quotes so a space in it cannot split the
// command. The binary paths come from FindCliTool, not from user input.
func shellQuote(s string) string {
	if s == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// InteractiveAgentLaunch is the command that opens the agent CLI as a live
// session, as opposed to the one-shot print mode used for headless steps.
//
// Aucun drapeau de contournement des permissions ici, contrairement au mode
// non interactif : un humain regarde la session, il peut répondre aux demandes
// de permission, et c'est précisément l'intérêt de ce mode.
func InteractiveAgentLaunch(settings *models.Settings) (string, error) {
	provider := "agy"
	if settings != nil && strings.TrimSpace(settings.AIProvider) != "" {
		provider = strings.ToLower(strings.TrimSpace(settings.AIProvider))
	}

	switch provider {
	case "agy", "vibe", "claude", "gemini":
		return resolveAgentBinary(provider, "")
	case "cursor":
		line, err := resolveAgentBinary("cursor", "")
		if err != nil {
			return "", err
		}
		return line + " agent", nil
	case "custom":
		// Un moteur personnalisé n'a que son modèle de commande : son premier mot
		// est le binaire, et c'est lui qu'on ouvre en interactif.
		return resolveAgentBinary(firstWord(settings.AICommandTemplate), provider)
	}
	return "", fmt.Errorf("le moteur %q n'a pas de mode interactif connu : configure un moteur agy, claude, gemini, cursor ou vibe sur le projet", provider)
}

// resolveAgentBinary finds an engine binary and says where it looked when it
// fails, because "binaire introuvable" alone leaves nothing to act on.
func resolveAgentBinary(tool, provider string) (string, error) {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return "", fmt.Errorf("aucun binaire à lancer pour le moteur %q : renseigne son modèle de commande", provider)
	}
	bin, err := FindCliTool(tool)
	if err != nil || bin == "" || bin == tool {
		if _, statErr := os.Stat(bin); statErr != nil {
			home, _ := os.UserHomeDir()
			return "", fmt.Errorf(
				"moteur %q introuvable : %s absent du PATH, de %s/.local/bin, /opt/homebrew/bin, /usr/local/bin et /usr/bin",
				strings.TrimSpace(provider+" "+tool), tool, home,
			)
		}
	}
	return shellQuote(bin), nil
}

// firstWord extracts the binary from a command template such as
// `claude --dangerously-skip-permissions -p "{prompt}"`.
func firstWord(template string) string {
	fields := strings.Fields(strings.TrimSpace(template))
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], "'\"")
}

// SkillCallLine is what gets typed into the running agent: the skill's slash
// command, the ticket key, and just enough context to place the work.
//
// C'est la skill qui porte les instructions, pas ce message : le SKILL.md est
// rendu depuis l'éditeur de Taskacao, l'agent le lit en exécutant la commande.
func SkillCallLine(skillID string, task *models.Task, trackerName string) string {
	dirName := models.SkillDirNames[skillID]
	if dirName == "" {
		dirName = skillID
	}
	return SkillCallLineWithCommand("/"+dirName, task, trackerName)
}

// SkillCallLineWithCommand builds the same line for an explicit slash command,
// so a project that renamed its skills keeps its own command.
func SkillCallLineWithCommand(command string, task *models.Task, trackerName string) string {
	command = "/" + strings.TrimPrefix(strings.TrimSpace(command), "/")
	if task == nil {
		return command
	}

	line := fmt.Sprintf("%s %s", command, task.Key)
	title := strings.TrimSpace(task.Title)
	if title != "" {
		line += fmt.Sprintf(" (%s)", collapseSpaces(title))
	}
	if trackerName != "" && trackerName != "local" {
		line += fmt.Sprintf(" suivi dans %s", trackerName)
	}
	return line
}

// collapseSpaces flattens a title to a single line: a newline in the injected
// text would be read as a validation by the agent's prompt.
func collapseSpaces(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}

