package db

import (
	"fmt"
	"strings"

	"tasks/internal/models"
	"tasks/internal/trackerapi"
)

// defaultGithubAPI is what an empty GitHub API URL resolves to.
const defaultGithubAPI = "https://api.github.com"

// trackerKindOf names the tracker a project writes to, by the rule
// Registry.ForProject applies: an explicit tracker, else GitHub when the
// project names a repository, else the local board.
func trackerKindOf(p *models.Project) string {
	kind := strings.ToLower(strings.TrimSpace(p.IssueTracker))
	if kind == "" || kind == "local" {
		if strings.TrimSpace(p.GithubRepo) != "" {
			return "github"
		}
		return "local"
	}
	return kind
}

// trackerAddress reduces a tracker URL to what tells two sites apart: no
// scheme, no trailing slash, the host lower-cased.
func trackerAddress(raw string) string {
	address := strings.TrimSpace(raw)
	if i := strings.Index(address, "://"); i >= 0 {
		address = address[i+3:]
	}
	address = strings.TrimRight(address, "/")
	if host, path, found := strings.Cut(address, "/"); found {
		return strings.ToLower(host) + "/" + path
	}
	return strings.ToLower(address)
}

// sameTrackerInstance reports whether a story created in target can take a
// macro of macroProject as its parent, and, when it cannot, the reason in the
// language the interface speaks.
//
// The parent link is the whole point of the check: a Jira epic parents any
// story of its own site, whatever the project; a GitHub milestone belongs to
// one repository; the local board is one board.
func (d *DB) sameTrackerInstance(macroProject, target *models.Project) (bool, string) {
	if macroProject.ID == target.ID {
		return true, ""
	}
	macroKind, targetKind := trackerKindOf(macroProject), trackerKindOf(target)
	if macroKind != targetKind {
		return false, fmt.Sprintf("le projet cible « %s » utilise le tracker %s, la macro %s : les deux doivent partager le même tracker", target.Name, targetKind, macroKind)
	}
	switch macroKind {
	case "local":
		return true, ""
	case "jira":
		macroSite, targetSite := trackerAddress(d.jiraSiteOf(macroProject)), trackerAddress(d.jiraSiteOf(target))
		if macroSite == "" || macroSite != targetSite {
			return false, fmt.Sprintf("le projet cible « %s » est sur une autre instance Jira (%s, la macro sur %s) : l'épic ne peut pas y être le parent de la story", target.Name, orUnknown(targetSite), orUnknown(macroSite))
		}
		return true, ""
	case "github":
		macroAPI, targetAPI := trackerAddress(d.githubAPIOf(macroProject)), trackerAddress(d.githubAPIOf(target))
		if macroAPI != targetAPI {
			return false, fmt.Sprintf("le projet cible « %s » est sur une autre instance GitHub (%s, la macro sur %s)", target.Name, targetAPI, macroAPI)
		}
		if !strings.EqualFold(strings.TrimSpace(macroProject.GithubRepo), strings.TrimSpace(target.GithubRepo)) {
			return false, fmt.Sprintf("le projet cible « %s » est un autre dépôt GitHub (%s) : un milestone de %s ne peut pas y rattacher la story", target.Name, target.GithubRepo, macroProject.GithubRepo)
		}
		return true, ""
	case "gitlab":
		// The macro is a pair of labels, which any project of the instance
		// could carry; the rule stays GitHub's until the owner widens it
		// (spec #398, open point O2).
		macroAPI, targetAPI := trackerAddress(d.gitlabAPIOf(macroProject)), trackerAddress(d.gitlabAPIOf(target))
		if macroAPI != targetAPI {
			return false, fmt.Sprintf("le projet cible « %s » est sur une autre instance GitLab (%s, la macro sur %s)", target.Name, targetAPI, macroAPI)
		}
		macroPath, targetPath := d.gitlabProjectOf(macroProject), d.gitlabProjectOf(target)
		if !strings.EqualFold(macroPath, targetPath) {
			return false, fmt.Sprintf("le projet cible « %s » est un autre projet GitLab (%s) : une macro de %s ne peut pas y rattacher la story", target.Name, orUnknown(targetPath), orUnknown(macroPath))
		}
		return true, ""
	}
	return false, fmt.Sprintf("le projet cible « %s » utilise le tracker %s, qui ne permet pas de rattacher la story à la macro", target.Name, targetKind)
}

// jiraSiteOf and githubAPIOf resolve a project's tracker address the way
// trackerCredentials does, project override first, then the settings, from
// the project given rather than from a re-read of it.
func (d *DB) jiraSiteOf(p *models.Project) string {
	if site := strings.TrimSpace(p.TrackerUrl); site != "" {
		return site
	}
	if settings, err := d.getSettingsUnsafe(); err == nil && settings != nil {
		return settings.JiraUrl
	}
	return ""
}

func (d *DB) githubAPIOf(p *models.Project) string {
	api := strings.TrimSpace(p.GithubApiUrl)
	if api == "" {
		if settings, err := d.getSettingsUnsafe(); err == nil && settings != nil {
			api = strings.TrimSpace(settings.GithubApiUrl)
		}
	}
	return orDefault(api, defaultGithubAPI)
}

// gitlabAPIOf and gitlabProjectOf resolve a project's GitLab instance and
// project path the same way.
func (d *DB) gitlabAPIOf(p *models.Project) string {
	api := strings.TrimSpace(p.GitlabUrl)
	if api == "" {
		if settings, err := d.getSettingsUnsafe(); err == nil && settings != nil {
			api = strings.TrimSpace(settings.GitlabUrl)
		}
	}
	return orDefault(api, trackerapi.DefaultGitlabURL)
}

func (d *DB) gitlabProjectOf(p *models.Project) string {
	path := strings.TrimSpace(p.GitlabProject)
	if path == "" {
		if settings, err := d.getSettingsUnsafe(); err == nil && settings != nil {
			path = strings.TrimSpace(settings.GitlabProject)
		}
	}
	return strings.Trim(path, "/")
}

func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func orUnknown(value string) string {
	if value == "" {
		return "site inconnu"
	}
	return value
}
