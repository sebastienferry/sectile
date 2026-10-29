package storelock

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/agentexec"
)

func TestLockPreventsConcurrentOwnersAndAllowsReacquisition(t *testing.T) {
	root := filepath.Join(t.TempDir(), "local-store")
	first, err := Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Acquire(root); !errors.Is(err, ErrOwned) || second != nil {
		t.Fatalf("concurrent owner: lock=%v error=%v", second, err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLockIsReleasedWhenItsOwnerIsKilled(t *testing.T) {
	root := filepath.Join(t.TempDir(), "local-store")
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := agentexec.Hidden(exec.Command(binary, "-test.run=^TestStoreLockChild$"))
	child.Env = append(os.Environ(), "SECTILE_STORELOCK_CHILD="+root)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if child.ProcessState == nil {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	}()
	ready := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		ready <- line
	}()
	select {
	case line := <-ready:
		if line != "ready\n" {
			t.Fatalf("child did not acquire store: %q", line)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("child did not report store ownership")
	}
	if lock, err := Acquire(root); !errors.Is(err, ErrOwned) || lock != nil {
		t.Fatalf("parent acquired live child store: lock=%v error=%v", lock, err)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	lock, err := Acquire(root)
	if err != nil {
		t.Fatalf("killed child retained store ownership: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreLockChild(t *testing.T) {
	root := os.Getenv("SECTILE_STORELOCK_CHILD")
	if root == "" {
		return
	}
	lock, err := Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	_, _ = os.Stdout.WriteString("ready\n")
	time.Sleep(time.Minute)
}

func TestLockRejectsRelativeAndSymlinkedRoots(t *testing.T) {
	if lock, err := Acquire("relative-root"); err == nil || lock != nil {
		t.Fatalf("relative root: lock=%v error=%v", lock, err)
	}
	root := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if lock, err := Acquire(alias); err == nil || lock != nil {
		t.Fatalf("symlink root: lock=%v error=%v", lock, err)
	}
}
