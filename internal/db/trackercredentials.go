package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"tasks/internal/models"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
	"time"
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
			JiraURL:       settings.JiraUrl,
			JiraEmail:     settings.JiraEmail,
			JiraToken:     settings.JiraAPIToken,
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
	// A project overrides the Jira site only: one Atlassian API token is valid
	// on every site of the account, so the e-mail and token stay global.
	if strings.EqualFold(strings.TrimSpace(proj.IssueTracker), "jira") {
		overrideWith(&cred.JiraURL, proj.TrackerUrl)
	}
	return cred
}

func overrideWith(target *string, override string) {
	if strings.TrimSpace(override) != "" {
		*target = override
	}
}

// tracker returns the tracker client carrying the credentials of one project.
func (d *DB) tracker(projectID string) *trackerapi.Client { return d.trackers.For(projectID) }

// trackerAs is tracker with the credentials of the person who asked for the
// work substituted where they stored any. A tracker attributes a write to the
// account behind the token, so an operation somebody asked for travels under
// their own token rather than the server's. Unattended work names nobody and
// keeps the project credential, which is why an empty user is not an error.
func (d *DB) trackerAs(userID, trackerName, projectID string) *trackerapi.Client {
	client, _, err := d.trackers.ForActingUser(userID, trackerName, projectID)
	if err != nil || client == nil {
		// A credential that cannot be resolved is not a reason to drop the
		// request: the project's own is still there, and the call will say for
		// itself whether it is accepted.
		return d.trackers.For(projectID)
	}
	return client
}

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
		s.JiraAPITokenFromEnv = !s.JiraAPITokenSet && d.trackers.JiraToken != ""
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

// credentialCheckTimeout bounds a credential check. Somebody is waiting in
// front of the screen for this one answer, so the generic sixty seconds of the
// tracker client is far too long: a site that has not answered in five seconds
// is not going to make the wait worthwhile, and a wrong host never answers at
// all.
const credentialCheckTimeout = 5 * time.Second

// CheckTrackerCredentials authenticates connection parameters against the
// instance and answers with the account they belong to. An empty URL or token
// falls back to what is already resolved for the server, so the setup screen can
// re-check a stored credential it never received back. The e-mail only matters
// to Jira, which authenticates the account rather than a bare token.
func (d *DB) CheckTrackerCredentials(ctx context.Context, trackerName, apiURL, email, token string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, credentialCheckTimeout)
	defer cancel()
	account, err := d.checkTrackerCredentials(ctx, trackerName, apiURL, email, token)
	if errors.Is(err, context.DeadlineExceeded) {
		// The deadline is ours, so the message says what it means rather than
		// leaving "context deadline exceeded" in front of somebody.
		return "", fmt.Errorf("l'instance n'a pas répondu en %d secondes : vérifiez l'adresse du site", int(credentialCheckTimeout.Seconds()))
	}
	return account, err
}

func (d *DB) checkTrackerCredentials(ctx context.Context, trackerName, apiURL, email, token string) (string, error) {
	client := d.tracker("")
	switch strings.ToLower(strings.TrimSpace(trackerName)) {
	case "github":
		site, own := "", ""
		if user := tracker.ActingUser(ctx); user != "" {
			var err error
			if site, _, own, err = d.UserTrackerCredentialsFor(user, "github"); err != nil {
				return "", err
			}
		}
		return client.CheckGithub(ctx,
			firstNonEmpty(apiURL, site, client.GithubURL),
			firstNonEmpty(token, own, client.GithubToken))
	case "gitlab":
		return client.CheckGitlab(ctx, firstNonEmpty(apiURL, client.GitlabURL), firstNonEmpty(token, client.GitlabToken))
	case "jira":
		// A Jira credential is personal, so re-checking a stored one falls back
		// to the caller's own token rather than the server's: the interface
		// never received the token back and cannot resend it.
		site, mail, own := "", "", ""
		if user := tracker.ActingUser(ctx); user != "" {
			var err error
			// A sealed credential the person has not unlocked is an error, not
			// an absence: falling through checked the server's token instead
			// and reported "connected as <the service account>", telling them
			// their own sealed credential works when it was never touched.
			if site, mail, own, err = d.UserTrackerCredentialsFor(user, "jira"); err != nil {
				return "", err
			}
		}
		return client.CheckJira(ctx,
			firstNonEmpty(apiURL, site, client.JiraURL),
			firstNonEmpty(email, mail, client.JiraEmail),
			firstNonEmpty(token, own, client.JiraToken))
	default:
		return "", fmt.Errorf("aucune vérification de connexion pour le tracker %q", trackerName)
	}
}

// SaveTrackerCredentials stores one tracker's connection parameters in the user
// configuration. The empty-means-unchanged and clear-sentinel rules of
// UpdateSettings apply, so the caller may leave the token out. For Jira,
// project is the default project key and email the account e-mail.
func (d *DB) SaveTrackerCredentials(tracker, apiURL, project, email, token string) (*models.Settings, error) {
	current, err := d.GetSettings()
	if err != nil {
		return nil, err
	}
	update := *current
	switch strings.ToLower(strings.TrimSpace(tracker)) {
	case "jira":
		update.JiraUrl = strings.TrimSpace(apiURL)
		update.JiraAPIToken = strings.TrimSpace(token)
		if strings.TrimSpace(email) != "" {
			update.JiraEmail = strings.TrimSpace(email)
		}
		if strings.TrimSpace(project) != "" {
			update.JiraProject = strings.ToUpper(strings.TrimSpace(project))
		}
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
