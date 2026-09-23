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
