// Package auth signs people in through an OpenID Connect provider.
//
// The server is a confidential client: it exchanges the authorization code
// over a direct TLS connection to the provider's token endpoint, then reads
// the subject from the UserInfo endpoint with the resulting access token.
// OpenID Connect Core allows a confidential client to rely on that channel
// rather than verifying the ID token signature itself (section 3.1.3.7), and
// it keeps the identity bound to a token the provider has just validated.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Identity is what the provider says about the person signing in.
type Identity struct {
	Subject     string
	Email       string
	DisplayName string
	// Role is the Sectile role the provider's claim resolves to, meaningful
	// only when RoleFromClaim is set: without a configured claim the provider
	// says nothing about roles and the stored role stands.
	Role          string
	RoleFromClaim bool
}

// Provider is a configured OpenID Connect provider.
type Provider struct {
	config       oauth2.Config
	userInfoURL  string
	issuer       string
	httpClient   *http.Client
	discoveredAt time.Time
	// roleClaim names the claim carrying the user's groups or roles, and
	// adminGroup the value that grants admin. Both empty means the provider
	// does not supply roles.
	roleClaim  string
	adminGroup string
}

type discovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
}

// Configured reports whether a provider is declared. Without one the server
// keeps its single implicit user, which is how a personal deployment runs.
func Configured() bool {
	return strings.TrimSpace(os.Getenv("SECTILE_OIDC_ISSUER")) != ""
}

// Discover reads the provider's metadata document and builds a client from the
// environment. It is called once at startup: a provider that cannot be reached
// is a configuration error, not something to retry per request.
func Discover(ctx context.Context) (*Provider, error) {
	issuer := strings.TrimSpace(os.Getenv("SECTILE_OIDC_ISSUER"))
	clientID := strings.TrimSpace(os.Getenv("SECTILE_OIDC_CLIENT_ID"))
	clientSecret := os.Getenv("SECTILE_OIDC_CLIENT_SECRET")
	redirect := strings.TrimSpace(os.Getenv("SECTILE_OIDC_REDIRECT_URL"))

	if issuer == "" || clientID == "" || redirect == "" {
		return nil, errors.New("SECTILE_OIDC_ISSUER, SECTILE_OIDC_CLIENT_ID and SECTILE_OIDC_REDIRECT_URL are required")
	}
	issuerURL, err := url.Parse(issuer)
	if err != nil || issuerURL.Scheme != "https" {
		// An issuer reached over plain HTTP would let the network choose who
		// the user is.
		return nil, fmt.Errorf("SECTILE_OIDC_ISSUER must be an HTTPS URL")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	metadataURL := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("read provider metadata: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider metadata returned HTTP %d", response.StatusCode)
	}
	var document discovery
	if err := json.NewDecoder(response.Body).Decode(&document); err != nil {
		return nil, fmt.Errorf("decode provider metadata: %w", err)
	}
	// A metadata document served by one issuer must not name another.
	if strings.TrimRight(document.Issuer, "/") != strings.TrimRight(issuer, "/") {
		return nil, fmt.Errorf("provider metadata declares issuer %q", document.Issuer)
	}
	if document.AuthorizationEndpoint == "" || document.TokenEndpoint == "" || document.UserInfoEndpoint == "" {
		return nil, errors.New("provider metadata is missing an endpoint")
	}

	roleClaim, adminGroup, err := RoleClaimConfig()
	if err != nil {
		return nil, err
	}
	scopes := []string{"openid", "profile", "email"}
	if roleClaim == "groups" {
		// Okta serves its groups claim only when the scope of the same name is
		// requested; asking for it elsewhere is harmless.
		scopes = append(scopes, "groups")
	}

	return &Provider{
		config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirect,
			Scopes:       scopes,
			Endpoint: oauth2.Endpoint{
				AuthURL:  document.AuthorizationEndpoint,
				TokenURL: document.TokenEndpoint,
			},
		},
		userInfoURL:  document.UserInfoEndpoint,
		issuer:       issuer,
		httpClient:   client,
		discoveredAt: time.Now(),
		roleClaim:    roleClaim,
		adminGroup:   adminGroup,
	}, nil
}

// SuppliesRoles reports whether sign-ins carry a role from the provider.
func (p *Provider) SuppliesRoles() bool { return p.roleClaim != "" }

// Issuer is the provider this server signs people in against.
func (p *Provider) Issuer() string { return p.issuer }

// NewVerifier returns a PKCE code verifier and its challenge. PKCE binds the
// authorization code to this sign-in, so a stolen code is useless elsewhere.
func NewVerifier() (verifier string, challenge string, err error) {
	buffer := make([]byte, 32)
	if _, err = rand.Read(buffer); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(buffer)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// NewNonce returns a value echoed by the provider, tying the response to this
// request.
func NewNonce() (string, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

// AuthCodeURL is where the browser is sent to sign in.
func (p *Provider) AuthCodeURL(state, nonce, challenge string) string {
	return p.config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"))
}

// Exchange trades the authorization code for tokens and reads the identity
// from the UserInfo endpoint.
func (p *Provider) Exchange(ctx context.Context, code, verifier string) (*Identity, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
	token, err := p.config.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	token.SetAuthHeader(request)
	response, err := p.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("read user info: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user info returned HTTP %d", response.StatusCode)
	}
	// UserInfo is decoded twice: once into the fields every sign-in needs, once
	// as a map so a role claim of any name can be read from it.
	var raw json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode user info: %w", err)
	}
	var claims struct {
		Subject           string `json:"sub"`
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, fmt.Errorf("decode user info: %w", err)
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, errors.New("provider returned no subject")
	}
	name := claims.Name
	if name == "" {
		name = claims.PreferredUsername
	}
	if name == "" {
		name = claims.Email
	}
	// The subject is the stable identifier. Email addresses get reassigned
	// between people; subjects do not.
	identity := &Identity{Subject: p.issuer + "|" + claims.Subject, Email: claims.Email, DisplayName: name}
	if p.roleClaim != "" {
		var userInfo map[string]any
		_ = json.Unmarshal(raw, &userInfo)
		idToken, _ := token.Extra("id_token").(string)
		identity.Role = RoleFromClaims(userInfo, IDTokenClaims(idToken), p.roleClaim, p.adminGroup)
		identity.RoleFromClaim = true
	}
	return identity, nil
}
