package agenthook

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recorder stands in for the local agent: it accepts any POST and hands the
// path and the body to the test.
type recorder struct {
	URL      string
	received chan request
}

type request struct{ path, body string }

func startRecorder(t *testing.T) *recorder {
	t.Helper()
	rec := &recorder{received: make(chan request, 4)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		select {
		case rec.received <- request{path: r.URL.Path, body: string(raw)}:
		default:
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)
	rec.URL = server.URL
	return rec
}

// one returns the single report the hook made, and fails when it made none or
// more than one.
func (rec *recorder) one(t *testing.T) request {
	t.Helper()
	var got request
	select {
	case got = <-rec.received:
	default:
		t.Fatal("no report reached the agent")
	}
	select {
	case extra := <-rec.received:
		t.Fatalf("a second report was made: %s", extra.body)
	default:
	}
	return got
}

func (rec *recorder) none(t *testing.T) {
	t.Helper()
	select {
	case got := <-rec.received:
		t.Fatalf("a report was made where none was due: %s %s", got.path, got.body)
	default:
	}
}

// launched is the environment of a session Sectile launched, pointed at a
// local agent.
func launched(loopback string) func(string) string {
	values := map[string]string{"SECTILE_RUN_ID": "run-1", "SECTILE_LOOPBACK_URL": loopback, "SECTILE_AGENT_TOKEN": "key"}
	return func(name string) string { return values[name] }
}

// unlaunched is the environment of a session Sectile did not launch: no run,
// nothing to report against but the workstation's own connection file.
func unlaunched(string) string { return "" }

// pairWorkstation publishes the connection file a session without a run reads,
// and returns the home directory holding it.
func pairWorkstation(t *testing.T, server string) string {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".taskflow"), 0700); err != nil {
		t.Fatal(err)
	}
	connection := `{"url":"` + server + `","token":"companion-token"}`
	if err := os.WriteFile(filepath.Join(home, ".taskflow", "agent-connection.json"), []byte(connection), 0600); err != nil {
		t.Fatal(err)
	}
	return home
}

// The state a launched session reports is decided by the event alone. A wait
// opens on a prompt and on the end of a turn — the agent then awaits the next
// prompt — and closes on anything that only happens while the agent works.
func TestLaunchedSessionReportsTheRunStateForEachEvent(t *testing.T) {
	for _, test := range []struct {
		event   string
		waiting bool
	}{
		{event: "Notification", waiting: true},
		{event: "Stop", waiting: true},
		{event: "UserPromptSubmit"},
		{event: "PreToolUse"},
		{event: "PostToolUse"},
	} {
		t.Run(test.event, func(t *testing.T) {
			rec := startRecorder(t)
			payload := `{"hook_event_name":"` + test.event + `","cwd":"/tmp/w","tool_name":"Bash","tool_input":{"command":"ls"}}`
			Report([]byte(payload), launched(rec.URL), t.TempDir())
			got := rec.one(t)
			if got.path != "/control/runs/run-1/waiting" {
				t.Fatalf("the run report went to %s", got.path)
			}
			var body struct {
				Waiting *bool `json:"waiting"`
			}
			if err := json.Unmarshal([]byte(got.body), &body); err != nil || body.Waiting == nil {
				t.Fatalf("the agent could not read the report %q: %v", got.body, err)
			}
			if *body.Waiting != test.waiting {
				t.Fatalf("%s reported waiting=%v, expected %v", test.event, *body.Waiting, test.waiting)
			}
		})
	}
}

// Claude Code notifies for more than prompts. Only a prompt means the session
// waits; a payload naming no type comes from an older Claude Code and is read
// as a prompt, which is what every payload was before the type existed.
func TestOnlyAPromptIsAWait(t *testing.T) {
	for _, test := range []struct {
		kind    string
		waiting bool
	}{
		{kind: "permission_prompt", waiting: true},
		{kind: "idle_prompt", waiting: true},
		{kind: "elicitation_dialog", waiting: true},
		{kind: "elicitation_url_dialog", waiting: true},
		{kind: "agent_needs_input", waiting: true},
		{kind: "", waiting: true},
		{kind: "auth_success"},
		{kind: "agent_completed"},
		{kind: "quota_auto_resume_fired"},
	} {
		name := test.kind
		if name == "" {
			name = "no type"
		}
		t.Run(name, func(t *testing.T) {
			rec := startRecorder(t)
			payload := `{"hook_event_name":"Notification","cwd":"/tmp/w","message":"hello"`
			if test.kind != "" {
				payload += `,"notification_type":"` + test.kind + `"`
			}
			payload += "}"
			Report([]byte(payload), launched(rec.URL), t.TempDir())
			if !test.waiting {
				rec.none(t)
				return
			}
			if body := rec.one(t).body; !strings.Contains(body, `"waiting":true`) {
				t.Fatalf("a prompt did not open a wait: %s", body)
			}
		})
	}
}

// A session Sectile did not launch reports itself through the connection file
// the agent publishes for its companions, naming itself by its directory. Only
// the two events that deserve a banner are announced: resuming is not one.
func TestUnlaunchedSessionAnnouncesItselfByItsDirectory(t *testing.T) {
	for _, test := range []struct {
		event string
		state string
	}{
		{event: "Notification", state: "waiting"},
		{event: "Stop", state: "completed"},
		{event: "UserPromptSubmit"},
		{event: "PreToolUse"},
		{event: "PostToolUse"},
	} {
		t.Run(test.event, func(t *testing.T) {
			rec := startRecorder(t)
			home := pairWorkstation(t, rec.URL)
			Report([]byte(`{"hook_event_name":"`+test.event+`","cwd":"/Users/x/worktrees/#174"}`), unlaunched, home)
			if test.state == "" {
				rec.none(t)
				return
			}
			got := rec.one(t)
			if got.path != "/desktop/session-alert" {
				t.Fatalf("the alert went to %s", got.path)
			}
			if !strings.Contains(got.body, `"session":"#174"`) {
				t.Fatalf("the session is not named by its directory: %s", got.body)
			}
			if !strings.Contains(got.body, `"state":"`+test.state+`"`) {
				t.Fatalf("the announced state is wrong: %s", got.body)
			}
		})
	}
}

// The defect this change exists for, on the reporting side: a Windows payload
// carries a path with backslashes. The script this replaced ran basename on it,
// got the whole string back, and then stripped the backslashes to protect the
// JSON, announcing the session as C:gitsectile. A name that would end the JSON
// string is escaped by the encoder instead of being thrown away.
func TestTheSessionNameSurvivesEveryPathShape(t *testing.T) {
	for _, test := range []struct {
		name string
		cwd  string
		want string
	}{
		{name: "a POSIX worktree", cwd: "/Users/x/worktrees/#174", want: "#174"},
		{name: "a POSIX path with a trailing separator", cwd: "/Users/x/sectile/", want: "sectile"},
		{name: "a Windows path", cwd: `C:\git\sectile`, want: "sectile"},
		{name: "a Windows worktree", cwd: `C:\git\sectile\.tasks\worktrees\#260`, want: "#260"},
		{name: "a name holding a quote", cwd: `/tmp/od"d`, want: `od"d`},
		{name: "no directory at all", cwd: "", want: fallbackSessionName},
		{name: "a root directory", cwd: "/", want: fallbackSessionName},
	} {
		t.Run(test.name, func(t *testing.T) {
			// filepath.Base only reads the backslash as a separator on Windows,
			// which is the only host where a payload carries one.
			want := test.want
			if strings.ContainsRune(test.cwd, '\\') && filepath.Separator != '\\' {
				want = filepath.Base(test.cwd)
			}
			rec := startRecorder(t)
			home := pairWorkstation(t, rec.URL)
			Report([]byte(`{"hook_event_name":"Stop","cwd":`+mustJSON(t, test.cwd)+`}`), unlaunched, home)
			var body struct {
				Session string `json:"session"`
			}
			raw := rec.one(t).body
			if err := json.Unmarshal([]byte(raw), &body); err != nil {
				t.Fatalf("the agent could not read the alert %q: %v", raw, err)
			}
			if body.Session != want {
				t.Fatalf("the session was announced as %q, expected %q", body.Session, want)
			}
		})
	}
}

// A session Sectile did launch reports against its run, and must not also file
// a session-level alert: that would announce the same session twice.
func TestALaunchedSessionFilesNoAlert(t *testing.T) {
	rec := startRecorder(t)
	home := pairWorkstation(t, rec.URL)
	Report([]byte(`{"hook_event_name":"Stop","cwd":"/tmp/w"}`), launched(rec.URL), home)
	got := rec.one(t)
	if got.path != "/control/runs/run-1/waiting" {
		t.Fatalf("a launched session reported to %s", got.path)
	}
}

// A workstation whose agent published nothing has no companion to report to.
// That is silence, not a failure.
func TestNoConnectionFileIsNotAFailure(t *testing.T) {
	rec := startRecorder(t)
	Report([]byte(`{"hook_event_name":"Stop","cwd":"/tmp/w"}`), unlaunched, t.TempDir())
	rec.none(t)
}

// Every path through the hook is checked for the same thing: it returns. The
// exit code the agent reads back is the process's, and nothing here can make
// it non-zero, because nothing here returns an error.
func TestTheHookAlwaysReturnsAndReportsNothingItCannotRead(t *testing.T) {
	for _, test := range []struct {
		name    string
		payload string
		env     func(string) string
	}{
		{name: "unparseable payload", payload: "this is not JSON at all"},
		{name: "empty payload", payload: ""},
		{name: "a JSON array", payload: `["Stop"]`},
		{name: "payload with no event", payload: `{"session_id":"abc","cwd":"/tmp/work"}`},
		{name: "event the hook does not answer", payload: `{"hook_event_name":"SessionStart"}`},
		{name: "a notification that is not a prompt", payload: `{"hook_event_name":"Notification","notification_type":"auth_success"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			rec := startRecorder(t)
			env := test.env
			if env == nil {
				env = launched(rec.URL)
			}
			Report([]byte(test.payload), env, pairWorkstation(t, rec.URL))
			rec.none(t)
		})
	}
}

// A run identity with no local agent to report to, and a local agent that
// refuses the connection, both end the same way: nothing happens, and the
// session carries on.
func TestAnUnreachableAgentIsSwallowed(t *testing.T) {
	// Port 1 refuses immediately, which is the stopped-agent case.
	dead := launched("http://127.0.0.1:1")
	for _, payload := range []string{
		`{"hook_event_name":"Notification","cwd":"/tmp/work"}`,
		`{"hook_event_name":"PreToolUse","cwd":"/tmp/work"}`,
		`{"hook_event_name":"Stop"}`,
	} {
		Report([]byte(payload), dead, t.TempDir())
	}
}

// A hook must never make a session wait on the network. An agent that accepts
// the connection and never answers is abandoned, not waited on.
func TestAnUnresponsiveAgentIsAbandoned(t *testing.T) {
	blocked := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-blocked }))
	defer func() { close(blocked); server.Close() }()

	done := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		Report([]byte(`{"hook_event_name":"Stop"}`), launched(server.URL), t.TempDir())
		done <- time.Since(start)
	}()
	select {
	case elapsed := <-done:
		if elapsed < reportTimeout {
			t.Fatalf("the report returned in %s, before the timeout it was given", elapsed)
		}
	case <-time.After(reportTimeout * 4):
		t.Fatal("the hook is still waiting on an agent that never answers")
	}
}

// SECTILE_AGENT_URL is the server on a launched session, and the loopback in
// the MCP bridge and some tests. The loopback has a name of its own and wins;
// the older name is the fallback, so a session launched before it existed
// still reports.
func TestTheLoopbackAddressWinsOverTheOlderName(t *testing.T) {
	loopback, server := startRecorder(t), startRecorder(t)
	env := map[string]string{
		"SECTILE_RUN_ID":       "run-1",
		"SECTILE_LOOPBACK_URL": loopback.URL,
		"SECTILE_AGENT_URL":    server.URL,
		"SECTILE_AGENT_TOKEN":  "key",
	}
	Report([]byte(`{"hook_event_name":"Stop"}`), func(name string) string { return env[name] }, t.TempDir())
	loopback.one(t)
	server.none(t)

	delete(env, "SECTILE_LOOPBACK_URL")
	Report([]byte(`{"hook_event_name":"Stop"}`), func(name string) string { return env[name] }, t.TempDir())
	server.one(t)
}

// An incomplete launched environment is not a launched session: it falls
// through to the companion connection rather than reporting nowhere.
func TestAnIncompleteRunEnvironmentFallsBackToTheCompanion(t *testing.T) {
	for _, missing := range []string{"SECTILE_RUN_ID", "SECTILE_LOOPBACK_URL", "SECTILE_AGENT_TOKEN"} {
		t.Run("without "+missing, func(t *testing.T) {
			rec := startRecorder(t)
			values := map[string]string{"SECTILE_RUN_ID": "run-1", "SECTILE_LOOPBACK_URL": rec.URL, "SECTILE_AGENT_TOKEN": "key"}
			delete(values, missing)
			home := pairWorkstation(t, rec.URL)
			Report([]byte(`{"hook_event_name":"Stop","cwd":"/tmp/work"}`), func(name string) string { return values[name] }, home)
			if got := rec.one(t); got.path != "/desktop/session-alert" {
				t.Fatalf("an incomplete run environment reported to %s", got.path)
			}
		})
	}
}

// The reports carry the credential the receiving endpoint checks: the
// workstation API key for a run, the companion token for a session alert.
func TestEachReportCarriesItsOwnCredential(t *testing.T) {
	seen := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	Report([]byte(`{"hook_event_name":"Stop"}`), launched(server.URL), t.TempDir())
	if got := <-seen; got != "Bearer key" {
		t.Fatalf("the run report carried %q", got)
	}
	Report([]byte(`{"hook_event_name":"Stop","cwd":"/tmp/w"}`), unlaunched, pairWorkstation(t, server.URL))
	if got := <-seen; got != "Bearer companion-token" {
		t.Fatalf("the session alert carried %q", got)
	}
}

// A connection file that is truncated, unreadable or missing its fields leaves
// the session unannounced rather than reporting to an address nobody serves.
func TestAnUnusableConnectionFileIsIgnored(t *testing.T) {
	for _, test := range []struct{ name, content string }{
		{name: "not JSON", content: "{ this is not JSON"},
		{name: "no url", content: `{"token":"companion-token"}`},
		{name: "no token", content: `{"url":"http://127.0.0.1:9"}`},
		{name: "empty", content: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			if err := os.MkdirAll(filepath.Join(home, ".taskflow"), 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(home, ".taskflow", "agent-connection.json")
			if err := os.WriteFile(path, []byte(test.content), 0600); err != nil {
				t.Fatal(err)
			}
			Report([]byte(`{"hook_event_name":"Stop","cwd":"/tmp/w"}`), unlaunched, home)
		})
	}
}

func mustJSON(t *testing.T, value string) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
