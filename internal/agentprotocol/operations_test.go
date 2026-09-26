package agentprotocol

import (
	"errors"
	"fmt"
	"testing"
)

func TestUnsupportedOperationErrorNamesAgentBuildOperationAndFix(t *testing.T) {
	cases := []struct {
		name  string
		build string
		want  string
	}{
		{"announced version", DescribeBuild("v0.4.0", ""), `the local agent on laptop (v0.4.0) does not support the "pr_evidence" operation; restart or update the Sectile desktop app, then retry`},
		{"announced version and commit", DescribeBuild("dev", "0123456789abcdef0123"), `the local agent on laptop (dev, commit 0123456789ab) does not support the "pr_evidence" operation; restart or update the Sectile desktop app, then retry`},
		{"legacy agent", "", `the local agent on laptop (unknown build) does not support the "pr_evidence" operation; restart or update the Sectile desktop app, then retry`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := error(&UnsupportedOperationError{Device: "laptop", Build: tc.build, Operation: "pr_evidence"})
			if err.Error() != tc.want {
				t.Fatalf("message = %q\nwant      %q", err.Error(), tc.want)
			}
			// Callers wrap it on the way up; the type must survive the wrap.
			wrapped := fmt.Errorf("pull request lookup failed on the local agent: %w", err)
			if !errors.Is(wrapped, ErrUnsupportedOperation) {
				t.Fatal("a wrapped UnsupportedOperationError does not match ErrUnsupportedOperation")
			}
			var typed *UnsupportedOperationError
			if !errors.As(wrapped, &typed) || typed.Operation != "pr_evidence" {
				t.Fatalf("errors.As lost the operation: %+v", typed)
			}
		})
	}
}

func TestIsUnknownOperationReplyMatchesOnlyTheExactRefusal(t *testing.T) {
	cases := []struct {
		reply string
		want  bool
	}{
		{`unknown local operation "pr_evidence"`, true},
		{`unknown local operation "git_evidence"`, false},
		{`local agent: unknown local operation "pr_evidence"`, false},
		{`unknown local operation "pr_evidence" (retry)`, false},
		{"pull request not found", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := IsUnknownOperationReply(tc.reply, "pr_evidence"); got != tc.want {
			t.Errorf("IsUnknownOperationReply(%q) = %v, want %v", tc.reply, got, tc.want)
		}
	}
}

func TestOperationsHasNoDuplicate(t *testing.T) {
	seen := map[string]bool{}
	for _, op := range Operations {
		if op == "" || seen[op] {
			t.Fatalf("operation %q is empty or listed twice", op)
		}
		seen[op] = true
	}
}
