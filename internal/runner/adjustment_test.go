package runner

import (
	"errors"
	"strings"
	"testing"
)

func TestForgeAdjustmentEvidence(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		gitlab bool
		valid  bool
		draft  bool
		merged bool
	}{
		{`{"url":"https://forge/pull/1","state":"OPEN","headRefName":"topic","headRefOid":"abc","isDraft":true}`, false, true, true, false},
		{`{"url":"https://forge/pull/1","state":"CLOSED","headRefName":"topic"}`, false, false, false, false},
		{`{"url":"https://forge/pull/1","state":"OPEN","headRefName":"other"}`, false, false, false, false},
		{`[{"web_url":"https://forge/-/merge_requests/1","state":"opened","source_branch":"topic","sha":"abc","draft":false}]`, true, true, false, false},
		{`[]`, true, false, false, false}, {`invalid`, false, false, false, false},
		{`{"url":"https://forge/pull/1","state":"OPEN","headRefName":"topic"}`, false, false, false, false},
		// A merged PR is the same task PR and needs no readiness flag.
		{`{"url":"https://forge/pull/1","state":"MERGED","headRefName":"topic","headRefOid":"abc"}`, false, true, false, true},
		{`[{"web_url":"https://forge/-/merge_requests/1","state":"merged","source_branch":"topic","sha":"abc"}]`, true, true, false, true},
		// An open MR still wins over a merged one on the same branch.
		{`[{"web_url":"https://forge/-/merge_requests/1","state":"merged","source_branch":"topic","sha":"abc"},{"web_url":"https://forge/-/merge_requests/2","state":"opened","source_branch":"topic","sha":"def","draft":false}]`, true, true, false, false},
		{`[{"web_url":"https://forge/-/merge_requests/1","state":"closed","source_branch":"topic","sha":"abc"}]`, true, false, false, false},
	} {
		p, err := parsePullRequestEvidence(tc.raw, "topic", tc.gitlab)
		if (err == nil) != tc.valid || (tc.valid && (p.Draft != tc.draft || p.Merged != tc.merged || p.Open == tc.merged)) {
			t.Fatalf("%s: %+v %v", tc.raw, p, err)
		}
	}
}

func TestGitLabEvidenceRefusesAmbiguityAndPrefersLatestMerge(t *testing.T) {
	twoOpen := `[{"web_url":"https://gitlab/-/merge_requests/2","state":"opened","source_branch":"topic","sha":"b","draft":false},{"web_url":"https://gitlab/-/merge_requests/1","state":"opened","source_branch":"topic","sha":"a","draft":true}]`
	if _, err := parsePullRequestEvidence(twoOpen, "topic", true); !errors.Is(err, ErrAmbiguousPullRequest) || !strings.Contains(err.Error(), "merge_requests/1") {
		t.Fatalf("several open MRs must be refused as ambiguous, naming them: %v", err)
	}
	// An open MR on another branch does not make the task branch ambiguous.
	otherBranch := `[{"web_url":"https://gitlab/-/merge_requests/2","state":"opened","source_branch":"other","sha":"b","draft":false},{"web_url":"https://gitlab/-/merge_requests/1","state":"opened","source_branch":"topic","sha":"a","draft":false}]`
	if p, err := parsePullRequestEvidence(otherBranch, "topic", true); err != nil || p.URL != "https://gitlab/-/merge_requests/1" {
		t.Fatalf("%+v %v", p, err)
	}
	merged := `[{"web_url":"https://gitlab/-/merge_requests/1","state":"merged","source_branch":"topic","sha":"a","merged_at":"2026-01-01T00:00:00Z"},{"web_url":"https://gitlab/-/merge_requests/2","state":"merged","source_branch":"topic","sha":"b","merged_at":"2026-02-01T00:00:00Z"},{"web_url":"https://gitlab/-/merge_requests/3","state":"closed","source_branch":"topic","sha":"c"}]`
	if p, err := parsePullRequestEvidence(merged, "topic", true); err != nil || p.URL != "https://gitlab/-/merge_requests/2" || !p.Merged || p.Open {
		t.Fatalf("the latest merge must win: %+v %v", p, err)
	}
	closed := `[{"web_url":"https://gitlab/-/merge_requests/3","state":"closed","source_branch":"topic","sha":"c"}]`
	if _, err := parsePullRequestEvidence(closed, "topic", true); !errors.Is(err, ErrNoMatchingPullRequest) {
		t.Fatalf("a closed MR is absence, not a lookup failure: %v", err)
	}
	// An unreadable answer is a lookup failure, never absence.
	if _, err := parsePullRequestEvidence("not json", "topic", true); err == nil || errors.Is(err, ErrNoMatchingPullRequest) {
		t.Fatalf("unparsable output reported as absence: %v", err)
	}
}
