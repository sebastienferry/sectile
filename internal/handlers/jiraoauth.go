package handlers

import (
	"errors"
	"log"
	"net/http"
	"net/url"

	"tasks/internal/db"
)

// JiraOAuthCallbackPath is where Atlassian sends a person back after the
// consent screen. It sits under /auth/, which is public: the handler reads the
// web session itself, and the session cookie is SameSite=Lax, so it travels
// with Atlassian's top-level redirect.
const JiraOAuthCallbackPath = "/auth/jira/callback"

// connectJira starts a consent: POST /api/me/tracker-credentials/jira/connect
// answers where to send the browser. Only a web session can start one, since
// the callback is bound to it; an agent's key cannot.
func (h *Handler) connectJira(w http.ResponseWriter, r *http.Request, userID string) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || h.db.UserForWebSession(cookie.Value) != userID {
		writeError(w, http.StatusForbidden, "Connectez Jira depuis votre profil, dans le navigateur.")
		return
	}
	authorizeURL, err := h.db.JiraOAuthAuthorizeURL(userID, cookie.Value)
	switch {
	case errors.Is(err, db.ErrJiraOAuthNotConfigured):
		writeJSON(w, http.StatusConflict, map[string]string{"code": "jira_oauth_not_configured", "error": "La connexion Jira n'est pas configurée sur ce serveur : enregistrez un jeton d'API."})
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"authorizeUrl": authorizeURL})
}

// HandleJiraOAuthCallback finishes a consent and always lands the person on
// Profile → Tracker credentials, on Jira, with the outcome. Nothing is stored
// unless the outcome is "connected". The log names the outcome and the person,
// never the code, a token or the secret.
func (h *Handler) HandleJiraOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	query := r.URL.Query()
	outcome := db.JiraOAuthInvalid
	userID, session := "", ""
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		session = cookie.Value
		userID = h.db.UserForWebSession(session)
	}
	var err error
	switch {
	case userID == "" && query.Get("error") == "access_denied":
		outcome = db.JiraOAuthCancelled
	case userID == "":
		err = errors.New("no web session")
	default:
		outcome, err = h.db.CompleteJiraOAuth(r.Context(), userID, session, query.Get("state"), query.Get("code"), query.Get("error") == "access_denied")
	}
	if err != nil {
		log.Printf("[JiraOAuth] connexion de %q : %s (%v)", userID, outcome, err)
	} else {
		log.Printf("[JiraOAuth] connexion de %q : %s", userID, outcome)
	}
	target := url.Values{}
	target.Set("trackerCredentials", "jira")
	target.Set("jiraOAuth", outcome)
	http.Redirect(w, r, "/?"+target.Encode(), http.StatusFound)
}
