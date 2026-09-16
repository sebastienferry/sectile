package terminal

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Exécution d'un pas du workflow dans une session PTY.
//
// Avant, un pas autonome tournait dans un exec.CommandContext avec des tubes
// anonymes : rien à quoi s'attacher, aucune sortie avant la fin, et un plafond
// de cinq minutes qui tuait un run long sans le dire. Ici la commande est
// injectée dans la session de la tâche, donc elle est visible, attachable, et
// l'utilisateur peut répondre à l'agent en cours de route.
//
// La fin de course est détectée par un marqueur imprimé par le shell après la
// commande, qui porte le code de sortie. Le début a son marqueur lui aussi :
// s'il n'apparaît pas, la session n'était pas au prompt, et l'appelant retombe
// sur l'exécution directe plutôt que d'envoyer la commande dans le vide.
type runWatcher struct {
	startRe *regexp.Regexp
	endRe   *regexp.Regexp
	// quietOnly : ce guetteur ne cherche aucun marqueur, il ne sert qu'à savoir
	// quand le flux se tait. Lui faire suivre la logique de marqueurs revenait à
	// lire un groupe de capture inexistant, et ça paniquait.
	quietOnly bool

	mu       sync.Mutex
	raw      strings.Builder
	startAt  int // index dans raw juste après le marqueur de début
	endAt    int // index dans raw du marqueur de fin
	started  bool
	done     bool
	exitCode int

	startedCh chan struct{}
	doneCh    chan struct{}
	// lastByteAt est remis à jour à chaque octet : c'est lui qui porte le
	// plafond de silence, à la place d'un plafond de durée.
	lastByteAt time.Time
}

func (s *Session) feedWatchers(chunk []byte) {
	s.watchersMu.Lock()
	watchers := make([]*runWatcher, 0, len(s.watchers))
	for w := range s.watchers {
		watchers = append(watchers, w)
	}
	s.watchersMu.Unlock()

	for _, w := range watchers {
		w.consume(string(chunk))
	}
}

// consume accumulates the stream and looks for the markers in it.
//
// Les marqueurs sont bornés par de vraies fins de ligne, et c'est indispensable :
// le shell réaffiche la ligne injectée, donc le texte du marqueur apparaît deux
// fois. Dans l'écho il est entouré des caractères littéraux d'un printf, jamais
// de sauts de ligne réels, ce qui suffit à ne pas s'y accrocher.
func (w *runWatcher) consume(text string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.done {
		return
	}
	w.lastByteAt = time.Now()
	if w.quietOnly {
		return
	}
	w.raw.WriteString(text)
	raw := w.raw.String()

	if !w.started {
		m := w.startRe.FindStringIndex(raw)
		if m == nil {
			return
		}
		w.started = true
		w.startAt = m[1]
		close(w.startedCh)
	}

	if m := w.endRe.FindStringSubmatchIndex(raw); m != nil && m[0] >= w.startAt {
		if code, err := strconv.Atoi(raw[m[2]:m[3]]); err == nil {
			w.exitCode = code
		}
		w.endAt = m[0]
		w.done = true
		close(w.doneCh)
	}
}

// output returns what the command printed, without the markers, without the
// echoed command line, and without the escape sequences a PTY carries. The text
// is published on the ticket, so it must read as text and not as a screen
// recording.
func (w *runWatcher) output() string {
	w.mu.Lock()
	raw := w.raw.String()
	start, end, started, done := w.startAt, w.endAt, w.started, w.done
	w.mu.Unlock()

	if !started {
		return ""
	}
	if !done || end < start || end > len(raw) {
		end = len(raw)
	}
	return StripTerminalNoise(raw[start:end])
}

// RunResult is what a workflow step produced in its session.
type RunResult struct {
	Output   string
	ExitCode int
	// IdleStopped is true when the step was stopped because nothing was printed
	// for the idle budget, as opposed to finishing on its own.
	IdleStopped bool
}

// RunCommandInSession injects a command into a session and waits for it, so the
// step runs where the user can watch it and type into it.
//
// It returns ErrSessionBusy when the start marker never shows up: the shell was
// running something else, and the caller must not assume the command ran.
//
// The markers are POSIX shell: printf calls around the command, and $? for the exit
// code. A Windows session runs PowerShell, which reads none of that, so this helper is
// POSIX-only until someone renders the markers per shell.
var ErrSessionBusy = fmt.Errorf("la session de terminal n'est pas au prompt")

func (m *Manager) RunCommandInSession(
	ctx context.Context,
	sessionID string,
	cwd string,
	envVars map[string]string,
	commandLine string,
	idleBudget time.Duration,
) (*RunResult, error) {
	sess, err := m.GetOrCreateSession(sessionID, cwd, envVars)
	if err != nil {
		return nil, err
	}

	runID := fmt.Sprintf("%d", time.Now().UnixNano())
	w := &runWatcher{
		startRe:    regexp.MustCompile(`(?:^|[\r\n])__SECTILE_START_` + runID + `__[\r\n]`),
		endRe:      regexp.MustCompile(`(?:^|[\r\n])__SECTILE_END_` + runID + `:(\d+)__[\r\n]`),
		startedCh:  make(chan struct{}),
		doneCh:     make(chan struct{}),
		lastByteAt: time.Now(),
	}

	sess.watchersMu.Lock()
	sess.watchers[w] = struct{}{}
	sess.watchersMu.Unlock()
	defer func() {
		sess.watchersMu.Lock()
		delete(sess.watchers, w)
		sess.watchersMu.Unlock()
	}()

	// printf plutôt que echo : le marqueur doit sortir tel quel, et le code de
	// sortie est celui de la commande, pas celui du printf de début.
	line := fmt.Sprintf(
		"printf '\\n__SECTILE_START_%s__\\n'; %s; printf '\\n__SECTILE_END_%s:%%s__\\n' \"$?\"\n",
		runID, commandLine, runID,
	)
	if err := m.SendInput(sessionID, line); err != nil {
		return nil, err
	}

	// Le marqueur de début doit apparaître vite : sinon le shell est occupé.
	select {
	case <-w.startedCh:
	case <-time.After(20 * time.Second):
		return nil, ErrSessionBusy
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if idleBudget <= 0 {
		idleBudget = 10 * time.Minute
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-w.doneCh:
			return &RunResult{Output: w.output(), ExitCode: w.exitCode}, nil

		case <-ctx.Done():
			// Annulation par l'utilisateur ou arrêt du serveur : on interrompt la
			// commande dans la session au lieu de laisser l'agent tourner seul.
			_ = m.SendInput(sessionID, "\x03")
			return &RunResult{Output: w.output(), ExitCode: -1}, ctx.Err()

		case <-ticker.C:
			w.mu.Lock()
			silence := time.Since(w.lastByteAt)
			w.mu.Unlock()
			if silence >= idleBudget {
				_ = m.SendInput(sessionID, "\x03")
				return &RunResult{Output: w.output(), ExitCode: -1, IdleStopped: true}, nil
			}
		}
	}
}

var (
	ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b[()#][0-9A-Za-z]|\x1b[=>NOc78]`)
	bracketedRe  = regexp.MustCompile(`\x1b\[\?200[04][hl]`)
)

// StripTerminalNoise turns a PTY stream into readable text: escape sequences
// dropped, carriage-return redraws collapsed to the last state of the line.
// A spinner that rewrote itself two hundred times must not reach the ticket.
func StripTerminalNoise(raw string) string {
	s := bracketedRe.ReplaceAllString(raw, "")
	s = ansiEscapeRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\x07", "")
	s = strings.ReplaceAll(s, "\r\n", "\n")

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		// Un retour chariot seul réécrit la ligne : seul le dernier état compte.
		if idx := strings.LastIndex(line, "\r"); idx >= 0 {
			line = line[idx+1:]
		}
		out = append(out, strings.TrimRight(line, " \t"))
	}

	joined := strings.Join(out, "\n")
	for strings.Contains(joined, "\n\n\n") {
		joined = strings.ReplaceAll(joined, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(joined)
}

// EnsureAgentReady starts the agent CLI in a session and waits until it stops
// printing, which is how a REPL says it is waiting for input.
//
// Le lancement n'a lieu qu'une fois par session : les pas suivants d'un même
// ticket tapent dans l'agent déjà ouvert, ce qui garde la conversation entière
// dans une seule console.
func (m *Manager) EnsureAgentReady(
	ctx context.Context,
	sessionID string,
	cwd string,
	envVars map[string]string,
	launchLine string,
) (bool, error) {
	sess, err := m.GetOrCreateSession(sessionID, cwd, envVars)
	if err != nil {
		return false, err
	}

	sess.agentMu.Lock()
	already := sess.agentLaunched
	sess.agentMu.Unlock()
	if already {
		return false, nil
	}

	if strings.TrimSpace(launchLine) == "" {
		return false, fmt.Errorf("aucune commande de lancement d'agent pour ce moteur")
	}

	if err := m.SendInput(sessionID, launchLine+"\n"); err != nil {
		return false, err
	}
	// L'agent affiche sa bannière puis son invite : on attend que le flux se
	// calme avant d'écrire dedans, sinon la commande part avant l'invite.
	m.waitQuiet(ctx, sess, 1500*time.Millisecond, 25*time.Second)

	sess.agentMu.Lock()
	sess.agentLaunched = true
	sess.agentMu.Unlock()
	return true, nil
}

// AgentLaunched says whether an agent CLI is already running in the session.
func (m *Manager) AgentLaunched(sessionID string) bool {
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok || sess == nil {
		return false
	}
	sess.agentMu.Lock()
	defer sess.agentMu.Unlock()
	return sess.agentLaunched
}

// InjectLine types a line into a session, newline included.
func (m *Manager) InjectLine(sessionID string, line string) error {
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	return m.SendInput(sessionID, line)
}

// waitQuiet returns once the session has printed nothing for quiet, or when cap
// is reached. It is how we wait for a TUI to settle without parsing its prompt,
// which every agent draws differently.
func (m *Manager) waitQuiet(ctx context.Context, sess *Session, quiet, cap time.Duration) {
	w := &runWatcher{
		quietOnly:  true,
		startedCh:  make(chan struct{}),
		doneCh:     make(chan struct{}),
		lastByteAt: time.Now(),
	}

	sess.watchersMu.Lock()
	sess.watchers[w] = struct{}{}
	sess.watchersMu.Unlock()
	defer func() {
		sess.watchersMu.Lock()
		delete(sess.watchers, w)
		sess.watchersMu.Unlock()
	}()

	deadline := time.Now().Add(cap)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.mu.Lock()
			silence := time.Since(w.lastByteAt)
			w.mu.Unlock()
			if silence >= quiet || time.Now().After(deadline) {
				return
			}
		}
	}
}

// RunInAgentSession types a line into an agent already running in the session and
// waits for its turn to end.
//
// Il n'y a ni code de sortie ni marqueur ici : une invite d'agent n'en produit
// pas, et chaque agent dessine la sienne différemment. La fin de tour est donc
// détectée par le silence, ce qui est fiable en pratique parce qu'un agent au
// travail écrit sans arrêt : appels d'outils, avancement, réponse.
func (m *Manager) RunInAgentSession(
	ctx context.Context,
	sessionID string,
	line string,
	turnQuiet time.Duration,
) (*RunResult, error) {
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok || sess == nil || sess.closed {
		return nil, fmt.Errorf("session %s absente", sessionID)
	}
	if turnQuiet <= 0 {
		turnQuiet = 45 * time.Second
	}

	w := &runWatcher{
		startedCh:  make(chan struct{}),
		doneCh:     make(chan struct{}),
		lastByteAt: time.Now(),
	}
	// Capture immédiate : tout ce qui suit l'injection est la réponse de l'agent.
	w.started = true
	close(w.startedCh)
	w.startRe = regexp.MustCompile(`$^`) // ne matche jamais
	w.endRe = regexp.MustCompile(`$^`)

	sess.watchersMu.Lock()
	sess.watchers[w] = struct{}{}
	sess.watchersMu.Unlock()
	defer func() {
		sess.watchersMu.Lock()
		delete(sess.watchers, w)
		sess.watchersMu.Unlock()
	}()

	injectedAt := time.Now()
	if err := m.InjectLine(sessionID, line); err != nil {
		return nil, err
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return &RunResult{Output: w.output(), ExitCode: -1}, ctx.Err()
		case <-ticker.C:
			w.mu.Lock()
			silence := time.Since(w.lastByteAt)
			printed := w.raw.Len()
			w.mu.Unlock()

			if silence < turnQuiet {
				continue
			}
			// Rien d'écrit du tout : l'agent n'a pas réagi à la ligne.
			if printed == 0 && time.Since(injectedAt) >= turnQuiet {
				return &RunResult{Output: "", ExitCode: -1, IdleStopped: true}, nil
			}
			// L'invite de l'agent réaffiche la ligne tapée : on retire cet écho
			// pour que le compte-rendu publié commence à la réponse.
			out := stripFirstOccurrence(w.output(), strings.TrimSpace(line))
			return &RunResult{Output: out, ExitCode: 0}, nil
		}
	}
}

// stripFirstOccurrence removes the echoed prompt line from a captured turn.
func stripFirstOccurrence(text, needle string) string {
	if needle == "" {
		return text
	}
	if idx := strings.Index(text, needle); idx >= 0 {
		text = text[:idx] + text[idx+len(needle):]
	}
	return strings.TrimSpace(text)
}

// ForgetAgent clears the "an agent runs here" flag, so the next launch really
// starts one. Needed when the user quit the agent by hand: the session is still
// alive, but its shell is back at the prompt.
func (m *Manager) ForgetAgent(sessionID string) {
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok || sess == nil {
		return
	}
	sess.agentMu.Lock()
	sess.agentLaunched = false
	sess.agentMu.Unlock()
}
