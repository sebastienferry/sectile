package handlers

import (
	"errors"
	"fmt"
	"testing"

	"tasks/internal/agentprotocol"
)

// The server decides whether to close a run from what the agent answered, so
// the two failures it must tell apart are checked directly: an agent that
// answers it does not have the run, and an agent that answers nothing.
func TestOrphanedRunIsDistinguishedFromAnUnreachableAgent(t *testing.T) {
	orphan := fmt.Errorf("%w: %s: this agent does not own the execution", ErrRunNotOwned, agentprotocol.RunNotOwned)
	if !errors.Is(orphan, ErrRunNotOwned) {
		t.Fatal("an orphaned run is not recognised, so it would stay open for good")
	}
	if errors.Is(orphan, ErrNoAgentConnected) {
		t.Fatal("an orphaned run reads as an unreachable agent")
	}

	if !errors.Is(ErrNoAgentConnected, ErrNoAgentConnected) {
		t.Fatal("an unreachable agent is not recognised")
	}
	if errors.Is(ErrNoAgentConnected, ErrRunNotOwned) {
		t.Fatal("an unreachable agent reads as an orphaned run, which would claim a stop nobody saw")
	}

	other := errors.New("local terminal launch failed: something else")
	if errors.Is(other, ErrRunNotOwned) || errors.Is(other, ErrNoAgentConnected) {
		t.Fatal("an unrelated failure matches one of the two cases")
	}
}

// The marker travels from the agent to the server, so it is the one string
// both sides must agree on.
func TestRunNotOwnedMarkerIsMatchedAsAPrefix(t *testing.T) {
	summary := agentprotocol.RunNotOwned + ": this agent does not own the execution"
	if len(agentprotocol.RunNotOwned) == 0 {
		t.Fatal("the marker is empty, so every failure would match it")
	}
	if summary[:len(agentprotocol.RunNotOwned)] != agentprotocol.RunNotOwned {
		t.Fatalf("summary %q does not start with the marker", summary)
	}
	for _, other := range []string{
		"local terminal launch failed: boom",
		"Stop requested but process exit is not confirmed",
		"",
	} {
		if len(other) >= len(agentprotocol.RunNotOwned) && other[:len(agentprotocol.RunNotOwned)] == agentprotocol.RunNotOwned {
			t.Fatalf("unrelated summary %q matches the marker", other)
		}
	}
}
