package handlers

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

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"tasks/internal/auth"
	"tasks/internal/db"
	"tasks/internal/models"
)

// guardedServer serves the routes the authorization rules touch behind the
// same guard main.go installs, so the tests exercise the real chain.
func guardedServer(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "tasks"})
	})
	mux.HandleFunc("/api/tasks/", h.HandleTaskDetail)
	mux.HandleFunc("/api/setup/tracker", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "tracker"})
	})
	mux.HandleFunc("/api/projects", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "projects"})
	})
	mux.HandleFunc("/api/projects/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"ok": "project"})
	})
	mux.HandleFunc("/api/settings", h.HandleSettings)
	mux.HandleFunc("/api/users", h.HandleUsers)
	mux.HandleFunc("/api/users/", h.HandleUsers)
	mux.HandleFunc(AdminStatsPath, h.HandleAdminStats)
	mux.HandleFunc("/api/devices", h.HandleDeviceCredentials)
	mux.HandleFunc("/api/pairing-codes", h.HandlePairingCode)
	mux.HandleFunc("/api/me", h.HandleCurrentUser)
	mux.HandleFunc("/auth/local", h.HandleLocalSignIn)
	mux.HandleFunc("/auth/login", h.HandleLogin)
	mux.HandleFunc("/api/agent/dispatch", h.HandleAgentDispatch)
	mux.HandleFunc("/ws/agent-connect", h.HandleAgentConnect)
	server := httptest.NewServer(h.EnableCORS(h.RequireSession(mux)))
	t.Cleanup(server.Close)
	return server
}

// account signs a local user in and returns their id and session cookie.
func account(t *testing.T, database *db.DB, email string) (string, *http.Cookie) {
	t.Helper()
	user, err := database.SignInLocal(email)
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := database.CreateWebSession(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	return user.ID, &http.Cookie{Name: sessionCookie, Value: token}
}

func call(t *testing.T, server *httptest.Server, cookie *http.Cookie, method, path, body string) (int, string) {
	t.Helper()
	request, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	text, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(text)
}

// connectAgentAs opens a fake agent for the user the key names.
func connectAgentAs(t *testing.T, server *httptest.Server, key, projectID string) *websocket.Conn {
	t.Helper()
	u, _ := url.Parse(server.URL)
	u.Scheme = "ws"
	u.Path = "/ws/agent-connect"
	u.RawQuery = "token=" + url.QueryEscape(key) + "&deviceId=laptop&projectId=" + projectID
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("agent dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// The table is what the guard enforces; a route wrongly listed locks members
// out of their work, a route wrongly omitted hands members the deployment.
func TestAdminOnlyRoutesAreExactlyTheseMutations(t *testing.T) {
	adminOnly := []struct{ method, path string }{
		{http.MethodGet, "/api/users"}, {http.MethodPut, "/api/users/u1"}, {http.MethodDelete, "/api/users/u1"},
		{http.MethodGet, "/api/admin/stats"},
	}
	for _, route := range adminOnly {
		if !adminOnlyRoute(route.method, route.path) {
			t.Errorf("%s %s is not admin-only", route.method, route.path)
		}
	}
	open := []struct{ method, path string }{
		// The board is the members' workspace: opening a project and pointing it
		// at its tracker are theirs, or the board waits on one person.
		{http.MethodPost, "/api/setup/tracker"}, {http.MethodPost, "/api/setup/tracker/check"},
		{http.MethodPost, "/api/projects"},
		{http.MethodPut, "/api/projects/p1"}, {http.MethodPatch, "/api/projects/p1"}, {http.MethodDelete, "/api/projects/p1"},
		{http.MethodGet, "/api/projects"}, {http.MethodGet, "/api/projects/p1"},
		{http.MethodPost, "/api/tasks"}, {http.MethodPut, "/api/tasks/t1"}, {http.MethodPost, "/api/tasks/t1/run-skill"},
		{http.MethodPost, "/api/tasks/t1/comment"}, {http.MethodPost, "/api/tasks/stage"},
		{http.MethodGet, "/api/settings"}, {http.MethodPost, "/api/settings"},
		{http.MethodPost, "/api/agent/dispatch"}, {http.MethodGet, "/api/devices"}, {http.MethodPost, "/api/pairing-codes"},
	}
	for _, route := range open {
		if adminOnlyRoute(route.method, route.path) {
			t.Errorf("%s %s is admin-only, members need it", route.method, route.path)
		}
	}
}

// Anonymous is told to sign in, a member is told the action is an admin's, and
// the two answers differ so the interface can act on them.
func TestGuardTellsAnonymousFromMember(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)

	// Nothing is reachable before a sign-in, on a deployment without accounts
	// as on any other (ADR 0015).
	if status, body := call(t, server, nil, http.MethodPost, "/api/setup/tracker", `{}`); status != http.StatusUnauthorized || !strings.Contains(body, msgSignIn) {
		t.Fatalf("an empty deployment served an anonymous admin action: %d %s", status, body)
	}

	_, alice := account(t, database, "alice@example.com")
	_, bob := account(t, database, "bob@example.com")

	if status, body := call(t, server, nil, http.MethodGet, "/api/tasks", ""); status != http.StatusUnauthorized || !strings.Contains(body, msgSignIn) {
		t.Fatalf("anonymous after the first account: %d %s", status, body)
	}
	if status, body := call(t, server, nil, http.MethodGet, "/api/me", ""); status != http.StatusOK || !strings.Contains(body, `"mode":"local"`) || !strings.Contains(body, `"signedIn":false`) {
		t.Fatalf("/api/me stays public and names the mode: %d %s", status, body)
	}
	// The roster is what stays an admin's, whichever way it is touched.
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/users"}, {http.MethodPut, "/api/users/u1"}, {http.MethodDelete, "/api/users/u1"},
	} {
		if status, body := call(t, server, bob, route.method, route.path, `{}`); status != http.StatusForbidden || !strings.Contains(body, msgAdminOnly) {
			t.Errorf("member on %s %s: %d %s, want 403 naming the role", route.method, route.path, status, body)
		}
	}
	if status, _ := call(t, server, alice, http.MethodGet, "/api/users", ""); status != http.StatusOK {
		t.Errorf("admin cannot read the roster")
	}
	// The board itself is not on that list: a member opens a project and
	// configures the tracker it reads from.
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/api/setup/tracker"}, {http.MethodPost, "/api/projects"}, {http.MethodPut, "/api/projects/p1"},
	} {
		if status, body := call(t, server, bob, route.method, route.path, `{}`); status != http.StatusOK {
			t.Errorf("member on %s %s: %d %s, want 200", route.method, route.path, status, body)
		}
	}
	if status, _ := call(t, server, bob, http.MethodGet, "/api/tasks", ""); status != http.StatusOK {
		t.Fatalf("member cannot read tasks: %d", status)
	}
}

func TestLocalSignInBootstrapsTheFirstAdmin(t *testing.T) {
	h, _, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)

	// The interface's one sign-in URL lands on the local screen without a provider.
	if status, _ := call(t, server, nil, http.MethodGet, "/auth/login?redirect=/board", ""); status != http.StatusFound {
		t.Fatalf("/auth/login without a provider: %d, want a redirect to /signin", status)
	}

	status, body := call(t, server, nil, http.MethodPost, "/auth/local", `{"email":" Alice@Example.com "}`)
	if status != http.StatusOK || !strings.Contains(body, `"role":"admin"`) || !strings.Contains(body, `"email":"alice@example.com"`) {
		t.Fatalf("first local sign-in: %d %s", status, body)
	}
	status, body = call(t, server, nil, http.MethodPost, "/auth/local", `{"email":"bob@example.com"}`)
	if status != http.StatusOK || !strings.Contains(body, `"role":"member"`) {
		t.Fatalf("second local sign-in: %d %s", status, body)
	}
	if status, body = call(t, server, nil, http.MethodPost, "/auth/local", `{"email":"nobody"}`); status != http.StatusBadRequest {
		t.Fatalf("bad address: %d %s", status, body)
	}

	// With a provider the local screen does not exist.
	h.SetIdentityProvider(&auth.Provider{})
	if status, _ = call(t, server, nil, http.MethodPost, "/auth/local", `{"email":"eve@example.com"}`); status != http.StatusNotFound {
		t.Fatalf("local sign-in with a provider: %d, want 404", status)
	}
}

// The settings row is split in two (ADR 0015): the preferences are personal
// and each account keeps its own, the deployment's configuration is shared and
// a member touching it is refused by name.
func TestMembersOnlyChangeTheirPreferencesInSettings(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)
	aliceID, alice := account(t, database, "alice@example.com")
	bobID, bob := account(t, database, "bob@example.com")

	current, _ := database.GetSettings()
	current.Theme = "light"
	payload, _ := json.Marshal(current)
	if status, body := call(t, server, bob, http.MethodPost, "/api/settings", string(payload)); status != http.StatusOK {
		t.Fatalf("member changing the theme: %d %s", status, body)
	}
	current.AIProvider = "codex"
	current.AutoSyncEnabled = !current.AutoSyncEnabled
	payload, _ = json.Marshal(current)
	status, body := call(t, server, bob, http.MethodPost, "/api/settings", string(payload))
	if status != http.StatusForbidden || !strings.Contains(body, "aiProvider") || !strings.Contains(body, "autoSyncEnabled") {
		t.Fatalf("member changing the provider: %d %s", status, body)
	}
	if status, body = call(t, server, alice, http.MethodPost, "/api/settings", string(payload)); status != http.StatusOK {
		t.Fatalf("admin changing the provider: %d %s", status, body)
	}
	saved, _ := database.GetSettings()
	if saved.AIProvider != "codex" {
		t.Fatalf("the deployment provider = %q, want codex", saved.AIProvider)
	}

	// The theme each of them saved is their own, and the deployment row is not
	// where it landed.
	bobSettings, _ := database.UserSettings(bobID)
	aliceSettings, _ := database.UserSettings(aliceID)
	if bobSettings.Theme != "light" {
		t.Fatalf("bob's theme = %q, want light", bobSettings.Theme)
	}
	if status, body = call(t, server, alice, http.MethodPost, "/api/settings", `{"theme":"dark","density":"compact"}`); status != http.StatusOK {
		t.Fatalf("admin changing her theme: %d %s", status, body)
	}
	aliceSettings, _ = database.UserSettings(aliceID)
	bobSettings, _ = database.UserSettings(bobID)
	if aliceSettings.Theme != "dark" || aliceSettings.Density != "compact" {
		t.Fatalf("alice's preferences = %+v", aliceSettings)
	}
	if bobSettings.Theme != "light" {
		t.Fatalf("alice's save reached bob: theme %q", bobSettings.Theme)
	}

	// Omitting a key is not a way around the rule: a partial payload from a
	// member leaves every admin-only value as it was, booleans included.
	before, _ := database.GetSettings()
	if status, body = call(t, server, bob, http.MethodPost, "/api/settings", `{"theme":"system"}`); status != http.StatusOK {
		t.Fatalf("member posting a partial payload: %d %s", status, body)
	}
	after, _ := database.GetSettings()
	bobSettings, _ = database.UserSettings(bobID)
	if bobSettings.Theme != "system" {
		t.Fatalf("member's own preference was dropped: theme %q", bobSettings.Theme)
	}
	if after.AIProvider != before.AIProvider || after.AutoSyncEnabled != before.AutoSyncEnabled || after.AutoSyncIntervalSec != before.AutoSyncIntervalSec {
		t.Fatalf("an omitted admin setting changed: provider %q->%q, autoSync %v->%v every %d->%d",
			before.AIProvider, after.AIProvider, before.AutoSyncEnabled, after.AutoSyncEnabled, before.AutoSyncIntervalSec, after.AutoSyncIntervalSec)
	}

	// userEmail is the account's address and is not writable.
	if status, body = call(t, server, bob, http.MethodPost, "/api/settings", `{"userEmail":"someone@else.test"}`); status != http.StatusOK {
		t.Fatalf("member posting an e-mail: %d %s", status, body)
	}
	status, body = call(t, server, bob, http.MethodGet, "/api/settings", "")
	if status != http.StatusOK || !strings.Contains(body, `"userEmail":"bob@example.com"`) {
		t.Fatalf("settings read back = %d %s, want bob's own address", status, body)
	}

	// userName is the account's name and is not writable either. The post is
	// ignored rather than refused, so a whole-row payload from the interface
	// still answers 200, and the preferences it carries alongside are saved.
	if _, err := database.SetDisplayName(bobID, "Bob Martin"); err != nil {
		t.Fatal(err)
	}
	if status, body = call(t, server, bob, http.MethodPost, "/api/settings", `{"userName":"someone else","density":"compact"}`); status != http.StatusOK {
		t.Fatalf("member posting a name: %d %s", status, body)
	}
	status, body = call(t, server, bob, http.MethodGet, "/api/settings", "")
	if status != http.StatusOK || !strings.Contains(body, `"userName":"Bob Martin"`) {
		t.Fatalf("settings read back = %d %s, want bob's own name", status, body)
	}
	if strings.Contains(body, "someone else") {
		t.Fatalf("a written userName reached the answer: %s", body)
	}
	bobSettings, _ = database.UserSettings(bobID)
	if bobSettings.Density != "compact" {
		t.Fatalf("ignoring userName dropped the rest of the payload: density %q", bobSettings.Density)
	}
}

func TestUsersViewChangesRolesButNeverRemovesTheLastAdmin(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)
	aliceID, alice := account(t, database, "alice@example.com")
	bobID, bob := account(t, database, "bob@example.com")

	if status, _ := call(t, server, bob, http.MethodGet, "/api/users", ""); status != http.StatusForbidden {
		t.Fatalf("member listing users: %d", status)
	}
	status, body := call(t, server, alice, http.MethodGet, "/api/users", "")
	if status != http.StatusOK || !strings.Contains(body, "bob@example.com") || !strings.Contains(body, `"rolesFromProvider":false`) {
		t.Fatalf("admin listing users: %d %s", status, body)
	}
	if status, body = call(t, server, alice, http.MethodPut, "/api/users/"+aliceID, `{"role":"member"}`); status != http.StatusConflict {
		t.Fatalf("demoting the last admin: %d %s", status, body)
	}
	if status, body = call(t, server, alice, http.MethodPut, "/api/users/"+bobID, `{"role":"admin"}`); status != http.StatusOK || !strings.Contains(body, `"role":"admin"`) {
		t.Fatalf("promoting bob: %d %s", status, body)
	}
	// Bob is an admin now and may act as one on his next request.
	if status, _ = call(t, server, bob, http.MethodGet, "/api/users", ""); status != http.StatusOK {
		t.Fatalf("promoted member still refused: %d", status)
	}
	if status, _ = call(t, server, alice, http.MethodPut, "/api/users/"+aliceID, `{"role":"member"}`); status != http.StatusOK {
		t.Fatalf("demotion with another admin present: %d", status)
	}
	// Bob is the admin now; alice was demoted just above.
	if status, _ = call(t, server, bob, http.MethodPut, "/api/users/"+aliceID, `{"role":"owner"}`); status != http.StatusBadRequest {
		t.Fatalf("unknown role: %d", status)
	}
	// An admin may see another user's workstations; a member may not.
	if status, _ = call(t, server, alice, http.MethodGet, "/api/devices?userId="+bobID, ""); status != http.StatusForbidden {
		t.Fatalf("demoted alice listing bob's devices: %d, want 403", status)
	}
	if status, _ = call(t, server, bob, http.MethodGet, "/api/devices?userId="+aliceID, ""); status != http.StatusOK {
		t.Fatalf("admin bob listing alice's devices: %d", status)
	}
	// Seeing and revoking someone's workstations is not minting a credential
	// that acts as them.
	if status, _ = call(t, server, bob, http.MethodPost, "/api/devices?userId="+aliceID, `{"label":"forged"}`); status != http.StatusForbidden {
		t.Fatalf("admin creating a key for someone else: %d, want 403", status)
	}
}

// Everyone sees a colleague's run; only the owner or an admin stops it, and
// the stop reaches the owner's agent rather than the caller's.
func TestStopReachesTheOwnersAgentAndIsRefusedToOthers(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)
	_, alice := account(t, database, "alice@example.com") // admin
	_, bob := account(t, database, "bob@example.com")     // member
	carolID, carol := account(t, database, "carol@example.com")

	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Carol's work", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	run, err := database.StartAgentRun(task.ID, "implement", db.RunLaunch{UserID: carolID})
	if err != nil {
		t.Fatal(err)
	}

	// Bob sees the run with its owner, and cannot stop it.
	status, body := call(t, server, bob, http.MethodGet, "/api/tasks/"+task.ID+"/activities", "")
	if status != http.StatusOK || !strings.Contains(body, `"userName":"carol@example.com"`) {
		t.Fatalf("member reading the run: %d %s", status, body)
	}
	status, body = call(t, server, bob, http.MethodPost, "/api/tasks/"+task.ID+"/cancel-run", `{"runId":"`+run.ID+`"}`)
	if status != http.StatusForbidden || !strings.Contains(body, msgNotOwner) {
		t.Fatalf("member stopping a colleague's run: %d %s", status, body)
	}
	if still, _ := database.GetActivityByID(run.ID); still.Status != "running" {
		t.Fatalf("refused stop changed the run to %q", still.Status)
	}

	// Nobody's agent is connected: the owner's stop cannot be delivered.
	if status, body = call(t, server, carol, http.MethodPost, "/api/tasks/"+task.ID+"/cancel-run", `{"runId":"`+run.ID+`"}`); status != http.StatusBadGateway {
		t.Fatalf("owner without an agent: %d %s", status, body)
	}

	// Carol's agent connects. Alice, an admin, stops the run: the message
	// reaches Carol's agent, whose acknowledgement closes the run.
	key, _, err := database.CreateAPIKey(carolID, "carol laptop", 0)
	if err != nil {
		t.Fatal(err)
	}
	agent := connectAgentAs(t, server, key, "default")
	done := make(chan struct {
		status int
		body   string
	}, 1)
	go func() {
		s, b := call(t, server, alice, http.MethodPost, "/api/tasks/"+task.ID+"/cancel-run", `{"runId":"`+run.ID+`"}`)
		done <- struct {
			status int
			body   string
		}{s, b}
	}()
	_ = agent.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, raw, err := agent.ReadMessage()
	if err != nil {
		t.Fatalf("carol's agent did not receive the stop: %v", err)
	}
	var message AgentMessage
	_ = json.Unmarshal(raw, &message)
	if message.Type != "dispatch_step" || message.TaskID != task.ID || !strings.Contains(string(message.Payload), "cancel_run") {
		t.Fatalf("carol's agent received %+v", message)
	}
	ack, _ := json.Marshal(map[string]string{"status": "completed", "summary": "stopped"})
	if err := agent.WriteJSON(AgentMessage{MsgID: message.MsgID, TaskID: task.ID, Type: "step_status", Payload: ack}); err != nil {
		t.Fatal(err)
	}
	result := <-done
	if result.status != http.StatusOK {
		t.Fatalf("admin stop: %d %s", result.status, result.body)
	}
	if closed, _ := database.GetActivityByID(run.ID); closed.Status != "canceled" {
		t.Fatalf("run after the admin's stop = %q, want canceled", closed.Status)
	}
}

// A member's dispatch goes to their own agent whatever user the body names;
// an admin may address another user's agent.
func TestDispatchIgnoresAForeignUserUnlessAdmin(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)
	_, alice := account(t, database, "alice@example.com") // admin
	bobID, bob := account(t, database, "bob@example.com") // member
	carolID, _ := account(t, database, "carol@example.com")

	carolKey, _, _ := database.CreateAPIKey(carolID, "carol", 0)
	carolAgent := connectAgentAs(t, server, carolKey, "default")
	body := `{"userId":"` + carolID + `","projectId":"default","taskId":"t","action":"dispatch_step","payload":{}}`

	// Bob names Carol's agent but has none of his own: his request is his own
	// to route, so it finds no agent rather than reaching hers.
	if status, text := call(t, server, bob, http.MethodPost, "/api/agent/dispatch", body); status != http.StatusPreconditionRequired {
		t.Fatalf("member naming another agent: %d %s", status, text)
	}

	// With his own agent connected, the same request lands there.
	bobKey, _, _ := database.CreateAPIKey(bobID, "bob", 0)
	bobAgent := connectAgentAs(t, server, bobKey, "default")
	if status, text := call(t, server, bob, http.MethodPost, "/api/agent/dispatch", body); status != http.StatusOK {
		t.Fatalf("member dispatch: %d %s", status, text)
	}
	_ = bobAgent.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := bobAgent.ReadMessage(); err != nil {
		t.Fatalf("bob's own agent did not receive his dispatch: %v", err)
	}

	// An admin reaches Carol's agent by name.
	if status, text := call(t, server, alice, http.MethodPost, "/api/agent/dispatch", body); status != http.StatusOK {
		t.Fatalf("admin dispatch to another agent: %d %s", status, text)
	}
	_ = carolAgent.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := carolAgent.ReadMessage(); err != nil {
		t.Fatalf("carol's agent did not receive the admin's dispatch: %v", err)
	}

	// The admin's is the only message it ever received: the member's two
	// requests never reached her. This read leaves the connection unusable,
	// which is why it is the last thing done with it.
	_ = carolAgent.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, _, err := carolAgent.ReadMessage(); err == nil {
		t.Fatal("carol's agent received a second dispatch, so a member's request reached her")
	}
}

// The agent gateway forwards console calls to /api/ with the workstation key,
// not with a cookie. Those must keep working once accounts exist, and only a
// real key may stand in for a session.
func TestWorkstationKeyAuthenticatesInterfaceCalls(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)
	_, _ = account(t, database, "alice@example.com")
	bobID, _ := account(t, database, "bob@example.com")
	key, _, err := database.CreateAPIKey(bobID, "bob laptop", 0)
	if err != nil {
		t.Fatal(err)
	}

	withBearer := func(token, path string) (int, string) {
		t.Helper()
		request, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = response.Body.Close() }()
		body, _ := io.ReadAll(response.Body)
		return response.StatusCode, string(body)
	}

	if status, body := withBearer(key, "/api/tasks"); status != http.StatusOK {
		t.Fatalf("a console call through the gateway: %d %s", status, body)
	}
	// Bob's key carries Bob's role, so an admin-only route stays closed.
	if status, _ := withBearer(key, "/api/users"); status != http.StatusForbidden {
		t.Fatalf("member's key on an admin route: %d, want 403", status)
	}
	// An invented bearer is not a key, and there is no longer a mode in which it
	// becomes one (ADR 0019): it must not be a way around sign-in.
	if status, _ := withBearer("not-a-real-key", "/api/tasks"); status != http.StatusUnauthorized {
		t.Fatalf("invented bearer: %d, want 401", status)
	}
}

// Creating a workstation key must not mint an account. UpsertUser takes a
// provider subject, so passing it a user id used to add a second, inert row
// that then appeared in the users view with no name and no way to sign in.
func TestCreatingAKeyDoesNotMintAnAccount(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)
	_, alice := account(t, database, "alice@example.com")

	before, err := database.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	if status, body := call(t, server, alice, http.MethodPost, "/api/devices", `{"label":"laptop"}`); status != http.StatusCreated {
		t.Fatalf("create key: %d %s", status, body)
	}
	if status, body := call(t, server, alice, http.MethodPost, "/api/pairing-codes", ""); status != http.StatusCreated {
		t.Fatalf("create pairing code: %d %s", status, body)
	}
	after, err := database.ListUsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		rows := make([]string, 0, len(after))
		for _, user := range after {
			rows = append(rows, user.ID+" subject="+user.Subject)
		}
		t.Fatalf("users went from %d to %d: %s", len(before), len(after), strings.Join(rows, " | "))
	}
}

// A run reported over MCP belongs to the user whose key made the call, and the
// identity endpoint tells that key holder their role.
func TestMachineCallersCarryTheirUser(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	_, _ = database.SignInLocal("alice@example.com")
	bob, err := database.SignInLocal("bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := database.CreateAPIKey(bob.ID, "bob laptop", 0)
	if err != nil {
		t.Fatal(err)
	}
	task, err := database.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "MCP owned", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}

	identity := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/agent/identity", nil)
	request.Header.Set("Authorization", "Bearer "+key)
	h.HandleAgentIdentity(identity, request)
	if identity.Code != http.StatusOK || !strings.Contains(identity.Body.String(), `"role":"member"`) || !strings.Contains(identity.Body.String(), `"mode":"local"`) {
		t.Fatalf("identity: %d %s", identity.Code, identity.Body.String())
	}

	srv := httptest.NewServer(h.MCPHandler())
	defer srv.Close()
	session := mcpSession(t, srv.URL, key)
	defer session.Close()
	result := callTool(t, session, "start_run", map[string]any{"taskKey": task.ID, "skill": "implement"})
	var run models.TaskActivity
	if err := json.Unmarshal([]byte(result), &run); err != nil {
		t.Fatalf("start_run answered %q: %v", result, err)
	}
	stored, _ := database.GetActivityByID(run.ID)
	if stored == nil || stored.UserID != bob.ID || stored.UserName != "bob@example.com" {
		t.Fatalf("run owner = %+v, want bob", stored)
	}
	callTool(t, session, "add_comment", map[string]any{"taskKey": task.ID, "body": "from bob's client"})
	comments, _ := database.GetTaskComments(task.ID)
	if len(comments) != 1 || comments[0].UserID != bob.ID || comments[0].Author != "bob@example.com" {
		t.Fatalf("comment = %+v", comments)
	}
}

// mcpSession connects an MCP client authenticated with the given key.
func mcpSession(t *testing.T, endpoint, key string) *mcp.ClientSession {
	t.Helper()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(context.Background(),
		&mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: &http.Client{Transport: testTokenTransport{key}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return session
}

// callTool calls one tool and returns its structured result as JSON.
func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if result.IsError {
		t.Fatalf("%s: %+v", name, result)
	}
	raw, _ := json.Marshal(result.StructuredContent)
	return string(raw)
}

// Blocking is the measure that has to reach every door at once: the session
// already open, the next sign-in, and the workstation key that runs the agent.
func TestBlockingAnAccountClosesEveryDoor(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)

	_, alice := account(t, database, "alice@example.com")
	bobID, bob := account(t, database, "bob@example.com")

	key, _, err := database.CreateAPIKey(bobID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.resolveAgentCredential(key); err != nil {
		t.Fatalf("the key does not open before the block: %v", err)
	}
	if status, _ := call(t, server, bob, http.MethodGet, "/api/tasks", ""); status != http.StatusOK {
		t.Fatalf("bob cannot read the board before the block")
	}

	status, body := call(t, server, alice, http.MethodPut, "/api/users/"+bobID, `{"blocked":true}`)
	if status != http.StatusOK || !strings.Contains(body, `"blocked":true`) {
		t.Fatalf("block: %d %s", status, body)
	}

	// The session that was open stops working. It answers 401 rather than 403
	// because the block revoked it outright: the request now names nobody, and
	// the interface sends them to the sign-in, which is where the block is
	// spelled out.
	if status, body := call(t, server, bob, http.MethodGet, "/api/tasks", ""); status != http.StatusUnauthorized {
		t.Errorf("the open session survived the block: %d %s", status, body)
	}
	// So does the key, which is the credential the agent and the MCP tools use.
	if _, err := h.resolveAgentCredential(key); !errors.Is(err, db.ErrAccountBlocked) {
		t.Errorf("the workstation key survived the block: %v", err)
	}
	// And signing in again does not reopen it.
	if status, body := call(t, server, nil, http.MethodPost, "/auth/local", `{"email":"bob@example.com"}`); status != http.StatusForbidden || !strings.Contains(body, msgBlocked) {
		t.Errorf("a blocked account signed back in: %d %s", status, body)
	}

	// Unblocking gives everything back.
	if status, body := call(t, server, alice, http.MethodPut, "/api/users/"+bobID, `{"blocked":false}`); status != http.StatusOK {
		t.Fatalf("unblock: %d %s", status, body)
	}
	if _, err := h.resolveAgentCredential(key); err != nil {
		t.Errorf("the key stayed shut after the unblock: %v", err)
	}
	if status, _ := call(t, server, nil, http.MethodPost, "/auth/local", `{"email":"bob@example.com"}`); status != http.StatusOK {
		t.Errorf("the account stayed shut after the unblock")
	}
}

// The board must keep somebody able to administer it, and an admin must not be
// able to shut the door on themselves from the inside.
func TestTheLastAdminSurvivesEveryAccountOperation(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)

	aliceID, alice := account(t, database, "alice@example.com")
	bobID, _ := account(t, database, "bob@example.com")

	for _, attempt := range []struct{ method, body string }{
		{http.MethodPut, `{"role":"member"}`},
		{http.MethodPut, `{"blocked":true}`},
		{http.MethodDelete, ""},
	} {
		if status, body := call(t, server, alice, attempt.method, "/api/users/"+aliceID, attempt.body); status != http.StatusConflict {
			t.Errorf("%s %s on the last admin: %d %s, want 409", attempt.method, attempt.body, status, body)
		}
	}

	// With a second admin the board is no longer at stake, but self-blocking
	// and self-deletion stay refused: the account that could undo them is the
	// one being closed.
	if status, body := call(t, server, alice, http.MethodPut, "/api/users/"+bobID, `{"role":"admin"}`); status != http.StatusOK {
		t.Fatalf("promoting bob: %d %s", status, body)
	}
	if status, _ := call(t, server, alice, http.MethodPut, "/api/users/"+aliceID, `{"blocked":true}`); status != http.StatusConflict {
		t.Errorf("an admin blocked their own account")
	}
	if status, _ := call(t, server, alice, http.MethodDelete, "/api/users/"+aliceID, ""); status != http.StatusConflict {
		t.Errorf("an admin deleted their own account")
	}
	// Demoting themselves is another matter: somebody else holds the role.
	if status, body := call(t, server, alice, http.MethodPut, "/api/users/"+aliceID, `{"role":"member"}`); status != http.StatusOK {
		t.Errorf("demoting oneself with a second admin present: %d %s", status, body)
	}
}

// Deleting an account takes its credentials with it and leaves its work alone.
func TestDeletingAnAccountRemovesItsCredentialsOnly(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)

	aliceID, alice := account(t, database, "alice@example.com")
	bobID, bob := account(t, database, "bob@example.com")
	key, _, err := database.CreateAPIKey(bobID, "laptop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}
	// Alice keeps a key of her own, which the deletion must leave alone: that
	// is the "only" in this test's name. It used to be here for another reason
	// — a board with no key at all fell back to the legacy open mode and the
	// assertion below passed for the wrong reason — and that mode is gone
	// (ADR 0019), so the key now carries an assertion instead of a workaround.
	aliceKey, _, err := database.CreateAPIKey(aliceID, "desktop", db.DefaultAPIKeyTTL)
	if err != nil {
		t.Fatal(err)
	}

	if status, body := call(t, server, alice, http.MethodDelete, "/api/users/"+bobID, ""); status != http.StatusOK {
		t.Fatalf("delete: %d %s", status, body)
	}
	if user, err := database.GetUser(bobID); err != nil || user != nil {
		t.Errorf("the account survived the deletion: %v %v", user, err)
	}
	if status, _ := call(t, server, bob, http.MethodGet, "/api/tasks", ""); status != http.StatusUnauthorized {
		t.Errorf("the session of a deleted account still names somebody")
	}
	if _, err := h.resolveAgentCredential(key); err == nil {
		t.Errorf("the workstation key of a deleted account still opens")
	}
	if credential, err := h.resolveAgentCredential(aliceKey); err != nil || credential.UserID != aliceID {
		t.Errorf("another account's key did not survive the deletion: %+v %v", credential, err)
	}
	if status, _ := call(t, server, alice, http.MethodDelete, "/api/users/"+bobID, ""); status != http.StatusNotFound {
		t.Errorf("deleting an unknown account is not a 404")
	}
	// The implicit account is not a person and is not deletable.
	if err := database.EnsureUser(db.ImplicitUserID); err != nil {
		t.Fatal(err)
	}
	if status, _ := call(t, server, alice, http.MethodDelete, "/api/users/"+db.ImplicitUserID, ""); status != http.StatusConflict {
		t.Errorf("the implicit account was deletable")
	}
}

// The tracker keys sit on the shared row but are a member's to change: a
// member who may open a project may point it at the tracker it reads from.
// The rest of that row, the AI configuration and the prompts, stays an admin's.
func TestMembersConfigureTheTrackerOnTheSharedRow(t *testing.T) {
	h, database, cleanup := setupTestHandler(t)
	defer cleanup()
	server := guardedServer(t, h)
	_, _ = account(t, database, "alice@example.com")
	_, bob := account(t, database, "bob@example.com")

	current, _ := database.GetSettings()
	current.IssueTracker = "jira"
	current.JiraUrl = "https://acme.atlassian.net"
	current.JiraProject = "PE"
	payload, _ := json.Marshal(current)
	if status, body := call(t, server, bob, http.MethodPost, "/api/settings", string(payload)); status != http.StatusOK {
		t.Fatalf("member configuring the tracker: %d %s", status, body)
	}
	saved, _ := database.GetSettings()
	if saved.IssueTracker != "jira" || saved.JiraUrl != "https://acme.atlassian.net" || saved.JiraProject != "PE" {
		t.Fatalf("the tracker did not reach the shared row: %+v", saved)
	}

	// The same payload carrying a prompt is still refused, by name.
	current = saved
	current.PromptClarify = "whatever"
	payload, _ = json.Marshal(current)
	if status, body := call(t, server, bob, http.MethodPost, "/api/settings", string(payload)); status != http.StatusForbidden || !strings.Contains(body, "promptClarify") {
		t.Fatalf("member changing a prompt: %d %s", status, body)
	}
}
