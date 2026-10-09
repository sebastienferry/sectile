package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"tasks/internal/agenthttp"
	"tasks/internal/models"
)

// taskPageCapability tells Desktop this agent reads, edits and looks up the
// assignees of a task for its task page (#805).
const taskPageCapability = "task-page"

// desktopTaskDetail serves the task page: GET reads a task of the project,
// PUT forwards the fields the page edits. Any other field of the body is
// dropped, so the page cannot move a task's stage or project by accident.
func (d *agentDaemon) desktopTaskDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", 405)
		return
	}
	projectID, taskID := r.URL.Query().Get("projectId"), r.URL.Query().Get("taskId")
	if projectID == "" || taskID == "" {
		http.Error(w, "Project and task required", 400)
		return
	}
	var update models.UpdateTaskRequest
	if r.Method == http.MethodPut {
		var input struct {
			Title             *string                   `json:"title"`
			Description       *string                   `json:"description"`
			Assignee          *string                   `json:"assignee"`
			AssigneeAccountID *string                   `json:"assigneeAccountId"`
			AssigneeAvatar    *string                   `json:"assigneeAvatar"`
			PrLinks           *[]models.TaskPullRequest `json:"prLinks"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&input) != nil {
			http.Error(w, "Invalid request body", 400)
			return
		}
		if input.Title != nil {
			title := strings.TrimSpace(*input.Title)
			if title == "" {
				http.Error(w, "Title required", 400)
				return
			}
			input.Title = &title
		}
		if input.PrLinks != nil {
			for _, link := range *input.PrLinks {
				if !webLink(link.URL) {
					http.Error(w, "Invalid pull request link: "+link.URL, 400)
					return
				}
			}
		}
		update = models.UpdateTaskRequest{Title: input.Title, Description: input.Description, Assignee: input.Assignee, AssigneeAccountID: input.AssigneeAccountID, AssigneeAvatar: input.AssigneeAvatar, PrLinks: input.PrLinks}
	}
	if !d.desktopTaskOfProject(w, r, projectID, taskID) {
		return
	}
	if r.Method == http.MethodGet {
		d.relayServer(w, r, http.MethodGet, "/api/tasks/"+url.PathEscape(taskID), "")
		return
	}
	d.relayServer(w, r, http.MethodPut, "/api/tasks/"+url.PathEscape(taskID), mustJSON(update))
}

// desktopTaskAssignable relays the tracker's assignee search of a task.
func (d *agentDaemon) desktopTaskAssignable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", 405)
		return
	}
	projectID, taskID := r.URL.Query().Get("projectId"), r.URL.Query().Get("taskId")
	if projectID == "" || taskID == "" {
		http.Error(w, "Project and task required", 400)
		return
	}
	if !d.desktopTaskOfProject(w, r, projectID, taskID) {
		return
	}
	d.relayServer(w, r, http.MethodGet, "/api/tasks/"+url.PathEscape(taskID)+"/assignable?q="+url.QueryEscape(r.URL.Query().Get("q")), "")
}

// desktopTaskOfProject answers false, having written the refusal, unless the
// task is one of the project's: the page names a project, and a task id of
// another one must not be read or written through it.
func (d *agentDaemon) desktopTaskOfProject(w http.ResponseWriter, r *http.Request, projectID, taskID string) bool {
	if _, err := d.fetchConfig(r.Context(), projectID, ""); err != nil {
		http.Error(w, err.Error(), 400)
		return false
	}
	var task models.Task
	if err := d.readAPI(r.Context(), "/api/tasks/"+url.PathEscape(taskID), &task); err != nil {
		http.Error(w, err.Error(), 502)
		return false
	}
	if !taskInProject(task, projectID) {
		http.Error(w, "The task does not belong to this project", 400)
		return false
	}
	return true
}

// relayServer sends one request to the server with the paired token, so a
// tracker write is attributed to the paired user, and relays its answer.
func (d *agentDaemon) relayServer(w http.ResponseWriter, r *http.Request, method, path, body string) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(r.Context(), method, d.link.serverURL+path, reader)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := agenthttp.Client(d.link.token).Do(req)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	defer response.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 1<<20))
}

// webLink admits an absolute http or https address that carries no
// credentials, the only kind of pull request link the page may record.
func webLink(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}
