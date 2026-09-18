// Package agenthook answers the Claude Code hooks Sectile registers. It is a
// subcommand of the agent binary rather than the shell script it replaces:
// Claude Code hands the registered command to the host shell, and on Windows
// that shell is cmd.exe, which resolves a .sh file through its file association
// instead of running it — a detached Git Bash window on a workstation that has
// one, and "not recognized as an internal or external command" on one that does
// not. An executable is a program under every launcher, and it needs neither a
// shell nor curl, sed, basename or a $HOME the shell may not set.
//
// One subcommand answers every event that moves a session between working and
// waiting for the user. The payload names the event, and the table below turns
// it into a state:
//
//	Notification  a permission prompt, an idle prompt, an elicitation  -> waiting
//	Stop          the turn ended, the agent awaits the next prompt     -> waiting
//	UserPromptSubmit, PreToolUse, PostToolUse  the agent is working    -> working
//
// It raises no alert of its own: it reports the state to the local agent, and
// the desktop application turns that into a real system notification.
//
// Two rules govern everything below: never write on stdout, which the agent
// reads back, and never fail, which is the difference between a report and an
// interrupted session. That is why nothing here returns an error.
package agenthook

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// The two states a launched run reports, and the alert an unlaunched session
// announces when its turn ends. A wait announces itself under its own name.
const (
	stateWaiting   = "waiting"
	stateWorking   = "working"
	alertCompleted = "completed"
)

// fallbackSessionName names a session whose working directory the payload does
// not carry. The absence of a name is no reason to lose the banner.
const fallbackSessionName = "Claude Code"

// reportTimeout is the whole network budget of one hook invocation. A hook must
// never make a session wait on the network.
const reportTimeout = 2 * time.Second

// payloadTimeout bounds the wait on stdin. Claude Code writes the payload and
// closes the pipe, so this is never reached in practice; it exists because a
// hook that blocked on a pipe nobody closed would freeze the turn, and the
// signals that would otherwise free it are ignored below.
const payloadTimeout = 5 * time.Second

// hookPayload is the part of a Claude Code hook payload this hook reads. Three
// fields decide the whole report; everything else in the payload is ignored.
type hookPayload struct {
	Event            string `json:"hook_event_name"`
	NotificationType string `json:"notification_type"`
	Cwd              string `json:"cwd"`
}

// Run is the `sectile-hook` subcommand of the agent binary. It takes no flags:
// Claude Code passes the payload on stdin and reads the exit code back, so a
// flag parser's usage message would be read as the hook's answer.
func Run() {
	// A hook is a child of the session's process group, so a Ctrl-C in the
	// console reaches it too. The script this replaced trapped HUP, INT and
	// TERM and exited 0 for exactly that reason: a hook killed by a signal
	// exits non-zero, which Claude Code reads as a failed hook. Go's default
	// is to die on those signals, so they are ignored instead.
	signal.Ignore(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	payload := readPayload(os.Stdin, payloadTimeout)
	if payload == nil {
		return
	}
	// os.UserHomeDir reads USERPROFILE on Windows and $HOME elsewhere, so the
	// connection file resolves without the shell having set anything.
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	Report(payload, os.Getenv, home)
}

// readPayload reads the whole payload, or gives up. The read runs in a
// goroutine the process simply abandons: there is nothing to clean up in a
// program whose next act is to exit.
func readPayload(stdin io.Reader, budget time.Duration) []byte {
	read := make(chan []byte, 1)
	go func() {
		raw, err := io.ReadAll(stdin)
		if err != nil {
			raw = nil
		}
		read <- raw
	}()
	select {
	case raw := <-read:
		return raw
	case <-time.After(budget):
		return nil
	}
}

// Report reads one hook payload and makes the single report it calls for. The
// environment and the home directory are taken as parameters so the whole
// behaviour is exercised in-process, on any host: the script this replaced
// could only be tested by spawning /bin/sh, which is why its Windows defect
// went unseen.
func Report(payload []byte, getenv func(string) string, home string) {
	var read hookPayload
	if err := json.Unmarshal(payload, &read); err != nil {
		return
	}
	state, alert := classify(read)
	if state == "" && alert == "" {
		return
	}

	// A session Sectile launched carries its run, and reports against it: the
	// run is marked as waiting, or as working again, and the whole UI follows.
	// The agent decides what the report means for the run — an autonomous run,
	// for one, never waits — and relays it only when the state actually
	// changes. SECTILE_AGENT_URL is the server on a launched session, so the
	// loopback address has a name of its own and is preferred.
	loopback := getenv("SECTILE_LOOPBACK_URL")
	if loopback == "" {
		loopback = getenv("SECTILE_AGENT_URL")
	}
	runID, token := getenv("SECTILE_RUN_ID"), getenv("SECTILE_AGENT_TOKEN")
	if runID != "" && loopback != "" && token != "" {
		post(endpoint(loopback, "/control/runs/"+runID+"/waiting"), token, map[string]bool{"waiting": state == stateWaiting})
		return
	}

	// Any other Claude Code session still deserves the banner — the whole point
	// is telling several sessions apart. It has no run, so it reports itself by
	// name, using the connection the agent publishes for its companions. Only a
	// start of wait and an end of turn are announced: a banner is a transition,
	// and resuming is not one anybody needs told about.
	if alert == "" || home == "" {
		return
	}
	url, companion := connection(filepath.Join(home, ".taskflow", "agent-connection.json"))
	if url == "" || companion == "" {
		return
	}
	post(endpoint(url, "/desktop/session-alert"), companion, map[string]string{"session": sessionName(read.Cwd), "state": alert})
}

// classify turns an event into the two reports it may produce. The state is
// what a session carrying a run reports; the alert is what a session with no
// run announces about itself. An event outside the table produces neither.
func classify(payload hookPayload) (state, alert string) {
	switch payload.Event {
	case "Notification":
		// Claude Code notifies for more than prompts: a sign-in, a quota
		// notice, a finished sub-agent. Only a prompt means the session waits.
		// A payload with no type comes from an older Claude Code and is read as
		// a prompt, which is what every payload was before the type existed.
		switch payload.NotificationType {
		case "", "permission_prompt", "idle_prompt", "elicitation_dialog", "elicitation_url_dialog", "agent_needs_input":
			return stateWaiting, stateWaiting
		}
		return "", ""
	case "Stop":
		return stateWaiting, alertCompleted
	case "UserPromptSubmit", "PreToolUse", "PostToolUse":
		return stateWorking, ""
	}
	return "", ""
}

// sessionName names the session by the last element of its working directory.
// filepath.Base reads the backslash on Windows, where the payload carries a
// path such as C:\git\sectile; the script this replaced ran basename on it, got
// the whole string back, and then stripped its backslashes to protect the JSON,
// which announced the session as C:gitsectile. Nothing is stripped here: the
// name reaches the wire through json.Marshal, which escapes what it contains.
func sessionName(cwd string) string {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return fallbackSessionName
	}
	name := strings.TrimSpace(filepath.Base(cwd))
	if strings.Trim(name, `./\`) == "" {
		return fallbackSessionName
	}
	return name
}

// connection reads the loopback address and the companion credential the agent
// publishes for the sessions it did not launch.
func connection(path string) (url, token string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	var info struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return "", ""
	}
	return info.URL, info.Token
}

// endpoint joins a base address published by the agent with a path, tolerating
// the trailing slash a hand-written address may carry.
func endpoint(base, path string) string { return strings.TrimSuffix(base, "/") + path }

// post makes the one report and forgets it. Every failure is swallowed: a hook
// that reported a network error would report it to nobody, and the session it
// interrupted would be the only visible consequence.
func post(url, token string, body any) {
	raw, err := json.Marshal(body)
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: reportTimeout}).Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}
