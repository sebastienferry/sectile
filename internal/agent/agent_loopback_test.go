package agent

import (
	"net/http"
	"testing"
)

func loopbackRequest(t *testing.T, header string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:1234/mcp", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if header != "" {
		request.Header.Set("Authorization", header)
	}
	return request
}

func TestGatewayRequiresTheSessionSecret(t *testing.T) {
	daemon := &agentDaemon{loopbackToken: "session-secret", token: "device-credential"}

	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{"session secret", "Bearer session-secret", true},
		{"no header", "", false},
		{"wrong secret", "Bearer other-secret", false},
		{"missing scheme", "session-secret", false},
		// The credential that carries the identity must not open the gateway:
		// it never reaches local processes, so presenting it means it leaked.
		{"device credential", "Bearer device-credential", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := daemon.validLoopbackRequest(loopbackRequest(t, testCase.header)); got != testCase.want {
				t.Fatalf("validLoopbackRequest = %v, want %v", got, testCase.want)
			}
		})
	}
}

// An agent that never generated a secret must not accept an empty bearer.
func TestGatewayRejectsEveryCallerBeforeTheSecretExists(t *testing.T) {
	daemon := &agentDaemon{}
	if daemon.validLoopbackRequest(loopbackRequest(t, "Bearer ")) {
		t.Fatal("gateway accepted an empty secret")
	}
	if daemon.validLoopbackRequest(loopbackRequest(t, "")) {
		t.Fatal("gateway accepted a missing header")
	}
}
