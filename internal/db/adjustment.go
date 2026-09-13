package db

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"tasks/internal/models"
	"tasks/internal/runner"
)

func (d *DB) prCreationOwner(task *models.Task) string {
	p, _ := d.GetProjectByID(task.ProjectID)
	if p != nil && p.PRCreationStage == "specified" {
		return "specify"
	}
	return "implement"
}

func (d *DB) adjustmentPrerequisite(task *models.Task, ready bool) (runner.PullRequestEvidence, error) {
	origin, _ := adjustmentOverrideOrigin(d.projectSkillOverrides(task.ProjectID))
	settings, _ := d.GetSettings()
	project, _ := d.GetProjectByID(task.ProjectID)
	if origin != "adjust" && project != nil && strings.TrimSpace(project.SkillOverrides["adjust"]) == "" && (strings.TrimSpace(project.SkillOverrides["create_pr"]) != "" || strings.TrimSpace(project.SkillOverrides["review"]) != "") {
		return runner.PullRequestEvidence{}, fmt.Errorf("legacy command override requires reconciliation in Skills")
	}
	if origin == "create_pr" || origin == "review" || (origin != "adjust" && settings != nil && strings.TrimSpace(settings.PromptCreatePR) != "") {
		return runner.PullRequestEvidence{}, fmt.Errorf("legacy adjustment customization requires reconciliation in Skills: review and save under Adjust or reset")
	}
	branch := ""
	if task.BranchName != nil {
		branch = *task.BranchName
	}
	pr, err := d.lookupStagePR(d.adjustmentCheckout(task), branch)
	if err != nil {
		return pr, fmt.Errorf("adjustment prerequisite: %w (creation owner: %s)", err, d.prCreationOwner(task))
	}
	if task.PrURL != nil && strings.TrimSpace(*task.PrURL) != "" && *task.PrURL != pr.URL {
		return pr, fmt.Errorf("recorded PR does not match the open task-branch PR")
	}
	if ready && pr.Draft {
		return pr, fmt.Errorf("adjustment requires a ready PR")
	}
	if task.PrURL == nil || *task.PrURL == "" {
		d.mu.Lock()
		_, err = d.conn.Exec("UPDATE tasks SET pr_url = ? WHERE id = ?", pr.URL, task.ID)
		d.mu.Unlock()
		if err != nil {
			return pr, err
		}
		task.PrURL = &pr.URL
	}
	return pr, nil
}

func validatePullRequestEvidence(pr runner.PullRequestEvidence, branch, url, expected string, ready bool) error {
	if !pr.Open || pr.Branch != branch || pr.URL == "" || pr.URL != url {
		return fmt.Errorf("forge does not confirm the matching open PR")
	}
	if expected != "" && pr.URL != expected {
		return fmt.Errorf("adjustment replaced the original PR")
	}
	if ready && pr.Draft {
		return fmt.Errorf("adjustment PR is still a draft")
	}
	return nil
}

func (d *DB) validateStagePR(task *models.Task, skillID, repoPath, branch, url, expected string) (string, error) {
	skillID = models.NormalizeSkillID(skillID)
	required := skillID == "adjust" || skillID == "pickup" || skillID == "implement" || (skillID == "specify" && d.prCreationOwner(task) == "specify")
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
	if task.PrURL != nil && *task.PrURL != "" && expected == "" {
		expected = *task.PrURL
	}
	if err = validatePullRequestEvidence(pr, branch, url, expected, skillID == "adjust" || skillID == "pickup"); err != nil {
		return "", err
	}
	if skillID == "adjust" || skillID == "pickup" {
		status, err := exec.Command("git", "-C", repoPath, "status", "--porcelain").Output()
		if err != nil || strings.TrimSpace(string(status)) != "" {
			return "", fmt.Errorf("adjustment checkout contains uncommitted changes")
		}
	}
	head, err := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(head)) != pr.SHA {
		return "", fmt.Errorf("PR does not contain the final checkout commit")
	}
	return url, nil
}

func (d *DB) adjustmentCheckout(task *models.Task) string {
	if task.WorktreePath != nil && *task.WorktreePath != "" {
		return *task.WorktreePath
	}
	repo := d.ResolveTaskRepoPath(task)
	if task.BranchName != nil {
		for _, path := range getGitWorktreePaths(repo) {
			out, err := exec.Command("git", "-C", path, "branch", "--show-current").Output()
			if err == nil && strings.TrimSpace(string(out)) == *task.BranchName {
				return filepath.Clean(path)
			}
		}
	}
	return repo
}

func (d *DB) lookupStagePR(repo, branch string) (runner.PullRequestEvidence, error) {
	if d.prEvidenceLookup != nil {
		return d.prEvidenceLookup(repo, branch)
	}
	return d.runner.BranchPullRequest(repo, branch)
}
