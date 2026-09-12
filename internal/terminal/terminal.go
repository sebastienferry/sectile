package terminal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"tasks/internal/runner"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all local origins
	},
}

type WsMessage struct {
	Type string `json:"type"` // "input", "resize", "ping", "pong"
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

type Session struct {
	ID           string
	Cwd          string
	Cmd          *exec.Cmd
	PtyFile      *os.File
	clients      map[*websocket.Conn]bool
	clientsMu    sync.Mutex
	history      []byte
	historyMu    sync.RWMutex
	maxHistBytes int
	closed       bool
	closeChan    chan struct{}
	// Observateurs d'exécution : un pas du workflow lancé dans cette session
	// écoute le flux pour savoir quand la commande finit et avec quel code.
	watchers   map[*runWatcher]struct{}
	watchersMu sync.Mutex
	// agentLaunched dit qu'un agent tourne déjà dans cette session : les pas
	// suivants du même ticket lui parlent au lieu d'en relancer un.
	agentLaunched bool
	agentMu       sync.Mutex
	CreatedAt         time.Time
	LastActiveAt      time.Time
	outputListeners   []func([]byte)
	outputListenersMu sync.Mutex
}

type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
	}
}

func (m *Manager) GetOrCreateSession(sessionID string, cwd string, envVars map[string]string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, ok := m.sessions[sessionID]; ok && !sess.closed {
		sess.LastActiveAt = time.Now()
		return sess, nil
	}

	// Determine shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		if _, err := os.Stat("/bin/zsh"); err == nil {
			shell = "/bin/zsh"
		} else if _, err := os.Stat("/bin/bash"); err == nil {
			shell = "/bin/bash"
		} else {
			shell = "sh"
		}
	}

	workDir := cwd
	if workDir == "" {
		workDir, _ = os.Getwd()
	}
	workDir = filepath.Clean(workDir)
	if abs, err := filepath.Abs(workDir); err == nil {
		workDir = abs
	}
	_ = os.MkdirAll(workDir, 0755)

	cmd := exec.Command(shell, "-l")
	cmd.Dir = workDir

	// Prepare environment
	env := os.Environ()
	customPath := runner.GetDynamicCustomPath()
	foundPath := false
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = "PATH=" + customPath + ":" + strings.TrimPrefix(e, "PATH=")
			foundPath = true
			break
		}
	}
	if !foundPath {
		env = append(env, "PATH="+customPath)
	}

	env = append(env,
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
		"LANG=fr_FR.UTF-8",
		"LC_ALL=fr_FR.UTF-8",
	)

	for k, v := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Start PTY with initial size
	ptyFile, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: 24,
		Cols: 80,
	})
	if err != nil {
		return nil, fmt.Errorf("impossible de démarrer le terminal PTY: %w", err)
	}

	sess := &Session{
		ID:           sessionID,
		Cwd:          workDir,
		Cmd:          cmd,
		PtyFile:      ptyFile,
		clients:      make(map[*websocket.Conn]bool),
		history:      make([]byte, 0, 32768),
		watchers:     make(map[*runWatcher]struct{}),
		maxHistBytes: 65536,
		closeChan:    make(chan struct{}),
		CreatedAt:    time.Now(),
		LastActiveAt: time.Now(),
	}

	m.sessions[sessionID] = sess

	// Background reader to capture output and broadcast
	go m.readPtyLoop(sess)

	return sess, nil
}

// SessionInfo describes a live PTY session for the UI: which shell is running,
// where, since when, and whether a browser is currently attached. A session
// survives its viewers, so knowing what is still alive matters.
type SessionInfo struct {
	ID           string    `json:"id"`
	Cwd          string    `json:"cwd"`
	Clients      int       `json:"clients"`
	CreatedAt    time.Time `json:"createdAt"`
	LastActiveAt time.Time `json:"lastActiveAt"`
	HistoryBytes int       `json:"historyBytes"`
	// AgentRunning dit qu'un agent a été démarré dans cette session : l'interface
	// s'en sert pour proposer « démarrer l'agent » ou « lancer la skill ».
	AgentRunning bool `json:"agentRunning"`
}

// ListSessions returns the live sessions, most recently active first.
func (m *Manager) ListSessions() []SessionInfo {
	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, sess := range m.sessions {
		if sess != nil && !sess.closed {
			sessions = append(sessions, sess)
		}
	}
	m.mu.RUnlock()

	out := make([]SessionInfo, 0, len(sessions))
	for _, sess := range sessions {
		sess.clientsMu.Lock()
		clients := len(sess.clients)
		sess.clientsMu.Unlock()

		sess.historyMu.RLock()
		historyBytes := len(sess.history)
		sess.historyMu.RUnlock()

		sess.agentMu.Lock()
		agentRunning := sess.agentLaunched
		sess.agentMu.Unlock()

		out = append(out, SessionInfo{
			ID:           sess.ID,
			Cwd:          sess.Cwd,
			Clients:      clients,
			CreatedAt:    sess.CreatedAt,
			LastActiveAt: sess.LastActiveAt,
			HistoryBytes: historyBytes,
			AgentRunning: agentRunning,
		})
	}

	// Ordre alphabétique stable, et non par dernière activité : une liste qui se
	// réordonne à chaque octet écrit dans un terminal est impossible à viser.
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (m *Manager) CloseSession(sessionID string) error {
	m.mu.Lock()
	sess, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()

	if !ok || sess == nil {
		return nil
	}

	sess.closed = true
	close(sess.closeChan)

	if sess.PtyFile != nil {
		_ = sess.PtyFile.Close()
	}
	if sess.Cmd != nil && sess.Cmd.Process != nil {
		_ = sess.Cmd.Process.Kill()
	}

	sess.clientsMu.Lock()
	for conn := range sess.clients {
		_ = conn.Close()
	}
	sess.clients = make(map[*websocket.Conn]bool)
	sess.clientsMu.Unlock()

	return nil
}

func (m *Manager) SendInput(sessionID string, input string) error {
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	m.mu.RUnlock()

	if !ok || sess == nil || sess.closed {
		return fmt.Errorf("session de terminal %s non trouvée ou inactive", sessionID)
	}

	_, err := sess.PtyFile.Write([]byte(input))
	return err
}

// AddOutputListener adds a callback invoked for every byte chunk read from this session's PTY.
func (s *Session) AddOutputListener(fn func([]byte)) {
	s.outputListenersMu.Lock()
	defer s.outputListenersMu.Unlock()
	s.outputListeners = append(s.outputListeners, fn)
}

// AddOutputListener registers a callback on the named session to receive real-time PTY output.
func (m *Manager) AddOutputListener(sessionID string, fn func([]byte)) {
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if ok && sess != nil {
		sess.AddOutputListener(fn)
	}
}

func (m *Manager) readPtyLoop(sess *Session) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-sess.closeChan:
			return
		default:
		}

		n, err := sess.PtyFile.Read(buf)
		if n > 0 {
			chunk := buf[:n]

			// Append to history buffer
			sess.historyMu.Lock()
			sess.history = append(sess.history, chunk...)
			if len(sess.history) > sess.maxHistBytes {
				sess.history = sess.history[len(sess.history)-sess.maxHistBytes:]
			}
			sess.historyMu.Unlock()

			// Alimenter les observateurs d'exécution avant la diffusion : ils
			// n'ont pas de client WebSocket et doivent voir tout le flux.
			sess.feedWatchers(chunk)

			// Notify direct output listeners (e.g. agent CLI console tap)
			sess.outputListenersMu.Lock()
			for _, fn := range sess.outputListeners {
				fn(chunk)
			}
			sess.outputListenersMu.Unlock()

			// Broadcast to all active websockets
			sess.clientsMu.Lock()
			for conn := range sess.clients {
				err := conn.WriteMessage(websocket.BinaryMessage, chunk)
				if err != nil {
					_ = conn.Close()
					delete(sess.clients, conn)
				}
			}
			sess.clientsMu.Unlock()
		}

		if err != nil {
			if err != io.EOF {
				log.Printf("[PTY] Session %s read error: %v", sess.ID, err)
			}
			break
		}
	}

	// Shell closed
	m.CloseSession(sess.ID)
}

func (m *Manager) HandleWebSocket(w http.ResponseWriter, r *http.Request, sessionID string, cwd string, envVars map[string]string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade error: %v", err)
		return
	}
	defer conn.Close()

	sess, err := m.GetOrCreateSession(sessionID, cwd, envVars)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\n\x1b[31mErreur démarrage terminal: %v\x1b[0m\r\n", err)))
		return
	}

	// Register client
	sess.clientsMu.Lock()
	sess.clients[conn] = true
	sess.clientsMu.Unlock()

	// Send terminal history so screen is restored
	sess.historyMu.RLock()
	if len(sess.history) > 0 {
		_ = conn.WriteMessage(websocket.BinaryMessage, sess.history)
	}
	sess.historyMu.RUnlock()

	defer func() {
		sess.clientsMu.Lock()
		delete(sess.clients, conn)
		sess.clientsMu.Unlock()
	}()

	// Read messages from WebSocket
	for {
		msgType, msgData, err := conn.ReadMessage()
		if err != nil {
			break
		}

		sess.LastActiveAt = time.Now()

		if msgType == websocket.BinaryMessage {
			_, _ = sess.PtyFile.Write(msgData)
			continue
		}

		if msgType == websocket.TextMessage {
			// Check if JSON payload (like resize or structured input)
			if bytes.HasPrefix(msgData, []byte("{")) {
				var wsMsg WsMessage
				if err := json.Unmarshal(msgData, &wsMsg); err == nil {
					switch wsMsg.Type {
					case "resize":
						if wsMsg.Cols > 0 && wsMsg.Rows > 0 {
							_ = pty.Setsize(sess.PtyFile, &pty.Winsize{
								Cols: uint16(wsMsg.Cols),
								Rows: uint16(wsMsg.Rows),
							})
						}
					case "input":
						_, _ = sess.PtyFile.Write([]byte(wsMsg.Data))
					case "ping":
						_ = conn.WriteJSON(WsMessage{Type: "pong"})
					}
					continue
				}
			}

			// Raw text input
			_, _ = sess.PtyFile.Write(msgData)
		}
	}
}
