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

func TestParseClaudeInitializeModelsExtractsResolvedAndAliasModels(t *testing.T) {
	line := []byte(`{"type":"control_response","response":{"subtype":"success","request_id":"init-1","response":{"models":[{"value":"default","resolvedModel":"claude-opus-4-6"},{"value":"opus","resolvedModel":"claude-opus-4-6"},{"value":"sonnet","resolvedModel":"claude-sonnet-4-6"},{"value":"haiku","resolvedModel":"claude-haiku-4-5-20251001"},{"value":"bad model!","resolvedModel":""}]}}}`)
	got := parseClaudeInitializeModels(line)
	want := []string{"claude-opus-4-6", "opus", "claude-sonnet-4-6", "sonnet", "claude-haiku-4-5-20251001", "haiku"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseCodexModelListAndCacheFilterHiddenAndInvalidModels(t *testing.T) {
	rpcLine := []byte(`{"jsonrpc":"2.0","id":2,"result":{"data":[{"id":"gpt-5.4","model":"gpt-5.4","hidden":false},{"id":"gpt-5.3-codex","model":"gpt-5.3-codex","hidden":false},{"id":"secret-internal","model":"secret-internal","hidden":true},{"id":"bad space","model":"bad space","hidden":false}]}}`)
	gotRPC := parseCodexModelList(rpcLine)
	wantRPC := []string{"gpt-5.4", "gpt-5.3-codex"}
	if !reflect.DeepEqual(gotRPC, wantRPC) {
		t.Fatalf("got %v, want %v", gotRPC, wantRPC)
	}

	cacheJSON := []byte(`{"models":[{"slug":"gpt-5.4","visibility":"list"},{"slug":"gpt-5.3-codex","visibility":"list"},{"slug":"gpt-5-codex","visibility":"hide"}]}`)
	gotCache := parseCodexModelsCache(cacheJSON)
	wantCache := []string{"gpt-5.4", "gpt-5.3-codex"}
	if !reflect.DeepEqual(gotCache, wantCache) {
		t.Fatalf("got %v, want %v", gotCache, wantCache)
	}
}

