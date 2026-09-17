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

func (d *DB) adjustmentPrerequisite(task *models.Task, ready bool) (trackerapi.PullRequest, error) {
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
	pr, err := d.lookupStagePR(d.adjustmentCheckout(task), branch)
	if err != nil {
		return pr, fmt.Errorf("adjustment prerequisite: %w (creation owner: %s)", err, d.prCreationOwner(task))
	}
	// A task holds several PRs over its life: the one the forge reports for the
	// branch is a follow-up as long as it shares a branch with a recorded link.
	if err = models.AcceptPullRequest(task.PrLinks, pr.URL, pr.Branch); err != nil {
		return pr, err
	}
	if ready && pr.Draft {
		return pr, fmt.Errorf("adjustment requires a ready PR")
	}
	links := models.AppendPullRequestLink(task.PrLinks, pr.URL, pr.Branch)
	if len(links) != len(task.PrLinks) {
		d.mu.Lock()
		_, err = d.conn.Exec("UPDATE tasks SET pr_url = ?, pr_links = ? WHERE id = ?",
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

func validatePullRequestEvidence(pr trackerapi.PullRequest, branch, url string, recorded []models.TaskPullRequest, ready bool) error {
	// A merged PR is accepted: the human merge is the boundary adjustment stops at, not a reason to strand the task.
	if (!pr.Open && !pr.Merged) || pr.Branch != branch || pr.URL == "" || pr.URL != url {
		return fmt.Errorf("forge does not confirm the matching open or merged PR")
	}
	if err := models.AcceptPullRequest(recorded, pr.URL, pr.Branch); err != nil {
		return err
	}
	if ready && pr.Draft {
		return fmt.Errorf("adjustment PR is still a draft")
	}
	return nil
}

func (d *DB) validateStagePR(task *models.Task, skillID, repoPath, branch, url string) (string, error) {
	skillID = models.NormalizeSkillID(skillID)
	required := skillID == "create_pr" || skillID == "adjust" || skillID == "pickup" || skillID == "implement" || (skillID == "specify" && d.prCreationOwner(task) == "specify")
	if !required {
		return url, nil
	}
	pr, err := d.lookupStagePR(repoPath, branch)
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
	if err = d.callAgent(agentprotocol.Operation{ProjectID: task.ProjectID, TaskID: task.ID, Action: "git_evidence"}, &evidence); err != nil {
		return "", err
	}
	if evidence.Branch != branch || evidence.SHA != pr.SHA {
		return "", fmt.Errorf("PR does not contain the agent checkout commit")
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

func (d *DB) lookupStagePR(repo, branch string) (trackerapi.PullRequest, error) {
	if d.prEvidenceLookup != nil {
		return d.prEvidenceLookup(repo, branch)
	}
	return d.tracker("").BranchPullRequest(repo, branch)
}
