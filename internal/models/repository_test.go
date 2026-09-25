package models

import "testing"

func TestRepositoryIdentity(t *testing.T) {
	for remote, want := range map[string]string{
		"git@github.com:sebastienferry/sectile.git":                      "github.com/sebastienferry/sectile",
		"https://github.com/sebastienferry/sectile":                      "github.com/sebastienferry/sectile",
		"https://GitHub.com/SebastienFerry/Sectile.git/":                 "github.com/sebastienferry/sectile",
		"ssh://git@gitlab.com:22/smartadserver/private/arch/argocd-arch": "gitlab.com/smartadserver/private/arch/argocd-arch",
		"git@gitlab.com:smartadserver/private/arch/argocd-arch.git":      "gitlab.com/smartadserver/private/arch/argocd-arch",
		"deploy@gitlab.example.org:group/app.git":                        "gitlab.example.org/group/app",
		"https://user:secret@gitlab.example.org/group/app.git":           "gitlab.example.org/group/app",
	} {
		if got := RepositoryIdentity(remote); got != want {
			t.Errorf("RepositoryIdentity(%q) = %q, want %q", remote, got, want)
		}
	}
}

func TestParsePullRequestLink(t *testing.T) {
	cases := []struct {
		raw, githubHost string
		want            PullRequestLink
	}{
		{"https://github.com/sebastienferry/sectile/pull/392", "", PullRequestLink{"github", "github.com", "sebastienferry/sectile", 392}},
		{"https://gitlab.com/smartadserver/private/arch/argocd-arch/-/merge_requests/97", "", PullRequestLink{"gitlab", "gitlab.com", "smartadserver/private/arch/argocd-arch", 97}},
		{"https://code.example.org/group/app/-/merge_requests/3/", "", PullRequestLink{"gitlab", "code.example.org", "group/app", 3}},
		{"https://git.corp.example/acme/app/pull/5", "git.corp.example", PullRequestLink{"github", "git.corp.example", "acme/app", 5}},
	}
	for _, c := range cases {
		got, ok := ParsePullRequestLink(c.raw, c.githubHost)
		if !ok || got != c.want {
			t.Errorf("ParsePullRequestLink(%q) = %+v, %v; want %+v", c.raw, got, ok, c.want)
		}
	}
	if got, _ := ParsePullRequestLink(cases[1].raw, ""); got.Identity() != "gitlab.com/smartadserver/private/arch/argocd-arch" {
		t.Errorf("identity = %q", got.Identity())
	}
	for _, raw := range []string{
		"",
		"git@github.com:acme/app.git",
		"https://git.corp.example/acme/app/pull/5",
		"https://github.com/acme/app/pull/5?x=1",
		"https://github.com/acme/app/pull/5#top",
		"https://user@github.com/acme/app/pull/5",
		"https://github.com/acme/app/pull/abc",
		"https://github.com/acme/app/pull/0",
		"https://github.com/acme/app/issues/5",
		"https://gitlab.com/group/../app/-/merge_requests/5",
		"https://gitlab.com/-/merge_requests/5",
		"ftp://gitlab.com/group/app/-/merge_requests/5",
	} {
		if got, ok := ParsePullRequestLink(raw, ""); ok {
			t.Errorf("ParsePullRequestLink(%q) = %+v, want no link", raw, got)
		}
	}
}

func TestNormalizeProjectRepositories(t *testing.T) {
	got := NormalizeProjectRepositories("git@github.com:o/a.git", []string{
		"https://github.com/o/b", "", "https://github.com/O/A.git", "git@github.com:o/b.git", " git@gitlab.com:g/c.git ",
	})
	want := []ProjectRepository{
		{"git@github.com:o/a.git", "github.com/o/a"},
		{"https://github.com/o/b", "github.com/o/b"},
		{"git@gitlab.com:g/c.git", "gitlab.com/g/c"},
	}
	if len(got) != len(want) {
		t.Fatalf("NormalizeProjectRepositories = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if got := NormalizeProjectRepositories("", nil); len(got) != 0 {
		t.Errorf("no remote, no list = %+v, want empty", got)
	}
}

func TestDuplicateRepository(t *testing.T) {
	if got := DuplicateRepository("git@github.com:o/a.git", []string{"https://github.com/o/b", "https://github.com/o/a"}); got != "https://github.com/o/a" {
		t.Errorf("code remote respelled = %q", got)
	}
	if got := DuplicateRepository("", []string{"https://github.com/o/b", "git@github.com:o/b.git"}); got != "git@github.com:o/b.git" {
		t.Errorf("SSH and HTTPS of one remote = %q", got)
	}
	if got := DuplicateRepository("git@github.com:o/a.git", []string{"https://github.com/o/b", ""}); got != "" {
		t.Errorf("distinct remotes = %q, want none", got)
	}
}

func TestResolvePrimaryRepository(t *testing.T) {
	repositories := NormalizeProjectRepositories("git@github.com:o/a.git", []string{"git@github.com:o/b.git", "git@github.com:o/c.git"})
	mappedSet := func(ids ...string) func(string) bool {
		return func(identity string) bool {
			for _, id := range ids {
				if id == identity {
					return true
				}
			}
			return false
		}
	}
	cases := []struct {
		name     string
		pinned   string
		repos    []ProjectRepository
		monoRepo bool
		mapped   func(string) bool
		want     string
		outcome  PrimaryResolution
		pin      bool
	}{
		{"pinned and mapped", "github.com/o/b", repositories, false, mappedSet("github.com/o/b"), "github.com/o/b", PrimaryResolved, false},
		{"pinned by URL", "https://github.com/o/b.git", repositories, false, mappedSet("github.com/o/b"), "github.com/o/b", PrimaryResolved, false},
		{"pinned, not mapped", "github.com/o/c", repositories, false, mappedSet("github.com/o/a"), "github.com/o/c", PrimaryUnmapped, false},
		{"single repository", "", repositories[:1], false, mappedSet(), "", PrimaryDefault, false},
		{"mono-repo", "", repositories, true, mappedSet("github.com/o/a", "github.com/o/b"), "", PrimaryDefault, false},
		{"mono-repo ignores a pin", "github.com/o/c", repositories, true, mappedSet(), "", PrimaryDefault, false},
		{"one mapped", "", repositories, false, mappedSet("github.com/o/c"), "github.com/o/c", PrimaryResolved, true},
		{"none mapped", "", repositories, false, mappedSet(), "", PrimaryUnmapped, false},
		{"ambiguous", "", repositories, false, mappedSet("github.com/o/a", "github.com/o/b"), "", PrimaryAmbiguous, false},
		{"stale pin reads as absent", "github.com/o/gone", repositories, false, mappedSet("github.com/o/a", "github.com/o/b"), "", PrimaryAmbiguous, false},
	}
	for _, c := range cases {
		got, outcome, pin := ResolvePrimaryRepository(c.pinned, c.repos, c.monoRepo, c.mapped)
		if got.Identity != c.want || outcome != c.outcome || pin != c.pin {
			t.Errorf("%s: got %q, %v, pin %v; want %q, %v, pin %v", c.name, got.Identity, outcome, pin, c.want, c.outcome, c.pin)
		}
	}
}
