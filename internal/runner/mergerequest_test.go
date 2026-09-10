package runner

import "testing"

func TestOpenPRMustMatchBranchAndCommit(t *testing.T) {
	for _, tc := range []struct {
		name, raw  string
		gitlab, ok bool
	}{
		{"github open", `{"url":"https://github.com/o/r/pull/1","state":"OPEN","headRefName":"ticket","headRefOid":"abc"}`, false, true},
		{"github merged", `{"url":"https://github.com/o/r/pull/1","state":"MERGED","headRefName":"ticket","headRefOid":"abc"}`, false, false},
		{"github old commit", `{"url":"https://github.com/o/r/pull/1","state":"OPEN","headRefName":"ticket","headRefOid":"old"}`, false, false},
		{"github wrong branch", `{"url":"https://github.com/o/r/pull/1","state":"OPEN","headRefName":"other","headRefOid":"abc"}`, false, false},
		{"gitlab open", `[{"web_url":"https://gitlab.com/o/r/-/merge_requests/1","state":"opened","source_branch":"ticket","sha":"abc"}]`, true, true},
		{"gitlab closed", `[{"web_url":"https://gitlab.com/o/r/-/merge_requests/1","state":"closed","source_branch":"ticket","sha":"abc"}]`, true, false},
		{"gitlab old commit", `[{"web_url":"https://gitlab.com/o/r/-/merge_requests/1","state":"opened","source_branch":"ticket","sha":"old"}]`, true, false},
		{"malformed", "not json", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := openPRFromJSON(tc.raw, "ticket", "abc", tc.gitlab); (got != "") != tc.ok {
				t.Fatalf("PR URL = %q; want accepted = %v", got, tc.ok)
			}
		})
	}
}

// L'URL de la merge request doit être lue, jamais devinée : la version
// précédente fabriquait github.com/<repo>/pull/<numero de ticket>.
func TestDetectMergeRequestURL(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{
			name: "pull request GitHub",
			text: "PR ouverte : https://github.com/sebastienferry/taskacao/pull/12 pour revue.",
			want: "https://github.com/sebastienferry/taskacao/pull/12",
		},
		{
			name: "merge request GitLab autohébergée",
			text: "Voir https://gitlab.example.com/platform/core/-/merge_requests/447",
			want: "https://gitlab.example.com/platform/core/-/merge_requests/447",
		},
		{
			name: "la dernière citée gagne",
			text: "L'ancienne https://github.com/o/r/pull/1 est fermée, la nouvelle est https://github.com/o/r/pull/9",
			want: "https://github.com/o/r/pull/9",
		},
		{
			name: "ponctuation collée",
			text: "Ouverte ici : https://github.com/o/r/pull/42.",
			want: "https://github.com/o/r/pull/42",
		},
		{
			name: "aucune URL",
			text: "Aucun remote configuré, fusion locale effectuée.",
			want: "",
		},
		{
			name: "un lien de dépôt n'est pas une MR",
			text: "Dépôt : https://github.com/o/r et branche PROJ-1",
			want: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DetectMergeRequestURL(c.text); got != c.want {
				t.Fatalf("obtenu %q, attendu %q", got, c.want)
			}
		})
	}
}

func TestMergeRequestStepWording(t *testing.T) {
	if got := MergeRequestStep("", ""); got == "" {
		t.Fatal("l'absence de MR doit être dite explicitement")
	}
	if got := MergeRequestStep("https://github.com/o/r/pull/3", "forge"); got == "" {
		t.Fatal("une MR trouvée doit être tracée")
	}
}
