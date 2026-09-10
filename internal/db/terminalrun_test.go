package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"tasks/internal/models"
)

// Unexpected operations panic through the embedded interface: these tests must
// send a skill to the existing agent, never spawn a second shell or agent.
type skillRecordingTerminal struct {
	TerminalSessionRunner
	running bool
	line    string
	session string
}

func (r *skillRecordingTerminal) AgentLaunched(string) bool { return r.running }
func (r *skillRecordingTerminal) InjectLine(session, line string) error {
	r.session, r.line = session, line
	return nil
}
func (r *skillRecordingTerminal) RunInAgentSession(_ context.Context, session, line string, _ time.Duration) (string, int, bool, error) {
	r.session, r.line = session, line
	return "done", 0, false, nil
}

func TestPTYSkillUsesEffectiveProviderAndProjectName(t *testing.T) {
	for _, tc := range []struct{ name, global, project, override, command string }{
		{"project Codex", "claude", "codex", "", "clarify-issue"},
		{"global Codex", "codex", "", "", "clarify-issue"},
		{"project Claude", "codex", "claude", "", "/clarify-issue"},
		{"renamed Codex", "claude", " Codex ", "/clarify-workitem", "clarify-workitem"},
		{"renamed Claude", "codex", "claude", "clarify-workitem", "/clarify-workitem"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, cleanup := setupTestDB(t)
			defer cleanup()
			settings, err := d.GetSettings()
			if err != nil {
				t.Fatal(err)
			}
			settings.AIProvider = tc.global
			if _, err = d.UpdateSettings(*settings); err != nil {
				t.Fatal(err)
			}
			project, err := d.CreateProject(models.CreateProjectRequest{
				Name: "PTY test", Slug: "pty-test", IssueTracker: "local", AIProvider: tc.project,
				SkillOverrides: map[string]string{"clarify": tc.override},
			})
			if err != nil {
				t.Fatal(err)
			}
			task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: project.ID, Title: "Codex\ncase PTY", Source: "local"})
			if err != nil {
				t.Fatal(err)
			}
			terminal := &skillRecordingTerminal{running: true}
			d.SetTerminalRunner(terminal)
			want := fmt.Sprintf("%s %s (Codex case PTY)", tc.command, task.Key)
			launch, err := d.InjectSkillInTTY(task.ID, "clarify")
			if err != nil {
				t.Fatal(err)
			}
			if terminal.line != want || launch.Call != want || terminal.session != TaskSessionID(task.ID) {
				t.Fatalf("manual invocation: terminal=%+v launch=%+v want=%q", terminal, launch, want)
			}
			terminal.line = ""
			d.applyProjectSettings(settings, task, "clarify")
			out, _, err := d.runSkillInSession(context.Background(), settings, "clarify", task, "", "")
			if err != nil {
				t.Fatal(err)
			}
			if terminal.line != want || out != "done" || terminal.session != TaskSessionID(task.ID) {
				t.Fatalf("workflow invocation: terminal=%+v output=%q want=%q", terminal, out, want)
			}
			terminal.running, terminal.line = false, ""
			if _, err := d.InjectSkillInTTY(task.ID, "clarify"); err == nil || terminal.line != "" {
				t.Fatalf("missing agent must reject injection: line=%q err=%v", terminal.line, err)
			}
		})
	}
}
