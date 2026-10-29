//go:build !windows

package agentexec

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "provider-guard" {
		os.Exit(RunProviderGuard(os.Args[2:]))
	}
	os.Exit(m.Run())
}

func TestOwnedAgentHelper(t *testing.T) {
	if os.Getenv("SECTILE_OWNED_AGENT_HELPER") != "1" {
		return
	}
	child := Hidden(exec.Command(os.Args[0], "-test.run=^TestTreeHelper$"))
	release, err := StartOwned(child)
	if err != nil {
		os.Exit(2)
	}
	defer release()
	_ = child.Wait()
}

func TestOwnerSIGKILLStopsToolsInSeparateSessions(t *testing.T) {
	testOwnerDeath(t, false)
}

func TestOwnerDeathRetriesFailedTreeInspection(t *testing.T) {
	testOwnerDeath(t, true)
}

func testOwnerDeath(t *testing.T, failInspection bool) {
	t.Helper()
	dir := t.TempDir()
	marker, ready := filepath.Join(dir, "marker"), filepath.Join(dir, "ready")
	owner := Hidden(exec.Command(os.Args[0], "-test.run=^TestOwnedAgentHelper$"))
	owner.Env = append(os.Environ(), "SECTILE_OWNED_AGENT_HELPER=1", "SECTILE_TREE_HELPER=1", "SECTILE_TREE_MARKER="+marker, "SECTILE_TREE_READY="+ready)
	if failInspection {
		realPS, err := exec.LookPath("ps")
		if err != nil {
			t.Fatal(err)
		}
		bin := filepath.Join(dir, "bin")
		if err := os.Mkdir(bin, 0700); err != nil {
			t.Fatal(err)
		}
		fail := filepath.Join(dir, "fail-once")
		if err := os.WriteFile(fail, []byte("fail"), 0600); err != nil {
			t.Fatal(err)
		}
		script := `#!/bin/sh
if [ -f "$SECTILE_INSPECT_FAIL" ]; then
 /bin/rm -f "$SECTILE_INSPECT_FAIL"
 exit 1
fi
exec "$SECTILE_REAL_PS" "$@"
`
		if err := os.WriteFile(filepath.Join(bin, "ps"), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
		owner.Env = append(owner.Env, "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "SECTILE_REAL_PS="+realPS, "SECTILE_INSPECT_FAIL="+fail)
	}
	release, err := StartDetached(owner)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	defer func() { _ = owner.Process.Kill() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("provider tool did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// SIGKILL bypasses the simulated agent's defers, including release().
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = owner.Wait()
	time.Sleep(4 * time.Second)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("tool survived owner death: %v", err)
	}
	if failInspection {
		if _, err := os.Stat(filepath.Join(dir, "fail-once")); !os.IsNotExist(err) {
			t.Fatal("fixture never exercised the failed inspection")
		}
	}
}

func TestOwnedProviderPreservesStdioAndExitStatus(t *testing.T) {
	cmd := Hidden(exec.Command("sh", "-c", `read -r message; printf '%s' "$message"; printf diagnostic >&2; exit 42`))
	cmd.Stdin = bytes.NewBufferString("literal $(no-shell-expansion)\n")
	var out, diagnostics bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diagnostics
	release, err := StartOwned(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	err = cmd.Wait()
	status, ok := err.(*exec.ExitError)
	if !ok || status.ExitCode() != 42 || out.String() != "literal $(no-shell-expansion)" || diagnostics.String() != "diagnostic" {
		t.Fatalf("provider protocol changed: status %v, output %q, diagnostics %q", err, out.String(), diagnostics.String())
	}
}

func TestOwnedProviderStopsWhenLifetimeIsReleased(t *testing.T) {
	cmd := Hidden(exec.Command("sh", "-c", "exec sleep 60"))
	release, err := StartOwned(cmd)
	if err != nil {
		t.Fatal(err)
	}
	release()
	release()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = StopTree(cmd)
		t.Fatal("provider outlived its owner pipe")
	}
}

func TestOwnedStdinPipeForwardsMessagesAndNormalEOF(t *testing.T) {
	cmd := Hidden(exec.Command("sh", "-c", "cat"))
	input, err := OwnedStdinPipe(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	var output bytes.Buffer
	cmd.Stdout = &output
	release, err := StartOwned(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := input.Write([]byte("provider frame\n")); err != nil {
		t.Fatal(err)
	}
	if err := input.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil || output.String() != "provider frame\n" {
			t.Fatalf("stdin protocol changed: %v, %q", err, output.String())
		}
	case <-time.After(5 * time.Second):
		release()
		t.Fatal("normal stdin EOF was not forwarded")
	}
}
