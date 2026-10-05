package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// The two specifications folders of a project on this workstation (#736), named
// as the desktop project settings show them, so a refusal says which setting
// to fix.
const (
	macroSpecSetting = "Macro specifications folder"
	issueSpecSetting = "Issue specifications folder"
)

// resolveSpecFolder is a specifications folder as this workstation resolves
// it: the stored override when there is one, relative to the code checkout,
// else the code checkout itself (#484). The server holds no specifications
// path: it would name a directory on another machine.
func resolveSpecFolder(stored, root, setting string) (string, error) {
	mapped := strings.TrimSpace(stored)
	if mapped == "" {
		return root, nil
	}
	if !filepath.IsAbs(mapped) {
		mapped = filepath.Join(root, mapped)
	}
	if _, err := os.Stat(mapped); err != nil {
		return "", fmt.Errorf("le dossier des spécifications %s déclaré sur ce poste est introuvable (réglage « %s » du projet)", mapped, setting)
	}
	return mapped, nil
}

// localMacroSpecRepo is the folder the macro skills, the macro worktree and the
// slicing import read and write in.
func localMacroSpecRepo(overrides agentconfig.Settings, projectID, root string) (string, error) {
	return resolveSpecFolder(overrides.MacroSpecPath(projectID), root, macroSpecSetting)
}

// localIssueSpecRepo is the folder the issue skills write the tasks'
// clarification reports and specifications in.
func localIssueSpecRepo(overrides agentconfig.Settings, projectID, root string) (string, error) {
	return resolveSpecFolder(overrides.IssueSpecPath(projectID), root, issueSpecSetting)
}

// ensureTaskSpecWorktree prepares where a task's issue artefacts are written.
// When the Issue folder is the code checkout, nothing is prepared: the task's
// own worktree, workDir on branch, carries them as it always did. Otherwise
// the task gets a worktree of the Issue folder on a branch of the same name,
// prepared as a macro's is: started from the up-to-date default branch, or
// from the remote branch when it exists, reused as it is when it exists, the
// checkout itself with worktrees off, a plain folder written in place.
func ensureTaskSpecWorktree(ctx context.Context, issueFolder, codeRoot, workDir, branch, taskKey string, useWorktrees bool) (models.TaskSpecWorkspace, error) {
	issueFolder = strings.TrimSpace(issueFolder)
	if issueFolder == "" || sameDirectory(issueFolder, codeRoot) || sameDirectory(issueFolder, workDir) {
		return models.TaskSpecWorkspace{Repository: codeRoot, Path: workDir, Branch: branch, Worktree: workDir != "" && !sameDirectory(workDir, codeRoot)}, nil
	}
	key := strings.TrimSpace(taskKey)
	name, err := safeWorktreeName(key)
	if err != nil {
		return models.TaskSpecWorkspace{}, fmt.Errorf("clé de tâche invalide : %q", taskKey)
	}
	branch = strings.TrimSpace(branch)
	workspace, err := ensureSpecWorktree(ctx, issueFolder, name, "la tâche "+key, useWorktrees, func(string) (string, error) {
		if branch == "" {
			return "", fmt.Errorf("la tâche %s n'a pas de branche de travail : sa branche de spécifications ne peut pas être nommée", key)
		}
		return branch, nil
	})
	if err != nil {
		return models.TaskSpecWorkspace{}, err
	}
	return models.TaskSpecWorkspace{Repository: issueFolder, Path: workspace.Path, Branch: workspace.Branch, Worktree: workspace.Worktree,
		Distinct: true, Warning: workspace.Warning}, nil
}

// prepareTaskSpecWorkspace resolves the task's Issue folder on this workstation
// and prepares its specifications workspace beside the task's code worktree,
// workDir on branch, in the checkout root the task works in. An Issue folder
// left empty is the task's code checkout, even for a task pinned to another
// repository than the project's own. A distinct folder gets the dropped
// artefacts exclusion (#487) the task's checkout gets, since that is where its
// artefacts are written.
func prepareTaskSpecWorkspace(ctx context.Context, config agentconfig.Config, overrides agentconfig.Settings, projectRoot, taskRoot string, task models.Task, workDir, branch string) (models.TaskSpecWorkspace, error) {
	issue := taskRoot
	if overrides.IssueSpecPath(config.ProjectID) != "" {
		resolved, err := localIssueSpecRepo(overrides, config.ProjectID, projectRoot)
		if err != nil {
			return models.TaskSpecWorkspace{}, err
		}
		issue = resolved
	}
	workspace, err := ensureTaskSpecWorktree(ctx, issue, taskRoot, workDir, branch, task.Key, config.UseWorktrees)
	if err != nil || !workspace.Distinct || workspace.Branch == "" {
		return workspace, err
	}
	excluded := config
	if err := applySpecArtifacts(ctx, &excluded, issue, task.Key); err != nil {
		return models.TaskSpecWorkspace{}, err
	}
	return workspace, nil
}

// taskSpecWorkspace is the specifications workspace of a launch, prepared once
// the task's code worktree exists.
func (d *agentDaemon) taskSpecWorkspace(ctx context.Context, config agentconfig.Config, task models.Task, workDir, branch string) (models.TaskSpecWorkspace, error) {
	d.prepareMu.Lock()
	root, overrides, err := d.localProjectRoot(ctx, config)
	d.prepareMu.Unlock()
	if err != nil {
		return models.TaskSpecWorkspace{}, err
	}
	taskRoot, _, err := primaryRoot(ctx, config, overrides, root, task)
	if err != nil {
		return models.TaskSpecWorkspace{}, err
	}
	return prepareTaskSpecWorkspace(ctx, config, overrides, root, taskRoot, task, workDir, branch)
}

// taskSpecWorktreeFor answers the task_spec_worktree operation, which a skill
// invoked by hand calls through prepare_task_spec_worktree: the specifications
// workspace a launch would have prepared. It creates no code worktree: the
// task's branch is the one it has, or the one its worktree would carry, and its
// code worktree is wherever that branch is checked out, else the checkout.
func taskSpecWorktreeFor(ctx context.Context, config agentconfig.Config, overrides agentconfig.Settings, projectRoot string, task models.Task) (models.TaskSpecWorkspace, error) {
	taskRoot, _, err := primaryRoot(ctx, config, overrides, projectRoot, task)
	if err != nil {
		return models.TaskSpecWorkspace{}, err
	}
	branch := ""
	if task.BranchName != nil {
		branch = strings.TrimSpace(*task.BranchName)
	}
	if branch == "" && config.UseWorktrees {
		if branch, err = taskWorktreeBranch(task, config.BranchNameFormat); err != nil {
			return models.TaskSpecWorkspace{}, err
		}
	}
	if branch == "" {
		branch, _ = gitLocal(ctx, taskRoot, "branch", "--show-current")
	}
	workDir := taskRoot
	if existing, err := worktreeForBranch(ctx, taskRoot, branch); err == nil && existing != "" {
		workDir = existing
	}
	return prepareTaskSpecWorkspace(ctx, config, overrides, projectRoot, taskRoot, task, workDir, branch)
}

// taskSpecEnvironment is the specifications workspace as a task run reads it,
// under the names a macro run uses for its own.
func taskSpecEnvironment(workspace models.TaskSpecWorkspace) map[string]string {
	if workspace.Path == "" {
		return map[string]string{}
	}
	return map[string]string{
		"SECTILE_SPEC_REPO":     workspace.Path,
		"SECTILE_SPEC_BRANCH":   workspace.Branch,
		"SECTILE_SPEC_WORKTREE": fmt.Sprint(workspace.Worktree),
	}
}

// knownTaskSpecWorkspace is the specifications workspace a terminal opened on
// a task is told about. Such a terminal creates no worktree, so neither is a
// specifications worktree created for it: a distinct Issue folder is named
// only when the task's branch is already checked out there, or when it is a
// plain folder. Otherwise, or when the folder cannot be resolved, it returns an
// empty workspace and the skill typed there asks prepare_task_spec_worktree.
func (d *agentDaemon) knownTaskSpecWorkspace(ctx context.Context, config agentconfig.Config, task models.Task, workDir, branch string) models.TaskSpecWorkspace {
	d.prepareMu.Lock()
	root, overrides, err := d.localProjectRoot(ctx, config)
	d.prepareMu.Unlock()
	if err != nil {
		return models.TaskSpecWorkspace{}
	}
	taskRoot, _, err := primaryRoot(ctx, config, overrides, root, task)
	if err != nil {
		return models.TaskSpecWorkspace{}
	}
	if overrides.IssueSpecPath(config.ProjectID) == "" {
		return models.TaskSpecWorkspace{Repository: taskRoot, Path: workDir, Branch: branch, Worktree: !sameDirectory(workDir, taskRoot)}
	}
	issue, err := localIssueSpecRepo(overrides, config.ProjectID, root)
	if err != nil {
		return models.TaskSpecWorkspace{}
	}
	if sameDirectory(issue, taskRoot) {
		return models.TaskSpecWorkspace{Repository: taskRoot, Path: workDir, Branch: branch, Worktree: !sameDirectory(workDir, taskRoot)}
	}
	if _, err := gitLocal(ctx, issue, "rev-parse", "--git-dir"); err != nil {
		return models.TaskSpecWorkspace{Repository: issue, Path: issue, Distinct: true}
	}
	// The task's specifications are never written on that folder's default
	// branch, which a terminal on a task without a branch would name.
	if defaultBranch, _ := macroBaseBranch(ctx, issue); strings.TrimSpace(branch) == "" || branch == defaultBranch {
		return models.TaskSpecWorkspace{}
	}
	existing, err := worktreeForBranch(ctx, issue, branch)
	if err != nil || existing == "" {
		return models.TaskSpecWorkspace{}
	}
	return models.TaskSpecWorkspace{Repository: issue, Path: existing, Branch: branch, Worktree: !sameDirectory(existing, issue), Distinct: true}
}
