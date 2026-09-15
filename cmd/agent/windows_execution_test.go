package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestHostTerminalExecutionIsWindowsOnly(t *testing.T) {
	cases := []struct {
		goos, terminal string
		want           bool
		why            string
	}{
		{"windows", "wt", true, "Windows has no pseudo-terminal"},
		{"windows", "cmd", true, "Windows falls back to cmd.exe"},
		{"windows", "pty", false, "an explicit embedded console is honoured"},
		{"darwin", "iterm", false, "macOS keeps the embedded console"},
		{"darwin", "ghostty", false, "a configured terminal app is not an execution surface"},
		{"linux", "pty", false, "Linux keeps the embedded console"},
		{"linux", "gnome-terminal", false, "Linux keeps the embedded console"},
		{"darwin", "", false, "an unset terminal means the embedded console"},
	}
	for _, c := range cases {
		if got := hostTerminalExecution(c.goos, c.terminal); got != c.want {
			t.Errorf("hostTerminalExecution(%q, %q) = %v, want %v: %s", c.goos, c.terminal, got, c.want, c.why)
		}
	}
}

func TestQuoteWindowsArgProtectsSpacesAndPercent(t *testing.T) {
	got := quoteWindowsArg(`C:\Program Files\Sectile\agent.exe`)
	if got != `"C:\Program Files\Sectile\agent.exe"` {
		t.Fatalf("path with spaces not quoted as one argument: %s", got)
	}
	if got := quoteWindowsArg("100%done"); got != `"100%%done"` {
		t.Fatalf("percent not escaped for the batch layer: %s", got)
	}
	if got := quoteWindowsArg(`say "hi"`); !strings.Contains(got, `\"hi\"`) {
		t.Fatalf("embedded quote not escaped: %s", got)
	}
}

// A skill prompt spans several lines, which no batch line can carry; encoding is what makes the
// wrapper survive the trip through cmd.exe.
func TestEncodedCommandSurvivesAMultiLinePrompt(t *testing.T) {
	prompt := "/code-issue gh-144\nExisting PR identity: https://example.test/1\nPreserve artifacts."
	encoded := base64.StdEncoding.EncodeToString([]byte(prompt))
	if strings.ContainsAny(encoded, "\n\"%^&<>|") {
		t.Fatalf("encoded command still carries characters a batch line cannot hold: %s", encoded)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || string(raw) != prompt {
		t.Fatalf("round trip lost the prompt: %v %q", err, raw)
	}
}

func TestEmbeddedConsoleNaming(t *testing.T) {
	for _, name := range []string{"", "pty", "PTY", "  pty  "} {
		if !embeddedConsole(name) {
			t.Errorf("%q should name the embedded console", name)
		}
	}
	for _, name := range []string{"wt", "cmd", "iterm"} {
		if embeddedConsole(name) {
			t.Errorf("%q should not name the embedded console", name)
		}
	}
}
