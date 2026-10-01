package agent

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/terminal"
	"tasks/internal/testhome"
)

func TestClaudePromptPathQuotesOnlyWhenNeeded(t *testing.T) {
	for path, want := range map[string]string{
		"/a/b":       "/a/b",
		"/a/b c":     `"/a/b c"`,
		`/a/"b" c`:   `"/a/\"b\" c"`,
		`/a\b c`:     `"/a\\b c"`,
		"/a/é-ü_x.y": "/a/é-ü_x.y",
	} {
		if got := claudePromptPath(path); got != want {
			t.Errorf("%q: %q, want %q", path, got, want)
		}
	}
	for _, path := range []string{"", "/a\nb", "/a\tb", "/a\x7fb", "/a\rb", "/a\x1b[2Jb"} {
		if typeablePath(path) {
			t.Errorf("%q is typeable", path)
		}
	}
	if !typeablePath("/a/b c") {
		t.Error("a path with a space must be typeable")
	}
}

func TestDiscussionProviderIsRecordedForADiscussionOnly(t *testing.T) {
	for _, tt := range []struct {
		config agentconfig.Config
		skill  string
		want   string
	}{
		{agentconfig.Config{AIProvider: "Claude"}, "discuss", "claude"},
		{agentconfig.Config{AIProvider: "codex"}, "discuss", "codex"},
		{agentconfig.Config{}, "discuss", "agy"},
		{agentconfig.Config{AIProvider: "custom", AICommandTemplate: "my-cli {prompt}"}, "discuss", "custom"},
		{agentconfig.Config{AIProvider: "claude"}, "implement", ""},
	} {
		if got := discussionProvider(tt.config, tt.skill); got != tt.want {
			t.Errorf("%+v %s: %q, want %q", tt.config, tt.skill, got, tt.want)
		}
	}
}

// A folder attached from a run joins the project like one attached from the
// settings; a Claude Code discussion has /add-dir typed into it, once, and
// nothing else is typed anywhere (#676).
func TestAttachingAFolderFromARun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the sessions run a POSIX shell")
	}
	testhome.Temp(t)
	quiet, limit := runFolderQuiet, runFolderQuietCap
	runFolderQuiet, runFolderQuietCap = 200*time.Millisecond, 3*time.Second
	t.Cleanup(func() { runFolderQuiet, runFolderQuietCap = quiet, limit })
	root := checkoutOf(t, "git@github.com:o/a.git")
	c := checkoutOf(t, "git@github.com:o/c.git")
	base := t.TempDir()
	folder := func(name string) string {
		path := filepath.Join(base, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		return path
	}
	notes, spaced, other, tabbed, chat := folder("notes"), folder("my notes"), folder("other"), folder("tab\there"), folder("chat")
	if err := agentconfig.WriteSettings(agentconfig.Settings{ProjectSettings: map[string]agentconfig.ProjectSettings{"p": {Path: root}}}); err != nil {
		t.Fatal(err)
	}
	d, _, do := desktopAgent(t, root, models.Task{})
	d.terminal.manager = terminal.NewManager()
	// Each discussion is a shell whose input cat writes to a file, so the test
	// reads exactly what was typed into it.
	session := func(id string) string {
		t.Helper()
		dir := t.TempDir()
		if _, err := d.terminal.manager.GetOrCreateSession(id, dir, nil); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = d.terminal.manager.CloseSession(id) })
		if err := d.terminal.manager.InjectLine(id, "exec cat > typed.txt"); err != nil {
			t.Fatal(err)
		}
		return filepath.Join(dir, "typed.txt")
	}
	discussion := func(id, provider, status string) *controlledRun {
		return &controlledRun{exited: make(chan struct{}), interactiveProvider: provider,
			desktop: desktopRun{ID: id, ProjectID: "p", Skill: "discuss", SessionID: id, Status: status}}
	}
	claudeTyped, codexTyped := session("claude-run"), session("codex-run")
	finished := discussion("finished", "claude", "completed")
	close(finished.exited)
	skill := discussion("skill", "claude", "running")
	skill.desktop.Skill = "implement"
	d.queue.runs = map[string]*controlledRun{
		"chat":       {exited: make(chan struct{}), conversation: &claudeConversation{}, desktop: desktopRun{ID: "chat", ProjectID: "p", Conversation: true, Headless: true, Status: "running"}},
		"claude-run": discussion("claude-run", "claude", "running"),
		"codex-run":  discussion("codex-run", "codex", "running"),
		"finished":   finished,
		"skill":      skill,
	}
	attach := func(runID, path string) (int, string) {
		w := do("POST", "/desktop/run-folder", map[string]string{"runId": runID, "path": path})
		return w.Code, w.Body.String()
	}
	typed := func(file string) string {
		t.Helper()
		// cat writes what it reads at once; the wait only covers the PTY.
		time.Sleep(300 * time.Millisecond)
		raw, _ := os.ReadFile(file)
		return string(raw)
	}

	if code, body := attach("chat", chat); code != 200 || !strings.Contains(body, `"typed":false`) || !strings.Contains(body, `"appliesAt":"next-turn"`) {
		t.Fatalf("conversation: %d %s", code, body)
	}
	if code, body := attach("claude-run", spaced); code != 200 || !strings.Contains(body, `"typed":true`) || !strings.Contains(body, `"appliesAt":"now"`) {
		t.Fatalf("claude discussion: %d %s", code, body)
	}
	if got := typed(claudeTyped); got != `/add-dir "`+spaced+`"`+"\n" {
		t.Fatalf("typed into Claude: %q", got)
	}
	if code, body := attach("codex-run", other); code != 200 || !strings.Contains(body, `"typed":false`) || !strings.Contains(body, `"appliesAt":"next-launch"`) {
		t.Fatalf("codex discussion: %d %s", code, body)
	}
	if got := typed(codexTyped); got != "" {
		t.Fatalf("typed into Codex: %q", got)
	}
	// A checkout of a project repository becomes its folder, and Claude is
	// given it too.
	if code, body := attach("claude-run", c); code != 200 || !strings.Contains(body, `"mappedAs":"github.com/o/c"`) || !strings.Contains(body, `"typed":true`) {
		t.Fatalf("mapped: %d %s", code, body)
	}
	if got := typed(claudeTyped); !strings.HasSuffix(got, "/add-dir "+c+"\n") || strings.Count(got, "/add-dir") != 2 {
		t.Fatalf("typed into Claude: %q", got)
	}
	// A path that cannot be typed is attached all the same.
	if code, body := attach("claude-run", tabbed); code != 200 || !strings.Contains(body, `"typed":false`) || !strings.Contains(body, `"appliesAt":"next-launch"`) {
		t.Fatalf("tabbed: %d %s", code, body)
	}
	before := typed(claudeTyped)

	for _, tt := range []struct {
		run, path string
		code      int
		why       string
	}{
		{"claude-run", spaced, 409, "already attached"},
		{"claude-run", root, 409, "local repository"},
		{"claude-run", c, 409, "already the folder of github.com/o/c"},
		{"chat", notes + "/missing", 400, "does not exist"},
		{"unknown", notes, 404, "not found"},
		{"finished", notes, 409, "ended"},
		{"skill", notes, 409, "conversation or a running ticket discussion"},
	} {
		if code, body := attach(tt.run, tt.path); code != tt.code || !strings.Contains(body, tt.why) {
			t.Errorf("%s %s: %d %s, want %d %q", tt.run, tt.path, code, body, tt.code, tt.why)
		}
	}
	if got := typed(claudeTyped); got != before {
		t.Fatalf("a refusal typed into the session: %q", got)
	}

	settings, _ := agentconfig.ReadSettings(root)
	folders := settings.Project("p").Folders
	if len(folders) != 4 || !samePath(t, settings.Repositories["github.com/o/c"], c) {
		t.Fatalf("settings = %v %v", folders, settings.Repositories)
	}
	for _, want := range []string{chat, spaced, other, tabbed} {
		if !containsPath(t, folders, want) {
			t.Errorf("%s not attached: %v", want, folders)
		}
	}

	if w := do("GET", "/desktop/status", nil); !strings.Contains(w.Body.String(), `"`+runFoldersCapability+`"`) {
		t.Errorf("status = %s", w.Body.String())
	}
}
