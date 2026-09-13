package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"tasks/internal/runner"
)

func (d *agentDaemon) desktopGitDiff(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	fail := func(status int, code, message string) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": runner.DiffError{Code: code, Message: message}})
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		fail(405, "method_not_allowed", "Use GET to inspect changes.")
		return
	}
	id := r.URL.Query().Get("id")
	d.runsMu.Lock()
	run := d.runs[id]
	if run == nil {
		d.runsMu.Unlock()
		fail(404, "run_not_found", "This execution is no longer available.")
		return
	}
	entry, root := run.desktop, run.root
	d.runsMu.Unlock()
	if entry.Directory == "" || entry.Branch == "" || root == "" {
		fail(409, "checkout_unavailable", "This execution has no recorded checkout. Launch a new execution with the updated agent.")
		return
	}
	result, err := runner.InspectWorktree(r.Context(), entry.Directory, entry.Branch, root)
	if err != nil {
		var detail *runner.DiffError
		if !errors.As(err, &detail) {
			detail = &runner.DiffError{Code: "read_failed", Message: "The checkout could not be read. Check local access and refresh."}
		}
		status := 409
		switch detail.Code {
		case "timeout":
			status = 504
		case "limit_exceeded":
			status = 413
		case "git_failed", "read_failed":
			status = 500
		}
		fail(status, detail.Code, detail.Message)
		return
	}
	result.RunID = id
	result.TaskID = entry.TaskID
	result.ProjectID = entry.ProjectID
	_ = json.NewEncoder(w).Encode(result)
}
