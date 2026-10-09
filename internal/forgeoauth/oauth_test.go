package forgeoauth_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tasks/internal/forgeoauth"
	"tasks/internal/forgeoauth/forgeoauthtest"
)

func TestAuthorizeURLAsksForEveryScope(t *testing.T) {
	cases := []struct {
		tracker  string
		endpoint string
		scope    string
		extra    map[string]string
		absent   string
	}{
		{"github", "https://github.com/login/oauth/authorize", "repo read:project", map[string]string{"allow_signup": "false"}, "response_type"},
		{"gitlab", "https://gitlab.com/oauth/authorize", "api", map[string]string{"response_type": "code"}, "allow_signup"},
	}
	for _, tc := range cases {
		t.Run(tc.tracker, func(t *testing.T) {
			provider, ok := forgeoauth.ForTracker(tc.tracker)
			if !ok {
				t.Fatalf("%s has no provider", tc.tracker)
			}
			redirect := "https://sectile.example.com/auth/" + tc.tracker + "/callback"
			app := forgeoauth.App{ClientID: "client", ClientSecret: "secret", RedirectURL: redirect}
			raw := provider.AuthorizeURL(app, "the-state")
			parsed, err := url.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Scheme+"://"+parsed.Host+parsed.Path != tc.endpoint {
				t.Errorf("authorise endpoint: %s", raw)
			}
			query := parsed.Query()
			want := map[string]string{
				"client_id":    "client",
				"redirect_uri": redirect,
				"state":        "the-state",
				"scope":        tc.scope,
			}
			for name, value := range tc.extra {
				want[name] = value
			}
			for name, value := range want {
				if got := query.Get(name); got != value {
					t.Errorf("%s = %q, want %q", name, got, value)
				}
			}
			if query.Has(tc.absent) {
				t.Errorf("%s is not a %s parameter: %s", tc.absent, tc.tracker, raw)
			}
			if strings.Contains(raw, "secret") {
				t.Errorf("the client secret travels in the browser: %s", raw)
			}
		})
	}
	if _, ok := forgeoauth.ForTracker("jira"); ok {
		t.Error("Jira is Atlassian's, not a forge provider")
	}
}

func TestExchangeAndRefresh(t *testing.T) {
	ctx := context.Background()

	t.Run("gitlab rotates", func(t *testing.T) {
		fake := forgeoauthtest.New(t, forgeoauth.GitLab)
		provider, app := fake.Endpoints(), fake.App()
		code := fake.Consent("ada")
		tokens, err := provider.Exchange(ctx, nil, app, code)
		if err != nil {
			t.Fatal(err)
		}
		if tokens.AccessToken == "" || tokens.RefreshToken == "" {
			t.Fatalf("no tokens: %+v", tokens)
		}
		if until := time.Until(tokens.ExpiresAt); until < 119*time.Minute || until > 121*time.Minute {
			t.Errorf("expiry not parsed: %v", tokens.ExpiresAt)
		}
		if tokens.Scope != "api" {
			t.Errorf("scope: %q", tokens.Scope)
		}

		// A code works once.
		if _, err := provider.Exchange(ctx, nil, app, code); !errors.Is(err, forgeoauth.ErrInvalidGrant) {
			t.Errorf("a used code: %v", err)
		}

		refreshed, err := provider.Refresh(ctx, nil, app, tokens.RefreshToken)
		if err != nil {
			t.Fatal(err)
		}
		if refreshed.RefreshToken == tokens.RefreshToken || refreshed.AccessToken == tokens.AccessToken {
			t.Error("a refresh must rotate both tokens")
		}
		// The rotated token is dead at once.
		if _, err := provider.Refresh(ctx, nil, app, tokens.RefreshToken); !errors.Is(err, forgeoauth.ErrInvalidGrant) {
			t.Errorf("a used refresh token: %v", err)
		}
		if fake.Refreshes() != 1 {
			t.Errorf("refreshes: %d", fake.Refreshes())
		}
	})

	t.Run("gitlab needs the callback on a refresh", func(t *testing.T) {
		fake := forgeoauthtest.New(t, forgeoauth.GitLab)
		provider, app := fake.Endpoints(), fake.App()
		tokens, err := provider.Exchange(ctx, nil, app, fake.Consent("ada"))
		if err != nil {
			t.Fatal(err)
		}
		// The fake refuses a refresh without the callback, as GitLab does: the
		// one sent must be the app's.
		other := app
		other.RedirectURL = "https://elsewhere.example.com/callback"
		if _, err := provider.Refresh(ctx, nil, other, tokens.RefreshToken); err == nil {
			t.Error("a refresh with another callback must be refused")
		}
		if _, err := provider.Refresh(ctx, nil, app, tokens.RefreshToken); err != nil {
			t.Errorf("a refresh with the callback: %v", err)
		}
	})

	t.Run("github never expires", func(t *testing.T) {
		fake := forgeoauthtest.New(t, forgeoauth.GitHub)
		provider, app := fake.Endpoints(), fake.App()
		tokens, err := provider.Exchange(ctx, nil, app, fake.Consent("ada"))
		if err != nil {
			t.Fatal(err)
		}
		if tokens.AccessToken == "" {
			t.Fatalf("no access token: %+v", tokens)
		}
		if !tokens.ExpiresAt.IsZero() || tokens.RefreshToken != "" {
			t.Errorf("an OAuth App token never expires and has no refresh token: %+v", tokens)
		}
		if tokens.Scope != "repo,read:project" {
			t.Errorf("scope: %q", tokens.Scope)
		}
	})

	t.Run("github with expiring tokens", func(t *testing.T) {
		fake := forgeoauthtest.New(t, forgeoauth.GitHub)
		fake.SetAccessTTL(8 * time.Hour)
		provider, app := fake.Endpoints(), fake.App()
		tokens, err := provider.Exchange(ctx, nil, app, fake.Consent("ada"))
		if err != nil {
			t.Fatal(err)
		}
		if tokens.ExpiresAt.IsZero() || tokens.RefreshToken == "" {
			t.Fatalf("an expiring token comes with its expiry and a refresh token: %+v", tokens)
		}
		refreshed, err := provider.Refresh(ctx, nil, app, tokens.RefreshToken)
		if err != nil {
			t.Fatal(err)
		}
		if refreshed.RefreshToken == tokens.RefreshToken {
			t.Error("a refresh must rotate the refresh token")
		}
		if _, err := provider.Refresh(ctx, nil, app, tokens.RefreshToken); !errors.Is(err, forgeoauth.ErrInvalidGrant) {
			t.Errorf("a used refresh token: %v", err)
		}
	})
}

// GitHub refuses a code or a refresh token with status 200 and the reason in
// the body: an answer carrying an error is a refusal whatever its status.
func TestGitHubErrorBodiesWithStatus200AreRefusals(t *testing.T) {
	code := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":"` + code + `","error_description":"refused"}`))
	}))
	defer server.Close()
	provider := forgeoauth.GitHub
	provider.TokenURL = server.URL
	app := forgeoauth.App{ClientID: "c", ClientSecret: "s", RedirectURL: "https://sectile.example.com/auth/github/callback"}
	ctx := context.Background()

	for _, dead := range []string{"bad_verification_code", "bad_refresh_token", "invalid_grant"} {
		code = dead
		if _, err := provider.Exchange(ctx, nil, app, "the-code"); !errors.Is(err, forgeoauth.ErrInvalidGrant) {
			t.Errorf("%s on an exchange: %v", dead, err)
		}
		if _, err := provider.Refresh(ctx, nil, app, "the-refresh"); !errors.Is(err, forgeoauth.ErrInvalidGrant) {
			t.Errorf("%s on a refresh: %v", dead, err)
		}
	}

	// A wrong client secret is the admin's to fix, and disconnects nobody.
	code = "incorrect_client_credentials"
	_, err := provider.Refresh(ctx, nil, app, "the-refresh")
	if err == nil || errors.Is(err, forgeoauth.ErrInvalidGrant) {
		t.Fatalf("a client refusal is a refusal, not a dead grant: %v", err)
	}
	if !strings.Contains(err.Error(), "incorrect_client_credentials") {
		t.Errorf("the error code must be said: %v", err)
	}
}

func TestRefreshFailuresAreToldApart(t *testing.T) {
	fake := forgeoauthtest.New(t, forgeoauth.GitLab)
	provider, app := fake.Endpoints(), fake.App()
	ctx := context.Background()
	tokens, err := provider.Exchange(ctx, nil, app, fake.Consent("ada"))
	if err != nil {
		t.Fatal(err)
	}

	fake.FailRefreshes(http.StatusServiceUnavailable)
	_, err = provider.Refresh(ctx, nil, app, tokens.RefreshToken)
	if err == nil || errors.Is(err, forgeoauth.ErrInvalidGrant) {
		t.Fatalf("a 503 is a passing failure, not a dead grant: %v", err)
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("the status must be said: %v", err)
	}
	fake.FailRefreshes(0)

	fake.Revoke("ada")
	if _, err := provider.Refresh(ctx, nil, app, tokens.RefreshToken); !errors.Is(err, forgeoauth.ErrInvalidGrant) {
		t.Errorf("a revoked grant: %v", err)
	}

	// The network failing is a passing failure too.
	fake.Server.Close()
	_, err = provider.Refresh(ctx, nil, app, tokens.RefreshToken)
	if err == nil || errors.Is(err, forgeoauth.ErrInvalidGrant) {
		t.Errorf("an unreachable forge is not a dead grant: %v", err)
	}
}

// No error may carry the secret, the code or a token: errors reach logs and
// activities.
func TestErrorsNeverCarryASecret(t *testing.T) {
	secrets := []string{"the-client-secret", "the-code", "the-refresh-token", "the-access-token"}
	for _, provider := range []forgeoauth.Provider{forgeoauth.GitHub, forgeoauth.GitLab} {
		t.Run(provider.Name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// A refusal echoing what it received, which a careless client
				// would quote.
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"invalid_request","error_description":"` + strings.Join(secrets, " ") + `","message":"` + strings.Join(secrets, " ") + `"}`))
			}))
			defer server.Close()
			provider.TokenURL = server.URL
			provider.RevokeURL = server.URL + "/applications/{client_id}/grant"
			app := forgeoauth.App{ClientID: "c", ClientSecret: "the-client-secret", RedirectURL: "https://sectile.example.com/callback"}
			ctx := context.Background()

			var errs []error
			_, err := provider.Exchange(ctx, nil, app, "the-code")
			errs = append(errs, err)
			_, err = provider.Refresh(ctx, nil, app, "the-refresh-token")
			errs = append(errs, err)
			errs = append(errs, provider.Revoke(ctx, nil, app, "the-access-token"))
			// And the network failing, whose error names the URL.
			server.Close()
			_, err = provider.Exchange(ctx, nil, app, "the-code")
			errs = append(errs, err)
			errs = append(errs, provider.Revoke(ctx, nil, app, "the-access-token"))

			for _, err := range errs {
				if err == nil {
					t.Fatal("a refusal must be an error")
				}
				for _, secret := range secrets {
					if strings.Contains(err.Error(), secret) {
						t.Errorf("%q leaks %q", err, secret)
					}
				}
			}
		})
	}
}

func TestAppIsConfiguredOnlyWhole(t *testing.T) {
	if (forgeoauth.App{ClientID: "c", ClientSecret: "s"}).Configured() {
		t.Error("an app without a callback is not configured")
	}
	if (forgeoauth.App{ClientID: "c", RedirectURL: "https://sectile.example.com/auth/gitlab/callback"}).Configured() {
		t.Error("an app without a secret is not configured")
	}
	if !(forgeoauth.App{ClientID: "c", ClientSecret: "s", RedirectURL: "https://sectile.example.com/auth/gitlab/callback"}).Configured() {
		t.Error("a whole app is configured")
	}
}

func TestRevoke(t *testing.T) {
	ctx := context.Background()
	app := forgeoauth.App{ClientID: "the/client", ClientSecret: "the-secret", RedirectURL: "https://sectile.example.com/callback"}

	t.Run("github sends Basic auth and the token in a JSON body", func(t *testing.T) {
		var method, path, accept, contentType, body string
		var user, password string
		status := http.StatusNoContent
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method, path = r.Method, r.URL.EscapedPath()
			accept, contentType = r.Header.Get("Accept"), r.Header.Get("Content-Type")
			user, password, _ = r.BasicAuth()
			raw, _ := io.ReadAll(r.Body)
			body = string(raw)
			w.WriteHeader(status)
		}))
		defer server.Close()
		provider := forgeoauth.GitHub
		provider.RevokeURL = server.URL + "/applications/{client_id}/grant"

		if err := provider.Revoke(ctx, nil, app, "the-token"); err != nil {
			t.Fatal(err)
		}
		if method != http.MethodDelete || path != "/applications/the%2Fclient/grant" {
			t.Errorf("request: %s %s", method, path)
		}
		if user != "the/client" || password != "the-secret" {
			t.Errorf("Basic auth: %q:%q", user, password)
		}
		if accept != "application/vnd.github+json" || contentType != "application/json" {
			t.Errorf("headers: Accept %q, Content-Type %q", accept, contentType)
		}
		var sent map[string]string
		if err := json.Unmarshal([]byte(body), &sent); err != nil || sent["access_token"] != "the-token" || len(sent) != 1 {
			t.Errorf("body: %s", body)
		}

		// A grant GitHub no longer knows is already revoked.
		status = http.StatusNotFound
		if err := provider.Revoke(ctx, nil, app, "the-token"); err != nil {
			t.Errorf("a 404 counts as done: %v", err)
		}
		status = http.StatusUnprocessableEntity
		if err := provider.Revoke(ctx, nil, app, "the-token"); err == nil || !strings.Contains(err.Error(), "422") {
			t.Errorf("a refusal must say its status: %v", err)
		}
	})

	t.Run("gitlab posts a form", func(t *testing.T) {
		var method, contentType string
		var form url.Values
		status := http.StatusOK
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method, contentType = r.Method, r.Header.Get("Content-Type")
			_ = r.ParseForm()
			form = r.PostForm
			w.WriteHeader(status)
		}))
		defer server.Close()
		provider := forgeoauth.GitLab
		provider.RevokeURL = server.URL

		if err := provider.Revoke(ctx, nil, app, "the-token"); err != nil {
			t.Fatal(err)
		}
		if method != http.MethodPost || contentType != "application/x-www-form-urlencoded" {
			t.Errorf("request: %s, Content-Type %q", method, contentType)
		}
		for name, want := range map[string]string{"client_id": "the/client", "client_secret": "the-secret", "token": "the-token"} {
			if got := form.Get(name); got != want {
				t.Errorf("%s = %q, want %q", name, got, want)
			}
		}
		// Only GitHub's 404 means done; GitLab answers 200 for an unknown token.
		status = http.StatusNotFound
		if err := provider.Revoke(ctx, nil, app, "the-token"); err == nil {
			t.Error("a GitLab 404 is a refusal")
		}
	})

	for _, provider := range []forgeoauth.Provider{forgeoauth.GitHub, forgeoauth.GitLab} {
		t.Run(provider.Name+" against the fake", func(t *testing.T) {
			fake := forgeoauthtest.New(t, provider)
			endpoints, fakeApp := fake.Endpoints(), fake.App()
			tokens, err := endpoints.Exchange(ctx, nil, fakeApp, fake.Consent("ada"))
			if err != nil {
				t.Fatal(err)
			}
			if err := endpoints.Revoke(ctx, nil, fakeApp, tokens.AccessToken); err != nil {
				t.Fatal(err)
			}
			if got := fake.Revocations(); len(got) != 1 || got[0] != "ada" {
				t.Errorf("revocations: %v", got)
			}
			// The account read now fails: the token is dead.
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoints.APIBase+"/user", nil)
			req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = res.Body.Close()
			if res.StatusCode != http.StatusUnauthorized {
				t.Errorf("a revoked token reads the account: HTTP %d", res.StatusCode)
			}
			// Revoking twice is still done.
			if err := endpoints.Revoke(ctx, nil, fakeApp, tokens.AccessToken); err != nil {
				t.Errorf("a second revoke: %v", err)
			}
		})
	}
}

// The fake answers GET /user the way each forge does, which is where the
// account of a grant is read from.
func TestTheFakeReadsTheAccount(t *testing.T) {
	ctx := context.Background()
	for provider, field := range map[string]string{"github": "login", "gitlab": "username"} {
		t.Run(provider, func(t *testing.T) {
			p, _ := forgeoauth.ForTracker(provider)
			fake := forgeoauthtest.New(t, p)
			endpoints := fake.Endpoints()
			tokens, err := endpoints.Exchange(ctx, nil, fake.App(), fake.Consent("ada"))
			if err != nil {
				t.Fatal(err)
			}
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, endpoints.APIBase+"/user", nil)
			req.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = res.Body.Close() }()
			var account map[string]string
			if err := json.NewDecoder(res.Body).Decode(&account); err != nil || account[field] != "ada" {
				t.Errorf("account: HTTP %d %v", res.StatusCode, account)
			}
		})
	}
}
