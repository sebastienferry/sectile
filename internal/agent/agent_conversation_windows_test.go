package agent

import (
	"golang.org/x/sys/windows"
	"testing"
)

func TestConversationChildDoesNotOpenAWindowsConsole(t *testing.T) {
	cmd := claudeConversationCommand(t.TempDir(), "", "", "", "", []string{`C:\with space`}, nil)
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("Claude conversation child would open a console window")
	}
}
