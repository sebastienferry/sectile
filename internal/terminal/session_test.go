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

// TapOutput hands back what the session printed before it and every chunk
// after it, so a copy of the output started late still starts at the first
// byte and holds each chunk once.
func TestTapOutputReturnsTheHistoryThenTheNextChunks(t *testing.T) {
	m := NewManager()
	defer func() { _ = m.CloseSession("tapped") }()
	if _, err := m.GetOrCreateSession("tapped", t.TempDir(), nil); err != nil {
		t.Fatalf("session: %v", err)
	}
	if _, ok := m.TapOutput("missing", func([]byte) {}); ok {
		t.Fatal("a missing session was tapped")
	}
	time.Sleep(2 * time.Second)
	if err := m.SendInput("tapped", "echo BEFORE\"-\"TAP\n"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	var history []byte
	var mu sync.Mutex
	var after strings.Builder
	for {
		m.mu.RLock()
		sess := m.sessions["tapped"]
		m.mu.RUnlock()
		sess.historyMu.RLock()
		seen := strings.Contains(string(sess.history), "BEFORE-TAP")
		sess.historyMu.RUnlock()
		if seen {
			var ok bool
			history, ok = m.TapOutput("tapped", func(chunk []byte) { mu.Lock(); after.Write(chunk); mu.Unlock() })
			if !ok {
				t.Fatal("the session was not tapped")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the shell never ran the first line")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(string(history), "BEFORE-TAP") {
		t.Fatalf("the history lacks what was printed before the tap: %q", history)
	}
	if err := m.SendInput("tapped", "echo AFTER\"-\"TAP\n"); err != nil {
		t.Fatal(err)
	}
	for {
		mu.Lock()
		got := after.String()
		mu.Unlock()
		if strings.Contains(got, "AFTER-TAP") {
			if strings.Contains(got, "BEFORE-TAP") {
				t.Fatalf("a chunk of the history reached the listener too: %q", got)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the listener never saw the next line: %q", got)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
