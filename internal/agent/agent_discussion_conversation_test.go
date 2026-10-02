package agent

import (
	"testing"
	"time"

	"tasks/internal/agentconfig"
)

func TestPendingDiscussionViewsAreTakenOnce(t *testing.T) {
	var views pendingDiscussionViews
	views.mark("task-1")
	if views.take("other") {
		t.Fatal("a preference was taken for a task that never asked for one")
	}
	if !views.take("", "task-1") {
		t.Fatal("the preference recorded for the task was not found")
	}
	if views.take("task-1") {
		t.Fatal("a preference was taken twice")
	}
}

func TestPendingDiscussionViewsExpire(t *testing.T) {
	views := pendingDiscussionViews{at: map[string]time.Time{"task-1": time.Now().Add(-conversationDiscussionTTL)}}
	if views.take("task-1") {
		t.Fatal("an expired preference opened a conversation")
	}
	if len(views.at) != 0 {
		t.Fatalf("expired preferences were kept: %v", views.at)
	}
}

func TestConversationDiscussionEngine(t *testing.T) {
	for _, tc := range []struct {
		config agentconfig.Config
		want   bool
	}{
		{agentconfig.Config{AIProvider: "claude"}, true},
		{agentconfig.Config{AIProvider: " Claude "}, true},
		{agentconfig.Config{AIProvider: "codex"}, false},
		{agentconfig.Config{}, false},
		{agentconfig.Config{AIProvider: "claude", AICommandTemplate: "claude --model {model} '{prompt}'"}, true},
	} {
		if got := conversationDiscussionEngine(tc.config); got != tc.want {
			t.Errorf("conversationDiscussionEngine(%+v) = %v, want %v", tc.config, got, tc.want)
		}
	}
}

func TestConversationStoppedStatus(t *testing.T) {
	discussion := &controlledRun{desktop: desktopRun{TaskID: "task-1", Skill: "discuss"}}
	if got := conversationStoppedStatus(discussion); got != "completed" {
		t.Errorf("a stopped ticket discussion ends %q, want completed", got)
	}
	free := &controlledRun{}
	if got := conversationStoppedStatus(free); got != "canceled" {
		t.Errorf("a stopped free conversation ends %q, want canceled", got)
	}
}

func TestAConversationKeepsOnlyTheModelOfALaunchTemplate(t *testing.T) {
	templated := agentconfig.Config{AIProvider: "claude", AIModel: "opus", AICommandTemplate: "claude --model {model} '{prompt}'"}
	if got := conversationModel(templated); got != "opus" {
		t.Errorf("conversationModel = %q, want the engine's model", got)
	}
	if got := conversationOrigin(templated, "Here."); got == "Here." {
		t.Error("a templated engine's conversation does not say its template is not run")
	}
	if got := conversationOrigin(agentconfig.Config{AIProvider: "claude"}, "Here."); got != "Here." {
		t.Errorf("a built-in engine's notice changed: %q", got)
	}
}

func TestExtraConversationDirsKeepOnlyWhatTheLaunchAdded(t *testing.T) {
	got := extraConversationDirs([]string{"/repo/a", "/skills/custom", "/repo/b", "/skills/custom"}, []string{"/repo/a", "/repo/b"})
	if len(got) != 1 || got[0] != "/skills/custom" {
		t.Fatalf("extraConversationDirs = %v, want the custom skill folder once", got)
	}
	if got := extraConversationDirs([]string{"/repo/a"}, []string{"/repo/a"}); got != nil {
		t.Fatalf("a launch adding nothing gave %v", got)
	}
}
