// Package forgeoauthtest is a fake GitHub or GitLab for tests: the consent
// screen, a token endpoint answering the way that forge does, the revocation
// endpoint, and the GET /user a grant's account is read from.
//
// It is strict where the real ones are: a GitLab refresh token stops working
// the moment it is used, so two replicas refreshing one grant with the same
// token see exactly what they would see in production, and GitHub refuses
// with status 200 and the reason in the body.
package forgeoauthtest

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"tasks/internal/forgeoauth"
)

// ClientID and ClientSecret are what the fake accepts.
const (
	ClientID     = "fake-client"
	ClientSecret = "fake-client-secret-never-shown"
)

// gitlabAPIPath is where the fake GitLab serves its REST root, as gitlab.com
// does.
const gitlabAPIPath = "/api/v4"

type grant struct {
	account string
	refresh string
	revoked bool
}

type access struct {
	grant *grant
	// expires is zero for a token that never expires.
	expires time.Time
}

// Fake is the server. Its fields are read under mu.
type Fake struct {
	Server *httptest.Server

	provider forgeoauth.Provider
	redirect string

	mu        sync.Mutex
	codes     map[string]*grant
	grants    []*grant
	accesses  map[string]access
	refreshes int
	// refreshStatus, when set, is what every refresh answers instead.
	refreshStatus int
	// accessTTL is the lifetime of the access tokens issued; zero issues
	// GitHub-style tokens that never expire and come without a refresh token.
	accessTTL   time.Duration
	revocations []string
	// refreshGate, when set, holds each refresh until the test lets it go.
	refreshGate chan struct{}
}

// New starts a fake of the given provider. A GitHub fake issues tokens that
// never expire, a GitLab one tokens living two hours, as the real ones do.
func New(t testing.TB, provider forgeoauth.Provider) *Fake {
	t.Helper()
	f := &Fake{
		provider: provider,
		redirect: "https://sectile.example.com/auth/" + provider.Name + "/callback",
		codes:    map[string]*grant{},
		accesses: map[string]access{},
	}
	if provider.Name == forgeoauth.GitLab.Name {
		f.accessTTL = 2 * time.Hour
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Server.Close)
	return f
}

// Endpoints points the client at the fake. The name and the scopes stay the
// provider's, so the client takes the same branches it takes in production.
func (f *Fake) Endpoints() forgeoauth.Provider {
	p := f.provider
	p.AuthorizeEndpoint = f.Server.URL + "/oauth/authorize"
	p.TokenURL = f.Server.URL + "/oauth/token"
	if p.Name == forgeoauth.GitHub.Name {
		p.RevokeURL = f.Server.URL + "/applications/{client_id}/grant"
		p.APIBase = f.Server.URL
	} else {
		p.RevokeURL = f.Server.URL + "/oauth/revoke"
		p.APIBase = f.Server.URL + gitlabAPIPath
	}
	return p
}

// App is an app the fake accepts.
func (f *Fake) App() forgeoauth.App {
	return forgeoauth.App{ClientID: ClientID, ClientSecret: ClientSecret, RedirectURL: f.redirect}
}

// Consent is what a person accepting the consent screen produces: a code for
// a grant of account.
func (f *Fake) Consent(account string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	g := &grant{account: account}
	f.grants = append(f.grants, g)
	code := random()
	f.codes[code] = g
	return code
}

// SetAccessTTL sets the lifetime of the access tokens issued from now on;
// zero issues tokens that never expire, without a refresh token.
func (f *Fake) SetAccessTTL(ttl time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.accessTTL = ttl
}

// Revoke makes every grant of account fail its next refresh, and its access
// tokens refused, as when the person withdraws the app on the forge.
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

// Revocations returns the accounts whose grant the revocation endpoint
// withdrew, in order.
func (f *Fake) Revocations() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.revocations...)
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/oauth/authorize":
		w.WriteHeader(http.StatusOK)
	case r.URL.Path == "/oauth/token" && r.Method == http.MethodPost:
		f.token(w, r)
	case r.URL.Path == "/applications/"+ClientID+"/grant" && r.Method == http.MethodDelete && f.github():
		f.revokeGithub(w, r)
	case r.URL.Path == "/oauth/revoke" && r.Method == http.MethodPost && !f.github():
		f.revokeGitlab(w, r)
	case r.URL.Path == f.userPath() && r.Method == http.MethodGet:
		f.user(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *Fake) github() bool {
	return f.provider.Name == forgeoauth.GitHub.Name
}

func (f *Fake) userPath() string {
	if f.github() {
		return "/user"
	}
	return gitlabAPIPath + "/user"
}

func (f *Fake) token(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") || r.ParseForm() != nil {
		f.refuse(w, http.StatusBadRequest, "invalid_request")
		return
	}
	form := r.PostForm
	if form.Get("client_id") != ClientID || form.Get("client_secret") != ClientSecret {
		if f.github() {
			f.refuse(w, http.StatusUnauthorized, "incorrect_client_credentials")
		} else {
			f.refuse(w, http.StatusUnauthorized, "invalid_client")
		}
		return
	}
	// GitLab wants the callback again on the exchange and on every refresh.
	if (form.Get("grant_type") == "authorization_code" || !f.github()) && form.Get("redirect_uri") != f.redirect {
		f.refuse(w, http.StatusBadRequest, "invalid_request")
		return
	}
	f.mu.Lock()
	gate := f.refreshGate
	f.mu.Unlock()
	if form.Get("grant_type") == "refresh_token" && gate != nil {
		<-gate
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	var g *grant
	switch form.Get("grant_type") {
	case "authorization_code":
		g = f.codes[form.Get("code")]
		delete(f.codes, form.Get("code"))
		if g == nil {
			f.refuseDead(w, "bad_verification_code")
			return
		}
	case "refresh_token":
		if f.refreshStatus != 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(f.refreshStatus)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "temporarily_unavailable"})
			return
		}
		for _, candidate := range f.grants {
			if candidate.refresh != "" && candidate.refresh == form.Get("refresh_token") {
				g = candidate
			}
		}
		if g == nil || g.revoked {
			f.refuseDead(w, "bad_refresh_token")
			return
		}
		f.refreshes++
	default:
		f.refuse(w, http.StatusBadRequest, "unsupported_grant_type")
		return
	}
	token := random()
	answer := map[string]any{
		"access_token": token,
		"scope":        f.grantedScope(),
		"token_type":   "bearer",
	}
	if f.accessTTL > 0 {
		// Rotation: the refresh token just used, if any, is dead from now on.
		g.refresh = random()
		answer["refresh_token"] = g.refresh
		answer["expires_in"] = int(f.accessTTL.Seconds())
		f.accesses[token] = access{grant: g, expires: time.Now().Add(f.accessTTL)}
	} else {
		f.accesses[token] = access{grant: g}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(answer)
}

// grantedScope is the scope list as the forge writes it back: GitHub joins it
// with commas, GitLab with spaces.
func (f *Fake) grantedScope() string {
	if f.github() {
		return strings.Join(f.provider.Scopes, ",")
	}
	return strings.Join(f.provider.Scopes, " ")
}

// refuseDead answers a code or a refresh token the forge no longer honours:
// GitHub with its own code and status 200, GitLab with invalid_grant.
func (f *Fake) refuseDead(w http.ResponseWriter, githubCode string) {
	if f.github() {
		f.refuse(w, http.StatusOK, githubCode)
		return
	}
	f.refuse(w, http.StatusBadRequest, "invalid_grant")
}

// refuse answers a token refusal. GitHub always answers it with status 200.
func (f *Fake) refuse(w http.ResponseWriter, status int, code string) {
	if f.github() {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code, "error_description": "fake: " + code})
}

// valid answers the grant behind a live access token.
func (f *Fake) valid(token string) *grant {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.accesses[token]
	if !ok || a.grant.revoked || (!a.expires.IsZero() && time.Now().After(a.expires)) {
		return nil
	}
	return a.grant
}

func (f *Fake) user(w http.ResponseWriter, r *http.Request) {
	g := f.valid(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if g == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	field := "username"
	if f.github() {
		field = "login"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{field: g.account})
}

// revokeGithub withdraws every token of the app for the person behind the
// access token in the body, as GitHub's DELETE /applications/{id}/grant does.
func (f *Fake) revokeGithub(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	if !ok || id != ClientID || secret != ClientSecret {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	if !f.revokeToken(body.AccessToken) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// revokeGitlab withdraws the grant behind the token, answering 200 even for a
// token it does not know, as RFC 7009 has it.
func (f *Fake) revokeGitlab(w http.ResponseWriter, r *http.Request) {
	if r.ParseForm() != nil || r.PostForm.Get("client_id") != ClientID || r.PostForm.Get("client_secret") != ClientSecret {
		f.refuse(w, http.StatusUnauthorized, "invalid_client")
		return
	}
	f.revokeToken(r.PostForm.Get("token"))
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("{}"))
}

func (f *Fake) revokeToken(token string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.accesses[token]
	if !ok || a.grant.revoked {
		return false
	}
	a.grant.revoked = true
	f.revocations = append(f.revocations, a.grant.account)
	return true
}

func random() string {
	raw := make([]byte, 16)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}
