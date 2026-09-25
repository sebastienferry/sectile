package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"tasks/internal/agentprotocol"
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

// The fingerprint lets the companion notice a same-version rebuild: it is the
// content hash of the executable, taken when the agent started.
func TestDesktopVersionRouteReportsTheBinaryFingerprint(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	want := hex.EncodeToString(sum[:])
	if got := executableSha256(); got != want {
		t.Fatalf("executableSha256() = %q, want %q", got, want)
	}

	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private", binarySha256: want}}
	request := httptest.NewRequest(http.MethodGet, "/desktop/version", nil)
	request.Header.Set("Authorization", "Bearer private")
	response := httptest.NewRecorder()
	d.desktopHandler(response, request)
	var answer struct {
		Version      string `json:"version"`
		BinarySha256 string `json:"binarySha256"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &answer); err != nil {
		t.Fatalf("decode: %v (%s)", err, response.Body.String())
	}
	if answer.BinarySha256 != want || answer.Version == "" {
		t.Fatalf("answer = %+v, want the version and binarySha256 %s", answer, want)
	}
	if fileSha256(binary+".missing") != "" {
		t.Fatal("a missing file has a fingerprint")
	}
}

// The server learns what the agent can run from the connection URL alone, so no
// operation can race an announcement message.
func TestDialURLAnnouncesTheBuildAndTheOperations(t *testing.T) {
	restoreVersion, restoreCommit := version.Version, version.Commit
	t.Cleanup(func() { version.Version, version.Commit = restoreVersion, restoreCommit })
	version.Version, version.Commit = "v2.0.0", "fedcba9876543210fedcba9876543210fedcba98"

	d := &agentDaemon{link: serverLink{serverURL: "https://sectile.example.com", projectID: "p1", deviceID: "laptop"}}
	raw, err := d.buildWSURL()
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Scheme != "wss" || u.Path != "/ws/agent-connect" || q.Get("projectId") != "p1" || q.Get("deviceId") != "laptop" {
		t.Fatalf("url = %s", raw)
	}
	if q.Get("agentVersion") != "v2.0.0" || q.Get("agentCommit") != version.Commit {
		t.Fatalf("build = %q / %q", q.Get("agentVersion"), q.Get("agentCommit"))
	}
	if got := q.Get("operations"); got != strings.Join(agentprotocol.Operations, ",") {
		t.Fatalf("operations = %q", got)
	}
}

// Every announced operation is one the agent dispatches, and an action outside
// the list is still refused with the text legacy servers already expect.
func TestEveryAnnouncedOperationPassesTheDispatchList(t *testing.T) {
	var reached atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Add(1)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	d := &agentDaemon{link: serverLink{serverURL: server.URL}}
	ctx := context.Background()

	for _, action := range agentprotocol.Operations {
		before := reached.Load()
		_, err := d.executeOperation(ctx, agentprotocol.Operation{ProjectID: "p1", Action: action, MacroKey: "M-1"})
		if err != nil && strings.Contains(err.Error(), "unknown local operation") {
			t.Fatalf("%s is announced but refused: %v", action, err)
		}
		if reached.Load() == before {
			t.Fatalf("%s did not get past the dispatch list (err %v)", action, err)
		}
	}
	_, err := d.executeOperation(ctx, agentprotocol.Operation{ProjectID: "p1", Action: "arbitrary_shell"})
	if err == nil || err.Error() != `unknown local operation "arbitrary_shell"` {
		t.Fatalf("unknown action: %v", err)
	}
}
