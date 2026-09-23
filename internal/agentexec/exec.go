// Package agentexec supervises a command the agent owns from the terminal side.
// It runs in the process the user's terminal launched, never in the daemon, so
// it holds no daemon state and reports the run's fate over the loopback control
// endpoint it was handed on the command line.
package agentexec

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func mustJSON(v any) string { raw, _ := json.Marshal(v); return string(raw) }

// runAgentExec is the terminal-side supervisor for an agent-owned command.
func Run(args []string) error {
	flags := flag.NewFlagSet("agent-exec", flag.ContinueOnError)
	endpoint := flags.String("url", "", "Local run control endpoint")
	token := flags.String("token", "", "Run control token")
	command := flags.String("command", "", "Command")
	// A batch line carries no newline, and skill prompts are multi-line; Windows passes the
	// command encoded rather than trying to escape it for cmd.exe.
	encoded := flags.String("command-base64", "", "Command, base64 encoded")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *encoded != "" {
		raw, err := base64.StdEncoding.DecodeString(*encoded)
		if err != nil {
			return fmt.Errorf("invalid --command-base64: %w", err)
		}
		*command = string(raw)
	}
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	// A terminal hangup must not bypass child cleanup and the exit report.
	interrupts, stopSignals := terminationSignals()
	defer stopSignals()
	exitStatus := "failed"
	control := func(method string) (bool, error) {
		req, err := http.NewRequest(method, *endpoint, strings.NewReader(mustJSON(map[string]string{"status": exitStatus})))
		if err != nil {
			return false, err
		}
		req.Header.Set("Authorization", "Bearer "+*token)
		resp, err := client.Do(req)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return false, fmt.Errorf("run control returned %d", resp.StatusCode)
		}
		var state struct {
			Canceled bool `json:"canceled"`
		}
		err = json.NewDecoder(resp.Body).Decode(&state)
		return state.Canceled, err
	}
	reportExit := func() error {
		var err error
		for attempt := 0; attempt < 3; attempt++ {
			if _, err = control(http.MethodPost); err == nil {
				return nil
			}
			if attempt < 2 {
				time.Sleep(100 * time.Millisecond)
			}
		}
		return fmt.Errorf("could not confirm process exit: %w", err)
	}
	canceled, err := control(http.MethodGet)
	if err != nil {
		return err
	}
	if canceled {
		_ = reportExit()
		return fmt.Errorf("execution canceled before launch")
	}
	cmd := exec.Command("bash", "-lc", *command)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	restore, err := StartControlled(cmd)
	if err != nil {
		_ = reportExit()
		return err
	}
	defer restore()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stopAndReport := func() error {
		StopControlled(cmd, false)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			StopControlled(cmd, true)
			<-done
		}
		// Reap the child before acknowledging exit, then remove any descendants.
		StopControlled(cmd, true)
		return reportExit()
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			if err == nil {
				exitStatus = "completed"
			}
			if reportErr := reportExit(); reportErr != nil {
				return reportErr
			}
			return err
		case sig := <-interrupts:
			if err := stopAndReport(); err != nil {
				return err
			}
			return fmt.Errorf("execution interrupted by %s", sig)
		case <-ticker.C:
			canceled, err := control(http.MethodGet)
			if err != nil {
				continue
			}
			if !canceled {
				continue
			}
			if err := stopAndReport(); err != nil {
				return err
			}
			return fmt.Errorf("execution canceled")
		}
	}
}
