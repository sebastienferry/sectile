//go:build !windows

package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"tasks/internal/agentconfig"
	"tasks/internal/agentexec"
	"testing"
	"time"
)

func TestHeadlessOwnerHelper(t *testing.T) {
	if os.Getenv("SECTILE_HEADLESS_OWNER_HELPER") != "1" {
		return
	}
	directory := os.Getenv("SECTILE_HEADLESS_OWNER_DIRECTORY")
	server := newHeadlessServer(t)
	daemon := &agentDaemon{link: serverLink{serverURL: server.server.URL}}
	command := "printf '%s' $$ > " + quoteShell(filepath.Join(directory, "pid")) + "; printf ready > " + quoteShell(filepath.Join(directory, "ready")) + "; sleep 3; printf survived > " + quoteShell(filepath.Join(directory, "delayed"))
	if err := daemon.startHeadlessRun("fixture", agentconfig.Dispatch{RunID: "owned-headless"}, agentconfig.Config{ProjectID: "fixture"}, directory, "fixture", nil, command, "codex", "fixture-model"); err != nil {
		t.Fatal(err)
	}
	daemon.queue.mu.Lock()
	run := daemon.queue.runs["owned-headless"]
	daemon.queue.mu.Unlock()
	<-run.exited
}

func TestHeadlessToolsStopWhenAgentOwnerIsSIGKILLed(t *testing.T) {
	directory := t.TempDir()
	owner := agentexec.Hidden(exec.Command(os.Args[0], "-test.run=^TestHeadlessOwnerHelper$"))
	owner.Env = append(os.Environ(), "SECTILE_HEADLESS_OWNER_HELPER=1", "SECTILE_HEADLESS_OWNER_DIRECTORY="+directory)
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = owner.Process.Kill(); _ = owner.Wait() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(directory, "ready")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("headless shell did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = owner.Wait()
	time.Sleep(4 * time.Second)
	if _, err := os.Stat(filepath.Join(directory, "delayed")); !os.IsNotExist(err) {
		t.Fatalf("headless tool survived agent death: %v", err)
	}
}
