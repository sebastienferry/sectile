//go:build windows

package agent

import (
	"context"
	"golang.org/x/sys/windows"
	"testing"
)

func TestProviderStatusNeverOpensAConsole(t *testing.T) {
	cmd := providerStatusCommand(context.Background(), "claude.exe", "claude")
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatal("status command must hide its console")
	}
}
