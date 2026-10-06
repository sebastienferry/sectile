package agent

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// shortenBrowserSignIn bounds the wait for the browser for one test.
func shortenBrowserSignIn(t *testing.T, timeout time.Duration) {
	t.Helper()
	previous := browserSignInTimeout
	browserSignInTimeout = timeout
	t.Cleanup(func() { browserSignInTimeout = previous })
}

// followSignIn plays the browser and the server: it reads the loopback port and
// state from the sign-in URL and calls the callback with code and that state,
// or with state when it is not empty.
func followSignIn(t *testing.T, target, code, state string) int {
	t.Helper()
	parsed, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	if state == "" {
		state = parsed.Query().Get("state")
	}
	callback := fmt.Sprintf("http://127.0.0.1:%s/callback?%s", parsed.Query().Get("port"), url.Values{"code": {code}, "state": {state}}.Encode())
	resp, err := http.Get(callback)
	if err != nil {
		t.Fatalf("call back: %v", err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// The code the server sends to the loopback is the one handed back, and the
// URL opened carries only the port and the state: nothing the server does not
// validate, and no label.
func TestBrowserSignInReturnsTheCodeTheCallbackCarries(t *testing.T) {
	shortenBrowserSignIn(t, 5*time.Second)
	var opened string
	var notice bytes.Buffer
	code, err := browserSignInCode(context.Background(), "https://sectile.example.test/", func(target string) error {
		opened = target
		if status := followSignIn(t, target, "c1", ""); status != http.StatusOK {
			t.Errorf("callback answered %d", status)
		}
		return nil
	}, &notice)
	if err != nil || code != "c1" {
		t.Fatalf("code = %q, %v", code, err)
	}
	parsed, err := url.Parse(opened)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "https" || parsed.Host != "sectile.example.test" || parsed.Path != "/auth/workstation" {
		t.Fatalf("opened %s", opened)
	}
	query := parsed.Query()
	if len(query) != 2 || query.Get("port") == "" || len(query.Get("state")) != 43 || query.Has("label") {
		t.Fatalf("sign-in URL carries %v", query)
	}
	if !strings.Contains(notice.String(), opened) {
		t.Fatalf("the URL is not printed for a browser that does not open: %s", notice.String())
	}
}

// A hit without the right state is refused and does not end the wait: the
// browser carrying the state still completes the sign-in.
func TestBrowserSignInIgnoresAWrongState(t *testing.T) {
	shortenBrowserSignIn(t, 5*time.Second)
	code, err := browserSignInCode(context.Background(), "https://sectile.example.test", func(target string) error {
		if status := followSignIn(t, target, "forged", "wrong-state-0123456789"); status != http.StatusBadRequest {
			t.Errorf("wrong state answered %d", status)
		}
		if status := followSignIn(t, target, "good", ""); status != http.StatusOK {
			t.Errorf("right state answered %d", status)
		}
		return nil
	}, &bytes.Buffer{})
	if err != nil || code != "good" {
		t.Fatalf("code = %q, %v", code, err)
	}
}

func TestBrowserSignInTimesOut(t *testing.T) {
	shortenBrowserSignIn(t, 50*time.Millisecond)
	_, err := browserSignInCode(context.Background(), "https://sectile.example.test", func(string) error { return nil }, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "timed out") || !strings.Contains(err.Error(), "--code") {
		t.Fatalf("err = %v", err)
	}
}

// With --no-browser nothing is opened, so the notice gives the URL to visit
// without claiming a browser is opening.
func TestBrowserSignInWithoutABrowserOnlyPrintsTheURL(t *testing.T) {
	shortenBrowserSignIn(t, 50*time.Millisecond)
	var notice bytes.Buffer
	if _, err := browserSignInCode(context.Background(), "https://sectile.example.test", nil, &notice); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(notice.String(), "Opening the browser") || !strings.Contains(notice.String(), "https://sectile.example.test/auth/workstation?") {
		t.Fatalf("notice = %s", notice.String())
	}
}

// The code is handed to a listener on the loopback only: another machine on
// the network cannot receive it.
func TestBrowserSignInOnlyListensOnLoopback(t *testing.T) {
	shortenBrowserSignIn(t, 5*time.Second)
	code, err := browserSignInCode(context.Background(), "https://sectile.example.test", func(target string) error {
		parsed, err := url.Parse(target)
		if err != nil {
			return err
		}
		port := parsed.Query().Get("port")
		conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", port))
		if err != nil {
			t.Errorf("nothing listens on 127.0.0.1:%s: %v", port, err)
		} else {
			if host, _, _ := net.SplitHostPort(conn.RemoteAddr().String()); host != "127.0.0.1" {
				t.Errorf("listener address %s", conn.RemoteAddr())
			}
			conn.Close()
		}
		followSignIn(t, target, "c2", "")
		return nil
	}, &bytes.Buffer{})
	if err != nil || code != "c2" {
		t.Fatalf("code = %q, %v", code, err)
	}
}
