package db

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/forgeoauth"
	"tasks/internal/forgeoauth/forgeoauthtest"
	"tasks/internal/models"
	"tasks/internal/secrets"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

// forgeTrackers are the providers a forge grant serves.
var forgeTrackers = []string{"github", "gitlab"}

// forgeGrantFixture is a deployment with a forge's OAuth app, a fake of that
// forge, and a project on its public instance.
type forgeGrantFixture struct {
	d       *DB
	tracker string
	fake    *forgeoauthtest.Fake
	project *models.Project
}

func setForgeOAuthEnvironment(t *testing.T, trackerName string) {
	prefix := oauthAppEnvPrefix[trackerName]
	t.Setenv(prefix+"CLIENT_ID", forgeoauthtest.ClientID)
	t.Setenv(prefix+"CLIENT_SECRET", forgeoauthtest.ClientSecret)
	t.Setenv(prefix+"REDIRECT_URL", "https://sectile.example.com/auth/"+trackerName+"/callback")
}

func newForgeGrantFixture(t *testing.T, trackerName string) *forgeGrantFixture {
	t.Helper()
	provider, _ := forgeoauth.ForTracker(trackerName)
	setForgeOAuthEnvironment(t, trackerName)
	return newForgeGrantFixtureOn(t, testDB(t), forgeoauthtest.New(t, provider), trackerName)
}

// newForgeGrantFixtureOn is newForgeGrantFixture on a store the caller opened.
func newForgeGrantFixtureOn(t *testing.T, d *DB, fake *forgeoauthtest.Fake, trackerName string) *forgeGrantFixture {
	t.Helper()
	d.SetForgeEndpoints(trackerName, fake.Endpoints(), nil)
	request := models.CreateProjectRequest{Name: "Forge", IssueTracker: trackerName, GithubRepo: "acme/app"}
	if trackerName == "gitlab" {
		request = models.CreateProjectRequest{Name: "Forge", IssueTracker: trackerName, GitlabProject: "acme/app"}
	}
	project, err := d.CreateProject(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{"usr_ada", "usr_grace"} {
		if err := d.EnsureUser(user); err != nil {
			t.Fatal(err)
		}
	}
	return &forgeGrantFixture{d: d, tracker: trackerName, fake: fake, project: project}
}

// connect runs one consent of user, for account.
func (f *forgeGrantFixture) connect(t *testing.T, d *DB, user, account string) string {
	t.Helper()
	state, err := d.StartOAuthFlow(f.tracker, user, "session-"+user)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := d.CompleteForgeOAuth(context.Background(), f.tracker, user, "session-"+user, state, f.fake.Consent(account), false)
	if outcome == ForgeOAuthConnected && err != nil {
		t.Fatalf("connected with an error: %v", err)
	}
	return outcome
}

func (f *forgeGrantFixture) as(user string) context.Context {
	return tracker.WithTracker(tracker.WithActingUser(context.Background(), user), f.d.ProjectDefaultTracker(f.project.ID))
}

// writeToken is the token a write user makes on the project carries.
func (f *forgeGrantFixture) writeToken(t *testing.T, d *DB, user string) (string, error) {
	t.Helper()
	client, err := d.trackerForWrite(f.as(user), f.tracker, f.project.ID)
	if err != nil {
		return "", err
	}
	if f.tracker == "github" {
		return client.GithubToken, nil
	}
	return client.GitlabToken, nil
}

// account is who the fake forge says a token belongs to.
func (f *forgeGrantFixture) account(t *testing.T, token string) string {
	t.Helper()
	apiBase := f.fake.Endpoints().APIBase
	var account string
	var err error
	if f.tracker == "github" {
		account, err = f.d.trackers.CheckGithub(context.Background(), apiBase, token)
	} else {
		account, err = f.d.trackers.CheckGitlab(context.Background(), apiBase, token)
	}
	if err != nil {
		return ""
	}
	return account
}

// expire makes the stored access token of user expire now.
func (f *forgeGrantFixture) expire(t *testing.T, d *DB, user string) {
	t.Helper()
	row, err := d.readGrantRow(f.tracker, user)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := d.openForgeGrant(f.tracker, user, row.record)
	if err != nil {
		t.Fatal(err)
	}
	grant.ExpiresAt = time.Now().Add(-time.Second)
	record, err := d.sealForgeGrant(f.tracker, user, grant)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec(`UPDATE user_tracker_credentials SET record = ? WHERE user_id = ? AND tracker = ?`, record, user, f.tracker); err != nil {
		t.Fatal(err)
	}
}

func forgeCredential(t *testing.T, d *DB, user, trackerName string) *UserCredential {
	t.Helper()
	list, err := d.UserTrackerCredentials(user)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range list {
		if c.Tracker == trackerName {
			return &c
		}
	}
	return nil
}

// One consent stores a grant under the account the forge confirmed, and the
// person's writes carry the grant's token, which the forge attributes to them.
func TestAConnectedForgePersonWritesUnderTheirOwnAccount(t *testing.T) {
	for _, trackerName := range forgeTrackers {
		t.Run(trackerName, func(t *testing.T) {
			f := newForgeGrantFixture(t, trackerName)
			if outcome := f.connect(t, f.d, "usr_ada", "ada"); outcome != ForgeOAuthConnected {
				t.Fatalf("outcome %s", outcome)
			}
			credential := forgeCredential(t, f.d, "usr_ada", trackerName)
			if credential == nil || credential.Kind != CredentialKindOAuth || credential.Account != "ada" || credential.Sealed || !credential.Unlocked || credential.Disconnected || len(credential.GrantedSites) != 0 {
				t.Fatalf("stored: %+v", credential)
			}
			token, err := f.writeToken(t, f.d, "usr_ada")
			if err != nil || token == "" {
				t.Fatalf("a write: %q %v", token, err)
			}
			if got := f.account(t, token); got != "ada" {
				t.Errorf("the write must be attributed to ada, got %q", got)
			}
			// Somebody else, connected as nobody, is refused rather than
			// written under the server's name.
			if _, err := f.writeToken(t, f.d, "usr_grace"); trackerapi.MissingCredentialTracker(err) != trackerName {
				t.Errorf("a person with no credential: %v", err)
			}
		})
	}
}

// An expired GitLab access token is refreshed with no action from its owner,
// and the rotated refresh token is kept for the next one. A GitHub token,
// which never expires, is never refreshed.
func TestAnExpiredGitLabGrantIsRefreshedForQueuedWrites(t *testing.T) {
	f := newForgeGrantFixture(t, "gitlab")
	f.connect(t, f.d, "usr_ada", "ada")
	for round := 1; round <= 2; round++ {
		before, _ := f.d.readGrantRow("gitlab", "usr_ada")
		f.expire(t, f.d, "usr_ada")
		token, err := f.writeToken(t, f.d, "usr_ada")
		if err != nil || f.account(t, token) != "ada" {
			t.Fatalf("round %d: the write after the expiry: %q %v", round, token, err)
		}
		if f.fake.Refreshes() != round {
			t.Errorf("round %d: refreshes %d", round, f.fake.Refreshes())
		}
		after, _ := f.d.readGrantRow("gitlab", "usr_ada")
		// One claim and one write.
		if after.version != before.version+2 || after.disconnected || !after.claimedAt.IsZero() {
			t.Errorf("round %d: version %d → %d, row %+v", round, before.version, after.version, after)
		}
	}

	g := newForgeGrantFixture(t, "github")
	g.connect(t, g.d, "usr_ada", "ada")
	for range 2 {
		if _, err := g.writeToken(t, g.d, "usr_ada"); err != nil {
			t.Fatal(err)
		}
	}
	if g.fake.Refreshes() != 0 {
		t.Errorf("a GitHub token that never expires was refreshed %d times", g.fake.Refreshes())
	}
}

// Two instances refreshing one GitLab grant at once, against a token endpoint
// that kills a refresh token on use, both succeed and the person stays
// connected.
func TestTwoInstancesRefreshingOneGitLabGrantStayConnected(t *testing.T) {
	f := newForgeGrantFixture(t, "gitlab")
	f.connect(t, f.d, "usr_ada", "ada")
	other, err := NewDB(f.d.cfg.Path)
	if err != nil {
		t.Fatalf("a second instance on the same file: %v", err)
	}
	t.Cleanup(func() { other.Close() })
	other.SetForgeEndpoints("gitlab", f.fake.Endpoints(), nil)

	for round := 0; round < 3; round++ {
		f.expire(t, f.d, "usr_ada")
		release := f.fake.GateRefreshes()
		var wg sync.WaitGroup
		errs := make([]error, 2)
		tokens := make([]string, 2)
		for i, d := range []*DB{f.d, other} {
			wg.Add(1)
			go func(i int, d *DB) {
				defer wg.Done()
				credential, err := d.ResolvePersonalCredential("usr_ada", "gitlab", "")
				errs[i], tokens[i] = err, credential.Token
			}(i, d)
		}
		time.Sleep(300 * time.Millisecond)
		release()
		wg.Wait()
		f.fake.GateRefreshes()() // open for good
		for i, err := range errs {
			if err != nil || tokens[i] == "" {
				t.Fatalf("round %d, instance %d: %q %v", round, i, tokens[i], err)
			}
		}
		if tokens[0] != tokens[1] {
			t.Errorf("round %d: both must use the one refresh kept", round)
		}
		if row, _ := f.d.readGrantRow("gitlab", "usr_ada"); row.disconnected {
			t.Fatalf("round %d: a concurrent refresh disconnected a live grant", round)
		}
		if f.fake.Refreshes() != round+1 {
			t.Errorf("round %d: %d refreshes, want one per round", round, f.fake.Refreshes())
		}
	}
}

// A revoked GitLab grant is marked disconnected at its next refresh, the
// write fails with the missing-credential error, and the forge is not asked
// again.
func TestARevokedGitLabGrantDisconnectsAndRefusesTheWrite(t *testing.T) {
	f := newForgeGrantFixture(t, "gitlab")
	f.connect(t, f.d, "usr_ada", "ada")
	f.fake.Revoke("ada")
	f.expire(t, f.d, "usr_ada")

	_, err := f.writeToken(t, f.d, "usr_ada")
	var missing *trackerapi.MissingPersonalCredentialError
	if !errors.As(err, &missing) || missing.Reason != trackerapi.ReasonDisconnected || missing.Tracker != "gitlab" {
		t.Fatalf("a revoked grant: %v", err)
	}
	if c := forgeCredential(t, f.d, "usr_ada", "gitlab"); c == nil || !c.Disconnected {
		t.Fatalf("the profile must show the grant disconnected: %+v", c)
	}
	refreshes := f.fake.Refreshes()
	if _, err := f.d.ResolvePersonalCredential("usr_ada", "gitlab", ""); !errors.As(err, &missing) {
		t.Fatalf("a disconnected grant: %v", err)
	}
	if f.fake.Refreshes() != refreshes {
		t.Error("a disconnected grant must not be refreshed again")
	}
	// Reconnecting repairs it.
	f.connect(t, f.d, "usr_ada", "ada2")
	if token, err := f.writeToken(t, f.d, "usr_ada"); err != nil || f.account(t, token) != "ada2" {
		t.Fatalf("after reconnecting: %v", err)
	}
}

// GitHub tokens never expire, so a 401 is the only sign the person revoked the
// app: the client's hook marks the grant disconnected. A hook resolved before
// the person reconnected never disconnects the new grant.
func TestAGitHub401DisconnectsTheGrant(t *testing.T) {
	f := newForgeGrantFixture(t, "github")
	f.connect(t, f.d, "usr_ada", "ada")
	stale, err := f.d.ResolvePersonalCredential("usr_ada", "github", "")
	if err != nil || !stale.OAuth || stale.Token == "" || stale.OnUnauthorized == nil {
		t.Fatalf("resolved: %+v %v", stale, err)
	}
	stale.OnUnauthorized()
	if c := forgeCredential(t, f.d, "usr_ada", "github"); c == nil || !c.Disconnected {
		t.Fatalf("a 401 must disconnect the grant: %+v", c)
	}
	var missing *trackerapi.MissingPersonalCredentialError
	if _, err := f.writeToken(t, f.d, "usr_ada"); !errors.As(err, &missing) || missing.Reason != trackerapi.ReasonDisconnected {
		t.Fatalf("a disconnected grant must refuse the write: %v", err)
	}

	f.connect(t, f.d, "usr_ada", "ada")
	stale.OnUnauthorized()
	if c := forgeCredential(t, f.d, "usr_ada", "github"); c == nil || c.Disconnected {
		t.Fatalf("a stale 401 disconnected the new grant: %+v", c)
	}
	if _, err := f.writeToken(t, f.d, "usr_ada"); err != nil {
		t.Fatalf("the new grant: %v", err)
	}

	// A GitLab grant is disconnected by its refresh only.
	g := newForgeGrantFixture(t, "gitlab")
	g.connect(t, g.d, "usr_ada", "ada")
	if credential, err := g.d.ResolvePersonalCredential("usr_ada", "gitlab", ""); err != nil || !credential.OAuth || credential.OnUnauthorized != nil {
		t.Errorf("a GitLab grant: %+v %v", credential, err)
	}
}

// GitLab failing briefly fails the call and keeps the grant.
func TestAPassingRefreshFailureKeepsTheForgeGrant(t *testing.T) {
	f := newForgeGrantFixture(t, "gitlab")
	f.connect(t, f.d, "usr_ada", "ada")
	f.expire(t, f.d, "usr_ada")
	f.fake.FailRefreshes(http.StatusServiceUnavailable)
	_, err := f.d.ResolvePersonalCredential("usr_ada", "gitlab", "")
	if err == nil || trackerapi.MissingCredentialTracker(err) != "" {
		t.Fatalf("a 503 is a plain failure: %v", err)
	}
	if c := forgeCredential(t, f.d, "usr_ada", "gitlab"); c.Disconnected {
		t.Fatal("a 503 must not disconnect")
	}
	f.fake.FailRefreshes(0)
	if _, err := f.d.ResolvePersonalCredential("usr_ada", "gitlab", ""); err != nil {
		t.Fatalf("the next call tries again: %v", err)
	}
}

// Connecting replaces a pasted token, sealed and locked included, without its
// passphrase; a pasted token replaces a grant and revokes it at the forge.
func TestConnectingReplacesATokenAndATokenReplacesAForgeGrant(t *testing.T) {
	for _, trackerName := range forgeTrackers {
		t.Run(trackerName, func(t *testing.T) {
			f := newForgeGrantFixture(t, trackerName)
			if err := f.d.SetUserTrackerCredential("usr_ada", trackerName, "", "", "pasted", "ma phrase"); err != nil {
				t.Fatal(err)
			}
			if err := f.d.LockUserTrackerCredential("usr_ada", trackerName); err != nil {
				t.Fatal(err)
			}
			if outcome := f.connect(t, f.d, "usr_ada", "ada"); outcome != ForgeOAuthConnected {
				t.Fatalf("outcome %s", outcome)
			}
			c := forgeCredential(t, f.d, "usr_ada", trackerName)
			if c.Kind != CredentialKindOAuth || c.Sealed {
				t.Fatalf("the grant must replace the sealed token: %+v", c)
			}
			var unlocks int
			_ = f.d.conn.QueryRow(`SELECT COUNT(*) FROM user_credential_unlocks WHERE user_id = 'usr_ada'`).Scan(&unlocks)
			if unlocks != 0 {
				t.Error("the unlock of the replaced token must go")
			}
			// Saving the form without a token cannot keep a grant's token.
			if err := f.d.SetUserTrackerCredential("usr_ada", trackerName, "", "", "", ""); err == nil {
				t.Error("a grant holds no token to keep")
			}
			if len(f.fake.Revocations()) != 0 {
				t.Fatal("a refused save must not revoke the grant")
			}
			if err := f.d.SetUserTrackerCredential("usr_ada", trackerName, "", "", "pasted-2", ""); err != nil {
				t.Fatal(err)
			}
			if c := forgeCredential(t, f.d, "usr_ada", trackerName); c.Kind != CredentialKindAPIToken || c.Disconnected {
				t.Fatalf("the token must replace the grant: %+v", c)
			}
			if got := f.fake.Revocations(); !slices.Equal(got, []string{"ada"}) {
				t.Errorf("the replaced grant must be revoked at the forge: %v", got)
			}
			if token, err := f.writeToken(t, f.d, "usr_ada"); err != nil || token != "pasted-2" {
				t.Errorf("the pasted token serves the writes: %q %v", token, err)
			}
		})
	}
}

// A callback stores nothing unless its state is known, unused, unexpired, and
// was started by the same session and person for the same tracker.
func TestAForgedOrStaleForgeCallbackStoresNothing(t *testing.T) {
	f := newForgeGrantFixture(t, "gitlab")
	ctx := context.Background()
	start := func(trackerName string) string {
		state, err := f.d.StartOAuthFlow(trackerName, "usr_ada", "session-usr_ada")
		if err != nil {
			t.Fatal(err)
		}
		return state
	}
	try := func(user, session, state string) string {
		outcome, _ := f.d.CompleteForgeOAuth(ctx, "gitlab", user, session, state, f.fake.Consent("ada"), false)
		return outcome
	}

	if got := try("usr_ada", "session-usr_ada", ""); got != ForgeOAuthInvalid {
		t.Errorf("no state: %s", got)
	}
	if got := try("usr_ada", "session-other", start("gitlab")); got != ForgeOAuthInvalid {
		t.Errorf("another session: %s", got)
	}
	if got := try("usr_grace", "session-usr_ada", start("gitlab")); got != ForgeOAuthInvalid {
		t.Errorf("another person: %s", got)
	}
	if got := try("usr_ada", "session-usr_ada", start("jira")); got != ForgeOAuthInvalid {
		t.Errorf("a Jira state on the GitLab callback: %s", got)
	}
	if got := try("usr_ada", "session-usr_ada", start("github")); got != ForgeOAuthInvalid {
		t.Errorf("a GitHub state on the GitLab callback: %s", got)
	}
	expired := start("gitlab")
	if _, err := f.d.conn.Exec(`UPDATE jira_oauth_flows SET expires_at = ? WHERE state_hash = ?`, time.Now().Add(-time.Minute).UTC(), hashSecret(expired)); err != nil {
		t.Fatal(err)
	}
	if got := try("usr_ada", "session-usr_ada", expired); got != ForgeOAuthInvalid {
		t.Errorf("an expired state: %s", got)
	}
	if outcome, _ := f.d.CompleteForgeOAuth(ctx, "gitlab", "usr_ada", "session-usr_ada", start("gitlab"), "a-code-nobody-issued", false); outcome != ForgeOAuthInvalid {
		t.Errorf("an unknown code: %s", outcome)
	}
	if c := forgeCredential(t, f.d, "usr_ada", "gitlab"); c != nil {
		t.Fatalf("nothing may be stored: %+v", c)
	}

	// A declined consent stores nothing.
	if outcome, _ := f.d.CompleteForgeOAuth(ctx, "gitlab", "usr_ada", "session-usr_ada", start("gitlab"), "", true); outcome != ForgeOAuthCancelled {
		t.Errorf("declined: %s", outcome)
	}
	if c := forgeCredential(t, f.d, "usr_ada", "gitlab"); c != nil {
		t.Fatalf("a declined consent stored %+v", c)
	}

	// One state, two instances: exactly one consumes it.
	other, err := NewDB(f.d.cfg.Path)
	if err != nil {
		t.Fatalf("a second instance on the same file: %v", err)
	}
	t.Cleanup(func() { other.Close() })
	state := start("gitlab")
	first := other.ConsumeOAuthFlow("gitlab", state, "usr_ada", "session-usr_ada")
	second := f.d.ConsumeOAuthFlow("gitlab", state, "usr_ada", "session-usr_ada")
	if first != nil || !errors.Is(second, ErrJiraOAuthFlow) {
		t.Fatalf("first %v, replay %v", first, second)
	}
}

// Each tracker's app follows ADR 0028 on its own row: a saved app wins over
// the environment, and saving or clearing one leaves the others alone.
func TestTheForgeOAuthAppConfiguration(t *testing.T) {
	d := testDB(t)
	for _, trackerName := range []string{"jira", "github", "gitlab"} {
		prefix := oauthAppEnvPrefix[trackerName]
		t.Setenv(prefix+"CLIENT_ID", "")
		t.Setenv(prefix+"CLIENT_SECRET", "")
		t.Setenv(prefix+"REDIRECT_URL", "")
	}
	for _, trackerName := range forgeTrackers {
		if d.OAuthAppConfigured(trackerName) {
			t.Fatalf("%s: nothing configured", trackerName)
		}
		setForgeOAuthEnvironment(t, trackerName)
		if app, source, err := d.OAuthApp(trackerName); err != nil || source != ServerCredentialEnvironment || app.ClientID != forgeoauthtest.ClientID || !app.Configured() {
			t.Fatalf("%s: environment: %+v %s %v", trackerName, app, source, err)
		}
	}

	if err := d.SaveOAuthApp("gitlab", "page-client", "page-secret", "https://sectile.example.com/auth/gitlab/callback", "admin"); err != nil {
		t.Fatal(err)
	}
	if app, source, err := d.OAuthApp("gitlab"); err != nil || source != ServerCredentialStored || app.ClientID != "page-client" || app.ClientSecret != "page-secret" {
		t.Fatalf("the saved GitLab app must win: %+v %s %v", app, source, err)
	}
	if err := d.SaveOAuthApp("gitlab", "page-client-2", "", "https://sectile.example.com/auth/gitlab/callback", "admin"); err != nil {
		t.Fatalf("an empty secret keeps the saved one: %v", err)
	}
	if state, err := d.OAuthAppState("gitlab"); err != nil || !state.Configured || !state.SecretSet || state.ClientID != "page-client-2" || state.Source != ServerCredentialStored {
		t.Fatalf("state: %+v %v", state, err)
	}
	if _, source, _ := d.OAuthApp("github"); source != ServerCredentialEnvironment {
		t.Errorf("the GitHub app must stay the environment's: %s", source)
	}
	if _, source, _ := d.JiraOAuthApp(); source != ServerCredentialNone {
		t.Errorf("the Jira app must stay unconfigured: %s", source)
	}
	if err := d.SaveJiraOAuthApp("jira-client", "jira-secret", "https://sectile.example.com/auth/jira/callback", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := d.ClearOAuthApp("gitlab"); err != nil {
		t.Fatal(err)
	}
	if _, source, _ := d.OAuthApp("gitlab"); source != ServerCredentialEnvironment {
		t.Errorf("cleared, the environment applies: %s", source)
	}
	if app, source, err := d.JiraOAuthApp(); err != nil || source != ServerCredentialStored || app.ClientSecret != "jira-secret" {
		t.Errorf("clearing GitLab must leave Jira's app: %+v %s %v", app, source, err)
	}

	if _, _, err := d.OAuthApp("trello"); err == nil {
		t.Error("a tracker with no OAuth app must be refused")
	}
	if err := d.SaveOAuthApp("trello", "c", "s", "https://sectile.example.com/auth/trello/callback", "admin"); err == nil {
		t.Error("saving an app for a tracker with none must be refused")
	}
}

// A grant serves the public instance only: a self-hosted GitLab or a GitHub
// Enterprise tracker refuses it, naming the site, and nothing falls back to
// the server credential.
func TestASelfHostedGitLabTrackerRefusesTheGrant(t *testing.T) {
	cases := []struct {
		tracker, site string
	}{
		{"gitlab", "https://gitlab.acme.io/api/v4"},
		{"github", "https://github.acme.io/api/v3"},
	}
	for _, c := range cases {
		t.Run(c.tracker, func(t *testing.T) {
			f := newForgeGrantFixture(t, c.tracker)
			f.connect(t, f.d, "usr_ada", "ada")
			var missing *trackerapi.MissingPersonalCredentialError
			if _, err := f.d.ResolvePersonalCredential("usr_ada", c.tracker, c.site); !errors.As(err, &missing) || missing.Reason != trackerapi.ReasonSiteNotGranted || missing.Site != c.site {
				t.Fatalf("a self-hosted site: %v", err)
			}
			if !strings.Contains(missing.Error(), "Profile → Tracker credentials") {
				t.Errorf("the error must say where to add a token: %s", missing.Error())
			}

			request := models.CreateProjectRequest{Name: "Self-hosted", IssueTracker: c.tracker, GithubRepo: "acme/app", GithubApiUrl: c.site}
			if c.tracker == "gitlab" {
				request = models.CreateProjectRequest{Name: "Self-hosted", IssueTracker: c.tracker, GitlabProject: "acme/app", GitlabUrl: c.site}
			}
			project, err := f.d.CreateProject(request)
			if err != nil {
				t.Fatal(err)
			}
			ctx := tracker.WithActingUser(context.Background(), "usr_ada")
			if _, err := f.d.trackerForWrite(ctx, c.tracker, project.ID); !errors.As(err, &missing) || missing.Reason != trackerapi.ReasonSiteNotGranted {
				t.Fatalf("a write on the self-hosted project: %v", err)
			}
			// The public instance still works.
			if token, err := f.writeToken(t, f.d, "usr_ada"); err != nil || f.account(t, token) != "ada" {
				t.Fatalf("the public instance: %v", err)
			}
		})
	}
	for _, c := range []struct {
		tracker, site string
		granted       bool
	}{
		{"github", "", true},
		{"github", "https://api.github.com", true},
		{"github", "https://API.GitHub.com/", true},
		{"github", "https://github.acme.io/api/v3", false},
		{"gitlab", "", true},
		{"gitlab", "https://gitlab.com/api/v4", true},
		{"gitlab", "https://gitlab.acme.io/api/v4", false},
		{"jira", "https://acme.atlassian.net", false},
	} {
		if got := ForgeSiteGranted(c.tracker, c.site); got != c.granted {
			t.Errorf("ForgeSiteGranted(%s, %q) = %v", c.tracker, c.site, got)
		}
	}
}

// A refresh claimed on a row the person then deleted and created again never
// writes over the new row, even once the new row is back at the claimed
// version: the version restarts with the row.
func TestAStaleForgeRefreshNeverOverwritesARecreatedRow(t *testing.T) {
	f := newForgeGrantFixture(t, "gitlab")
	f.connect(t, f.d, "usr_ada", "ada")
	f.expire(t, f.d, "usr_ada")
	row, _ := f.d.readGrantRow("gitlab", "usr_ada")
	stale, err := f.d.openForgeGrant("gitlab", "usr_ada", row.record)
	if err != nil {
		t.Fatal(err)
	}
	if claimed, err := f.d.claimGrantRefresh("gitlab", "usr_ada", row.version); err != nil || !claimed {
		t.Fatalf("claim: %v %v", claimed, err)
	}

	if err := f.d.ClearUserTrackerCredential("usr_ada", "gitlab"); err != nil {
		t.Fatal(err)
	}
	f.connect(t, f.d, "usr_ada", "ada")
	if _, err := f.d.conn.Exec(`UPDATE user_tracker_credentials SET version = ? WHERE user_id = 'usr_ada' AND tracker = 'gitlab'`, row.version+1); err != nil {
		t.Fatal(err)
	}
	fresh, _ := f.d.readGrantRow("gitlab", "usr_ada")

	if _, _, err := f.d.refreshForgeGrant(context.Background(), "gitlab", "usr_ada", row.version+1, stale); err == nil {
		t.Fatal("the stale refresh reported a write")
	}
	after, _ := f.d.readGrantRow("gitlab", "usr_ada")
	if string(after.record) != string(fresh.record) || after.version != fresh.version || after.disconnected {
		t.Fatalf("the recreated row was touched: %+v", after)
	}
}

// Disconnecting deletes the row and revokes the grant at the forge; a forge
// that cannot be reached still leaves the row deleted.
func TestDisconnectRevokesBestEffort(t *testing.T) {
	for _, trackerName := range forgeTrackers {
		t.Run(trackerName, func(t *testing.T) {
			f := newForgeGrantFixture(t, trackerName)
			f.connect(t, f.d, "usr_ada", "ada")
			if err := f.d.ClearUserTrackerCredential("usr_ada", trackerName); err != nil {
				t.Fatal(err)
			}
			if c := forgeCredential(t, f.d, "usr_ada", trackerName); c != nil {
				t.Fatalf("deleting must forget the grant: %+v", c)
			}
			if got := f.fake.Revocations(); !slices.Equal(got, []string{"ada"}) {
				t.Fatalf("the grant must be revoked at the forge: %v", got)
			}

			f.connect(t, f.d, "usr_ada", "ada")
			unreachable := f.fake.Endpoints()
			unreachable.RevokeURL = "http://127.0.0.1:1/revoke"
			f.d.SetForgeEndpoints(trackerName, unreachable, nil)
			if err := f.d.ClearUserTrackerCredential("usr_ada", trackerName); err != nil {
				t.Fatalf("a failed revocation must not fail the disconnect: %v", err)
			}
			if c := forgeCredential(t, f.d, "usr_ada", trackerName); c != nil {
				t.Fatalf("the row must be deleted all the same: %+v", c)
			}
			if got := f.fake.Revocations(); len(got) != 1 {
				t.Errorf("nothing more reached the forge: %v", got)
			}
		})
	}
}

// The forge grant on PostgreSQL: connecting, two instances refreshing one
// GitLab grant at once, and a revocation, against the engine production runs
// on.
func TestPostgresForgeGrant(t *testing.T) {
	first, second := openPostgresPair(t)
	key, err := secrets.ServerKey(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []*DB{first, second} {
		d.serverKey, d.serverKeyErr = key, nil
	}
	fake := forgeoauthtest.New(t, forgeoauth.GitLab)
	setForgeOAuthEnvironment(t, "gitlab")
	f := newForgeGrantFixtureOn(t, first, fake, "gitlab")
	second.SetForgeEndpoints("gitlab", fake.Endpoints(), nil)

	if outcome := f.connect(t, first, "usr_ada", "ada"); outcome != ForgeOAuthConnected {
		t.Fatalf("outcome %s", outcome)
	}
	if c := forgeCredential(t, second, "usr_ada", "gitlab"); c == nil || c.Kind != CredentialKindOAuth {
		t.Fatalf("seen from the other instance: %+v", c)
	}

	for round := 0; round < 3; round++ {
		f.expire(t, first, "usr_ada")
		release := fake.GateRefreshes()
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, d := range []*DB{first, second} {
			wg.Add(1)
			go func(i int, d *DB) {
				defer wg.Done()
				_, errs[i] = d.ResolvePersonalCredential("usr_ada", "gitlab", "")
			}(i, d)
		}
		time.Sleep(300 * time.Millisecond)
		release()
		wg.Wait()
		fake.GateRefreshes()()
		for i, err := range errs {
			if err != nil {
				t.Fatalf("round %d, instance %d: %v", round, i, err)
			}
		}
		if fake.Refreshes() != round+1 {
			t.Errorf("round %d: %d refreshes", round, fake.Refreshes())
		}
	}

	fake.Revoke("ada")
	f.expire(t, first, "usr_ada")
	var missing *trackerapi.MissingPersonalCredentialError
	if _, err := second.ResolvePersonalCredential("usr_ada", "gitlab", ""); !errors.As(err, &missing) || missing.Reason != trackerapi.ReasonDisconnected {
		t.Fatalf("a revoked grant: %v", err)
	}
	if c := forgeCredential(t, first, "usr_ada", "gitlab"); !c.Disconnected {
		t.Fatalf("disconnected for every instance: %+v", c)
	}
}
