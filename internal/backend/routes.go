package backend

import (
	"net/http"

	"tasks/internal/handlers"
)

// RegisterRoutes installs the API, authentication, MCP, and agent transport
// endpoints without the standalone server's website assets.
func RegisterRoutes(mux *http.ServeMux, h *handlers.Handler) {
	// API Routes
	mux.HandleFunc(handlers.HealthPath, h.HandleHealth)
	mux.HandleFunc(handlers.ReadyPath, h.HandleReady)
	mux.HandleFunc(handlers.VersionPath, h.HandleVersion)
	mux.HandleFunc(handlers.ChangelogPath, h.HandleChangelog)
	mux.HandleFunc("/api/cli-status", h.HandleCliStatus)
	mux.HandleFunc("/api/git-status", h.HandleGitStatus)
	mux.HandleFunc("/api/git/status", h.HandleGitStatus)
	mux.HandleFunc("/api/git/branches", h.HandleGitBranches)
	mux.HandleFunc("/api/git-branches", h.HandleGitBranches)
	mux.HandleFunc("/api/git/branches/clean", h.HandleGitCleanBranches)
	mux.HandleFunc("/api/git/branches/delete", h.HandleGitDeleteBranch)
	mux.HandleFunc("/api/git/checkout", h.HandleGitCheckout)
	mux.HandleFunc("/api/git-checkout", h.HandleGitCheckout)
	mux.HandleFunc("/api/sync/all", h.HandleSyncAll)
	mux.HandleFunc("/api/sync/github", h.HandleSyncGithub)
	mux.HandleFunc("/api/sync/jira", h.HandleSyncJira)
	mux.HandleFunc("/api/sync/gitlab", h.HandleSyncGitlab)
	mux.HandleFunc("/api/sync/auto", h.HandleAutoSyncStatus)
	mux.HandleFunc("/api/setup/tracker", h.HandleTrackerSetup)
	mux.HandleFunc("/api/setup/tracker/check", h.HandleTrackerSetup)
	mux.HandleFunc("/api/skills", h.HandleSkills)
	mux.HandleFunc("/api/spec-framework/status", h.HandleSpecFrameworkStatus)
	mux.HandleFunc("/api/spec-framework/install", h.HandleSpecFrameworkInstall)
	mux.HandleFunc("/api/projects", h.HandleProjects)
	mux.HandleFunc("/api/projects/", h.HandleProjectDetail)
	mux.HandleFunc("/api/tasks", h.HandleTasks)
	mux.HandleFunc("/api/tasks/stage", h.HandleTasks)
	mux.HandleFunc("/api/tasks/transition", h.HandleTasks)
	mux.HandleFunc("/api/tasks/postback", h.HandleTaskPostBack)
	mux.HandleFunc("/api/events", h.HandleEventsSSE)
	mux.HandleFunc("/api/events/sse", h.HandleEventsSSE)
	mux.HandleFunc("/api/tasks/facets", h.HandleTaskFacets)
	mux.HandleFunc("/api/teams", h.HandleTeams)
	mux.HandleFunc("/api/teams/", h.HandleTeams)
	mux.HandleFunc("/api/tasks/pins", h.HandleTaskPins)
	mux.HandleFunc("/api/tasks/", h.HandleTaskDetail)
	mux.HandleFunc("/api/activities", h.HandleActivities)
	mux.HandleFunc("/api/activities/", h.HandleActivityDetail)
	mux.HandleFunc("/api/settings", h.HandleSettings)
	mux.HandleFunc("/api/open-editor", h.HandleOpenEditor)
	mux.HandleFunc("/api/editor/open", h.HandleOpenEditor)
	mux.HandleFunc("/api/open-terminal", h.HandleOpenExternalTerminal)
	mux.HandleFunc("/api/terminal/external", h.HandleOpenExternalTerminal)

	// Interactive PTY Terminal Routes & WebSocket
	mux.HandleFunc("/ws/terminal", h.HandleTerminalWs)
	mux.HandleFunc("/api/terminal/sessions", h.HandleTerminalSessions)
	mux.HandleFunc("/api/terminal/send", h.HandleTerminalSend)
	mux.HandleFunc("/api/terminal/reset", h.HandleTerminalReset)

	// Sign-in routes exist only when a provider is configured; the interface
	// asks /api/me which of the two modes it is in.
	mux.HandleFunc("/auth/login", h.HandleLogin)
	mux.HandleFunc("/auth/callback", h.HandleAuthCallback)
	mux.HandleFunc(handlers.DesktopOAuthPath, h.HandleDesktopOAuth)
	mux.HandleFunc(handlers.DesktopOAuthStartPath, h.HandleDesktopOAuthStart)
	mux.HandleFunc(handlers.JiraOAuthCallbackPath, h.HandleJiraOAuthCallback)
	mux.HandleFunc(handlers.GithubOAuthCallbackPath, h.HandleGithubOAuthCallback)
	mux.HandleFunc(handlers.GitlabOAuthCallbackPath, h.HandleGitlabOAuthCallback)
	mux.HandleFunc("/auth/logout", h.HandleLogout)
	// Local sign-in exists only without a provider: an e-mail, no password, the
	// temporary mode of a team that has not connected its identity provider yet.
	mux.HandleFunc("/auth/local", h.HandleLocalSignIn)
	// Desktop and `sectile-agent pair` sign in through the browser: a session hands a pairing code to a loopback port (#717).
	mux.HandleFunc("/auth/workstation", h.HandleWorkstationSignIn)
	mux.HandleFunc("/api/me", h.HandleCurrentUser)
	mux.HandleFunc("/api/me/tracker-credentials", h.HandleUserTrackerCredentials)
	mux.HandleFunc("/api/me/tracker-credentials/", h.HandleUserTrackerCredentials)
	mux.HandleFunc("/api/me/assignee-identities", h.HandleAssigneeIdentities)
	mux.HandleFunc("/api/me/project-bookmarks", h.HandleUserProjectBookmarks)
	mux.HandleFunc("/api/me/project-bookmarks/", h.HandleUserProjectBookmarks)
	mux.HandleFunc("/api/me/board-views", h.HandleBoardViews)
	mux.HandleFunc("/api/me/board-views/", h.HandleBoardViews)
	// The admin's users view: list accounts and change roles.
	mux.HandleFunc("/api/users", h.HandleUsers)
	mux.HandleFunc("/api/users/", h.HandleUsers)
	mux.HandleFunc(handlers.AdminStatsPath, h.HandleAdminStats)
	// The server credential of each tracker provider, an admin's to set.
	mux.HandleFunc(handlers.ServerTrackerCredentialsPath, h.HandleServerTrackerCredentials)
	mux.HandleFunc(handlers.ServerTrackerCredentialsPath+"/", h.HandleServerTrackerCredentials)
	// Trackers are configured by an admin, picked and synchronised by members
	// (#741).
	mux.HandleFunc(handlers.AdminTrackersPath, h.HandleAdminTrackers)
	mux.HandleFunc(handlers.AdminTrackersPath+"/", h.HandleAdminTrackers)
	mux.HandleFunc(handlers.TrackersPath, h.HandleTrackers)
	mux.HandleFunc(handlers.TrackersPath+"/", h.HandleTrackers)
	mux.HandleFunc(handlers.JiraOAuthAppPath, h.HandleJiraOAuthApp)
	mux.HandleFunc(handlers.GithubOAuthAppPath, h.HandleGithubOAuthApp)
	mux.HandleFunc(handlers.GitlabOAuthAppPath, h.HandleGitlabOAuthApp)

	// Prometheus metrics. Outside /api/, so the session guard leaves them
	// public; registered before the interface's catch-all.

	mux.Handle("/mcp", h.MCPHandler())
	mux.HandleFunc("/api/mcp/sessions", h.HandleMCPSessions)
	// Pairing binds one workstation to one user; the code is the only
	// unauthenticated credential, and it is single use and short lived.
	mux.HandleFunc("/api/pairing-codes", h.HandlePairingCode)
	mux.HandleFunc("/api/devices", h.HandleDeviceCredentials)
	mux.HandleFunc("/api/v1/devices", h.HandleDeviceCredentials)
	mux.HandleFunc("/api/v1/agent/pair", h.HandleAgentPair)
	mux.Handle("/api/v1/agent/identity", h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentIdentity)))
	mux.Handle("/api/v1/agent/config", h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentConfig)))
	mux.Handle("/api/v1/agent/projects", h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentProjects)))
	mux.Handle("/api/v1/agent/execution-seed", h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentExecutionSeed)))
	mux.Handle("/api/v1/agent/capabilities", h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentCapabilities)))
	mux.Handle("/api/v1/agent/run-output", h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentRunOutput)))

	// Remote Agent WebSocket & Dispatch Routes
	mux.HandleFunc("/ws/agent-connect", h.HandleAgentConnect)
	mux.HandleFunc("/api/agent/status", h.HandleAgentStatus)
	mux.HandleFunc("/api/agent/dispatch", h.HandleAgentDispatch)
	mux.HandleFunc("/api/agent/pull", h.HandleAgentPull)

}
