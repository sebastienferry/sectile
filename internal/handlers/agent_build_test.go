package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"tasks/internal/agentprotocol"
)

// announcedBuild is what a current agent announces, minus the operations named.
func announcedBuild(version, commit string, without ...string) AgentBuild {
	build := AgentBuild{Announced: true, Version: version, Commit: commit, Operations: map[string]bool{}}
	for _, op := range agentprotocol.Operations {
		if !slices.Contains(without, op) {
			build.Operations[op] = true
		}
	}
	return build
}

func TestParseAgentBuildTellsAnAnnouncingAgentFromALegacyOne(t *testing.T) {
	legacy := ParseAgentBuild(url.Values{"projectId": {"p"}, "deviceId": {"laptop"}})
	if legacy.Announced || legacy.Operations != nil || !legacy.Outdated() || legacy.Describe() != "" {
		t.Fatalf("legacy build = %+v", legacy)
	}
	if !legacy.Supports("pr_evidence") {
		t.Fatal("a legacy agent must still be sent operations")
	}

	announced := ParseAgentBuild(url.Values{"agentVersion": {"v1.2.0"}, "agentCommit": {"0123456789abcdef"}, "operations": {strings.Join(agentprotocol.Operations, ",")}})
	if !announced.Announced || announced.Outdated() || announced.Describe() != "v1.2.0, commit 0123456789ab" {
		t.Fatalf("announced build = %+v (%q)", announced, announced.Describe())
	}

	// An empty list is an announcement: every known operation is refused.
	empty := ParseAgentBuild(url.Values{"agentVersion": {"v1.2.0"}, "operations": {""}})
	if !empty.Announced || empty.Supports("git_status") || !empty.Outdated() {
		t.Fatalf("empty announcement = %+v", empty)
	}
	// An action this server does not list is the agent's to answer.
	if !empty.Supports("future_operation") {
		t.Fatal("an action unknown to the server was refused by it")
	}
}

// The reported incident: the server needs pr_evidence and the agent announced a
// list without it. Nothing reaches the agent and the caller is told why.
func TestAnAnnouncedAgentWithoutTheOperationIsNotSentIt(t *testing.T) {
	d := NewAgentDispatcher()
	_, conn := operationConnectionBuild(t, d, announcedBuild("v0.4.0", "", "pr_evidence"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := d.CallOperation(ctx, agentprotocol.Operation{ProjectID: "project", Action: "pr_evidence"})
	var typed *agentprotocol.UnsupportedOperationError
	if !errors.As(err, &typed) || !errors.Is(err, agentprotocol.ErrUnsupportedOperation) {
		t.Fatalf("err = %v, want an UnsupportedOperationError", err)
	}
	if typed.Device != "test" || typed.Build != "v0.4.0" || typed.Operation != "pr_evidence" {
		t.Fatalf("error = %+v", typed)
	}
	want := `the local agent on test (v0.4.0) does not support the "pr_evidence" operation; restart or update the Sectile desktop app, then retry`
	if err.Error() != want {
		t.Fatalf("message = %q", err.Error())
	}
	_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var msg AgentMessage
	if conn.ReadJSON(&msg) == nil {
		t.Fatalf("the agent was sent %q although it cannot run it", msg.Type)
	}
}

// An agent built before the announcement is still asked; its refusal of the
// operation becomes the same error, and any other failure is left as it is.
func TestALegacyAgentRefusalBecomesTheSameError(t *testing.T) {
	for _, tc := range []struct {
		reply     string
		outdated  bool
		wantError string
	}{
		{`unknown local operation "pr_evidence"`, true, `the local agent on test (unknown build) does not support the "pr_evidence" operation; restart or update the Sectile desktop app, then retry`},
		{`unknown local operation "git_status"`, false, `local agent: unknown local operation "git_status"`},
		{"GitLab token rejected", false, "local agent: GitLab token rejected"},
	} {
		d := NewAgentDispatcher()
		_, conn := operationConnection(t, d)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		done := make(chan error, 1)
		go func() {
			_, err := d.CallOperation(ctx, agentprotocol.Operation{ProjectID: "project", Action: "pr_evidence"})
			done <- err
		}()
		var request AgentMessage
		if err := conn.ReadJSON(&request); err != nil {
			t.Fatal(err)
		}
		payload, _ := json.Marshal(agentprotocol.Result{Error: tc.reply})
		if err := conn.WriteJSON(AgentMessage{Type: "workspace_result", MsgID: request.MsgID, Payload: payload}); err != nil {
			t.Fatal(err)
		}
		err := <-done
		cancel()
		if err == nil || err.Error() != tc.wantError || errors.Is(err, agentprotocol.ErrUnsupportedOperation) != tc.outdated {
			t.Errorf("reply %q: err = %v, want %q (outdated %v)", tc.reply, err, tc.wantError, tc.outdated)
		}
	}
}

// Launches and terminals are dispatches every agent knows, never checked.
func TestDispatchesAreNotCheckedAgainstTheAnnouncement(t *testing.T) {
	d := NewAgentDispatcher()
	_, conn := operationConnectionBuild(t, d, AgentBuild{Announced: true, Version: "v0.4.0", Operations: map[string]bool{}})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	go func() {
		_, _ = d.CallOperation(ctx, agentprotocol.Operation{ProjectID: "project", Action: "open_terminal", TaskID: "t1"})
	}()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	var msg AgentMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("the terminal was not dispatched: %v", err)
	}
}

// The instance serving the request gives the same text and the same type as
// the one holding the agent.
func TestAnOutdatedAgentHeldByAnotherInstanceReadsTheSame(t *testing.T) {
	c := newCluster(t, "A", "B")
	agent := connectClusterAgentBuild(t, c[0].dispatcher, "default", "project", "laptop", announcedBuild("v0.4.0", "", "pr_evidence"))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, local := c[0].dispatcher.CallOperation(ctx, agentprotocol.Operation{ProjectID: "project", Action: "pr_evidence"})
	_, forwarded := c[1].dispatcher.CallOperation(ctx, agentprotocol.Operation{ProjectID: "project", Action: "pr_evidence"})
	if local == nil || forwarded == nil || local.Error() != forwarded.Error() {
		t.Fatalf("local %v, forwarded %v", local, forwarded)
	}
	if !errors.Is(forwarded, agentprotocol.ErrUnsupportedOperation) {
		t.Fatalf("the forwarded error lost its type: %T %v", forwarded, forwarded)
	}
	_ = agent.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	var msg AgentMessage
	if agent.ReadJSON(&msg) == nil {
		t.Fatalf("the agent was sent %q", msg.Type)
	}
}

// Build fields appear only where the connection is held; elsewhere they are
// unknown, never guessed.
func TestAgentStatusReportsTheBuildOnTheHoldingInstanceOnly(t *testing.T) {
	c := newCluster(t, "A", "B")
	connectClusterAgentBuild(t, c[0].dispatcher, "u1", "current", "laptop", announcedBuild("v1.0.0", "abc"))
	connectClusterAgentBuild(t, c[0].dispatcher, "u1", "old", "laptop", announcedBuild("v0.9.0", "", "pr_evidence"))
	connectClusterAgentBuild(t, c[0].dispatcher, "u1", "legacy", "laptop", AgentBuild{})

	byProject := func(infos []AgentConnInfo) map[string]AgentConnInfo {
		out := map[string]AgentConnInfo{}
		for _, info := range infos {
			out[info.ProjectID] = info
		}
		return out
	}
	holding := byProject(c[0].dispatcher.ConnectedAgents())
	if got := holding["current"]; got.AgentVersion != "v1.0.0" || got.AgentCommit != "abc" || got.Outdated == nil || *got.Outdated ||
		!slices.IsSorted(got.Operations) || len(got.Operations) != len(agentprotocol.Operations) {
		t.Errorf("current = %+v", got)
	}
	if got := holding["old"]; got.Outdated == nil || !*got.Outdated || slices.Contains(got.Operations, "pr_evidence") {
		t.Errorf("old = %+v", got)
	}
	if got := holding["legacy"]; got.Outdated == nil || !*got.Outdated || got.AgentVersion != "" || got.Operations != nil {
		t.Errorf("legacy = %+v", got)
	}
	for project, got := range byProject(c[1].dispatcher.ConnectedAgents()) {
		if got.Outdated != nil || got.AgentVersion != "" || got.Operations != nil {
			t.Errorf("%s seen from another instance = %+v", project, got)
		}
	}

	// Without a cluster the same fields come from the local connection.
	single := NewAgentDispatcher()
	operationConnectionBuild(t, single, announcedBuild("v1.0.0", ""))
	infos := single.ConnectedAgents()
	if len(infos) != 1 || infos[0].Outdated == nil || *infos[0].Outdated || infos[0].AgentVersion != "v1.0.0" {
		t.Fatalf("single instance = %+v", infos)
	}
}
