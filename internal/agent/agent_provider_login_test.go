package agent

import (
	"reflect"
	"testing"
)

func TestProviderLoginUsesOnlyOfficialAuthenticationCommands(t *testing.T) {
	for _, tc := range []struct {
		provider string
		login    bool
		want     []string
	}{
		{"claude", true, []string{"auth", "login"}}, {"claude", false, []string{"auth", "status"}},
		{"codex", true, []string{"login"}}, {"codex", false, []string{"login", "status"}},
	} {
		if got := providerAuthArgs(tc.provider, tc.login); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("unexpected command: %v", got)
		}
	}
}
func TestProviderLoginRejectsUnknownProvidersAndMethods(t *testing.T) {
	d := &agentDaemon{loopback: loopbackServer{desktopToken: "private"}}
	for _, tc := range []struct {
		method, path string
		code         int
	}{
		{"POST", "/desktop/provider-login?provider=shell", 400},
		{"DELETE", "/desktop/provider-login?provider=claude", 405},
	} {
		response := conversationRequest(d, tc.method, tc.path, "", "private")
		if response.Code != tc.code {
			t.Fatalf("got %d want %d", response.Code, tc.code)
		}
	}
}
