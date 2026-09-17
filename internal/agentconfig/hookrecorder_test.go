package agentconfig

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// startJSONRecorder stands in for the local agent: it accepts any POST and
// hands the body to the test.
func startJSONRecorder(t *testing.T, bodies chan<- string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		select {
		case bodies <- string(raw):
		default:
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)
	return server.URL
}
