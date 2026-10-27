package agent

import (
	"golang.org/x/sys/windows"
	"testing"
)

func TestConversationChildDoesNotOpenAWindowsConsole(t *testing.T) {
	cmd := claudeConversationCommand(t.TempDir(), "", "", "", "", "", []string{`C:\with space`}, nil)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("Claude conversation child would open a console window")
	}
}

func TestABangCommandDoesNotOpenAWindowsConsole(t *testing.T) {
	cmd := conversationShellCommand(t.Context(), t.TempDir(), "dir", nil)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("a command typed in a conversation would open a console window")
	}
}

func TestCodexConversationChildDoesNotOpenAWindowsConsole(t *testing.T) {
	cmd := codexConversationCommand(t.TempDir(), nil)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("Codex conversation child would open a console window")
	}
}

func TestAgyConversationChildDoesNotOpenAWindowsConsole(t *testing.T) {
	cmd := agyConversationCommand(t.TempDir(), "", "", "accept-edits", "", "", []string{`C:\with space`}, nil)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("Antigravity conversation child would open a console window")
	}
}

func TestProfileProviderProbeDoesNotOpenAWindowsConsole(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		cmd := profileCommand(t.Context(), provider, "--help")
		if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
			t.Fatal("profile probe would open a console window")
		}
	}
}
