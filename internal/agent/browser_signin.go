package agent

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"tasks/internal/agentexec"
)

// browserSignInTimeout bounds how long `pair` waits for the browser; a variable so tests shorten it.
var browserSignInTimeout = 5 * time.Minute

// openBrowser opens a URL in the default browser; a variable so tests follow the redirect themselves.
var openBrowser = func(target string) error {
	return browserCommand(runtime.GOOS, target).Start()
}

// browserCommand is the hidden command that hands target to the default browser.
func browserCommand(goos, target string) *exec.Cmd {
	var cmd *exec.Cmd
	switch goos {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target) // no console window, unlike cmd /c start
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return agentexec.Hidden(cmd)
}

// signedInPage is what the browser shows once the code reached this workstation.
const signedInPage = `<!doctype html><html><head><meta charset="utf-8"><title>Sectile</title></head>` +
	`<body><p>Sectile: this workstation is signed in. You can close this tab.</p></body></html>`

// browserSignInCode signs this workstation in through the server's web sign-in and returns the pairing code the server
// sends back to a one-shot loopback listener. With open nil, the URL is only printed (--no-browser).
func browserSignInCode(ctx context.Context, server string, open func(string) error, notice io.Writer) (string, error) {
	endpoint, err := url.Parse(server)
	if err != nil || (endpoint.Scheme != "http" && endpoint.Scheme != "https") || endpoint.Host == "" || endpoint.User != nil {
		return "", fmt.Errorf("use an HTTP or HTTPS server URL without credentials")
	}
	server = strings.TrimRight(server, "/")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("listen for the browser sign-in: %w", err)
	}
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		listener.Close()
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(nonce)
	codes := make(chan string, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		// A stray or forged hit is refused without ending the wait: only the
		// browser that carries this state can finish the sign-in.
		if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(state)) != 1 || code == "" {
			http.Error(w, "Sign-in state mismatch", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = io.WriteString(w, signedInPage)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		select {
		case codes <- code:
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(listener) }()
	defer srv.Close()

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	target := server + "/auth/workstation?" + url.Values{"port": {port}, "state": {state}}.Encode()
	if open != nil {
		fmt.Fprintf(notice, "Opening the browser to sign in to %s.\nIf it does not open, visit:\n  %s\n", server, target)
		if err := open(target); err != nil {
			fmt.Fprintf(notice, "Could not open the browser (%v); visit the URL above.\n", err)
		}
	} else {
		fmt.Fprintf(notice, "To sign in to %s, visit:\n  %s\n", server, target)
	}
	timeout := time.NewTimer(browserSignInTimeout)
	defer timeout.Stop()
	select {
	case code := <-codes:
		return code, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-timeout.C:
		return "", fmt.Errorf("browser sign-in timed out: run `sectile-agent pair` again, or pass --code")
	}
}
