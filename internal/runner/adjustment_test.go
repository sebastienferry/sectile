package runner

import "testing"

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
