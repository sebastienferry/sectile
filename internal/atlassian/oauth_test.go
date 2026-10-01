package atlassian_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"tasks/internal/atlassian"
	"tasks/internal/atlassian/atlassiantest"
)

const redirect = "https://sectile.example.com/auth/jira/callback"

func TestAuthorizeURLAsksForConsentOnEveryScope(t *testing.T) {
	app := atlassian.App{ClientID: "client", ClientSecret: "secret", RedirectURL: redirect}
	raw := atlassian.DefaultEndpoints.AuthorizeURL(app, "the-state")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme+"://"+parsed.Host+parsed.Path != "https://auth.atlassian.com/authorize" {
		t.Errorf("authorise endpoint: %s", raw)
	}
	query := parsed.Query()
	for name, want := range map[string]string{
		"audience":      "api.atlassian.com",
		"client_id":     "client",
		"redirect_uri":  redirect,
		"state":         "the-state",
		"response_type": "code",
		"prompt":        "consent",
		"scope":         strings.Join(atlassian.Scopes, " "),
	} {
		if got := query.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if strings.Contains(raw, "secret") {
		t.Errorf("the client secret travels in the browser: %s", raw)
	}
	if !strings.Contains(query.Get("scope"), "offline_access") {
		t.Error("without offline_access there is no refresh token")
	}
}

func TestExchangeRefreshAndSites(t *testing.T) {
	fake := atlassiantest.New(t,
		atlassiantest.FakeSite{CloudID: "c-acme", URL: "https://acme.atlassian.net"},
		atlassiantest.FakeSite{CloudID: "c-beta", URL: "https://beta.atlassian.net"})
	endpoints, app := fake.Endpoints(), fake.App(redirect)
	ctx := context.Background()

	code := fake.Consent("Ada", "c-acme", "c-beta")
	tokens, err := endpoints.Exchange(ctx, nil, app, code)
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("no tokens: %+v", tokens)
	}
	if until := time.Until(tokens.ExpiresAt); until < 59*time.Minute || until > 61*time.Minute {
		t.Errorf("expiry not parsed: %v", tokens.ExpiresAt)
	}
	sites, err := endpoints.Sites(ctx, nil, tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 || sites[0].CloudID != "c-acme" || sites[1].URL != "https://beta.atlassian.net" {
		t.Errorf("sites: %+v", sites)
	}

	// A code works once.
	if _, err := endpoints.Exchange(ctx, nil, app, code); !errors.Is(err, atlassian.ErrInvalidGrant) {
		t.Errorf("a used code: %v", err)
	}

	refreshed, err := endpoints.Refresh(ctx, nil, app, tokens.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.RefreshToken == tokens.RefreshToken || refreshed.AccessToken == tokens.AccessToken {
		t.Error("a refresh must rotate both tokens")
	}
	// The rotated token is dead at once.
	if _, err := endpoints.Refresh(ctx, nil, app, tokens.RefreshToken); !errors.Is(err, atlassian.ErrInvalidGrant) {
		t.Errorf("a used refresh token: %v", err)
	}
	if got := endpoints.JiraAPIBase("c-acme"); got != fake.Server.URL+"/ex/jira/c-acme" {
		t.Errorf("API base: %s", got)
	}
}

func TestRefreshFailuresAreToldApart(t *testing.T) {
	fake := atlassiantest.New(t, atlassiantest.FakeSite{CloudID: "c-acme", URL: "https://acme.atlassian.net"})
	endpoints, app := fake.Endpoints(), fake.App(redirect)
	ctx := context.Background()
	tokens, err := endpoints.Exchange(ctx, nil, app, fake.Consent("Ada", "c-acme"))
	if err != nil {
		t.Fatal(err)
	}

	fake.FailRefreshes(http.StatusServiceUnavailable)
	_, err = endpoints.Refresh(ctx, nil, app, tokens.RefreshToken)
	if err == nil || errors.Is(err, atlassian.ErrInvalidGrant) {
		t.Fatalf("a 503 is a passing failure, not a dead grant: %v", err)
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("the status must be said: %v", err)
	}
	fake.FailRefreshes(0)

	fake.Revoke("Ada")
	if _, err := endpoints.Refresh(ctx, nil, app, tokens.RefreshToken); !errors.Is(err, atlassian.ErrInvalidGrant) {
		t.Errorf("a revoked grant: %v", err)
	}
}

// Atlassian also answers a forgotten refresh token with unauthorized_client;
// a wrong client id answers the same code and must not read as a dead grant.
func TestUnauthorizedClientIsADeadGrantOnlyForTheRefreshToken(t *testing.T) {
	description := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"unauthorized_client","error_description":"` + description + `"}`))
	}))
	defer server.Close()
	endpoints := atlassian.Endpoints{Token: server.URL}
	app := atlassian.App{ClientID: "c", ClientSecret: "s", RedirectURL: redirect}

	description = "refresh_token is invalid"
	if _, err := endpoints.Refresh(context.Background(), nil, app, "r"); !errors.Is(err, atlassian.ErrInvalidGrant) {
		t.Errorf("a forgotten refresh token: %v", err)
	}
	description = "client is not allowed"
	if _, err := endpoints.Refresh(context.Background(), nil, app, "r"); err == nil || errors.Is(err, atlassian.ErrInvalidGrant) {
		t.Errorf("a client refusal is not a dead grant: %v", err)
	}
}

// No error may carry the secret, the code or a token: errors reach logs and
// activities.
func TestErrorsNeverCarryASecret(t *testing.T) {
	secrets := []string{"the-client-secret", "the-code", "the-refresh-token", "the-access-token"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A refusal echoing what it received, which a careless client would
		// quote.
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_request","error_description":"` + strings.Join(secrets, " ") + `"}`))
	}))
	defer server.Close()
	endpoints := atlassian.Endpoints{Token: server.URL, Resources: server.URL}
	app := atlassian.App{ClientID: "c", ClientSecret: "the-client-secret", RedirectURL: redirect}
	ctx := context.Background()

	var errs []error
	_, err := endpoints.Exchange(ctx, nil, app, "the-code")
	errs = append(errs, err)
	_, err = endpoints.Refresh(ctx, nil, app, "the-refresh-token")
	errs = append(errs, err)
	_, err = endpoints.Sites(ctx, nil, "the-access-token")
	errs = append(errs, err)
	// And the network failing, whose error names the URL.
	server.Close()
	_, err = endpoints.Exchange(ctx, nil, app, "the-code")
	errs = append(errs, err)

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
}

func TestSitesLeaveOutResourcesWithoutJira(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`[
			{"id":"c1","url":"https://acme.atlassian.net","name":"acme","scopes":["read:jira-work"]},
			{"id":"c2","url":"https://acme.atlassian.net/wiki","name":"wiki","scopes":["read:confluence-content.all"]}
		]`))
	}))
	defer server.Close()
	sites, err := atlassian.Endpoints{Resources: server.URL}.Sites(context.Background(), nil, "tok")
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].CloudID != "c1" {
		t.Errorf("sites: %+v", sites)
	}
}

func TestAppIsConfiguredOnlyWhole(t *testing.T) {
	if (atlassian.App{ClientID: "c", ClientSecret: "s"}).Configured() {
		t.Error("an app without a callback is not configured")
	}
	if !(atlassian.App{ClientID: "c", ClientSecret: "s", RedirectURL: redirect}).Configured() {
		t.Error("a whole app is configured")
	}
}
