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

// The gateway takes the workstation's own API key, the credential the server
// itself would ask for, so a console configured by the agent and a client
// configured by hand present the same thing.
func TestGatewayRequiresTheAPIKey(t *testing.T) {
	daemon := &agentDaemon{link: serverLink{token: "sectile_workstation_key"}}

	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{"api key", "Bearer sectile_workstation_key", true},
		{"no header", "", false},
		{"other key", "Bearer sectile_other_key", false},
		{"missing scheme", "sectile_workstation_key", false},
		{"empty bearer", "Bearer ", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := daemon.validLoopbackRequest(loopbackRequest(t, testCase.header)); got != testCase.want {
				t.Fatalf("validLoopbackRequest = %v, want %v", got, testCase.want)
			}
		})
	}
}

// An agent without a key must not accept an empty bearer.
func TestGatewayRejectsEveryCallerWithoutAKey(t *testing.T) {
	daemon := &agentDaemon{}
	if daemon.validLoopbackRequest(loopbackRequest(t, "Bearer ")) {
		t.Fatal("gateway accepted an empty key")
	}
	if daemon.validLoopbackRequest(loopbackRequest(t, "")) {
		t.Fatal("gateway accepted a missing header")
	}
}
