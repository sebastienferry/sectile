package auth

import (
	"encoding/base64"
	"testing"
)

func TestModeFollowsTheIssuer(t *testing.T) {
	t.Setenv("SECTILE_OIDC_ISSUER", "")
	if Mode() != ModeLocal {
		t.Errorf("Mode() without an issuer = %q, want local", Mode())
	}
	t.Setenv("SECTILE_OIDC_ISSUER", "https://issuer.example")
	if Mode() != ModeOIDC {
		t.Errorf("Mode() with an issuer = %q, want oidc", Mode())
	}
}

// Half a role configuration would silently make everyone a member; it is
// refused instead.
func TestRoleClaimConfigRequiresBothValues(t *testing.T) {
	t.Setenv("SECTILE_OIDC_ROLE_CLAIM", "groups")
	t.Setenv("SECTILE_OIDC_ADMIN_GROUP", "")
	if _, _, err := RoleClaimConfig(); err == nil {
		t.Error("claim without admin group accepted")
	}
	t.Setenv("SECTILE_OIDC_ROLE_CLAIM", "")
	t.Setenv("SECTILE_OIDC_ADMIN_GROUP", "sectile-admins")
	if _, _, err := RoleClaimConfig(); err == nil {
		t.Error("admin group without claim accepted")
	}
	t.Setenv("SECTILE_OIDC_ROLE_CLAIM", "groups")
	claim, group, err := RoleClaimConfig()
	if err != nil || claim != "groups" || group != "sectile-admins" {
		t.Errorf("RoleClaimConfig = %q, %q, %v", claim, group, err)
	}
}

func TestRoleFromClaims(t *testing.T) {
	const claim, admins = "https://sectile.example/roles", "sectile-admins"
	for _, testCase := range []struct {
		name     string
		userInfo map[string]any
		idToken  map[string]any
		want     string
	}{
		{"array with the group", map[string]any{claim: []any{"dev", admins}}, nil, RoleAdmin},
		{"array without the group", map[string]any{claim: []any{"dev"}}, nil, RoleMember},
		{"single string", map[string]any{claim: admins}, nil, RoleAdmin},
		{"absent everywhere", map[string]any{"email": "x"}, map[string]any{}, RoleMember},
		{"only in the id token", map[string]any{}, map[string]any{claim: []any{admins}}, RoleAdmin},
		{"user info wins over the id token", map[string]any{claim: []any{"dev"}}, map[string]any{claim: []any{admins}}, RoleMember},
		{"unrelated value type", map[string]any{claim: 42}, nil, RoleMember},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := RoleFromClaims(testCase.userInfo, testCase.idToken, claim, admins); got != testCase.want {
				t.Errorf("RoleFromClaims = %q, want %q", got, testCase.want)
			}
		})
	}
	if got := RoleFromClaims(map[string]any{claim: admins}, nil, "", ""); got != RoleMember {
		t.Errorf("no configured claim resolved %q, want member", got)
	}
}

func TestIDTokenClaimsDecodesThePayloadOnly(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"1","groups":["a"]}`))
	claims := IDTokenClaims("eyJhbGciOiJSUzI1NiJ9." + payload + ".signature")
	if claims["sub"] != "1" {
		t.Fatalf("claims = %v", claims)
	}
	if IDTokenClaims("not.a.jwt.at.all") != nil || IDTokenClaims("") != nil || IDTokenClaims("a.!!!.c") != nil {
		t.Error("malformed tokens yielded claims")
	}
}
