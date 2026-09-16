package db

import (
	"context"
	"fmt"
	"strings"
	"tasks/internal/models"
	"tasks/internal/trackerapi"
)

// keptToken applies the write-only rule shared by every tracker credential: the
// interface never receives a token back, so an empty field means "leave the
// stored one alone" and only the clear sentinel deletes it.
func keptToken(incoming, stored string) string {
	switch incoming {
	case "":
		return stored
	case TrackerTokenClearSentinel:
		return ""
	default:
		return incoming
	}
}

// trackerCredentials resolves one project's connection parameters: the project
// override first, then the user configuration, then nothing — the caller
// (trackerapi.Client) keeps its environment-derived value for every field left
// empty here. That order is what makes a token typed in the interface win over
// a stale one exported in the server's shell.
//
// It reads through the unsafe getters on purpose: it is called from the tracker
// client, itself called from code paths that already hold d.mu, and a plain
// SELECT needs no lock of its own.
func (d *DB) trackerCredentials(projectID string) trackerapi.Credentials {
	var cred trackerapi.Credentials
	if settings, err := d.getSettingsUnsafe(); err == nil && settings != nil {
		cred = trackerapi.Credentials{
			GithubURL:     settings.GithubApiUrl,
			GithubToken:   settings.GithubToken,
			GitlabURL:     settings.GitlabUrl,
			GitlabProject: settings.GitlabProject,
			GitlabToken:   settings.GitlabToken,
		}
	}
	if strings.TrimSpace(projectID) == "" {
		return cred
	}
	proj, err := d.getProjectByIDUnsafe(projectID)
	if err != nil || proj == nil {
		return cred
	}
	overrideWith(&cred.GithubURL, proj.GithubApiUrl)
	overrideWith(&cred.GithubToken, proj.GithubToken)
	overrideWith(&cred.GitlabURL, proj.GitlabUrl)
	overrideWith(&cred.GitlabProject, proj.GitlabProject)
	overrideWith(&cred.GitlabToken, proj.GitlabToken)
	return cred
}

func overrideWith(target *string, override string) {
	if strings.TrimSpace(override) != "" {
		*target = override
	}
}

// tracker returns the tracker client carrying the credentials of one project.
func (d *DB) tracker(projectID string) *trackerapi.Client { return d.trackers.For(projectID) }

// withoutTrackerTokens strips the credentials from a settings row on its way to
// a client and reports them through flags instead: whether one is stored, and
// whether the server environment supplies one in its absence. The interface
// needs that second flag to say "provided by the environment" rather than
// showing an empty field on a server that is in fact configured.
func (d *DB) withoutTrackerTokens(s *models.Settings) *models.Settings {
	if s == nil {
		return nil
	}
	s.JiraAPITokenSet = s.JiraAPIToken != ""
	s.GithubTokenSet = s.GithubToken != ""
	s.GitlabTokenSet = s.GitlabToken != ""
	if d != nil && d.trackers != nil {
		s.GithubTokenFromEnv = !s.GithubTokenSet && d.trackers.GithubToken != ""
		s.GitlabTokenFromEnv = !s.GitlabTokenSet && d.trackers.GitlabToken != ""
	}
	s.JiraAPIToken, s.GithubToken, s.GitlabToken = "", "", ""
	return s
}

// withoutProjectTokens does the same for a project's overrides. A project has no
// FromEnv flag: the environment is a property of the server, not of a project.
func withoutProjectTokens(p *models.Project) *models.Project {
	if p == nil {
		return nil
	}
	p.GithubTokenSet = p.GithubToken != ""
	p.GitlabTokenSet = p.GitlabToken != ""
	p.GithubToken, p.GitlabToken = "", ""
	return p
}

// CheckTrackerCredentials authenticates connection parameters against the
// instance and answers with the account they belong to. An empty URL or token
// falls back to what is already resolved for the server, so the setup screen can
// re-check a stored credential it never received back.
func (d *DB) CheckTrackerCredentials(ctx context.Context, tracker, apiURL, token string) (string, error) {
	client := d.tracker("")
	switch strings.ToLower(strings.TrimSpace(tracker)) {
	case "github":
		return client.CheckGithub(ctx, firstNonEmpty(apiURL, client.GithubURL), firstNonEmpty(token, client.GithubToken))
	case "gitlab":
		return client.CheckGitlab(ctx, firstNonEmpty(apiURL, client.GitlabURL), firstNonEmpty(token, client.GitlabToken))
	default:
		return "", fmt.Errorf("aucune vérification de connexion pour le tracker %q", tracker)
	}
}

// SaveTrackerCredentials stores one tracker's connection parameters in the user
// configuration. The empty-means-unchanged and clear-sentinel rules of
// UpdateSettings apply, so the caller may leave the token out.
func (d *DB) SaveTrackerCredentials(tracker, apiURL, project, token string) (*models.Settings, error) {
	current, err := d.GetSettings()
	if err != nil {
		return nil, err
	}
	update := *current
	switch strings.ToLower(strings.TrimSpace(tracker)) {
	case "github":
		update.GithubApiUrl = strings.TrimSpace(apiURL)
		update.GithubToken = strings.TrimSpace(token)
		if strings.TrimSpace(project) != "" {
			update.GithubRepo = strings.TrimSpace(project)
		}
	case "gitlab":
		update.GitlabUrl = strings.TrimSpace(apiURL)
		update.GitlabToken = strings.TrimSpace(token)
		update.GitlabProject = strings.TrimSpace(project)
	default:
		return nil, fmt.Errorf("tracker %q inconnu", tracker)
	}
	return d.UpdateSettings(update)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
