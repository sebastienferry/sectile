package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"tasks/internal/version"
)

// The desktop app shows the agent's build in its settings. The route answering
// that question carries the companion's credential like every other one: the
// version is not a reason to open an unauthenticated hole on the loopback port.
func TestDesktopVersionRouteReportsTheAgentBuild(t *testing.T) {
	restore := version.Version
	t.Cleanup(func() { version.Version = restore })
	version.Version = "v3.2.1"

	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}

	unauthenticated := httptest.NewRecorder()
	d.desktopHandler(unauthenticated, httptest.NewRequest(http.MethodGet, "/desktop/version", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 without the companion token", unauthenticated.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/desktop/version", nil)
	request.Header.Set("Authorization", "Bearer private")
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", response.Code, response.Body.String())
	}
	var info version.Info
	if err := json.Unmarshal(response.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode: %v (%s)", err, response.Body.String())
	}
	if info.Version != "v3.2.1" {
		t.Errorf("version = %q, want v3.2.1", info.Version)
	}
}
