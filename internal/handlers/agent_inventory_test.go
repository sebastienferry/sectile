package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"tasks/internal/agentprotocol"
	"tasks/internal/db"
	"tasks/internal/handlers"
	"tasks/internal/models"
	"tasks/internal/testsqlite"
)

func TestRelayInventoryLifecycleAndStatusPrivacy(t *testing.T) {
	database, err := testsqlite.New(t, filepath.Join(t.TempDir(), "inventory.db"), db.NewDB)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	p, err := database.CreateProject(models.CreateProjectRequest{Name: "Inventory P"})
	if err != nil {
		t.Fatal(err)
	}
	q, err := database.CreateProject(models.CreateProjectRequest{Name: "Inventory Q"})
	if err != nil {
		t.Fatal(err)
	}
	h := handlers.NewHandler(database)
	server := httptest.NewServer(http.HandlerFunc(h.HandleAgentConnect))
	defer server.Close()
	key := defaultAgentKey(t, database)
	cookie := defaultSession(t, database)
	dial := func(device, scope string) *websocket.Conn {
		t.Helper()
		u, _ := url.Parse(server.URL)
		u.Scheme = "ws"
		u.RawQuery = url.Values{"token": {key}, "deviceId": {device}, "projectId": {scope}}.Encode()
		c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { c.Close() })
		return c
	}
	send := func(c *websocket.Conn, ids ...string) {
		t.Helper()
		if ids == nil {
			ids = []string{}
		}
		raw, _ := json.Marshal(agentprotocol.ProjectInventory{SchemaVersion: 1, ProjectIDs: ids})
		if err := c.WriteJSON(handlers.AgentMessage{Type: agentprotocol.ProjectInventoryType, Payload: raw}); err != nil {
			t.Fatal(err)
		}
	}
	type response struct {
		Agents       []handlers.AgentConnInfo        `json:"agents"`
		Workstations []handlers.AvailableWorkstation `json:"availableWorkstations"`
	}
	query := func(project string, session *http.Cookie) (int, response) {
		req := httptest.NewRequest("GET", "/api/agent/status?projectId="+url.QueryEscape(project), nil)
		if session != nil {
			req.AddCookie(session)
		}
		rr := httptest.NewRecorder()
		h.HandleAgentStatus(rr, req)
		var out response
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return rr.Code, out
	}
	await := func(project string, agents, devices int) response {
		t.Helper()
		var out response
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			code, r := query(project, cookie)
			out = r
			if code == 200 && len(r.Agents) == agents && len(r.Workstations) == devices {
				return r
			}
		}
		t.Fatalf("status never settled: %+v", out)
		return out
	}
	a := dial("a", "all")
	b := dial("b", "all")
	legacy := dial("legacy", "all")
	send(a, p.ID)
	send(b, q.ID)
	await(p.ID, 1, 1)
	await(q.ID, 1, 1)
	all := await("", 3, 0)
	unknown := 0
	for _, a := range all.Agents {
		if a.InventoryState == "unknown" {
			unknown++
		}
	}
	if unknown != 1 {
		t.Fatalf("legacy inventory state: %+v", all)
	}
	send(b, p.ID, q.ID)
	r := await(p.ID, 2, 2)
	if r.Workstations[0].DeviceID != "a" || r.Workstations[1].DeviceID != "b" {
		t.Fatal("unstable device ordering")
	}
	another := dial("a", "default")
	send(another, p.ID)
	_ = a.SetReadDeadline(time.Now().Add(time.Second))
	var rebound handlers.AgentMessage
	if err := a.ReadJSON(&rebound); !websocket.IsCloseError(err, 4001) {
		t.Fatalf("scope replacement did not close old session: %v", err)
	}
	await(p.ID, 2, 2)
	// A malformed or unknown project leaves the previous complete snapshot intact.
	if err := b.WriteJSON(handlers.AgentMessage{Type: agentprotocol.ProjectInventoryType, Payload: []byte(`{"schemaVersion":1,"projectIds":null}`)}); err != nil {
		t.Fatal(err)
	}
	_ = b.SetReadDeadline(time.Now().Add(time.Second))
	var rejected handlers.AgentMessage
	if err := b.ReadJSON(&rejected); err != nil || rejected.Type != "error" {
		t.Fatalf("rejection=%+v %v", rejected, err)
	}
	send(b, "unknown-project")
	if err := b.ReadJSON(&rejected); err != nil || rejected.Type != "error" {
		t.Fatalf("unknown project rejection=%+v %v", rejected, err)
	}
	await(p.ID, 2, 2)
	send(another)
	await(p.ID, 1, 1)
	send(another, p.ID)
	await(p.ID, 2, 2)
	replacement := dial("b", "all")
	send(replacement, q.ID)
	await(p.ID, 1, 1)
	await(q.ID, 1, 1)
	b.Close()
	await(q.ID, 1, 1)
	another.Close()
	await(p.ID, 0, 0)
	replacement.Close()
	await(q.ID, 0, 0)
	if code, _ := query(p.ID, nil); code != 401 {
		t.Fatalf("anonymous status=%d", code)
	}
	if code, _ := query("unknown-project", cookie); code != 403 {
		t.Fatalf("unknown project status=%d", code)
	}
	if err := database.EnsureUser("other"); err != nil {
		t.Fatal(err)
	}
	token, _, err := database.CreateWebSession("other")
	if err != nil {
		t.Fatal(err)
	}
	if code, r := query("", &http.Cookie{Name: cookie.Name, Value: token}); code != 200 || len(r.Agents) != 0 {
		t.Fatalf("foreign inventory disclosed: %d %+v", code, r)
	}
	legacy.Close()
}
