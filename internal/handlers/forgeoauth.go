package handlers

import (
	"errors"
	"log"
	"net/http"
	"net/url"

	"tasks/internal/db"
)

// GithubOAuthCallbackPath and GitlabOAuthCallbackPath are where GitHub and
// gitlab.com send a person back after the consent screen (#804). Like the Jira
// callback, they sit under /auth/, which is public: the handler reads the web
// session itself.
const (
	GithubOAuthCallbackPath = "/auth/github/callback"
	GitlabOAuthCallbackPath = "/auth/gitlab/callback"
)

// forgeDisplayName is how the messages name each forge.
var forgeDisplayName = map[string]string{"github": "GitHub", "gitlab": "GitLab"}

// connectForge starts a consent on a forge:
// POST /api/me/tracker-credentials/{github,gitlab}/connect answers where to
// send the browser. Only a web session can start one, since the callback is
// bound to it; an agent's key cannot.
func (h *Handler) connectForge(tracker string) func(http.ResponseWriter, *http.Request, string) {
	name := forgeDisplayName[tracker]
	return func(w http.ResponseWriter, r *http.Request, userID string) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || h.db.UserForWebSession(cookie.Value) != userID {
			writeError(w, http.StatusForbidden, "Connectez "+name+" depuis votre profil, dans le navigateur.")
			return
		}
		authorizeURL, err := h.db.ForgeOAuthAuthorizeURL(tracker, userID, cookie.Value)
		switch {
		case errors.Is(err, db.ErrForgeOAuthNotConfigured):
			writeJSON(w, http.StatusConflict, map[string]string{"code": tracker + "_oauth_not_configured", "error": "La connexion " + name + " n'est pas configurée sur ce serveur : enregistrez un jeton personnel."})
			return
		case err != nil:
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"authorizeUrl": authorizeURL})
	}
}

// HandleGithubOAuthCallback finishes a GitHub consent, on /auth/github/callback.
func (h *Handler) HandleGithubOAuthCallback(w http.ResponseWriter, r *http.Request) {
	h.handleForgeOAuthCallback("github")(w, r)
}

// HandleGitlabOAuthCallback finishes a gitlab.com consent, on
// /auth/gitlab/callback.
func (h *Handler) HandleGitlabOAuthCallback(w http.ResponseWriter, r *http.Request) {
	h.handleForgeOAuthCallback("gitlab")(w, r)
}

// handleForgeOAuthCallback finishes a consent on a forge and always lands the
// person on Profile → Tracker credentials, on that forge, with the outcome.
// Nothing is stored unless the outcome is "connected". The log names the
// forge, the outcome and the person, never the code, a token or the secret.
func (h *Handler) handleForgeOAuthCallback(tracker string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		query := r.URL.Query()
		outcome := db.ForgeOAuthInvalid
		userID, session := "", ""
		if cookie, err := r.Cookie(sessionCookie); err == nil {
			session = cookie.Value
			userID = h.db.UserForWebSession(session)
		}
		var err error
		switch {
		case userID == "" && query.Get("error") == "access_denied":
			outcome = db.ForgeOAuthCancelled
		case userID == "":
			err = errors.New("no web session")
		default:
			outcome, err = h.db.CompleteForgeOAuth(r.Context(), tracker, userID, session, query.Get("state"), query.Get("code"), query.Get("error") == "access_denied")
		}
		if err != nil {
			log.Printf("[ForgeOAuth] %s connexion de %q : %s (%v)", tracker, userID, outcome, err)
		} else {
			log.Printf("[ForgeOAuth] %s connexion de %q : %s", tracker, userID, outcome)
		}
		target := url.Values{}
		target.Set("trackerCredentials", tracker)
		target.Set("oauth", outcome)
		http.Redirect(w, r, "/?"+target.Encode(), http.StatusFound)
	}
}

// forgeOAuthOffered says whether people can connect a forge through its
// consent screen: its app is configured, and at least one of its trackers is
// on the public instance a grant serves.
func (h *Handler) forgeOAuthOffered(tracker string) bool {
	if !h.db.OAuthAppConfigured(tracker) {
		return false
	}
	sites, err := h.db.ConfiguredForgeTrackers(tracker)
	if err != nil {
		return false
	}
	for _, site := range sites {
		if db.ForgeSiteGranted(tracker, site) {
			return true
		}
	}
	return false
}
