package trackerapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"tasks/internal/models"
	"tasks/internal/tracker"
)

// forgeSite is a GitHub or GitLab instance that records every request and
// honours only the person's grant token.
type forgeSite struct {
	mu       sync.Mutex
	server   *httptest.Server
	requests []string // "METHOD path authorization"
}

func newForgeSite(t *testing.T) *forgeSite {
	t.Helper()
	s := &forgeSite{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r.Method+" "+r.URL.EscapedPath()+" "+r.Header.Get("Authorization"))
		s.mu.Unlock()
		if r.Header.Get("Authorization") != "Bearer ada-grant" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(s.server.Close)
	return s
}

func (s *forgeSite) seen() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.requests...)
}

// comment posts a comment as ada on the forge's issue 7, through the adapter
// of that forge.
func forgeComment(trackerName string, c *Client) error {
	ctx := tracker.WithActingUser(context.Background(), "ada")
	request := tracker.AddCommentRequest{Tracker: &models.Tracker{ID: "p1", Provider: trackerName, Scope: "acme/app"}, Key: "#7", Body: "Hello"}
	if trackerName == "github" {
		return NewGithubAdapter(c).AddComment(ctx, request)
	}
	return NewGitlabAdapter(c).AddComment(ctx, request)
}

// forgeClient is a client of the forge site whose server credential is not
// the person's, resolving ada to credential.
func forgeClient(s *forgeSite, trackerName string, credential PersonalCredential) *Client {
	c := &Client{HTTP: s.server.Client(), GithubURL: s.server.URL, GithubToken: "server-token", GitlabURL: s.server.URL + "/api/v4", GitlabProject: "acme/app", GitlabToken: "server-token"}
	c.ResolveUser = func(userID, _, _ string) (PersonalCredential, error) {
		if userID == "ada" {
			return credential, nil
		}
		return PersonalCredential{}, nil
	}
	return c
}

// A person connected through a forge grant writes with the grant's token, as
// a Bearer, on the forge's usual paths; the server's token never travels.
func TestAForgeGrantTokenTravelsAsBearer(t *testing.T) {
	for _, trackerName := range []string{"github", "gitlab"} {
		t.Run(trackerName, func(t *testing.T) {
			s := newForgeSite(t)
			c := forgeClient(s, trackerName, PersonalCredential{Token: "ada-grant", OAuth: true})
			if err := forgeComment(trackerName, c); err != nil {
				t.Fatal(err)
			}
			seen := s.seen()
			if len(seen) == 0 {
				t.Fatal("nothing reached the forge")
			}
			for _, request := range seen {
				if !strings.HasSuffix(request, " Bearer ada-grant") || !strings.Contains(request, "/issues/7/") {
					t.Errorf("not through the grant: %s", request)
				}
			}
		})
	}
}

// GitHub refusing a grant's token means the person revoked the app: the hook
// is called, so the store marks the grant disconnected, and the person is
// told to reconnect. GitLab only rewords it: its refresh disconnects.
func TestAGitHub401OnAGrantCallsOnUnauthorizedAndSaysReconnect(t *testing.T) {
	for _, trackerName := range []string{"github", "gitlab"} {
		t.Run(trackerName, func(t *testing.T) {
			s := newForgeSite(t)
			called := 0
			c := forgeClient(s, trackerName, PersonalCredential{Token: "revoked", OAuth: true, OnUnauthorized: func() { called++ }})
			err := forgeComment(trackerName, c)
			want := map[string]string{"github": "Reconnectez GitHub", "gitlab": "Reconnectez GitLab"}[trackerName]
			if err == nil || !strings.Contains(err.Error(), want) || !isUnauthorized(err) {
				t.Fatalf("a refused grant: %v", err)
			}
			if trackerName == "github" && called != 1 {
				t.Errorf("the hook must be called once, got %d", called)
			}
			if trackerName == "gitlab" && called != 0 {
				t.Errorf("a GitLab 401 must not disconnect, the hook was called %d times", called)
			}
		})
	}
}

// A pasted token refused by GitHub disconnects nothing and keeps its wording.
func TestAGitHub401OnAPastedTokenDoesNotDisconnect(t *testing.T) {
	s := newForgeSite(t)
	called := 0
	c := forgeClient(s, "github", PersonalCredential{Token: "stale-pasted", OnUnauthorized: func() { called++ }})
	err := forgeComment("github", c)
	if err == nil || !isUnauthorized(err) || strings.Contains(err.Error(), "Reconnectez") {
		t.Fatalf("a refused pasted token: %v", err)
	}
	if called != 0 {
		t.Errorf("a pasted token must not call the hook: %d", called)
	}
	g := forgeClient(s, "gitlab", PersonalCredential{Token: "stale-pasted"})
	if err := forgeComment("gitlab", g); err == nil || strings.Contains(err.Error(), "Reconnectez") || !strings.Contains(err.Error(), "jeton GitLab refusé") {
		t.Fatalf("a refused pasted GitLab token keeps its wording: %v", err)
	}
}

// The resolver is told the site of every provider, so the store can refuse a
// grant on a self-hosted instance; an instance left empty is the public one.
func TestASelfHostedTrackerPassesItsSiteToTheResolver(t *testing.T) {
	cases := []struct {
		tracker string
		client  *Client
		want    string
	}{
		{"github", &Client{GithubURL: "https://github.acme.io/api/v3"}, "https://github.acme.io/api/v3"},
		{"github", &Client{}, DefaultGithubURL},
		{"gitlab", &Client{GitlabURL: "https://gitlab.acme.io/api/v4"}, "https://gitlab.acme.io/api/v4"},
		{"gitlab", &Client{}, DefaultGitlabURL},
		{"jira", &Client{JiraURL: "https://acme.atlassian.net"}, "https://acme.atlassian.net"},
	}
	for _, c := range cases {
		var asked []string
		c.client.ResolveUser = func(_, _, site string) (PersonalCredential, error) {
			asked = append(asked, site)
			return PersonalCredential{}, nil
		}
		if _, _, err := c.client.ForActingUser("ada", c.tracker, ""); err != nil {
			t.Fatal(err)
		}
		if len(asked) != 1 || asked[0] != c.want {
			t.Errorf("%s: the resolver was asked for %v, want %q", c.tracker, asked, c.want)
		}
	}
}

// A person with no grant gets the client as it was: their pasted token, or
// none, and no grant behaviour.
func TestNoForgeGrantLeavesTheClientUnchanged(t *testing.T) {
	for _, trackerName := range []string{"github", "gitlab"} {
		s := newForgeSite(t)
		c := forgeClient(s, trackerName, PersonalCredential{Token: "pasted"})
		personal, own, err := c.ForActingUser("ada", trackerName, "")
		if err != nil || !own || personal.oauthGrant || personal.onUnauthorized != nil {
			t.Fatalf("%s: a pasted token: %+v %v %v", trackerName, personal, own, err)
		}
		nobody, own, err := c.ForActingUser("grace", trackerName, "")
		if err != nil || own || nobody.oauthGrant || nobody.GithubToken != "server-token" || nobody.GitlabToken != "server-token" {
			t.Fatalf("%s: no credential: %+v %v %v", trackerName, nobody, own, err)
		}
	}
}
