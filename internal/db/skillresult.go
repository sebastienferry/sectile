package db

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// A successful CLI exit only means the agent answered. This receipt is required
// before a worker can advance a workflow stage. Check output is agent-reported;
// files, the checkout branch and the PR are checked separately by TaskFlow.
type skillResult struct {
	RunID     string       `json:"runId"`
	Outcome   string       `json:"outcome"`
	Summary   string       `json:"summary"`
	Branch    string       `json:"branch"`
	PRURL     string       `json:"prUrl"`
	Artifacts []string     `json:"artifacts"`
	Checks    []skillCheck `json:"checks"`
}

type skillCheck struct {
	Kind       string `json:"kind"`
	Command    string `json:"command"`
	ExitCode   *int   `json:"exitCode"`
	Output     string `json:"output"`
	SkipReason string `json:"skipReason"`
}

func workflowResultRequired(skillID string) bool {
	s, ok := StageSkillByID(skillID)
	return ok && s.FromStage != "" && s.Scope != "macro" && s.ID != "pickup_issues"
}

// Standalone transition calls must not race the worker's completion gate.
// Caller holds d.mu (read or write).
func (d *DB) managedStageRunningUnsafe(taskID string) (bool, error) {
	var running bool
	err := d.conn.QueryRow(`SELECT EXISTS (
		SELECT 1 FROM task_activities WHERE task_id = ? AND status = 'running'
		AND skill_id IN ('clarify', 'specify', 'implement', 'create_pr', 'review', 'pickup', 'pick', 'handoff')
	)`, taskID).Scan(&running)
	return running, err
}

func skillResultPrompt(runID, resultPath string) string {
	return fmt.Sprintf(`TaskFlow managed execution contract (takes precedence over standalone skill transitions):
- Run autonomously. A terminal/PTY does not make this an interactive clarification.
- Resolve reversible implementation choices using the code and document assumptions. Repair routine issues and update the plan while preserving product scope. If an essential decision or unavailable dependency prevents completion, report blocked; use retryable for a transient external failure after bounded recovery attempts.
- TaskFlow owns status changes and tracker synchronization in this run. Do not call taskflow stage, transition/postback APIs, or edit workflow labels. Do not merge or remove the worktree before review. Preserve existing work and reuse the assigned branch, specs and PR on a retry.
- At the end, write one JSON object to the absolute file %q. The directory already exists. Write it even when blocked. Do not commit this file. Do not merely print the JSON in the terminal.
- Schema: {"runId":%q,"outcome":"completed|blocked|retryable","summary":"decisions, completed work, or concrete blocker and next action","branch":"actual work branch","prUrl":"actual PR URL when applicable","artifacts":["repository-relative file paths"],"checks":[{"kind":"build|lint|test|merge","command":"actual command run","exitCode":0,"output":"actual output or explicit successful exit for silent commands"}]}.
- Set completed only when the requested step is fully done. For specification, include existing spec.md, tasks.md and plan.md or design.md paths. For implementation and PR, include build, lint and test results on the final code. If a check does not apply, use {"kind":"...","skipReason":"concrete reason"}; a failing or unavailable check cannot be skipped. For handoff, include a successful merge check (Git ancestry or forge merged state, supporting squash merges).
- For PR completion, the branch must be pushed and the PR must be discoverable through the forge CLI. No remote or unavailable forge means blocked/retryable, never a local merge.
`, resultPath, runID)
}

func readSkillResult(path, runID string) (*skillResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("résultat structuré manquant : %w", err)
	}
	var result skillResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("résultat structuré invalide : %w", err)
	}
	if result.RunID != runID {
		return nil, fmt.Errorf("le résultat ne correspond pas à cette exécution")
	}
	if strings.TrimSpace(result.Summary) == "" {
		return nil, fmt.Errorf("résumé du résultat manquant")
	}
	switch result.Outcome {
	case "completed":
		return &result, nil
	case "blocked", "retryable":
		return &result, fmt.Errorf("%s : %s", result.Outcome, result.Summary)
	default:
		return nil, fmt.Errorf("issue d'exécution inconnue : %q", result.Outcome)
	}
}

func validateSkillChecks(checks []skillCheck, required ...string) error {
	seen := map[string]bool{}
	for _, check := range checks {
		if seen[check.Kind] {
			return fmt.Errorf("contrôle dupliqué : %s", check.Kind)
		}
		seen[check.Kind] = true
		if strings.TrimSpace(check.SkipReason) != "" {
			if check.Kind == "merge" || check.ExitCode != nil || check.Command != "" || check.Output != "" {
				return fmt.Errorf("contrôle ignoré incohérent : %s", check.Kind)
			}
			continue
		}
		if strings.TrimSpace(check.Command) == "" || check.ExitCode == nil || *check.ExitCode != 0 || strings.TrimSpace(check.Output) == "" {
			return fmt.Errorf("contrôle absent, incomplet ou en échec : %s", check.Kind)
		}
	}
	for _, kind := range required {
		if !seen[kind] {
			return fmt.Errorf("contrôle requis manquant : %s", kind)
		}
	}
	return nil
}

func validateSkillArtifacts(repoPath string, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("aucun fichier de spécification fourni")
	}
	root, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		return err
	}
	names := map[string]bool{}
	for _, path := range paths {
		if !filepath.IsLocal(path) {
			return fmt.Errorf("fichier hors dépôt : %s", path)
		}
		resolved, err := filepath.EvalSymlinks(filepath.Join(root, path))
		if err != nil {
			return fmt.Errorf("fichier de spécification introuvable : %s", path)
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || !filepath.IsLocal(rel) {
			return fmt.Errorf("fichier hors dépôt : %s", path)
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("fichier de spécification vide ou invalide : %s", path)
		}
		names[filepath.Base(path)] = true
	}
	if !names["spec.md"] || !names["tasks.md"] || (!names["plan.md"] && !names["design.md"]) {
		return fmt.Errorf("spécification incomplète : spec.md, tasks.md et plan.md ou design.md requis")
	}
	return nil
}

// prLookup is injected so missing forge evidence can be tested without network.
func validateSkillResult(result *skillResult, skillID, repoPath string, prLookup func(string, string) string) error {
	s, ok := StageSkillByID(skillID)
	if !ok {
		return fmt.Errorf("skill inconnue : %s", skillID)
	}
	if s.ID == "specify" || s.ID == "pickup" {
		if err := validateSkillArtifacts(repoPath, result.Artifacts); err != nil {
			return err
		}
	}
	if s.ID == "implement" || s.ID == "create_pr" || s.ID == "pickup" {
		if err := validateSkillChecks(result.Checks, "build", "lint", "test"); err != nil {
			return err
		}
	}
	if s.ID == "handoff" {
		return validateSkillChecks(result.Checks, "merge")
	}
	if s.ID == "clarify" {
		return nil
	}
	branch, err := exec.Command("git", "-C", repoPath, "branch", "--show-current").Output()
	actualBranch := strings.TrimSpace(string(branch))
	if err != nil || actualBranch == "" || actualBranch != result.Branch || actualBranch == "main" || actualBranch == "master" {
		return fmt.Errorf("branche de travail absente ou incohérente : %q", result.Branch)
	}
	if base, err := exec.Command("git", "-C", repoPath, "symbolic-ref", "--short", "refs/remotes/origin/HEAD").Output(); err == nil && strings.TrimPrefix(strings.TrimSpace(string(base)), "origin/") == actualBranch {
		return fmt.Errorf("la branche de travail est la branche par défaut")
	}
	if s.ID == "create_pr" || s.ID == "pickup" {
		status, err := exec.Command("git", "-C", repoPath, "status", "--porcelain").Output()
		if err != nil || strings.TrimSpace(string(status)) != "" {
			return fmt.Errorf("la branche contient encore des modifications non commitées")
		}
		if strings.TrimSpace(result.PRURL) == "" {
			return fmt.Errorf("URL de PR manquante")
		}
		if url := prLookup(repoPath, actualBranch); url == "" || url != result.PRURL {
			return fmt.Errorf("la forge ne confirme pas la PR de la branche %s", actualBranch)
		}
	}
	return nil
}
