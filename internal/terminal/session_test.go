package terminal

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// A console session is only useful if a line typed into it actually runs. The check is the
// same on every platform, and it is the one that caught the Windows console reading a bare
// newline as a line continuation: the command was echoed, and nothing ever ran.
//
// The marker is assembled by the shell so the echoed command line cannot contain it: what
// is typed is SEC"-"42, what only a shell that ran it prints is SEC-42.
func TestSessionRunsAnInjectedLine(t *testing.T) {
	m := NewManager()
	defer func() { _ = m.CloseSession("injected-line") }()

	sess, err := m.GetOrCreateSession("injected-line", t.TempDir(), nil)
	if err != nil {
		t.Fatalf("session: %v", err)
	}

	var mu sync.Mutex
	var seen strings.Builder
	done := make(chan struct{})
	var once sync.Once
	sess.AddOutputListener(func(chunk []byte) {
		mu.Lock()
		seen.Write(chunk)
		hit := strings.Contains(seen.String(), "SEC-42")
		mu.Unlock()
		if hit {
			once.Do(func() { close(done) })
		}
	})

	// The shell prints a prompt before it reads: typing into it earlier loses the line.
	time.Sleep(2 * time.Second)
	if err := m.SendInput("injected-line", `echo SEC"-"42`+"\n"); err != nil {
		t.Fatalf("SendInput: %v", err)
	}

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		mu.Lock()
		out := seen.String()
		mu.Unlock()
		t.Fatalf("the shell never ran the line it was given; it printed %q", out)
	}
}
