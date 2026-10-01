package db

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"tasks/internal/atlassian/atlassiantest"
	"tasks/internal/models"
	"tasks/internal/secrets"
	"tasks/internal/tracker"
	"tasks/internal/trackerapi"
)

const (
	acmeSite = "https://acme.atlassian.net"
	betaSite = "https://beta.atlassian.net"
)

// grantFixture is a deployment with a Jira OAuth app, a fake Atlassian
// knowing two sites, and a Jira project on the first.
type grantFixture struct {
	d       *DB
	fake    *atlassiantest.Fake
	project *models.Project
}

func setJiraOAuthEnvironment(t *testing.T) {
	t.Setenv(JiraOAuthClientIDVar, atlassiantest.ClientID)
	t.Setenv(JiraOAuthClientSecretVar, atlassiantest.ClientSecret)
	t.Setenv(JiraOAuthRedirectURLVar, "https://sectile.example.com/auth/jira/callback")
}

func newGrantFixture(t *testing.T) *grantFixture {
	t.Helper()
	fake := atlassiantest.New(t,
		atlassiantest.FakeSite{CloudID: "c-acme", URL: acmeSite},
		atlassiantest.FakeSite{CloudID: "c-beta", URL: betaSite + "/"})
	setJiraOAuthEnvironment(t)
	return newGrantFixtureOn(t, testDB(t), fake)
}

// newGrantFixtureOn is newGrantFixture on a store the caller opened.
func newGrantFixtureOn(t *testing.T, d *DB, fake *atlassiantest.Fake) *grantFixture {
	t.Helper()
	d.SetAtlassianEndpoints(fake.Endpoints(), nil)
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Jira", IssueTracker: "jira", JiraProject: "PE", TrackerUrl: acmeSite})
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{"usr_ada", "usr_grace"} {
		if err := d.EnsureUser(user); err != nil {
			t.Fatal(err)
		}
	}
	return &grantFixture{d: d, fake: fake, project: project}
}

// connect runs one consent of user, for account, over the given sites.
func (f *grantFixture) connect(t *testing.T, d *DB, user, account string, cloudIDs ...string) string {
	t.Helper()
	state, err := d.StartJiraOAuthFlow(user, "session-"+user)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := d.CompleteJiraOAuth(context.Background(), user, "session-"+user, state, f.fake.Consent(account, cloudIDs...), false)
	if outcome == JiraOAuthConnected && err != nil {
		t.Fatalf("connected with an error: %v", err)
	}
	return outcome
}

func (f *grantFixture) jira(t *testing.T) *trackerapi.JiraAdapter {
	t.Helper()
	ts, ok := f.d.trackerRegistry.Get("jira")
	if !ok {
		t.Fatal("no Jira adapter")
	}
	return ts.(*trackerapi.JiraAdapter)
}

func (f *grantFixture) as(user string) context.Context {
	return tracker.WithProject(tracker.WithActingUser(context.Background(), user), f.project.ID)
}

// expire makes the stored access token of user expire now.
func (f *grantFixture) expire(t *testing.T, d *DB, user string) {
	t.Helper()
	row, err := d.readJiraGrantRow(user)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := d.openJiraGrant(user, row.record)
	if err != nil {
		t.Fatal(err)
	}
	grant.ExpiresAt = time.Now().Add(-time.Second)
	record, err := d.sealJiraGrant(user, grant)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.conn.Exec(`UPDATE user_tracker_credentials SET record = ? WHERE user_id = ? AND tracker = 'jira'`, record, user); err != nil {
		t.Fatal(err)
	}
}

func jiraCredential(t *testing.T, d *DB, user string) *UserCredential {
	t.Helper()
	list, err := d.UserTrackerCredentials(user)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range list {
		if c.Tracker == "jira" {
			return &c
		}
	}
	return nil
}

// US1, US4.1, AC1: one consent stores a grant covering both sites under the
// account Atlassian confirmed, and the person's transition, comment and
// assignment are made by that account.
func TestAConnectedPersonWritesUnderTheirOwnAccount(t *testing.T) {
	f := newGrantFixture(t)
	if outcome := f.connect(t, f.d, "usr_ada", "Ada", "c-acme", "c-beta"); outcome != JiraOAuthConnected {
		t.Fatalf("outcome %s", outcome)
	}
	credential := jiraCredential(t, f.d, "usr_ada")
	if credential == nil || credential.Kind != CredentialKindOAuth || credential.Account != "Ada" || credential.Sealed || !credential.Unlocked || credential.Disconnected {
		t.Fatalf("stored: %+v", credential)
	}
	if len(credential.GrantedSites) != 2 || credential.GrantedSites[0] != acmeSite || credential.GrantedSites[1] != betaSite {
		t.Errorf("granted sites: %v", credential.GrantedSites)
	}

	jira, ctx := f.jira(t), f.as("usr_ada")
	if err := jira.Transition(ctx, "PE-7", "Done"); err != nil {
		t.Fatalf("transition: %v", err)
	}
	if err := jira.AddComment(ctx, tracker.AddCommentRequest{Project: f.project, Key: "PE-7", Body: "Hello"}); err != nil {
		t.Fatalf("comment: %v", err)
	}
	if err := jira.Assign(ctx, "PE-7", "acc-ada"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	writes := f.fake.Writes()
	if len(writes) != 3 {
		t.Fatalf("writes: %+v", writes)
	}
	for _, w := range writes {
		if w.Account != "Ada" || w.CloudID != "c-acme" {
			t.Errorf("write not attributed to Ada on acme: %+v", w)
		}
	}

	// US4.1: a project on the other granted site writes there.
	beta, err := f.d.CreateProject(models.CreateProjectRequest{Name: "Beta", IssueTracker: "jira", JiraProject: "BE", TrackerUrl: betaSite})
	if err != nil {
		t.Fatal(err)
	}
	if err := jira.AddComment(tracker.WithActingUser(context.Background(), "usr_ada"), tracker.AddCommentRequest{Project: beta, Key: "BE-1", Body: "Hi"}); err != nil {
		t.Fatal(err)
	}
	if last := f.fake.Writes()[3]; last.CloudID != "c-beta" || last.Account != "Ada" {
		t.Errorf("a beta write must go to beta: %+v", last)
	}
}

// AC3: an expired access token is refreshed with no action from its owner,
// for a queued write and for a managed run's stage report alike.
func TestAnExpiredGrantIsRefreshedForQueuedWrites(t *testing.T) {
	f := newGrantFixture(t)
	f.connect(t, f.d, "usr_ada", "Ada", "c-acme")
	task, err := f.d.CreateTaskAs(f.as("usr_ada"), models.CreateTaskRequest{ProjectID: f.project.ID, Title: "Seven"})
	if err != nil {
		t.Fatal(err)
	}
	if !f.d.jobs.drain(10 * time.Second) {
		t.Fatal("the queue did not settle")
	}
	before, _ := f.d.readJiraGrantRow("usr_ada")

	f.expire(t, f.d, "usr_ada")
	moved, err := f.d.EnqueueTrackerOp(f.as("usr_ada"), TrackerOp{
		Kind: TrackerOpTransition, ProjectID: f.project.ID, TaskID: task.ID, TaskKey: task.Key, TargetStatus: "Done",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !f.d.jobs.drain(10 * time.Second) {
		t.Fatal("the queue did not settle")
	}
	done, err := f.d.GetActivityByID(moved.ID)
	if err != nil || done.Status != string(models.ActivityStatusCompleted) {
		t.Fatalf("the queued transition: %+v %v", done, err)
	}
	if f.fake.Refreshes() != 1 {
		t.Errorf("refreshes: %d", f.fake.Refreshes())
	}
	after, _ := f.d.readJiraGrantRow("usr_ada")
	// One claim and one write.
	if after.version != before.version+2 || after.disconnected {
		t.Errorf("version %d → %d, disconnected %v", before.version, after.version, after.disconnected)
	}

	// A managed run reports a stage on its owner's behalf, hours later.
	f.expire(t, f.d, "usr_ada")
	writes := len(f.fake.Writes())
	_, stage, err := f.d.TransitionTaskStageBy("usr_ada", task.ID, "clarified", "Clarified while you were away", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !f.d.jobs.drain(10 * time.Second) {
		t.Fatal("the queue did not settle")
	}
	if done, err := f.d.GetActivityByID(stage.ID); err != nil || done.Status != string(models.ActivityStatusCompleted) {
		t.Fatalf("the stage report: %+v %v", done, err)
	}
	if f.fake.Refreshes() != 2 || len(f.fake.Writes()) <= writes {
		t.Errorf("refreshes %d, writes %+v", f.fake.Refreshes(), f.fake.Writes())
	}
	for _, w := range f.fake.Writes()[writes:] {
		if w.Account != "Ada" {
			t.Errorf("a stage write not attributed to Ada: %+v", w)
		}
	}
}

// AC4: two instances refreshing one grant at once, against a token endpoint
// that kills a refresh token on use, both succeed and the person stays
// connected.
func TestTwoInstancesRefreshingOneGrantStayConnected(t *testing.T) {
	f := newGrantFixture(t)
	f.connect(t, f.d, "usr_ada", "Ada", "c-acme")
	other, err := NewDB(f.d.cfg.Path)
	if err != nil {
		t.Fatalf("a second instance on the same file: %v", err)
	}
	t.Cleanup(func() { other.Close() })
	other.SetAtlassianEndpoints(f.fake.Endpoints(), nil)

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
				credential, err := d.ResolvePersonalCredential("usr_ada", "jira", acmeSite)
				errs[i], tokens[i] = err, credential.Bearer
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
		if row, _ := f.d.readJiraGrantRow("usr_ada"); row.disconnected {
			t.Fatalf("round %d: a concurrent refresh disconnected a live grant", round)
		}
		if f.fake.Refreshes() != round+1 {
			t.Errorf("round %d: %d refreshes, want one per round", round, f.fake.Refreshes())
		}
	}
}

// AC5, US3: a revoked grant is marked disconnected at its next refresh, the
// write fails with the missing-credential error, and nothing is written.
func TestARevokedGrantDisconnectsAndRefusesTheWrite(t *testing.T) {
	f := newGrantFixture(t)
	f.connect(t, f.d, "usr_ada", "Ada", "c-acme")
	f.fake.Revoke("Ada")
	f.expire(t, f.d, "usr_ada")

	err := f.jira(t).AddComment(f.as("usr_ada"), tracker.AddCommentRequest{Project: f.project, Key: "PE-7", Body: "Hello"})
	var missing *trackerapi.MissingPersonalCredentialError
	if !errors.As(err, &missing) || missing.Reason != trackerapi.ReasonDisconnected || missing.Tracker != "jira" {
		t.Fatalf("a revoked grant: %v", err)
	}
	if len(f.fake.Writes()) != 0 {
		t.Fatalf("nothing may be written: %+v", f.fake.Writes())
	}
	if c := jiraCredential(t, f.d, "usr_ada"); c == nil || !c.Disconnected {
		t.Fatalf("the profile must show the grant disconnected: %+v", c)
	}
	// And later calls fail the same without asking Atlassian again.
	refreshes := f.fake.Refreshes()
	if _, err := f.d.ResolvePersonalCredential("usr_ada", "jira", acmeSite); !errors.As(err, &missing) {
		t.Fatalf("a disconnected grant: %v", err)
	}
	if f.fake.Refreshes() != refreshes {
		t.Error("a disconnected grant must not be refreshed again")
	}
	// A read falls back to the project client, as reads do.
	if client := f.d.trackerAs("usr_ada", "jira", f.project.ID); client == nil || client.JiraBearer != "" {
		t.Errorf("a read must fall back to the project client: %+v", client)
	}

	// US3.3: reconnecting repairs it.
	f.connect(t, f.d, "usr_ada", "Ada2", "c-acme")
	if err := f.jira(t).AddComment(f.as("usr_ada"), tracker.AddCommentRequest{Project: f.project, Key: "PE-7", Body: "Back"}); err != nil {
		t.Fatalf("after reconnecting: %v", err)
	}
}

// US2.4: Atlassian failing briefly fails the call and keeps the grant.
func TestAPassingRefreshFailureKeepsTheGrant(t *testing.T) {
	f := newGrantFixture(t)
	f.connect(t, f.d, "usr_ada", "Ada", "c-acme")
	f.expire(t, f.d, "usr_ada")
	f.fake.FailRefreshes(http.StatusServiceUnavailable)
	_, err := f.d.ResolvePersonalCredential("usr_ada", "jira", acmeSite)
	if err == nil || trackerapi.MissingCredentialTracker(err) != "" {
		t.Fatalf("a 503 is a plain failure: %v", err)
	}
	if c := jiraCredential(t, f.d, "usr_ada"); c.Disconnected {
		t.Fatal("a 503 must not disconnect")
	}
	f.fake.FailRefreshes(0)
	if _, err := f.d.ResolvePersonalCredential("usr_ada", "jira", acmeSite); err != nil {
		t.Fatalf("the next call tries again: %v", err)
	}
}

// US4.2, AC7: a project on a site the grant lacks is refused, naming it.
func TestAProjectOnASiteTheGrantLacksIsRefused(t *testing.T) {
	f := newGrantFixture(t)
	f.connect(t, f.d, "usr_ada", "Ada", "c-acme")
	other, err := f.d.CreateProject(models.CreateProjectRequest{Name: "Gamma", IssueTracker: "jira", JiraProject: "GA", TrackerUrl: "https://gamma.atlassian.net"})
	if err != nil {
		t.Fatal(err)
	}
	err = f.jira(t).AddComment(tracker.WithActingUser(context.Background(), "usr_ada"), tracker.AddCommentRequest{Project: other, Key: "GA-1", Body: "x"})
	var missing *trackerapi.MissingPersonalCredentialError
	if !errors.As(err, &missing) || missing.Reason != trackerapi.ReasonSiteNotGranted || missing.Site != "https://gamma.atlassian.net" {
		t.Fatalf("a site the grant lacks: %v", err)
	}
	if len(f.fake.Writes()) != 0 {
		t.Fatal("nothing may be written")
	}
}

// US4.3, AC7: a grant covering no configured site stores nothing, and an
// existing API token stays.
func TestAGrantCoveringNoConfiguredSiteStoresNothing(t *testing.T) {
	f := newGrantFixture(t)
	if _, err := f.d.conn.Exec(`UPDATE projects SET tracker_url = 'https://gamma.atlassian.net' WHERE id = ?`, f.project.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.d.SetUserTrackerCredential("usr_ada", "jira", "https://gamma.atlassian.net", "ada@example.com", "ATATT", ""); err != nil {
		t.Fatal(err)
	}
	if outcome := f.connect(t, f.d, "usr_ada", "Ada", "c-acme"); outcome != JiraOAuthNoSite {
		t.Fatalf("outcome %s", outcome)
	}
	if _, _, token, err := f.d.userTrackerCredential("usr_ada", "jira"); err != nil || token != "ATATT" {
		t.Fatalf("the API token must stay: %q %v", token, err)
	}
	sites, err := f.d.ConfiguredJiraSites()
	if err != nil || len(sites) == 0 || sites[len(sites)-1] != "https://gamma.atlassian.net" {
		t.Errorf("configured sites: %v %v", sites, err)
	}
}

// US5, AC2: connecting replaces an API token, sealed and locked included,
// without its passphrase; a token replaces a grant; deleting forgets it.
func TestConnectingReplacesATokenAndATokenReplacesAGrant(t *testing.T) {
	f := newGrantFixture(t)
	if err := f.d.SetUserTrackerCredential("usr_ada", "jira", acmeSite, "ada@example.com", "ATATT", "ma phrase"); err != nil {
		t.Fatal(err)
	}
	if err := f.d.LockUserTrackerCredential("usr_ada", "jira"); err != nil {
		t.Fatal(err)
	}
	if outcome := f.connect(t, f.d, "usr_ada", "Ada", "c-acme"); outcome != JiraOAuthConnected {
		t.Fatalf("outcome %s", outcome)
	}
	c := jiraCredential(t, f.d, "usr_ada")
	if c.Kind != CredentialKindOAuth || c.Sealed || c.Email != "" || c.SiteURL != "" {
		t.Fatalf("the grant must replace the sealed token: %+v", c)
	}
	var unlocks int
	_ = f.d.conn.QueryRow(`SELECT COUNT(*) FROM user_credential_unlocks WHERE user_id = 'usr_ada'`).Scan(&unlocks)
	if unlocks != 0 {
		t.Error("the unlock of the replaced token must go")
	}
	if err := f.d.UnlockUserTrackerCredential("usr_ada", "jira", "ma phrase"); !errors.Is(err, ErrNotSealed) {
		t.Errorf("a grant has nothing to unlock: %v", err)
	}
	// Saving the form without a token cannot keep a grant's token.
	if err := f.d.SetUserTrackerCredential("usr_ada", "jira", acmeSite, "ada@example.com", "", ""); err == nil {
		t.Error("a grant holds no token to keep")
	}
	// US5.3
	if err := f.d.SetUserTrackerCredential("usr_ada", "jira", acmeSite, "ada@example.com", "ATATT-2", ""); err != nil {
		t.Fatal(err)
	}
	if c := jiraCredential(t, f.d, "usr_ada"); c.Kind != CredentialKindAPIToken || c.Disconnected {
		t.Fatalf("the token must replace the grant: %+v", c)
	}
	// US5.2
	f.connect(t, f.d, "usr_ada", "Ada", "c-acme")
	if err := f.d.ClearUserTrackerCredential("usr_ada", "jira"); err != nil {
		t.Fatal(err)
	}
	if c := jiraCredential(t, f.d, "usr_ada"); c != nil {
		t.Fatalf("deleting must forget the grant: %+v", c)
	}
}

// AC6, US7: a callback stores nothing unless its state is known, unused,
// unexpired, and was started by the same session and person; on any
// instance, exactly once.
func TestAForgedOrStaleCallbackStoresNothing(t *testing.T) {
	f := newGrantFixture(t)
	ctx := context.Background()
	start := func() string {
		state, err := f.d.StartJiraOAuthFlow("usr_ada", "session-usr_ada")
		if err != nil {
			t.Fatal(err)
		}
		return state
	}
	try := func(user, session, state string) string {
		outcome, _ := f.d.CompleteJiraOAuth(ctx, user, session, state, f.fake.Consent("Ada", "c-acme"), false)
		return outcome
	}

	if got := try("usr_ada", "session-usr_ada", ""); got != JiraOAuthInvalid {
		t.Errorf("no state: %s", got)
	}
	if got := try("usr_ada", "session-other", start()); got != JiraOAuthInvalid {
		t.Errorf("another session: %s", got)
	}
	if got := try("usr_grace", "session-usr_ada", start()); got != JiraOAuthInvalid {
		t.Errorf("another person: %s", got)
	}
	expired := start()
	if _, err := f.d.conn.Exec(`UPDATE jira_oauth_flows SET expires_at = ? WHERE state_hash = ?`, time.Now().Add(-time.Minute).UTC(), hashSecret(expired)); err != nil {
		t.Fatal(err)
	}
	if got := try("usr_ada", "session-usr_ada", expired); got != JiraOAuthInvalid {
		t.Errorf("an expired state: %s", got)
	}
	if c := jiraCredential(t, f.d, "usr_ada"); c != nil {
		t.Fatalf("nothing may be stored: %+v", c)
	}

	// A declined consent stores nothing.
	if outcome, _ := f.d.CompleteJiraOAuth(ctx, "usr_ada", "session-usr_ada", start(), "", true); outcome != JiraOAuthCancelled {
		t.Errorf("declined: %s", outcome)
	}
	if c := jiraCredential(t, f.d, "usr_ada"); c != nil {
		t.Fatalf("a declined consent stored %+v", c)
	}

	// One state, two instances: exactly one consumes it.
	other, err := NewDB(f.d.cfg.Path)
	if err != nil {
		t.Fatalf("a second instance on the same file: %v", err)
	}
	t.Cleanup(func() { other.Close() })
	state := start()
	first := other.ConsumeJiraOAuthFlow(state, "usr_ada", "session-usr_ada")
	second := f.d.ConsumeJiraOAuthFlow(state, "usr_ada", "session-usr_ada")
	if first != nil || !errors.Is(second, ErrJiraOAuthFlow) {
		t.Fatalf("first %v, replay %v", first, second)
	}
}

// The OAuth app follows ADR 0028: a saved app wins as a whole, an empty secret
// keeps the saved one, and one the key cannot open is an error, never the
// environment's.
func TestTheOAuthAppConfiguration(t *testing.T) {
	d := testDB(t)
	t.Setenv(JiraOAuthClientIDVar, "")
	t.Setenv(JiraOAuthClientSecretVar, "")
	t.Setenv(JiraOAuthRedirectURLVar, "")
	if d.JiraOAuthConfigured() {
		t.Fatal("nothing configured")
	}
	setJiraOAuthEnvironment(t)
	if app, source, err := d.JiraOAuthApp(); err != nil || source != ServerCredentialEnvironment || app.ClientID != atlassiantest.ClientID {
		t.Fatalf("environment: %+v %s %v", app, source, err)
	}

	if err := d.SaveJiraOAuthApp("page-client", "", "https://sectile.example.com/auth/jira/callback", "admin"); !errors.Is(err, ErrOAuthAppSecretRequired) {
		t.Fatalf("a first save without a secret: %v", err)
	}
	if err := d.SaveJiraOAuthApp("page-client", "page-secret", "http://sectile.example.com/cb", "admin"); err == nil {
		t.Fatal("a plain-HTTP callback off localhost must be refused")
	}
	if err := d.SaveJiraOAuthApp("page-client", "page-secret", "https://sectile.example.com/auth/jira/callback", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveJiraOAuthApp("page-client-2", "", "http://localhost:8090/auth/jira/callback", "admin"); err != nil {
		t.Fatalf("an empty secret keeps the saved one: %v", err)
	}
	app, source, err := d.JiraOAuthApp()
	if err != nil || source != ServerCredentialStored || app.ClientID != "page-client-2" || app.ClientSecret != "page-secret" || app.RedirectURL != "http://localhost:8090/auth/jira/callback" {
		t.Fatalf("saved: %+v %s %v", app, source, err)
	}
	state, err := d.JiraOAuthAppState()
	if err != nil || !state.Configured || !state.SecretSet || state.ClientID != "page-client-2" {
		t.Fatalf("state: %+v %v", state, err)
	}

	// Another server key: the saved app is unreadable and nothing falls back.
	d.serverKey[0] ^= 0xff
	if _, _, err := d.JiraOAuthApp(); !errors.Is(err, ErrOAuthAppUnreadable) {
		t.Fatalf("an unreadable app: %v", err)
	}
	if d.JiraOAuthConfigured() {
		t.Error("an unreadable app is not configured")
	}
	if state, err := d.JiraOAuthAppState(); err != nil || !state.Unreadable || state.Configured {
		t.Errorf("unreadable state: %+v %v", state, err)
	}
	d.serverKey[0] ^= 0xff

	if err := d.ClearJiraOAuthApp(); err != nil {
		t.Fatal(err)
	}
	if _, source, _ := d.JiraOAuthApp(); source != ServerCredentialEnvironment {
		t.Errorf("cleared, the environment applies: %s", source)
	}
}

// AC10: an existing API token reads as one, and the resolver answers it as
// before.
func TestAnAPITokenResolvesAsBefore(t *testing.T) {
	d := testDB(t)
	if err := d.SetUserTrackerCredential("usr_ada", "jira", acmeSite, "ada@example.com", "ATATT", ""); err != nil {
		t.Fatal(err)
	}
	credential, err := d.ResolvePersonalCredential("usr_ada", "jira", acmeSite)
	if err != nil || credential.Token != "ATATT" || credential.Email != "ada@example.com" || credential.Bearer != "" {
		t.Fatalf("resolved: %+v %v", credential, err)
	}
	if c := jiraCredential(t, d, "usr_ada"); c.Kind != CredentialKindAPIToken {
		t.Errorf("kind: %+v", c)
	}
}

// The grant on PostgreSQL: connecting, two instances refreshing one grant at
// once, and a revocation, against the engine production runs on.
func TestPostgresJiraGrant(t *testing.T) {
	first, second := openPostgresPair(t)
	key, err := secrets.ServerKey(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []*DB{first, second} {
		d.serverKey, d.serverKeyErr = key, nil
	}
	fake := atlassiantest.New(t, atlassiantest.FakeSite{CloudID: "c-acme", URL: acmeSite})
	setJiraOAuthEnvironment(t)
	f := newGrantFixtureOn(t, first, fake)
	second.SetAtlassianEndpoints(fake.Endpoints(), nil)

	if outcome := f.connect(t, first, "usr_ada", "Ada", "c-acme"); outcome != JiraOAuthConnected {
		t.Fatalf("outcome %s", outcome)
	}
	if c := jiraCredential(t, second, "usr_ada"); c == nil || c.Kind != CredentialKindOAuth || len(c.GrantedSites) != 1 {
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
				_, errs[i] = d.ResolvePersonalCredential("usr_ada", "jira", acmeSite)
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

	if err := f.jira(t).AddComment(f.as("usr_ada"), tracker.AddCommentRequest{Project: f.project, Key: "PE-7", Body: "Hello"}); err != nil {
		t.Fatalf("a write: %v", err)
	}
	fake.Revoke("Ada")
	f.expire(t, first, "usr_ada")
	var missing *trackerapi.MissingPersonalCredentialError
	if _, err := second.ResolvePersonalCredential("usr_ada", "jira", acmeSite); !errors.As(err, &missing) || missing.Reason != trackerapi.ReasonDisconnected {
		t.Fatalf("a revoked grant: %v", err)
	}
	if c := jiraCredential(t, first, "usr_ada"); !c.Disconnected {
		t.Fatalf("disconnected for every instance: %+v", c)
	}

	// The app saved on one instance is the other's too.
	if err := first.SaveJiraOAuthApp("page-client", "page-secret", "https://sectile.example.com/auth/jira/callback", "admin"); err != nil {
		t.Fatal(err)
	}
	if app, source, err := second.JiraOAuthApp(); err != nil || source != ServerCredentialStored || app.ClientSecret != "page-secret" {
		t.Fatalf("the saved app: %+v %s %v", app, source, err)
	}
}
