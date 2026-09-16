package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

func TestConfiguredFollowsTheIssuer(t *testing.T) {
	t.Setenv("SECTILE_OIDC_ISSUER", "")
	if Configured() {
		t.Error("reported configured without an issuer")
	}
	t.Setenv("SECTILE_OIDC_ISSUER", "https://issuer.example")
	if !Configured() {
		t.Error("reported unconfigured with an issuer set")
	}
}

// An issuer reached over plain HTTP would let the network decide who the user
// is, so it is refused before any request is made.
func TestDiscoverRefusesAnInsecureOrIncompleteConfiguration(t *testing.T) {
	for _, testCase := range []struct {
		name                             string
		issuer, clientID, redirect, want string
	}{
		{"plain http issuer", "http://issuer.example", "id", "https://app.example/auth/callback", "HTTPS"},
		{"missing issuer", "", "id", "https://app.example/auth/callback", "required"},
		{"missing client id", "https://issuer.example", "", "https://app.example/auth/callback", "required"},
		{"missing redirect", "https://issuer.example", "id", "", "required"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("SECTILE_OIDC_ISSUER", testCase.issuer)
			t.Setenv("SECTILE_OIDC_CLIENT_ID", testCase.clientID)
			t.Setenv("SECTILE_OIDC_REDIRECT_URL", testCase.redirect)
			_, err := Discover(context.Background())
			if err == nil {
				t.Fatal("accepted the configuration")
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("error %q does not mention %q", err, testCase.want)
			}
		})
	}
}

// PKCE only binds the code to this sign-in if the challenge is the hash of the
// verifier, not the verifier itself.
func TestVerifierChallengeIsTheHashOfTheVerifier(t *testing.T) {
	verifier, challenge, err := NewVerifier()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(verifier))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); challenge != want {
		t.Fatalf("challenge %q, want %q", challenge, want)
	}
	if verifier == challenge {
		t.Fatal("challenge equals the verifier, which defeats PKCE")
	}
	if len(verifier) < 43 {
		t.Fatalf("verifier is %d characters, below the 43 minimum", len(verifier))
	}
}

func TestVerifiersAndNoncesDoNotRepeat(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		verifier, _, err := NewVerifier()
		if err != nil {
			t.Fatal(err)
		}
		nonce, err := NewNonce()
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range []string{verifier, nonce} {
			if seen[value] {
				t.Fatalf("value repeated: %q", value)
			}
			seen[value] = true
		}
	}
}

func TestAuthCodeURLCarriesStateNonceAndChallenge(t *testing.T) {
	provider := &Provider{
		config: oauth2.Config{
			ClientID:    "client",
			RedirectURL: "https://app.example/auth/callback",
			Scopes:      []string{"openid", "profile", "email"},
			Endpoint:    oauth2.Endpoint{AuthURL: "https://issuer.example/authorize"},
		},
		issuer: "https://issuer.example",
	}
	raw := provider.AuthCodeURL("the-state", "the-nonce", "the-challenge")
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	for field, want := range map[string]string{
		"state":                 "the-state",
		"nonce":                 "the-nonce",
		"code_challenge":        "the-challenge",
		"code_challenge_method": "S256",
		"response_type":         "code",
		"client_id":             "client",
	} {
		if got := query.Get(field); got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
	if !strings.Contains(query.Get("scope"), "openid") {
		t.Errorf("scope %q does not request openid", query.Get("scope"))
	}
}
