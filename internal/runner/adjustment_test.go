package runner

import "testing"

func TestForgeAdjustmentEvidence(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		gitlab bool
		valid  bool
		draft  bool
	}{
		{`{"url":"https://forge/pull/1","state":"OPEN","headRefName":"topic","headRefOid":"abc","isDraft":true}`, false, true, true},
		{`{"url":"https://forge/pull/1","state":"CLOSED","headRefName":"topic"}`, false, false, false},
		{`{"url":"https://forge/pull/1","state":"OPEN","headRefName":"other"}`, false, false, false},
		{`[{"web_url":"https://forge/-/merge_requests/1","state":"opened","source_branch":"topic","sha":"abc","draft":false}]`, true, true, false},
		{`[]`, true, false, false}, {`invalid`, false, false, false},
		{`{"url":"https://forge/pull/1","state":"OPEN","headRefName":"topic"}`, false, false, false},
	} {
		p, err := parsePullRequestEvidence(tc.raw, "topic", tc.gitlab)
		if (err == nil) != tc.valid || (tc.valid && p.Draft != tc.draft) {
			t.Fatalf("%s: %+v %v", tc.raw, p, err)
		}
	}
}
