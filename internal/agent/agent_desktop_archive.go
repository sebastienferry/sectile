package agent

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"tasks/internal/agenthttp"
)

// archiveWorkspaceCapability tells Desktop this agent cleans a task's
// worktrees before it is archived (#755).
const archiveWorkspaceCapability = "archive-workspace"

// archiveTaskNotFound is the server route's refusal of a task it does not
// know, which tells it apart from a server that has no such route.
const archiveTaskNotFound = "Task not found"

// desktopArchiveWorkspace relays Desktop's archive of a ticket task to the
// server, which decides what may go and asks this agent back for the Git
// work through archive_workspace. The server's answer is copied as it is.
func (d *agentDaemon) desktopArchiveWorkspace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var input struct {
		TaskID string `json:"taskId"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&input) != nil {
		http.Error(w, "Invalid request body", 400)
		return
	}
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" || strings.TrimSpace(input.TaskID) == "" {
		http.Error(w, "Project and task required", 400)
		return
	}
	if _, err := d.fetchConfig(r.Context(), projectID, ""); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, d.link.serverURL+"/api/tasks/"+url.PathEscape(input.TaskID)+"/archive-workspace", nil)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	response, err := agenthttp.Client(d.link.token).Do(req)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	defer response.Body.Close()
	// A server that predates the route answers 404 or 405 with its own
	// message: say which side to update rather than echo it. The route's only
	// 404 is a task it does not know.
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		var refusal struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(body, &refusal) != nil || refusal.Error != archiveTaskNotFound {
			http.Error(w, "The Sectile server cannot remove a task's worktree on archive: update the server.", 502)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.StatusCode)
		_, _ = w.Write(body)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 1<<20))
}
