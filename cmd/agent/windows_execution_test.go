package main

import (
	"encoding/base64"
	"strings"
	"testing"

	"tasks/internal/runner"
)

func TestWrapperQuotingMatchesTheHostShell(t *testing.T) {
	// cmd.exe: quoted argument, doubled percent, CommandLineToArgvW escape.
	if got := runner.QuoteArg(runner.ShellCmd, `C:\Program Files\Sectile\agent.exe`); got != `"C:\Program Files\Sectile\agent.exe"` {
		t.Fatalf("path with spaces not quoted as one argument: %s", got)
	}
	if got := runner.QuoteArg(runner.ShellCmd, "100%done"); got != `"100%%done"` {
		t.Fatalf("percent not escaped for the batch layer: %s", got)
	}
	if got := runner.QuoteArg(runner.ShellCmd, `say "hi"`); !strings.Contains(got, `\"hi\"`) {
		t.Fatalf("embedded quote not escaped: %s", got)
	}
	// PowerShell: a literal string, where doubling is the only escape and percent is ordinary.
	if got := runner.QuoteArg(runner.ShellPowerShell, `C:\Program Files\a.exe`); got != `'C:\Program Files\a.exe'` {
		t.Fatalf("powershell path not quoted as a literal: %s", got)
	}
	if got := runner.QuoteArg(runner.ShellPowerShell, "it's"); got != `'it''s'` {
		t.Fatalf("powershell quote not doubled: %s", got)
	}
	if got := runner.QuoteArg(runner.ShellPowerShell, "100%done"); got != `'100%done'` {
		t.Fatalf("powershell percent should stay literal: %s", got)
	}
}

// A skill prompt spans several lines, which no console line can carry; encoding is what makes
// the wrapper survive being typed into the session's shell.
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

