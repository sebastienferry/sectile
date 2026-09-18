package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"
)

// The two Sectile roles, spelled here so the auth package does not import the
// storage layer for two strings.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Sign-in modes. The mode is a property of the deployment, not of a request:
// a configured provider is the only authority, and without one the local
// e-mail sign-in is what identifies people.
const (
	ModeOIDC  = "oidc"
	ModeLocal = "local"
)

// Mode is the deployment's sign-in mode.
func Mode() string {
	if Configured() {
		return ModeOIDC
	}
	return ModeLocal
}

// RoleClaimConfig reads which claim carries the user's groups or roles and
// which value grants admin. Naming one without the other is a configuration
// error rather than a silent "everyone is a member".
func RoleClaimConfig() (claim string, adminGroup string, err error) {
	claim = strings.TrimSpace(os.Getenv("SECTILE_OIDC_ROLE_CLAIM"))
	adminGroup = strings.TrimSpace(os.Getenv("SECTILE_OIDC_ADMIN_GROUP"))
	if (claim == "") != (adminGroup == "") {
		return "", "", errors.New("SECTILE_OIDC_ROLE_CLAIM and SECTILE_OIDC_ADMIN_GROUP must be set together")
	}
	return claim, adminGroup, nil
}

// RoleFromClaims resolves the role from the provider's claims: admin when the
// named claim, read from UserInfo first and from the ID token otherwise, lists
// the admin group; member in every other case, an absent claim included. The
// claim may be a single string or a list of strings, which is how Okta and
// Auth0 respectively tend to shape it.
func RoleFromClaims(userInfo, idToken map[string]any, claim, adminGroup string) string {
	if claim == "" || adminGroup == "" {
		return RoleMember
	}
	value, found := userInfo[claim]
	if !found {
		value, found = idToken[claim]
	}
	if !found {
		return RoleMember
	}
	for _, group := range claimValues(value) {
		if group == adminGroup {
			return RoleAdmin
		}
	}
	return RoleMember
}

// claimValues flattens a claim to the strings it names.
func claimValues(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{strings.TrimSpace(v)}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		return v
	}
	return nil
}

// IDTokenClaims decodes the payload of an ID token without verifying its
// signature. The token came straight from the token endpoint over TLS, the
// same channel ADR 0008 trusts for UserInfo, so the payload is the provider's
// word; it is only consulted for a claim UserInfo did not carry. Anything that
// is not a three-part JWT yields no claims.
func IDTokenClaims(token string) map[string]any {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return claims
}
