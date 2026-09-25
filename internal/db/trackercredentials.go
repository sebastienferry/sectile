package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"tasks/internal/models"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
	"time"
)

// trackerCredentials resolves one project's connection parameters. The URLs and
// the GitLab project come from the project override first, then the user
// configuration; the caller (trackerapi.Client) keeps its environment-derived
// value for every field left empty here.
//
// The credentials come from the stored server credential of each provider and
// from nowhere else (#464): a project has none of its own, and the settings row
// no longer holds any. A stored credential wins as a whole, the Jira e-mail
// with its token, so it is never completed with half of the environment's
// pair. Nothing stored leaves the fields empty and the environment answers. A
// stored credential the key does not open is flagged, so the call fails
// instead of reaching the tracker under the environment's account.
//
// It reads through the unsafe getters on purpose: it is called from the tracker
// client, itself called from code paths that already hold d.mu, and a plain
// SELECT needs no lock of its own.
func (d *DB) trackerCredentials(projectID string) trackerapi.Credentials {
	var cred trackerapi.Credentials
	if settings, err := d.getSettingsUnsafe(); err == nil && settings != nil {
		cred = trackerapi.Credentials{
			GithubURL:     settings.GithubApiUrl,
			GitlabURL:     settings.GitlabUrl,
			GitlabProject: settings.GitlabProject,
			JiraURL:       settings.JiraUrl,
		}
	}
	for _, tracker := range ServerCredentialTrackers {
		email, token, found, err := d.storedServerTrackerCredential(tracker)
		if !found {
			continue
		}
		unreadable := err != nil
		switch tracker {
		case "github":
			cred.GithubToken, cred.GithubUnreadable = token, unreadable
		case "gitlab":
			cred.GitlabToken, cred.GitlabUnreadable = token, unreadable
		case "jira":
			cred.JiraEmail, cred.JiraToken, cred.JiraUnreadable = email, token, unreadable
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
	overrideWith(&cred.GitlabURL, proj.GitlabUrl)
	overrideWith(&cred.GitlabProject, proj.GitlabProject)
	// A project overrides the Jira site only: one Atlassian API token is valid
	// on every site of the account, so the server credential serves them all.
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

// withoutTrackerTokens strips the credential columns from a settings row on its
// way to a client and reports the server credentials through flags instead:
// Set when one is stored, FromEnv when the environment supplies it in the
// absence of a stored one. That is all a member may know of them; the
// Administration page reads the rest from ServerTrackerCredentialStates.
func (d *DB) withoutTrackerTokens(s *models.Settings) *models.Settings {
	if s == nil {
		return nil
	}
	s.JiraAPIToken, s.GithubToken, s.GitlabToken, s.JiraEmail = "", "", "", ""
	if d == nil {
		return s
	}
	for _, tracker := range ServerCredentialTrackers {
		state, err := d.ServerTrackerCredentialState(tracker)
		if err != nil {
			continue
		}
		set, fromEnv := state.Source == ServerCredentialStored, state.Source == ServerCredentialEnvironment
		switch tracker {
		case "github":
			s.GithubTokenSet, s.GithubTokenFromEnv = set, fromEnv
		case "gitlab":
			s.GitlabTokenSet, s.GitlabTokenFromEnv = set, fromEnv
		case "jira":
			s.JiraAPITokenSet, s.JiraAPITokenFromEnv = set, fromEnv
		}
	}
	return s
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
		probed := firstNonEmpty(apiURL, site, client.GithubURL)
		account, err := client.CheckGithub(ctx, probed, firstNonEmpty(token, own, client.GithubToken))
		// Only a check of the caller's own stored token, on the site it is
		// stored for, says whose it is. A typed token, or the server's one,
		// proves nothing about the caller: the admin setup screen goes
		// through here too.
		if err == nil && token == "" && own != "" && sameGithubSite(probed, firstNonEmpty(site, client.GithubURL)) {
			d.recordTrackerAccount(tracker.ActingUser(ctx), "github", account)
		}
		return account, err
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
		probedSite, probedEmail := firstNonEmpty(apiURL, site, client.JiraURL), firstNonEmpty(email, mail, client.JiraEmail)
		account, err := client.CheckJira(ctx, probedSite, probedEmail, firstNonEmpty(token, own, client.JiraToken))
		// As for GitHub: the stored token, on its own site and with its own
		// e-mail, or nothing is learnt.
		if err == nil && token == "" && own != "" &&
			trackerapi.JiraSite(probedSite) == trackerapi.JiraSite(firstNonEmpty(site, client.JiraURL)) &&
			strings.EqualFold(strings.TrimSpace(probedEmail), strings.TrimSpace(firstNonEmpty(mail, client.JiraEmail))) {
			d.recordTrackerAccount(tracker.ActingUser(ctx), "jira", account)
		}
		return account, err
	default:
		return "", fmt.Errorf("aucune vérification de connexion pour le tracker %q", trackerName)
	}
}

// ConfirmUserTrackerCredential asks the tracker whom one person's stored
// credential belongs to, and records the answer for My Tasks (#468). It is
// called right after the credential is saved, while a sealed one is still
// open. A failure leaves the account empty and is only reported: the
// credential itself is stored either way.
func (d *DB) ConfirmUserTrackerCredential(ctx context.Context, userID, trackerName string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, credentialCheckTimeout)
	defer cancel()
	trackerName = strings.ToLower(strings.TrimSpace(trackerName))
	d.mu.RLock()
	site, email, token, err := d.userTrackerCredential(userID, trackerName)
	d.mu.RUnlock()
	if err != nil {
		return "", err
	}
	client := d.tracker("")
	var account string
	switch trackerName {
	case "github":
		account, err = client.CheckGithub(ctx, firstNonEmpty(site, client.GithubURL), token)
	case "gitlab":
		account, err = client.CheckGitlab(ctx, firstNonEmpty(site, client.GitlabURL), token)
	case "jira":
		account, err = client.CheckJira(ctx, firstNonEmpty(site, client.JiraURL), firstNonEmpty(email, client.JiraEmail), token)
	default:
		return "", fmt.Errorf("aucune vérification de connexion pour le tracker %q", trackerName)
	}
	if err != nil {
		return "", err
	}
	return account, d.SetUserTrackerCredentialAccount(userID, trackerName, account)
}

// recordTrackerAccount keeps what a check learnt about the caller's own
// credential. Failing to write it down does not fail the check, which did
// answer.
func (d *DB) recordTrackerAccount(userID, trackerName, account string) {
	if strings.TrimSpace(userID) == "" {
		return
	}
	if err := d.SetUserTrackerCredentialAccount(userID, trackerName, account); err != nil {
		log.Printf("[TrackerCredentials] compte %s non enregistré : %v", trackerName, err)
	}
}

// sameGithubSite says whether two GitHub API addresses are one, an empty one
// being the public instance, as the client reads it.
func sameGithubSite(a, b string) bool {
	norm := func(raw string) string {
		raw = strings.TrimRight(strings.TrimSpace(raw), "/")
		if raw == "" {
			raw = trackerapi.DefaultGithubURL
		}
		return strings.ToLower(raw)
	}
	return norm(a) == norm(b)
}

// SaveTrackerCredentials stores one tracker's connection parameters in the user
// configuration: its URL and its default project. The credential is not one of
// them any more; an admin stores it with SaveServerTrackerCredential (#464).
func (d *DB) SaveTrackerCredentials(tracker, apiURL, project string) (*models.Settings, error) {
	current, err := d.GetSettings()
	if err != nil {
		return nil, err
	}
	update := *current
	switch strings.ToLower(strings.TrimSpace(tracker)) {
	case "jira":
		update.JiraUrl = strings.TrimSpace(apiURL)
		if strings.TrimSpace(project) != "" {
			update.JiraProject = strings.ToUpper(strings.TrimSpace(project))
		}
	case "github":
		update.GithubApiUrl = strings.TrimSpace(apiURL)
		if strings.TrimSpace(project) != "" {
			update.GithubRepo = strings.TrimSpace(project)
		}
	case "gitlab":
		update.GitlabUrl = strings.TrimSpace(apiURL)
		update.GitlabProject = strings.TrimSpace(project)
	default:
		return nil, fmt.Errorf("tracker %q inconnu", tracker)
	}
	return d.UpdateSettings(update)
}

// CheckServerTrackerCredentials authenticates a server credential against the
// deployment's instance of its provider and answers the account it belongs to.
// An empty token checks the credential already resolved, stored or from the
// environment; an empty URL, the deployment's instance. Unlike CheckTrackerCredentials it never falls back to the
// caller's personal token: an admin checking the server's access must not be
// told "connected as" themselves.
func (d *DB) CheckServerTrackerCredentials(ctx context.Context, trackerName, apiURL, email, token string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, credentialCheckTimeout)
	defer cancel()
	account, err := d.checkServerTrackerCredentials(ctx, strings.ToLower(strings.TrimSpace(trackerName)), strings.TrimSpace(apiURL), strings.TrimSpace(email), strings.TrimSpace(token))
	if errors.Is(err, context.DeadlineExceeded) {
		return "", fmt.Errorf("l'instance n'a pas répondu en %d secondes : vérifiez l'adresse du site", int(credentialCheckTimeout.Seconds()))
	}
	return account, err
}

func (d *DB) checkServerTrackerCredentials(ctx context.Context, trackerName, apiURL, email, token string) (string, error) {
	if !IsServerCredentialTracker(trackerName) {
		return "", fmt.Errorf("aucune vérification de connexion pour le tracker %q", trackerName)
	}
	if token == "" {
		if _, _, found, err := d.storedServerTrackerCredential(trackerName); found && err != nil {
			return "", fmt.Errorf("l'accès serveur %s enregistré ne peut pas être déchiffré avec la clé du serveur : enregistrez-le à nouveau", trackerName)
		}
	}
	client := d.tracker("")
	switch trackerName {
	case "github":
		return client.CheckGithub(ctx, firstNonEmpty(apiURL, client.GithubURL), firstNonEmpty(token, client.GithubToken))
	case "gitlab":
		return client.CheckGitlab(ctx, firstNonEmpty(apiURL, client.GitlabURL), firstNonEmpty(token, client.GitlabToken))
	default:
		// The pair travels together: a new token is checked with the e-mail
		// typed next to it, never with the stored one.
		if token == "" {
			email, token = client.JiraEmail, client.JiraToken
		}
		return client.CheckJira(ctx, firstNonEmpty(apiURL, client.JiraURL), email, token)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
