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
