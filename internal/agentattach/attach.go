package agentattach

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

// ConnectionInfo holds the resolved connection parameters for an attach session.
type ConnectionInfo struct {
	SessionID string
	URL       string
	Token     string
}

// ResolveConnection determines the session ID, loopback URL, and auth token
// using CLI flags, environment variables, and ~/.taskflow/agent-connection.json.
func ResolveConnection(sessionID, rawURL, token string) (ConnectionInfo, error) {
	sessionID = strings.TrimSpace(sessionID)
	rawURL = strings.TrimSpace(rawURL)
	token = strings.TrimSpace(token)

	if rawURL == "" {
		rawURL = strings.TrimSpace(os.Getenv("SECTILE_LOOPBACK_URL"))
	}
	if token == "" {
		token = strings.TrimSpace(os.Getenv("SECTILE_AGENT_TOKEN"))
	}

	if rawURL == "" || token == "" {
		info, err := readAgentConnectionFile()
		if err == nil {
			if rawURL == "" {
				rawURL = info.URL
			}
			if token == "" {
				token = info.Token
			}
		}
	}

	if sessionID == "" {
		return ConnectionInfo{}, fmt.Errorf("session ID is required (--session)")
	}
	if rawURL == "" {
		return ConnectionInfo{}, fmt.Errorf("loopback URL is required (--url or SECTILE_LOOPBACK_URL)")
	}

	return ConnectionInfo{
		SessionID: sessionID,
		URL:       rawURL,
		Token:     token,
	}, nil
}

type agentConnection struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func readAgentConnectionFile() (*agentConnection, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(home, ".taskflow", "agent-connection.json"))
	if err != nil {
		return nil, err
	}
	var conn agentConnection
	if err := json.Unmarshal(data, &conn); err != nil {
		return nil, err
	}
	return &conn, nil
}

// FormatWebSocketURL converts an HTTP/WS loopback base URL and session ID into a valid WebSocket terminal endpoint.
func FormatWebSocketURL(rawURL, sessionID string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid loopback URL: %w", err)
	}

	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
		// scheme already websocket
	default:
		parsed.Scheme = "ws"
	}

	if parsed.Path == "" || parsed.Path == "/" {
		parsed.Path = "/desktop/terminal"
	} else if !strings.HasSuffix(parsed.Path, "/desktop/terminal") {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + "/desktop/terminal"
	}

	query := parsed.Query()
	query.Set("id", sessionID)
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

// Run executes the attach subcommand.
func Run(args []string) error {
	fs := flag.NewFlagSet("attach", flag.ContinueOnError)
	sessionFlag := fs.String("session", "", "Session ID or run ID to attach to")
	urlFlag := fs.String("url", "", "Agent loopback URL (default: from agent-connection.json)")
	tokenFlag := fs.String("token", "", "Agent authentication token")

	if err := fs.Parse(args); err != nil {
		return err
	}

	info, err := ResolveConnection(*sessionFlag, *urlFlag, *tokenFlag)
	if err != nil {
		return err
	}

	wsURL, err := FormatWebSocketURL(info.URL, info.SessionID)
	if err != nil {
		return err
	}

	header := http.Header{}
	if info.Token != "" {
		header.Set("Authorization", "Bearer "+info.Token)
	}

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to connect to session %s (status %d: %s)", wsURL, resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return fmt.Errorf("failed to connect to session %s: %w", wsURL, err)
	}
	defer conn.Close()

	stdinFd := int(os.Stdin.Fd())
	stdoutFd := int(os.Stdout.Fd())

	if term.IsTerminal(stdinFd) {
		oldState, err := term.MakeRaw(stdinFd)
		if err == nil {
			defer func() {
				_ = term.Restore(stdinFd, oldState)
			}()
		}
	}

	// Send initial terminal size
	if term.IsTerminal(stdoutFd) {
		if cols, rows, err := term.GetSize(stdoutFd); err == nil && cols > 0 && rows > 0 {
			_ = conn.WriteJSON(map[string]any{
				"type": "resize",
				"cols": cols,
				"rows": rows,
			})
		}
	}

	// Watch resize signals
	sigCh := make(chan os.Signal, 1)
	notifyResize(sigCh)
	defer stopResize(sigCh)

	go func() {
		for range sigCh {
			if term.IsTerminal(stdoutFd) {
				if cols, rows, err := term.GetSize(stdoutFd); err == nil && cols > 0 && rows > 0 {
					_ = conn.WriteJSON(map[string]any{
						"type": "resize",
						"cols": cols,
						"rows": rows,
					})
				}
			}
		}
	}()

	done := make(chan struct{})

	// Read from WebSocket -> write to Stdout
	go func() {
		defer close(done)
		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if msgType == websocket.BinaryMessage || msgType == websocket.TextMessage {
				_, _ = os.Stdout.Write(msg)
			}
		}
	}()

	// Read from Stdin -> write to WebSocket
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	<-done
	return nil
}
