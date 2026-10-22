package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"tasks/internal/db"
)

// JiraOAuthAppPath is where an admin configures the Atlassian OAuth app that
// people's Jira grants are issued to (#654, ADR 0044).
const JiraOAuthAppPath = "/api/admin/jira-oauth"

// GithubOAuthAppPath and GitlabOAuthAppPath are where an admin configures the
// GitHub OAuth App and the gitlab.com application that people's forge grants
// are issued to (#804).
const (
	GithubOAuthAppPath = "/api/admin/github-oauth"
	GitlabOAuthAppPath = "/api/admin/gitlab-oauth"
)

// HandleJiraOAuthApp is the Administration page's view of the Jira OAuth app:
//
//	GET    /api/admin/jira-oauth  {configured, clientId, secretSet, redirectUrl, source}
//	PUT    /api/admin/jira-oauth  {clientId, clientSecret?, redirectUrl}
//	DELETE /api/admin/jira-oauth
//
// All of it is an admin's, and the secret is write-only: no answer carries it,
// and an empty one on a save keeps the saved secret.
func (h *Handler) HandleJiraOAuthApp(w http.ResponseWriter, r *http.Request) {
	h.oauthAppHandler("jira")(w, r)
}

// HandleGithubOAuthApp is HandleJiraOAuthApp for the GitHub OAuth App, on
// /api/admin/github-oauth.
func (h *Handler) HandleGithubOAuthApp(w http.ResponseWriter, r *http.Request) {
	h.oauthAppHandler("github")(w, r)
}

// HandleGitlabOAuthApp is HandleJiraOAuthApp for the gitlab.com application,
// on /api/admin/gitlab-oauth.
func (h *Handler) HandleGitlabOAuthApp(w http.ResponseWriter, r *http.Request) {
	h.oauthAppHandler("gitlab")(w, r)
}

// oauthAppHandler serves one tracker's OAuth app to the Administration page,
// as HandleJiraOAuthApp describes.
func (h *Handler) oauthAppHandler(tracker string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller, ok := h.requireAdmin(w, r)
		if !ok {
			return
		}
		switch r.Method {
		case http.MethodGet:
		case http.MethodPut:
			var payload struct {
				ClientID     string `json:"clientId"`
				ClientSecret string `json:"clientSecret"`
				RedirectURL  string `json:"redirectUrl"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&payload); err != nil {
				writeError(w, http.StatusBadRequest, "Invalid payload")
				return
			}
			if err := h.db.SaveOAuthApp(tracker, payload.ClientID, payload.ClientSecret, payload.RedirectURL, caller.UserID); err != nil {
				// The refusals name the field; none quotes the secret.
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		case http.MethodDelete:
			if err := h.db.ClearOAuthApp(tracker); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		state, err := h.db.OAuthAppState(tracker)
		if err != nil && !errors.Is(err, db.ErrOAuthAppUnreadable) {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, state)
	}
}
