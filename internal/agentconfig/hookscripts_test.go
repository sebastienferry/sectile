package agentconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runHook executes a managed hook script the way Claude Code does: payload on
// stdin, environment supplied, output read back. It returns stdout, which must
// always be empty, and the exit code, which must always be 0.
func runHook(t *testing.T, source, payload string, env map[string]string, home string) (string, int) {
	t.Helper()
	raw, err := hookScripts.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), filepath.Base(source))
	if err := os.WriteFile(script, raw, 0700); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("/bin/sh", script)
	command.Stdin = strings.NewReader(payload)
	// A clean environment, not the test process's: these tests would otherwise
	// inherit the SECTILE_* variables of whatever session runs them, and a hook
	// reads exactly those to decide which report it makes.
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home}
	for key, value := range env {
		command.Env = append(command.Env, key+"="+value)
	}
	var stdout strings.Builder
	command.Stdout = &stdout
	_ = command.Run()
	return stdout.String(), command.ProcessState.ExitCode()
}

// Every path through a hook is checked for the same two things, because they are
// what separates a notification from an interrupted session: the agent reads the
// exit code, and it reads standard output.
func TestHooksAlwaysSucceedAndStaySilent(t *testing.T) {
	for _, source := range []string{"hooks/notification.sh", "hooks/stop.sh"} {
		for _, test := range []struct {
			name    string
			payload string
			env     map[string]string
		}{
			{name: "unparseable payload", payload: "this is not JSON at all"},
			{name: "empty payload", payload: ""},
			{name: "no connection and no run", payload: `{"cwd":"/tmp/work"}`},
			{
				name:    "dead loopback",
				payload: `{"cwd":"/tmp/work"}`,
				// Port 1 refuses immediately, which is the stopped-agent case.
				env: map[string]string{"SECTILE_RUN_ID": "run-1", "SECTILE_LOOPBACK_URL": "http://127.0.0.1:1", "SECTILE_AGENT_TOKEN": "key"},
			},
			{
				name:    "payload with no working directory",
				payload: `{"session_id":"abc"}`,
				env:     map[string]string{"SECTILE_RUN_ID": "run-1", "SECTILE_LOOPBACK_URL": "http://127.0.0.1:1", "SECTILE_AGENT_TOKEN": "key"},
			},
		} {
			t.Run(filepath.Base(source)+"/"+test.name, func(t *testing.T) {
				stdout, code := runHook(t, source, test.payload, test.env, t.TempDir())
				if code != 0 {
					t.Fatalf("exit code %d would be read back by the agent", code)
				}
				if stdout != "" {
					t.Fatalf("the hook wrote %q on stdout, which the agent reads back", stdout)
				}
			})
		}
	}
}

// A session Sectile did not launch reports itself through the connection file
// the agent publishes for its companions, naming itself by its directory.
func TestHookReportsAForeignSessionByItsDirectory(t *testing.T) {
	received := make(chan string, 4)
	server := startJSONRecorder(t, received)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".taskflow"), 0700); err != nil {
		t.Fatal(err)
	}
	connection := `{"url":"` + server + `","token":"companion-token"}`
	if err := os.WriteFile(filepath.Join(home, ".taskflow/agent-connection.json"), []byte(connection), 0600); err != nil {
		t.Fatal(err)
	}

	stdout, code := runHook(t, "hooks/notification.sh", `{"cwd":"/Users/x/worktrees/#174"}`, nil, home)
	if code != 0 || stdout != "" {
		t.Fatalf("exit %d, stdout %q", code, stdout)
	}
	select {
	case body := <-received:
		if !strings.Contains(body, `"session":"#174"`) {
			t.Fatalf("the session is not named by its directory: %s", body)
		}
		if !strings.Contains(body, `"state":"waiting"`) {
			t.Fatalf("the reported state is wrong: %s", body)
		}
	default:
		t.Fatal("no report reached the agent, so the session would never be announced")
	}
}

// A session Sectile did launch reports against its run, and must not also file
// a session-level alert: that would announce the same session twice.
func TestHookWithARunReportsOnlyTheRun(t *testing.T) {
	received := make(chan string, 4)
	server := startJSONRecorder(t, received)
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".taskflow"), 0700); err != nil {
		t.Fatal(err)
	}
	connection := `{"url":"` + server + `","token":"companion-token"}`
	if err := os.WriteFile(filepath.Join(home, ".taskflow/agent-connection.json"), []byte(connection), 0600); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{"SECTILE_RUN_ID": "run-1", "SECTILE_LOOPBACK_URL": server, "SECTILE_AGENT_TOKEN": "key"}
	if _, code := runHook(t, "hooks/notification.sh", `{"cwd":"/tmp/w"}`, env, home); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if body := <-received; !strings.Contains(body, `"waiting":true`) {
		t.Fatalf("the run report is wrong: %s", body)
	}
	select {
	case body := <-received:
		t.Fatalf("the session was announced twice: %s", body)
	default:
	}
}

// The Stop hook is the symmetric report: it clears the wait.
func TestStopHookClearsTheWait(t *testing.T) {
	received := make(chan string, 4)
	server := startJSONRecorder(t, received)
	env := map[string]string{"SECTILE_RUN_ID": "run-1", "SECTILE_LOOPBACK_URL": server, "SECTILE_AGENT_TOKEN": "key"}
	if _, code := runHook(t, "hooks/stop.sh", `{"cwd":"/tmp/w"}`, env, t.TempDir()); code != 0 {
		t.Fatalf("exit %d", code)
	}
	if body := <-received; !strings.Contains(body, `"waiting":false`) {
		t.Fatalf("the stop report is wrong: %s", body)
	}
}
