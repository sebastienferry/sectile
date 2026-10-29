//go:build !windows

package agentexec

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestTreeHelper(t *testing.T) {
	if os.Getenv("SECTILE_TREE_HELPER") != "1" {
		return
	}
	child := exec.Command("sh", "-c", "sleep 3; printf escaped > \"$1\"", "sh", os.Getenv("SECTILE_TREE_MARKER"))
	release, err := StartDetached(child)
	if err != nil {
		os.Exit(2)
	}
	defer release()
	if err := os.WriteFile(os.Getenv("SECTILE_TREE_READY"), []byte("ready"), 0600); err != nil {
		os.Exit(3)
	}
	_ = child.Wait()
	time.Sleep(time.Minute)
}
func TestStopTreeStopsToolsInSeparateSessions(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	ready := filepath.Join(dir, "ready")
	cmd := exec.Command(os.Args[0], "-test.run=^TestTreeHelper$")
	cmd.Env = append(os.Environ(), "SECTILE_TREE_HELPER=1", "SECTILE_TREE_MARKER="+marker, "SECTILE_TREE_READY="+ready)
	release, err := StartDetached(cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	defer func() { _ = cmd.Process.Kill() }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("helper did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := StopTree(cmd); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	time.Sleep(4 * time.Second)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("escaped tool wrote marker: %v", err)
	}
}
