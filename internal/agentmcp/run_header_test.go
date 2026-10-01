package agentmcp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"tasks/internal/agentprotocol"
)

// A console the agent launched names its run on every request, with the
// agent's credential still attached; a bridge started outside a launch names
// none (#498).
func TestBridgeClientNamesTheLaunchedRun(t *testing.T) {
	for _, tc := range []struct{ env, want string }{{"run-42", "run-42"}, {"  ", ""}, {"", ""}} {
		var gotRun, gotAuth string
		var present bool
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, present = r.Header[http.CanonicalHeaderKey(agentprotocol.RunIDHeader)]
			gotRun, gotAuth = r.Header.Get(agentprotocol.RunIDHeader), r.Header.Get("Authorization")
		}))
		resp, err := bridgeHTTPClient("secret", tc.env).Get(server.URL)
		server.Close()
		if err != nil {
			t.Fatalf("SECTILE_RUN_ID=%q: %v", tc.env, err)
		}
		resp.Body.Close()
		if gotRun != tc.want || present != (tc.want != "") {
			t.Errorf("SECTILE_RUN_ID=%q: run header = %q (present %v), want %q", tc.env, gotRun, present, tc.want)
		}
		if gotAuth != "Bearer secret" {
			t.Errorf("SECTILE_RUN_ID=%q: Authorization = %q, want the agent credential", tc.env, gotAuth)
		}
	}
}
