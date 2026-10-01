// Package atlassiantest is a fake Atlassian for tests: the consent screen, a
// token endpoint that rotates refresh tokens, the accessible resources of a
// grant, and the Jira gateway a grant's calls go through.
//
// It is strict where the real one is: a refresh token stops working the
// moment it is used, so two replicas refreshing one grant with the same token
// see exactly what they would see in production.
package atlassiantest

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/atlassian"
)

// ClientID and ClientSecret are what the fake accepts.
const (
	ClientID     = "fake-client"
	ClientSecret = "fake-client-secret-never-shown"
)

// FakeSite is one Jira site the fake knows.
type FakeSite struct {
	CloudID string
	URL     string
}

// Write is one call the gateway served with a method other than GET.
type Write struct {
	Method  string
	Path    string
	CloudID string
	Account string
}

type grant struct {
	account string
	sites   []FakeSite
	refresh string
	revoked bool
}

type access struct {
	grant   *grant
	expires time.Time
}

// Fake is the server. Its fields are read under mu.
type Fake struct {
	Server *httptest.Server

	mu        sync.Mutex
	sites     map[string]FakeSite // cloudId → site
	codes     map[string]*grant
	grants    []*grant
	accesses  map[string]access
	refreshes int
	// refreshStatus, when set, is what every refresh answers instead.
	refreshStatus int
	// accessTTL is the lifetime of the access tokens issued.
	accessTTL time.Duration
	writes    []Write
	authorize []url.Values
	// refreshGate, when set, holds each refresh until the test lets it go.
	refreshGate chan struct{}
}

// New starts a fake knowing the given sites.
func New(t testing.TB, sites ...FakeSite) *Fake {
	t.Helper()
	f := &Fake{
		sites:     map[string]FakeSite{},
		codes:     map[string]*grant{},
		accesses:  map[string]access{},
		accessTTL: time.Hour,
	}
	for _, site := range sites {
		f.sites[site.CloudID] = site
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Server.Close)
	return f
}

// Endpoints points the client at the fake.
func (f *Fake) Endpoints() atlassian.Endpoints {
	return atlassian.Endpoints{
		Authorize: f.Server.URL + "/authorize",
		Token:     f.Server.URL + "/oauth/token",
		Resources: f.Server.URL + "/oauth/token/accessible-resources",
		API:       f.Server.URL,
	}
}

// App is an app the fake accepts, calling back to redirect.
func (f *Fake) App(redirect string) atlassian.App {
	return atlassian.App{ClientID: ClientID, ClientSecret: ClientSecret, RedirectURL: redirect}
}

// Consent is what a person accepting the consent screen produces: a code
// for a grant of account over the given sites.
func (f *Fake) Consent(account string, cloudIDs ...string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	g := &grant{account: account}
	for _, id := range cloudIDs {
		g.sites = append(g.sites, f.sites[id])
	}
	f.grants = append(f.grants, g)
	code := random()
	f.codes[code] = g
	return code
}

// SetAccessTTL sets the lifetime of the access tokens issued from now on.
func (f *Fake) SetAccessTTL(ttl time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.accessTTL = ttl
}

// Revoke makes every grant of account fail its next refresh with
// invalid_grant, and its access tokens refused.
func (f *Fake) Revoke(account string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, g := range f.grants {
		if g.account == account {
			g.revoked = true
		}
	}
}

// FailRefreshes makes every refresh answer status, 0 restoring the normal
// behaviour.
func (f *Fake) FailRefreshes(status int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshStatus = status
}

// GateRefreshes holds every refresh until the returned function is called,
// so a test can line several of them up.
func (f *Fake) GateRefreshes() (release func()) {
	gate := make(chan struct{})
	f.mu.Lock()
	f.refreshGate = gate
	f.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { close(gate) }) }
}

// Refreshes counts the refreshes the fake honoured.
func (f *Fake) Refreshes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refreshes
}

// Writes returns the writes the gateway served.
func (f *Fake) Writes() []Write {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Write{}, f.writes...)
}

// AuthorizeRequests returns the queries the consent screen received.
func (f *Fake) AuthorizeRequests() []url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]url.Values{}, f.authorize...)
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/authorize":
		f.mu.Lock()
		f.authorize = append(f.authorize, r.URL.Query())
		f.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	case r.URL.Path == "/oauth/token" && r.Method == http.MethodPost:
		f.token(w, r)
	case r.URL.Path == "/oauth/token/accessible-resources":
		f.resources(w, r)
	case strings.HasPrefix(r.URL.Path, "/ex/jira/"):
		f.gateway(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *Fake) token(w http.ResponseWriter, r *http.Request) {
	var form map[string]string
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		refuse(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if form["client_id"] != ClientID || form["client_secret"] != ClientSecret {
		refuse(w, http.StatusUnauthorized, "access_denied")
		return
	}
	f.mu.Lock()
	gate := f.refreshGate
	f.mu.Unlock()
	if form["grant_type"] == "refresh_token" && gate != nil {
		<-gate
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	var g *grant
	switch form["grant_type"] {
	case "authorization_code":
		g = f.codes[form["code"]]
		delete(f.codes, form["code"])
		if g == nil {
			refuse(w, http.StatusForbidden, "invalid_grant")
			return
		}
	case "refresh_token":
		if f.refreshStatus != 0 {
			refuse(w, f.refreshStatus, "temporarily_unavailable")
			return
		}
		for _, candidate := range f.grants {
			if candidate.refresh != "" && candidate.refresh == form["refresh_token"] {
				g = candidate
			}
		}
		if g == nil || g.revoked {
			refuse(w, http.StatusForbidden, "invalid_grant")
			return
		}
		f.refreshes++
	default:
		refuse(w, http.StatusBadRequest, "unsupported_grant_type")
		return
	}
	// Rotation: the refresh token just used, if any, is dead from now on.
	g.refresh = random()
	token := random()
	f.accesses[token] = access{grant: g, expires: time.Now().Add(f.accessTTL)}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token":  token,
		"refresh_token": g.refresh,
		"expires_in":    int(f.accessTTL.Seconds()),
		"scope":         strings.Join(atlassian.Scopes, " "),
		"token_type":    "Bearer",
	})
}

// valid answers the grant behind a live access token.
func (f *Fake) valid(r *http.Request) *grant {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.accesses[token]
	if !ok || a.grant.revoked || time.Now().After(a.expires) {
		return nil
	}
	return a.grant
}

func (f *Fake) resources(w http.ResponseWriter, r *http.Request) {
	g := f.valid(r)
	if g == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	out := []map[string]any{}
	for _, site := range g.sites {
		out = append(out, map[string]any{
			"id": site.CloudID, "url": site.URL, "name": site.CloudID,
			"scopes": []string{"read:jira-work", "write:jira-work"},
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// gateway serves the Jira REST paths of a site behind a grant: enough of them
// to confirm an account and to accept the writes a person causes.
func (f *Fake) gateway(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/ex/jira/")
	cloudID, path, _ := strings.Cut(rest, "/")
	path = "/" + path
	g := f.valid(r)
	if g == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	covered := false
	for _, site := range g.sites {
		covered = covered || site.CloudID == cloudID
	}
	if !covered {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		f.mu.Lock()
		f.writes = append(f.writes, Write{Method: r.Method, Path: path, CloudID: cloudID, Account: g.account})
		f.mu.Unlock()
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case path == "/rest/api/3/myself":
		_ = json.NewEncoder(w).Encode(map[string]string{"accountId": "acc-" + g.account, "displayName": g.account})
	case strings.HasSuffix(path, "/transitions") && r.Method == http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]any{"transitions": []map[string]any{
			{"id": "31", "name": "Done", "to": map[string]any{"name": "Done", "statusCategory": map[string]string{"key": "done"}}},
		}})
	case path == "/rest/api/3/issue" && r.Method == http.MethodPost:
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "10001", "key": "PE-1"})
	case r.Method == http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]any{})
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func refuse(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": fmt.Sprintf("fake: %s", code)})
}

func random() string {
	raw := make([]byte, 16)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}
