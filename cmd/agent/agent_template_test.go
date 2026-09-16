package main

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

func shellArguments(t *testing.T, shell, line string) []string {
	t.Helper()
	out, err := exec.Command(shell, "-c", line).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v: %s", shell, err, out)
	}
	return strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
}

func TestAgentTemplateLiteralArguments(t *testing.T) {
	value := "quotes '\" \\ $HOME $(printf INJECTED) `printf INJECTED` ; & | > * ? !\nsecond line {prompt} {issueKey} café"
	names := []string{"prompt", "issueKey", "issueTitle", "issueDesc", "branchName", "repoPath", "tracker", "repo"}
	for _, shell := range []string{"sh", "bash", "zsh"} {
		if _, err := exec.LookPath(shell); err != nil {
			continue
		}
		for _, wrapper := range []string{"%s", "'%s'", `"%s"`, "pre%s/post", "'pre%s/post'", `"pre%s/post"`} {
			for _, empty := range []bool{false, true} {
				values := map[string]string{}
				template := `printf '%s\000'`
				var want []string
				for _, name := range names {
					v := value
					if empty {
						v = ""
					}
					values[name] = v
					token := strings.ReplaceAll(wrapper, "%s", "{"+name+"}")
					template += " " + token + " " + token
					expected := v
					if strings.Contains(wrapper, "pre") {
						expected = "pre" + v + "/post"
					}
					want = append(want, expected, expected)
				}
				line, err := expandAgentTemplate(template, values)
				if err != nil {
					t.Fatal(err)
				}
				if got := shellArguments(t, shell, line); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s %s empty=%v: got %#v want %#v", shell, wrapper, empty, got, want)
				}
			}
		}
	}
}

func TestAgentTemplateUnknownAndEscapedTokens(t *testing.T) {
	line, err := expandAgentTemplate(`printf '%s\000' {unknown} \{issueKey} "escaped \"{prompt}\"" 'it'\''s {issueKey}'`, map[string]string{"prompt": "$HOME", "issueKey": "#63"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"{unknown}", "{issueKey}", `escaped "$HOME"`, "it's #63"}
	if got := shellArguments(t, "sh", line); !reflect.DeepEqual(got, want) {
		t.Fatalf("%#v", got)
	}
	for _, template := range []string{"echo {issueKey}", `echo \{prompt}`} {
		if _, err := expandAgentTemplate(template, map[string]string{"prompt": "hello"}); err == nil {
			t.Fatal("missing active prompt accepted")
		}
	}
	if _, err := expandAgentTemplate("echo {prompt}", map[string]string{"prompt": "a\x00b"}); err == nil {
		t.Fatal("NUL accepted")
	}
}

func TestAgentCommandContextFallbacks(t *testing.T) {
	c := agentCommandContext{Directory: "/local/worktrees/task", Tracker: "LINEAR", Repo: "owner/project"}
	if got := c.values("instructions"); got["tracker"] != "linear" || got["repo"] != "owner/project" {
		t.Fatal(got)
	}
	c.Task.Source = "GITHUB"
	if c.values("")["tracker"] != "github" {
		t.Fatal("task source must win")
	}
	c.Task.Source = ""
	c.Tracker = ""
	c.Repo = ""
	if got := c.values(""); got["tracker"] != "github" || got["repo"] != "task" {
		t.Fatal(got)
	}
}

func TestDispatchExpandsTaskLaunches(t *testing.T) {
	config := agentconfig.Config{AIProvider: "custom", AICommandTemplate: `printf '%s\000' {issueKey} "{issueTitle}" '{issueDesc}' {branchName} {repoPath} {tracker} {repo} {prompt}`, Skills: []agentconfig.Skill{{ID: "implement", Directory: "code-issue", Command: "/code-issue"}}}
	launch := agentCommandContext{Task: models.Task{ID: "full-task-id", Key: "#63", Title: "Title ' $HOME", Description: "", Source: "GITHUB"}, Branch: "feat/63", Directory: "/local path/worktree", Repo: "owner/repo"}
	for _, route := range []struct{ skill, action, prompt, want string }{
		{"implement", "implement", "run metadata", "/code-issue full-task-id\n\nrun metadata"},
		{"implement", "open_terminal", "run metadata", "/code-issue full-task-id\n\nrun metadata"},
		{"custom", "custom", "custom instructions", "Sectile task: full-task-id\n\ncustom instructions"},
	} {
		for _, title := range []string{launch.Task.Title, "Updated title"} {
			launch.Task.Title = title
			line, err := dispatchCommand(config, launch.Task.ID, route.skill, route.action, route.prompt, "", models.SkillModeInteractive, launch)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"#63", title, "", "feat/63", "/local path/worktree", "github", "owner/repo", route.want}
			if got := shellArguments(t, "sh", line); !reflect.DeepEqual(got, want) {
				t.Fatalf("%#v", got)
			}
		}
	}
	raw := "echo {issueKey} {prompt}"
	if got, err := dispatchCommand(config, launch.Task.ID, "", "open_terminal", "", raw, models.SkillModeInteractive, launch); err != nil || got != raw {
		t.Fatalf("%q %v", got, err)
	}
}

func TestAgentTemplateInteractiveHistory(t *testing.T) {
	value := "!sectile_missing_history_event 'quoted' $HOME `printf INJECTED`"
	for _, shell := range []string{"bash", "zsh"} {
		if _, err := exec.LookPath(shell); err != nil {
			continue
		}
		for _, template := range []string{`printf '%s\000' {prompt}`, `printf '%s\000' "prefix {prompt} suffix"`, `printf '%s\000' '{prompt}'`} {
			line, err := expandAgentTemplate(template, map[string]string{"prompt": value})
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"--noprofile", "--norc", "-i"}
			if shell == "zsh" {
				args = []string{"-f", "-i"}
			}
			cmd := exec.Command(shell, args...)
			cmd.SysProcAttr = detachedSession()
			cmd.Stdin = strings.NewReader(line + "\nexit\n")
			out, err := cmd.Output()
			want := value + "\x00"
			if strings.Contains(template, "prefix") {
				want = "prefix " + value + " suffix\x00"
			}
			if err != nil || string(out) != want {
				t.Fatalf("%s: got %q want %q: %v", shell, out, want, err)
			}
		}
	}
}
