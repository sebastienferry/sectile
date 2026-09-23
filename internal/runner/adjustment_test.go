package runner

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"
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

func TestGitLabEvidenceReadsLaterPages(t *testing.T) {
	closed := `{"web_url":"https://gitlab/mr/closed","source_branch":"topic","state":"closed"}`
	open := `{"web_url":"https://gitlab/mr/open","source_branch":"topic","state":"opened","draft":false,"sha":"head"}`
	merged := `{"web_url":"https://gitlab/mr/merged","source_branch":"topic","state":"merged","merged_at":"2026-01-01T00:00:00Z"}`
	latest := `{"web_url":"https://gitlab/mr/latest","source_branch":"topic","state":"merged","merged_at":"2026-02-01T00:00:00Z"}`
	for _, tc := range []struct {
		name, first, second, wantURL string
		wantErr                      error
	}{
		{"older open", closed, open, "https://gitlab/mr/open", nil},
		{"ambiguity across pages", open, strings.ReplaceAll(open, "/open", "/other"), "", ErrAmbiguousPullRequest},
		{"latest merge on later page", merged, latest, "https://gitlab/mr/latest", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			raw, err := gitLabEvidencePages([]string{"mr", "list", "--all"}, func(args ...string) (string, error) {
				calls++
				want := fmt.Sprintf("--per-page 100 --page %d", calls)
				if !strings.HasSuffix(strings.Join(args, " "), want) {
					t.Fatalf("missing pagination: %v", args)
				}
				if calls == 1 {
					return "[" + strings.Repeat(closed+",", 99) + tc.first + "]", nil
				}
				if calls == 2 {
					return "[" + tc.second + "]", nil
				}
				t.Fatal("unexpected extra page")
				return "", nil
			})
			if err != nil {
				t.Fatal(err)
			}
			pr, err := parsePullRequestEvidence(raw, "topic", true)
			if !errors.Is(err, tc.wantErr) || pr.URL != tc.wantURL || calls != 2 {
				t.Fatalf("evidence=%+v error=%v calls=%d", pr, err, calls)
			}
		})
	}
}

func TestGitLabEvidenceRejectsIncompleteListing(t *testing.T) {
	for _, invalid := range []string{"network failure", "malformed JSON"} {
		t.Run(invalid, func(t *testing.T) {
			calls := 0
			raw, err := gitLabEvidencePages(nil, func(...string) (string, error) {
				calls++
				if calls == 1 {
					return "[" + strings.Repeat(`{"state":"closed"},`, 99) + `{"state":"closed"}]`, nil
				}
				if invalid == "network failure" {
					return "", errors.New("offline")
				}
				return "invalid", nil
			})
			if err == nil || raw != "" || errors.Is(err, ErrNoMatchingPullRequest) {
				t.Fatalf("partial listing returned as evidence: %q %v", raw, err)
			}
		})
	}
}

func TestBranchPullRequestNamesMissingOrigin(t *testing.T) {
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	_, err := NewRunner().BranchPullRequest(dir, "feature/SFE-360-remove-arch-api")
	if !errors.Is(err, ErrNoOriginRemote) || !strings.Contains(err.Error(), "no origin remote") {
		t.Fatalf("expected the missing origin to be named, got %v", err)
	}
	// A missing remote is a failed lookup, never a refusal that reads as absence.
	if errors.Is(err, ErrNoMatchingPullRequest) || errors.Is(err, ErrAmbiguousPullRequest) {
		t.Fatalf("missing origin reported as a refusal: %v", err)
	}
}

func TestEvidenceCommandNamesTheRepository(t *testing.T) {
	cli, args := evidenceCommand(true, "topic", "gitlab.com/smartadserver/private/arch/argocd-arch")
	if cli != "glab" || !strings.HasSuffix(strings.Join(args, " "), "--all --output json -R https://gitlab.com/smartadserver/private/arch/argocd-arch") {
		t.Fatalf("glab call = %s %v", cli, args)
	}
	cli, args = evidenceCommand(false, "topic", "github.com/acme/app")
	if cli != "gh" || !strings.HasSuffix(strings.Join(args, " "), "-R github.com/acme/app") {
		t.Fatalf("gh call = %s %v", cli, args)
	}
	// Without a repository the CLI keeps asking about the checkout's own remote.
	for _, gitlab := range []bool{true, false} {
		if _, args := evidenceCommand(gitlab, "topic", ""); slices.Contains(args, "-R") {
			t.Fatalf("unexpected -R without a repository: %v", args)
		}
	}
	if _, err := NewRunner().RepositoryPullRequest(t.TempDir(), "bitbucket", "bitbucket.org/acme/app", "topic"); err == nil {
		t.Fatal("an unsupported forge must be refused")
	}
}
