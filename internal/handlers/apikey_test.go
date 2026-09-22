package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tasks/internal/db"
)

// expireKey backdates a key so the next lookup finds it run out.
func expireKey(t *testing.T, h *Handler, key string) {
	t.Helper()
	credential, err := h.db.LookupDeviceToken(key)
	if err != nil {
		t.Fatalf("lookup before expiry: %v", err)
	}
	if _, err := h.db.RenewDeviceCredential(credential.UserID, credential.ID, time.Nanosecond); err != nil {
		t.Fatalf("expire key: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
}

func TestAgentAPIAuthAcceptsAKeyAndNamesItsExpiry(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	t.Setenv("SECTILE_SERVER_TOKEN", "shared")
	userID, _ := database.UpsertUser("okta|ivan", "ivan@example.com", "Ivan")
	key, _, err := database.CreateAPIKey(userID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	handler := h.AgentAPIAuth(http.HandlerFunc(h.HandleAgentIdentity))
	call := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/identity", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	rr := call(key)
	if rr.Code != http.StatusOK {
		t.Fatalf("valid key refused: %d %s", rr.Code, rr.Body.String())
	}
	var identity struct {
		UserID      string     `json:"userId"`
		Label       string     `json:"label"`
		ExpiresAt   *time.Time `json:"expiresAt"`
		SharedToken bool       `json:"sharedToken"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &identity); err != nil {
		t.Fatal(err)
	}
	if identity.UserID != userID || identity.Label != "laptop" || identity.ExpiresAt == nil || identity.SharedToken {
		t.Fatalf("identity = %+v", identity)
	}

	// The deprecated shared token still resolves, to the implicit user.
	rr = call("shared")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"sharedToken":true`) {
		t.Fatalf("shared token: %d %s", rr.Code, rr.Body.String())
	}

	if rr = call("sectile_unknown"); rr.Code != http.StatusUnauthorized || strings.Contains(rr.Body.String(), "expired") {
		t.Fatalf("unknown key: %d %s", rr.Code, rr.Body.String())
	}
	if rr = call(""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("missing header: %d", rr.Code)
	}

	expireKey(t, h, key)
	rr = call(key)
	if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), "API key expired") {
		t.Fatalf("expired key: %d %s", rr.Code, rr.Body.String())
	}
}

func TestAgentHandshakeRefusesAnExpiredKeyByName(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	userID, _ := database.UpsertUser("okta|judy", "", "")
	key, _, err := database.CreateAPIKey(userID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	expireKey(t, h, key)
	req := httptest.NewRequest(http.MethodGet, "/ws/agent-connect?token="+key, nil)
	rr := httptest.NewRecorder()
	h.HandleAgentConnect(rr, req)
	if rr.Code != http.StatusUnauthorized || !strings.Contains(rr.Body.String(), "API key expired") {
		t.Fatalf("handshake with expired key: %d %s", rr.Code, rr.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/ws/agent-connect?token=sectile_never", nil)
	rr = httptest.NewRecorder()
	t.Setenv("SECTILE_SERVER_TOKEN", "pinned")
	h.HandleAgentConnect(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("handshake with unknown key: %d %s", rr.Code, rr.Body.String())
	}
}

func TestProfileCreatesRenewsAndRevokesKeys(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	// The workstation routes act on the signed-in account, and signing in is
	// mandatory, so the calls carry a session.
	user, err := database.SignInLocal("ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := database.CreateWebSession(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, target, body string) *httptest.ResponseRecorder {
		var reader *strings.Reader
		if body != "" {
			reader = strings.NewReader(body)
		} else {
			reader = strings.NewReader("")
		}
		req := httptest.NewRequest(method, target, reader)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		rr := httptest.NewRecorder()
		h.HandleDeviceCredentials(rr, req)
		return rr
	}

	rr := call(http.MethodPost, "/api/devices", `{"label":"laptop"}`)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rr.Code, rr.Body.String())
	}
	var created struct {
		Token  string `json:"token"`
		Device struct {
			ID        string
			Label     string
			ExpiresAt *time.Time
		} `json:"device"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Token, db.APIKeyPrefix) || created.Device.ExpiresAt == nil {
		t.Fatalf("created = %+v", created)
	}
	if h.resolveAgentUser(created.Token) != user.ID {
		t.Fatal("the created key does not authenticate")
	}

	// A key without expiry is an explicit choice.
	rr = call(http.MethodPost, "/api/devices", `{"label":"server","ttlDays":0}`)
	if rr.Code != http.StatusCreated || !strings.Contains(rr.Body.String(), `"ExpiresAt":null`) {
		t.Fatalf("create without expiry: %d %s", rr.Code, rr.Body.String())
	}

	// The listing never carries the secret.
	rr = call(http.MethodGet, "/api/devices", "")
	if rr.Code != http.StatusOK || strings.Contains(rr.Body.String(), created.Token) || !strings.Contains(rr.Body.String(), "laptop") {
		t.Fatalf("list: %d %s", rr.Code, rr.Body.String())
	}

	// Renewal moves the expiry and keeps the secret working.
	rr = call(http.MethodPut, "/api/devices?id="+created.Device.ID, `{"ttlDays":30}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("renew: %d %s", rr.Code, rr.Body.String())
	}
	var renewed struct {
		ExpiresAt *time.Time `json:"expiresAt"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &renewed)
	if renewed.ExpiresAt == nil || time.Until(*renewed.ExpiresAt) > 31*24*time.Hour || time.Until(*renewed.ExpiresAt) < 29*24*time.Hour {
		t.Fatalf("renewed expiry = %v", renewed.ExpiresAt)
	}
	if h.resolveAgentUser(created.Token) != user.ID {
		t.Fatal("renewal broke the key")
	}
	if rr = call(http.MethodPut, "/api/devices?id=dev_missing", `{}`); rr.Code != http.StatusNotFound {
		t.Fatalf("renew unknown: %d", rr.Code)
	}

	// Revocation cuts the key.
	if rr = call(http.MethodDelete, "/api/devices?id="+created.Device.ID, ""); rr.Code != http.StatusOK {
		t.Fatalf("revoke: %d %s", rr.Code, rr.Body.String())
	}
	if h.resolveAgentUser(created.Token) != "" {
		t.Fatal("a revoked key still authenticates")
	}
}

// agentTestKey issues a real workstation key for the implicit user. A machine
// surface no longer accepts an invented bearer, so every test that reaches one
// needs a key the deployment actually issued.
func agentTestKey(t *testing.T, h *Handler) string {
	t.Helper()
	if err := h.db.EnsureUser(ImplicitUser); err != nil {
		t.Fatalf("seed the implicit user: %v", err)
	}
	key, _, err := h.db.CreateAPIKey(ImplicitUser, "test-workstation", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatalf("issue a workstation key: %v", err)
	}
	return key
}

// The legacy open mode is gone. A deployment that holds no key and pins no
// shared token authenticates nobody, whatever bearer is presented: signing in
// is mandatory (ADR 0015) and the machine surfaces are not an exception.
func TestOpenModeIsGoneOnADeploymentWithoutKeys(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	t.Setenv("SECTILE_SERVER_TOKEN", "")
	if h.resolveAgentUser("anything") != "" {
		t.Fatal("an invented bearer authenticated on a deployment without keys")
	}
	key, _, err := database.CreateAPIKey(ImplicitUser, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	if h.resolveAgentUser("anything") != "" {
		t.Fatal("an invented bearer authenticated once a key existed")
	}
	if h.resolveAgentUser(key) != ImplicitUser {
		t.Fatal("the key itself does not authenticate")
	}
	// The deprecated shared token, when configured, still opens the door: it is
	// the one upgrade path that survives, and it is a pinned value rather than
	// an open door.
	t.Setenv("SECTILE_SERVER_TOKEN", "shared")
	if h.resolveAgentUser("shared") != ImplicitUser || h.resolveAgentUser("anything") != "" {
		t.Fatal("shared token handling changed with keys present")
	}
}

// The key store losing its last key must decide nothing. Revocation only marks
// a row today, but deleting the account that holds a key removes it outright,
// and the door used to be armed by a live count of the rows: a deployment that
// had long since left the open mode behind would have fallen back into it. The
// resolver now consults no count at all, so neither shape of loss reopens it.
func TestALastKeyGoingAwayDoesNotReopenTheDoor(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	t.Setenv("SECTILE_SERVER_TOKEN", "")
	user, err := database.SignInLocal("ada@example.com")
	if err != nil {
		t.Fatal(err)
	}
	key, credential, err := database.CreateAPIKey(user.ID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RevokeDeviceCredential(user.ID, credential.ID); err != nil {
		t.Fatal(err)
	}
	if h.resolveAgentUser(key) != "" {
		t.Fatal("a revoked key still authenticates")
	}
	if h.resolveAgentUser("anything") != "" {
		t.Fatal("a deployment whose only key is gone reopened the legacy door")
	}
}

func TestCurrentUserReportsTheSharedTokenDeprecation(t *testing.T) {
	h, _, cleanup := setupTestHandler(t)
	defer cleanup()
	read := func() bool {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		rr := httptest.NewRecorder()
		h.HandleCurrentUser(rr, req)
		var body struct {
			SharedServerToken bool `json:"sharedServerToken"`
		}
		_ = json.Unmarshal(rr.Body.Bytes(), &body)
		return body.SharedServerToken
	}
	t.Setenv("SECTILE_SERVER_TOKEN", "")
	if read() {
		t.Fatal("deprecation reported without the variable")
	}
	t.Setenv("SECTILE_SERVER_TOKEN", "legacy")
	if !read() {
		t.Fatal("deprecation not reported with the variable set")
	}
}
