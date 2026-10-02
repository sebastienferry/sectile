package trackerapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// gateway is Atlassian's API gateway for one site: Bearer only, paths under
// /ex/jira/{cloudId}.
type gateway struct {
	mu       sync.Mutex
	server   *httptest.Server
	requests []string // "METHOD path authorization"
}

func newGateway(t *testing.T) *gateway {
	t.Helper()
	resetJiraFieldCache()
	resetJiraPriorityCache()
	resetJiraCreatePriorityCache()
	g := &gateway{}
	g.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		g.requests = append(g.requests, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		g.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer ada-access" || !strings.HasPrefix(r.URL.Path, "/ex/jira/c-acme/") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/ex/jira/c-acme")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case path == "/rest/api/3/field":
			fmt.Fprint(w, `[]`)
		case path == "/rest/api/3/priority":
			fmt.Fprint(w, `[{"id":"3","name":"Medium"}]`)
		case path == "/rest/api/3/myself":
			fmt.Fprint(w, `{"accountId":"acc-ada","displayName":"Ada Lovelace"}`)
		case path == "/rest/api/3/issue/PE-7" && r.Method == http.MethodGet:
			fmt.Fprint(w, `{"id":"10007","key":"PE-7","fields":{"summary":"Seven","status":{"name":"To Do","statusCategory":{"key":"new"}}}}`)
		case path == "/rest/api/3/issue/PE-7/comment" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(g.server.Close)
	return g
}

func (g *gateway) seen() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string{}, g.requests...)
}

// A person connected through a grant reaches Jira through Atlassian's gateway
// for the project's site, with their access token, on the same paths; the
// links people see keep the site.
func TestAGrantGoesThroughTheGatewayAndKeepsTheSite(t *testing.T) {
	g := newGateway(t)
	const site = "https://acme.atlassian.net"
	var asked []string
	c := &Client{HTTP: g.server.Client(), JiraURL: site, JiraEmail: "service@example.com", JiraToken: "service-token"}
	c.ResolveUser = func(userID, trackerName, forSite string) (PersonalCredential, error) {
		asked = append(asked, forSite)
		if userID == "ada" {
			return PersonalCredential{APIBase: g.server.URL + "/ex/jira/c-acme", Bearer: "ada-access"}, nil
		}
		return PersonalCredential{}, nil
	}
	adapter := NewJiraAdapter(c)
	project := jiraProject()
	ctx := tracker.WithActingUser(context.Background(), "ada")

	task, err := adapter.GetIssue(ctx, tracker.GetIssueRequest{Project: project, Key: "PE-7"})
	if err != nil {
		t.Fatal(err)
	}
	if task.ExternalURL == nil || !strings.HasPrefix(*task.ExternalURL, site+"/browse/PE-7") {
		t.Errorf("the link must keep the site: %v", task.ExternalURL)
	}
	if err := adapter.AddComment(ctx, tracker.AddCommentRequest{Project: project, Key: "PE-7", Body: "Hello"}); err != nil {
		t.Fatal(err)
	}
	for _, request := range g.seen() {
		if !strings.HasSuffix(request, " Bearer ada-access") || !strings.Contains(request, " /ex/jira/c-acme/rest/") {
			t.Errorf("not through the grant: %s", request)
		}
		if strings.Contains(request, "/priority/search") {
			t.Errorf("a grant has no scope for the priority search: %s", request)
		}
	}
	for _, s := range asked {
		if s != site {
			t.Errorf("the grant must be resolved for the project's site, got %q", s)
		}
	}

	account, err := c.CheckJiraBearer(context.Background(), site, g.server.URL+"/ex/jira/c-acme", "ada-access")
	if err != nil || account != "Ada Lovelace" {
		t.Errorf("checking a grant: %q %v", account, err)
	}
}

// A grant Atlassian stopped honouring within its access token's hour says
// to reconnect, not to check an API token the person never had.
func TestARefusedGrantSaysToReconnect(t *testing.T) {
	g := newGateway(t)
	c := &Client{HTTP: g.server.Client(), JiraURL: "https://acme.atlassian.net"}
	c.ResolveUser = func(string, string, string) (PersonalCredential, error) {
		return PersonalCredential{APIBase: g.server.URL + "/ex/jira/c-acme", Bearer: "revoked"}, nil
	}
	err := NewJiraAdapter(c).AddComment(tracker.WithActingUser(context.Background(), "ada"),
		tracker.AddCommentRequest{Project: jiraProject(), Key: "PE-7", Body: "Hello"})
	if err == nil || !strings.Contains(err.Error(), "Reconnectez Jira") || strings.Contains(err.Error(), "id.atlassian.com") {
		t.Fatalf("a refused grant: %v", err)
	}
}

// A grant that cannot serve the project's site refuses the write with the
// missing-credential error, under its code, and the server credential is
// never tried.
func TestAnUnusableGrantRefusesTheWrite(t *testing.T) {
	g := newGateway(t)
	for _, reason := range []string{ReasonSiteNotGranted, ReasonDisconnected} {
		c := &Client{HTTP: g.server.Client(), JiraURL: g.server.URL, JiraEmail: "service@example.com", JiraToken: "service-token"}
		c.ResolveUser = func(string, string, string) (PersonalCredential, error) {
			return PersonalCredential{}, &MissingPersonalCredentialError{Tracker: "jira", Reason: reason, Site: "https://other.atlassian.net"}
		}
		before := len(g.seen())
		err := NewJiraAdapter(c).AddComment(tracker.WithActingUser(context.Background(), "ada"),
			tracker.AddCommentRequest{Project: &models.Project{ID: "p1", IssueTracker: "jira", JiraProject: "PE"}, Key: "PE-7", Body: "Hello"})
		if MissingCredentialTracker(err) != "jira" {
			t.Fatalf("%s: not the missing-credential error: %v", reason, err)
		}
		if !strings.Contains(err.Error(), "Profile → Tracker credentials") {
			t.Errorf("%s: the error must name where to reconnect: %v", reason, err)
		}
		if len(g.seen()) != before {
			t.Errorf("%s: nothing may reach Jira", reason)
		}
	}
	site := (&MissingPersonalCredentialError{Tracker: "jira", Reason: ReasonSiteNotGranted, Site: "https://other.atlassian.net"}).Error()
	if !strings.Contains(site, "https://other.atlassian.net") || !strings.Contains(site, "consent screen") {
		t.Errorf("the site must be named: %s", site)
	}
	// Today's text is unchanged for somebody with no credential at all.
	if got := (&MissingPersonalCredentialError{Tracker: "jira"}).Error(); got != "no personal Jira token for this user: add one in Profile → Tracker credentials, or the work would be attributed to the server account" {
		t.Errorf("the missing-token text moved: %s", got)
	}
}

// Without bearer fields, nothing changes: Basic on the site itself.
func TestWithoutAGrantTheSiteIsCalledAsBefore(t *testing.T) {
	c := &Client{JiraURL: "https://acme.atlassian.net", JiraEmail: "a@example.com", JiraToken: "t"}
	if c.jiraEndpoint() != "https://acme.atlassian.net" || !strings.HasPrefix(c.jiraAuthorization(), "Basic ") {
		t.Fatalf("endpoint %s, auth %s", c.jiraEndpoint(), c.jiraAuthorization())
	}
	if err := (&Client{JiraURL: "https://acme.atlassian.net", JiraBearer: "b", JiraAPIBase: "https://api.atlassian.com/ex/jira/c"}).jiraConfigured(); err != nil {
		t.Errorf("a grant needs no e-mail: %v", err)
	}
	if !isUnauthorized(fmt.Errorf("wrap: %w", &HTTPError{Status: 401})) || isUnauthorized(&HTTPError{Status: 403}) {
		t.Error("only a 401 is unauthorised")
	}
}
