package agentconfig

import "testing"

func TestEffectiveCommandTemplate(t *testing.T) {
	for _, tc := range []struct{ name, provider, template, want string }{
		{"bare name for named provider", "claude", "claude", ""},
		{"bare name for legacy default provider", "", "agy", ""},
		{"named provider with prompt", "claude", `claude -p "{prompt}"`, `claude -p "{prompt}"`},
		{"empty template", "codex", "", ""},
		{"custom always runs its template", "custom", "/opt/cli run", "/opt/cli run"},
		{"custom with prompt", "custom", "/opt/cli {prompt}", "/opt/cli {prompt}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := EffectiveCommandTemplate(tc.provider, tc.template); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if got := UsesCommandTemplate(tc.provider, tc.template); got != (tc.want != "") {
				t.Fatalf("UsesCommandTemplate=%v for %q", got, tc.want)
			}
		})
	}
}
