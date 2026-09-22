package db

import (
	"fmt"
	"strings"
	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/trackerapi"
)

func (d *DB) prCreationOwner(task *models.Task) string {
	p, _ := d.GetProjectByID(task.ProjectID)
	if p != nil && p.PRCreationStage == "specified" {
		return "specify"
	}
	return "implement"
}

func (d *DB) adjustmentPrerequisite(task *models.Task, actorID string, ready bool) (trackerapi.PullRequest, error) {
	origin, _ := adjustmentOverrideOrigin(d.projectSkillOverrides(task.ProjectID))
	settings, _ := d.GetSettings()
	project, _ := d.GetProjectByID(task.ProjectID)
	if origin != "adjust" && project != nil && strings.TrimSpace(project.SkillOverrides["adjust"]) == "" && (strings.TrimSpace(project.SkillOverrides["review"]) != "") {
		return trackerapi.PullRequest{}, fmt.Errorf("legacy command override requires reconciliation in Skills")
	}
	if origin == "review" || (origin != "adjust" && settings != nil && strings.TrimSpace(settings.PromptCreatePR) != "") {
		return trackerapi.PullRequest{}, fmt.Errorf("legacy adjustment customization requires reconciliation in Skills: review and save under Adjust or reset")
	}
	branch := ""
	if task.BranchName != nil {
		branch = *task.BranchName
	}
	pr, err := d.lookupStagePR(task, actorID, d.adjustmentCheckout(task), branch)
	if err != nil {
		return pr, fmt.Errorf("adjustment prerequisite: %w (creation owner: %s)", err, d.prCreationOwner(task))
	}
	// A task holds several PRs over its life: the one the forge reports for the
	// branch is a follow-up as long as it shares a branch with a recorded link.
	if err = models.AcceptPullRequest(task.PrLinks, pr.URL, pr.Branch); err != nil {
		return pr, err
	}
	if ready && pr.Draft {
		_, request := evidenceTerms(pr.Forge)
		return pr, fmt.Errorf("adjustment requires a ready %s", request)
	}
	links := models.AppendPullRequestLink(task.PrLinks, pr.URL, pr.Branch)
	if len(links) != len(task.PrLinks) {
		d.mu.Lock()
		_, err = d.conn.Exec("UPDATE tasks SET pr_url = ?, pr_links = ?, pr_links_detached = 0 WHERE id = ?",
			pullRequestURLValue(links), encodePullRequestLinks(links), task.ID)
		d.mu.Unlock()
		if err != nil {
			return pr, err
		}
		task.PrLinks = links
		task.PrURL = pullRequestURLValue(links)
	}
	return pr, nil
}

// evidenceTerms words a refusal in the terms of the forge that answered, so a
// GitLab user is told about a merge request rather than a GitHub PR.
func evidenceTerms(forge string) (source, request string) {
	if forge == "gitlab" {
		return "GitLab", "merge request"
	}
	return "forge", "PR"
}

func validatePullRequestEvidence(pr trackerapi.PullRequest, branch, url string, recorded []models.TaskPullRequest, ready bool) error {
	source, request := evidenceTerms(pr.Forge)
	// A merged PR is accepted: the human merge is the boundary adjustment stops at, not a reason to strand the task.
	if (!pr.Open && !pr.Merged) || pr.Branch != branch || pr.URL == "" || pr.URL != url {
		return fmt.Errorf("%s does not confirm the matching open or merged %s", source, request)
	}
	if err := models.AcceptPullRequest(recorded, pr.URL, pr.Branch); err != nil {
		return err
	}
	if ready && pr.Draft {
		return fmt.Errorf("adjustment %s is still a draft", request)
	}
	return nil
}

func (d *DB) validateStagePR(task *models.Task, actorID, skillID, repoPath, branch, url string) (string, error) {
	skillID = models.NormalizeSkillID(skillID)
	required := skillID == "create_pr" || skillID == "adjust" || skillID == "pickup" || skillID == "implement" || (skillID == "specify" && d.prCreationOwner(task) == "specify")
	if !required {
		return url, nil
	}
	pr, err := d.lookupStagePR(task, actorID, repoPath, branch)
	if err != nil {
		return "", err
	}
	if url == "" {
		url = pr.URL
	}
	if err = validatePullRequestEvidence(pr, branch, url, task.PrLinks, skillID == "adjust" || skillID == "pickup"); err != nil {
		return "", err
	}
	var evidence struct {
		SHA    string
		Branch string
		Clean  bool
	}
	if err = d.callAgent(agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "git_evidence", UserID: actorID}, &evidence); err != nil {
		return "", err
	}
	if evidence.Branch != branch || evidence.SHA != pr.SHA {
		_, request := evidenceTerms(pr.Forge)
		return "", fmt.Errorf("%s does not contain the agent checkout commit", request)
	}
	if (skillID == "adjust" || skillID == "pickup") && !evidence.Clean {
		return "", fmt.Errorf("agent checkout contains uncommitted changes")
	}

	return url, nil
}

func (d *DB) adjustmentCheckout(task *models.Task) string {
	if p, _ := d.GetProjectByID(task.ProjectID); p != nil {
		if p.GithubRepo != "" {
			return p.GithubRepo
		}
		return p.GitRemoteUrl
	}
	return ""
}

// lookupStagePR reads the pull request that carries a stage's evidence. It is
// resolved for the task's own project and for whoever asked, because a lookup
// with neither reached only the server environment: a deployment configured
// through the project, or through the person's profile, had no credential on
// this path at all while every other tracker call had one.
//
// The forge is the one hosting the code, whatever tracks the issues: GitHub is
// read by the server, any other forge by the local agent, which already holds
// the CLI login for it.
func (d *DB) lookupStagePR(task *models.Task, userID, repo, branch string) (trackerapi.PullRequest, error) {
	if d.prEvidenceLookup != nil {
		return d.prEvidenceLookup(repo, branch)
	}
	// An unreadable project is a failed lookup: guessing a forge would send the
	// question to the wrong one and report its answer as the task's evidence.
	project, err := d.GetProjectByID(task.ProjectID)
	if err != nil {
		return trackerapi.PullRequest{}, fmt.Errorf("read project for the stage PR lookup: %w", err)
	}
	forge := stagePRForge(project)
	if forge == "github" {
		return d.trackerAs(userID, "github", task.ProjectID).BranchPullRequest(repo, branch)
	}
	return d.agentBranchPullRequest(task, userID, branch, forge)
}

// stagePRForge chooses where stage evidence is read from the code remote:
// "github" for the server's GitHub client, "gitlab" or "" (a forge the remote
// does not name) for the local agent. A configured GitHub repository only
// decides when the remote names no forge, so a stale one never sends a GitLab
// project to GitHub, while a GitHub project keeps the path it always had.
func stagePRForge(p *models.Project) string {
	if p == nil {
		return ""
	}
	host := remoteHost(p.GitRemoteUrl)
	switch {
	case strings.Contains(host, "github"):
		return "github"
	case strings.Contains(host, "gitlab"):
		return "gitlab"
	case strings.TrimSpace(p.GithubRepo) != "":
		return "github"
	}
	return ""
}

// remoteHost is the host of a Git remote, in URL or scp-like form, lowercased;
// empty when the remote names none, such as a local path or a bare owner/repo.
func remoteHost(remote string) string {
	r := strings.ToLower(strings.TrimSpace(remote))
	if i := strings.Index(r, "://"); i >= 0 {
		r = r[i+3:]
		if j := strings.Index(r, "/"); j >= 0 {
			r = r[:j]
		}
	} else if j := strings.Index(r, ":"); j > 1 && !strings.Contains(r[:j], "/") {
		r = r[:j]
	} else {
		return ""
	}
	if k := strings.LastIndex(r, "@"); k >= 0 {
		r = r[k+1:]
	}
	if k := strings.Index(r, ":"); k >= 0 {
		r = r[:k]
	}
	return r
}

// agentBranchPullRequest asks the local agent which request carries the branch.
// The agent tells a refusal (the forge answered, nothing usable) from a failed
// lookup, and the two stay distinct here: neither is permission to proceed, but
// only the first means the request is really absent.
func (d *DB) agentBranchPullRequest(task *models.Task, userID, branch, forge string) (trackerapi.PullRequest, error) {
	var answer struct {
		Forge   string
		URL     string
		Branch  string
		SHA     string
		Open    bool
		Draft   bool
		Merged  bool
		Refusal string
	}
	if err := d.callAgent(agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "pr_evidence", Branch: branch, UserID: userID}, &answer); err != nil {
		lookup := "pull request"
		if forge == "gitlab" {
			lookup = "GitLab merge request"
		}
		return trackerapi.PullRequest{}, fmt.Errorf("%s lookup failed on the local agent: %w", lookup, err)
	}
	if answer.Forge != "gitlab" {
		answer.Forge = ""
	}
	if answer.Refusal != "" {
		if answer.Forge == "gitlab" {
			return trackerapi.PullRequest{}, fmt.Errorf("GitLab: %s", answer.Refusal)
		}
		return trackerapi.PullRequest{}, fmt.Errorf("%s", answer.Refusal)
	}
	return trackerapi.PullRequest{URL: answer.URL, Branch: answer.Branch, SHA: answer.SHA, Open: answer.Open, Draft: answer.Draft, Merged: answer.Merged, Forge: answer.Forge}, nil
}
