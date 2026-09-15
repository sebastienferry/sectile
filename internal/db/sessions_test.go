package db

import (
	"errors"
	"testing"
)

func TestWebSessionResolvesToItsUser(t *testing.T) {
	database := identityDB(t)
	userID, err := database.UpsertUser("https://issuer.example|alice", "alice@example.com", "Alice")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	token, _, err := database.CreateWebSession(userID)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if got := database.UserForWebSession(token); got != userID {
		t.Fatalf("session resolved to %q, want %q", got, userID)
	}
}

func TestSignOutEndsTheSessionOnTheServer(t *testing.T) {
	database := identityDB(t)
	userID, _ := database.UpsertUser("https://issuer.example|bob", "", "")
	token, _, _ := database.CreateWebSession(userID)

	if err := database.RevokeWebSession(token); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if got := database.UserForWebSession(token); got != "" {
		t.Fatalf("revoked session resolved to %q, want empty", got)
	}
}

func TestSessionCookiesAreNotStoredInPlaintext(t *testing.T) {
	database := identityDB(t)
	userID, _ := database.UpsertUser("https://issuer.example|carol", "", "")
	token, _, _ := database.CreateWebSession(userID)

	var count int
	if err := database.conn.QueryRow(`SELECT COUNT(*) FROM web_sessions WHERE token_hash = ?`, token).Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 0 {
		t.Fatal("session cookie is stored in plaintext")
	}
}

func TestUnknownSessionResolvesToNobody(t *testing.T) {
	database := identityDB(t)
	for _, token := range []string{"", "   ", "not-a-session"} {
		if got := database.UserForWebSession(token); got != "" {
			t.Fatalf("token %q resolved to %q, want empty", token, got)
		}
	}
}

func TestLoginFlowReturnsWhatTheCallbackNeeds(t *testing.T) {
	database := identityDB(t)
	state, err := database.StartLoginFlow(LoginFlow{Nonce: "n", CodeVerifier: "v", Redirect: "/board"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	flow, err := database.ConsumeLoginFlow(state)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if flow.Nonce != "n" || flow.CodeVerifier != "v" || flow.Redirect != "/board" {
		t.Fatalf("flow round-tripped as %+v", flow)
	}
}

// A replayed callback must not open a second session.
func TestLoginFlowIsSingleUse(t *testing.T) {
	database := identityDB(t)
	state, _ := database.StartLoginFlow(LoginFlow{Nonce: "n", CodeVerifier: "v"})
	if _, err := database.ConsumeLoginFlow(state); err != nil {
		t.Fatalf("first consume: %v", err)
	}
	if _, err := database.ConsumeLoginFlow(state); !errors.Is(err, ErrLoginFlow) {
		t.Fatalf("replayed callback returned %v, want ErrLoginFlow", err)
	}
}

func TestUnknownStateIsRejected(t *testing.T) {
	database := identityDB(t)
	if _, err := database.ConsumeLoginFlow("forged-state"); !errors.Is(err, ErrLoginFlow) {
		t.Fatalf("forged state returned %v, want ErrLoginFlow", err)
	}
}

func TestLoginFlowDefaultsToTheInterfaceRoot(t *testing.T) {
	database := identityDB(t)
	state, _ := database.StartLoginFlow(LoginFlow{Nonce: "n", CodeVerifier: "v"})
	flow, err := database.ConsumeLoginFlow(state)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if flow.Redirect != "/" {
		t.Fatalf("redirect defaulted to %q, want /", flow.Redirect)
	}
}
