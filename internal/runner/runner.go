package runner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

type Runner struct{}

func NewRunner() *Runner {
	return &Runner{}
}

func GetDynamicCustomPath() string {
	return dynamicCustomPathFor(runtime.GOOS)
}

// dynamicCustomPathFor builds the PATH prefix for a named platform. The Unix system directories
// are meaningless on Windows, where prepending them would only push the real toolchain down.
func dynamicCustomPathFor(goos string) string {
	homeDir, _ := os.UserHomeDir()
	var parts []string
	if homeDir != "" {
		parts = append(parts, filepath.Join(homeDir, ".local", "bin"))
	}
	if goos == "windows" {
		return strings.Join(parts, ";")
	}
	parts = append(parts, "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin")
	return strings.Join(parts, ":")
}

func FindCliTool(tool string) (string, error) {
	if p, err := exec.LookPath(tool); err == nil {
		return p, nil
	}
	homeDir, _ := os.UserHomeDir()
	var candidates []string
	if homeDir != "" {
		candidates = append(candidates, filepath.Join(homeDir, ".local", "bin", tool))
	}
	candidates = append(candidates, "/opt/homebrew/bin/"+tool, "/usr/local/bin/"+tool, "/usr/bin/"+tool)
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return tool, fmt.Errorf("tool %s not found", tool)
}

// Helper to execute command with timeout and full environment
func (r *Runner) runCommand(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	// Inherit and extend PATH dynamically to include ~/.local/bin and Homebrew paths
	env := os.Environ()
	customPath := GetDynamicCustomPath()
	foundPath := false
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = "PATH=" + joinPath(customPath, strings.TrimPrefix(e, "PATH="))
			foundPath = true
			break
		}
	}
	if !foundPath {
		env = append(env, "PATH="+customPath)
	}
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := stdout.String()
	errOutput := stderr.String()

	if err != nil {
		combined := strings.TrimSpace(output + "\n" + errOutput)
		if combined == "" {
			combined = err.Error()
		}
		return combined, fmt.Errorf("command '%s %s' failed: %w (output: %s)", name, strings.Join(args, " "), err, combined)
	}

	if output == "" && errOutput != "" {
		return errOutput, nil
	}
	return output, nil
}

func (r *Runner) CheckCliTools(repoPath string) []models.CliStatus {
	tools := []string{"git", "gh", "agy", "claude", "codex", "uv", "specify", "openspec"}
	var results []models.CliStatus

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, tool := range tools {
		path, err := FindCliTool(tool)

		status := models.CliStatus{
			Tool:      tool,
			Available: err == nil,
			Path:      path,
		}

		if status.Available {
			switch tool {
			case "gh":
				out, aErr := r.runCommand(ctx, repoPath, path, "auth", "status")
				if aErr == nil || strings.Contains(out, "Logged in to") {
					status.AuthStatus = "Authenticated"
					status.Details = "GitHub CLI connected"
				} else {
					status.AuthStatus = "Not Authenticated"
					status.Details = "Run 'gh auth login'"
				}
			case "uv":
				status.AuthStatus = "Ready"
				status.Details = "uv available (required to install GitHub Spec Kit)"
			case "specify":
				status.AuthStatus = "Ready"
				status.Details = "GitHub Spec Kit CLI installed"
			case "openspec":
				status.AuthStatus = "Ready"
				status.Details = "OpenSpec CLI installed"
			case "git":
				status.AuthStatus = "Ready"
				status.Details = "Git available"
			case "agy":
				status.AuthStatus = "Ready"
				status.Details = "Antigravity CLI Agent ready"
			case "vibe":
				status.AuthStatus = "Ready"
				status.Details = "Mistral Vibe CLI Agent ready"
			case "claude":
				status.AuthStatus = "Ready"
				status.Details = "Claude Code CLI Agent ready"
			case "gemini":
				status.AuthStatus = "Ready"
				status.Details = "Gemini CLI Agent ready"
			case "codex":
				status.AuthStatus = "Ready"
				status.Details = "Codex CLI Agent ready"
			}
		} else {
			status.AuthStatus = "Not Installed"
			switch tool {
			case "uv":
				status.Details = "uv missing — curl -LsSf https://astral.sh/uv/install.sh | sh"
			case "specify":
				status.Details = "GitHub Spec Kit missing — install it from a project (Spec Kit / OpenSpec panel)"
			case "openspec":
				status.Details = "OpenSpec missing — install it from a project (Spec Kit / OpenSpec panel)"
			default:
				status.Details = fmt.Sprintf("Tool '%s' not found in PATH", tool)
			}
		}

		results = append(results, status)
	}

	return results
}

// -------------------------------------------------------------
// JIRA CLI (acli) INTEGRATION
// -------------------------------------------------------------

// jiraSearchFields is the exact set of fields acli accepts for
// 'jira workitem search --fields'. Notably 'created' and 'updated' are
// rejected by the CLI, so task timestamps fall back to the import time.
const jiraSearchFields = "key,summary,description,status,priority,assignee,labels,issuetype"

// NormalizeIssueTypes cleans a configured list of work item types.
func NormalizeIssueTypes(types []string) []string { return models.NormalizeIssueTypes(types) }

// installedSkillPath returns the SKILL.md of a workflow skill inside a checkout,
// whichever agent directory holds it. Empty when the skill is not installed.
func installedSkillPath(repoDir, skillID string) string {
	dirName := models.SkillDirNames[skillID]
	if dirName == "" || repoDir == "" {
		return ""
	}

	// La commande slash d'abord : c'est elle qui rend « /clarify-issue »
	// invocable. Une skill seule est choisie par le modèle, jamais appelée par
	// son nom, et le prompt se contentait alors d'être recopié.
	cmdPath := filepath.Join(repoDir, ".claude", "commands", dirName+".md")
	if fi, err := os.Stat(cmdPath); err == nil && !fi.IsDir() {
		return cmdPath
	}

	for _, agent := range models.SkillAgentDirs {
		var p string
		if agent == "" {
			p = filepath.Join(repoDir, ".skills", dirName, "SKILL.md")
		} else {
			p = filepath.Join(repoDir, agent, "skills", dirName, "SKILL.md")
		}
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

// skillSlashPrompt is the unified invocation: the agent runs the skill installed
// in the repository, and the ticket context follows. The instructions then live
// in one place, the SKILL.md rendered from the in-app editor, instead of being
// written twice: once in a file, once in a prompt here.
func skillSlashPrompt(skillID string) string {
	dirName := models.SkillDirNames[skillID]
	return "/" + dirName + ` {issueKey}

Contexte du ticket
Clé : {issueKey}
Titre : {issueTitle}
Description : {issueDesc}
Branche Git : {branchName}
Dossier du projet : {repoPath}
Tracker : {tracker}`
}

// AIInvocation is everything needed to run one workflow step: where, with which
// engine, and with which prompt. Splitting it out of RunAI is what lets the same
// invocation run either headless or inside a PTY session the user can open.
type AIInvocation struct {
	RepoDir  string
	Provider string
	Template string // modèle de commande du projet, placeholders déjà résolus
	Model    string // modèle résolu pour cette étape, vide = défaut du moteur
	Prompt   string
	Steps    []string
}

// RunAI keeps the headless path: pipes, no terminal to attach to. It stays the
// fallback for when no session is available.
func (r *Runner) RunAI(settings *models.Settings, skillID string, task *models.Task, customPrompt string) (string, []string, error) {
	inv, err := r.PrepareAI(settings, skillID, task, customPrompt)
	if err != nil {
		return "", nil, err
	}
	steps := inv.Steps

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	output, execSteps, execErr := r.execAgentCommand(ctx, inv.RepoDir, inv.Provider, inv.Template, inv.Model, inv.Prompt)
	steps = append(steps, execSteps...)

	if execErr != nil {
		steps = append(steps, fmt.Sprintf("⚠️ Erreur d'exécution : %v", execErr))
		return fmt.Sprintf("### ⚠️ Erreur lors de l'exécution de la commande IA (%s)\n\n```text\n%s\n```\n\n*Vérifiez que le binaire '%s' est bien accessible et authentifié.*", inv.Provider, output, inv.Provider), steps, execErr
	}

	steps = append(steps, "✅ Réponse générée par le modèle IA avec succès")
	return output, steps, nil
}

// PrepareAI resolves the working directory, the engine and the prompt of one
// workflow step, without running anything.
func (r *Runner) PrepareAI(settings *models.Settings, skillID string, task *models.Task, customPrompt string) (*AIInvocation, error) {
	skillID = models.NormalizeSkillID(skillID)
	repoDir := ""
	if task != nil && task.WorktreePath != nil && *task.WorktreePath != "" {
		repoDir = *task.WorktreePath
	} else if settings != nil && settings.RepoPath != "" {
		repoDir = strings.TrimSpace(settings.RepoPath)
	}
	if repoDir == "" {
		repoDir = "."
	}
	repoDir = filepath.Clean(repoDir)
	if abs, err := filepath.Abs(repoDir); err == nil {
		repoDir = abs
	}

	var steps []string
	if stat, err := os.Stat(repoDir); err != nil || !stat.IsDir() {
		cwd, _ := os.Getwd()
		steps = append(steps, fmt.Sprintf("⚠️ Project directory '%s' not found, falling back to: %s", repoDir, cwd))
		repoDir = cwd
	} else {
		steps = append(steps, fmt.Sprintf("📁 Project working directory (CWD): %s", repoDir))
	}

	trackerName := "github"
	if task != nil && task.Source != "" {
		trackerName = strings.ToLower(task.Source)
	} else if settings != nil && settings.IssueTracker != "" {
		trackerName = strings.ToLower(settings.IssueTracker)
	}

	repoName := filepath.Base(repoDir)
	if settings != nil && settings.GithubRepo != "" {
		repoName = settings.GithubRepo
	}

	// La skill installée dans le dépôt fait référence quand elle est là : c'est
	// le fichier que l'éditeur de Taskacao produit. Les prompts ci-dessous ne
	// servent plus que de filet quand rien n'est installé.
	installedSkill := installedSkillPath(repoDir, skillID)
	if installedSkill != "" {
		steps = append(steps, fmt.Sprintf("📄 Skill du dépôt utilisée : %s", installedSkill))
	}

	var promptTemplate string
	switch skillID {
	case "clarify":
		promptTemplate = settings.PromptClarify
		if promptTemplate == "" {
			promptTemplate = "/clarify-issue {issueKey} tracked on {tracker} in {repo}"
		}
	case "specify":
		promptTemplate = settings.PromptSpecify
		if promptTemplate == "" {
			if NormalizeSpecFramework(settings.SpecFramework) == "openspec" {
				promptTemplate = `Tu es le Lead Architecte pour Sectile. Rédige une proposition de changement OpenSpec complète pour la tâche :
Clé : {issueKey}
Titre : {issueTitle}
Description : {issueDesc}
Branche Git cible : {branchName}
Dossier du projet : {repoPath}

Contenu attendu (Framework Spec-Driven Design : OpenSpec). Écris les fichiers dans
openspec/changes/{issueKey}-<titre-slug>/ :
1. proposal.md : problème, valeur, périmètre inclus et exclu.
2. design.md : décisions techniques et alternatives écartées.
3. tasks.md : checklist ordonnée et vérifiable de mise en œuvre.
4. specs/<capability>/spec.md : deltas de comportement en sections ## ADDED / ## MODIFIED / ## REMOVED, avec des scénarios Given / When / Then.
5. Valide avec 'openspec validate <change-id> --strict' et corrige les erreurs signalées.

Si le répertoire openspec/ est absent, signale-le au lieu de deviner la structure.`
			} else {
				promptTemplate = `Tu es le Product Owner & Architecte technique pour Sectile. Rédige une spécification GitHub Spec Kit complète pour la tâche :
Clé : {issueKey}
Titre : {issueTitle}
Description : {issueDesc}
Branche Git cible : {branchName}
Dossier du projet : {repoPath}

Contenu attendu (Framework Spec-Driven Design : GitHub Spec Kit). Écris les fichiers dans
specs/{issueKey}-<titre-slug>/ :
1. spec.md : contexte, user stories priorisées, périmètre exclu, exigences fonctionnelles numérotées et critères d'acceptation Given / When / Then. Pas de choix d'implémentation ici.
2. plan.md : pile technique, architecture, composants et fichiers cibles, diagrammes de flux Mermaid.
3. tasks.md : checklist ordonnée et vérifiable de mise en œuvre, plus le plan de tests.

Respecte .specify/memory/constitution.md s'il existe. Marque explicitement les points à
clarifier au lieu de les deviner.`
			}
		}
	case "implement":
		promptTemplate = settings.PromptImplement
		if promptTemplate == "" {
			promptTemplate = `Tu es le développeur senior autonome pour Sectile. Tu dois IMPLÉMENTER ET ÉCRIRE DIRECTEMENT les modifications de code dans le projet ({repoPath}) pour accomplir cette tâche.

Contexte de la tâche :
Clé : {issueKey}
Titre : {issueTitle}
Description : {issueDesc}
Branche Git : {branchName}
Dossier du projet : {repoPath}

INSTRUCTIONS D'EXÉCUTION OBLIGATOIRES :
1. Vérifie le code existant et assure-toi d'être sur la branche Git '{branchName}'.
2. Écris et modifie concrètement les fichiers nécessaires dans le projet pour implémenter complètement la fonctionnalité ou résoudre le bug.
3. Exécute les commandes de test et de build du projet (ex: npm run build ou go test ./... selon la stack) pour vérifier que le code compile et fonctionne parfaitement sans régression.
4. Fournis un compte-rendu clair des fichiers modifiés/créés et des résultats des validations.`
		}
	case "create_pr":
		promptTemplate = `Create or reuse a draft pull request for {issueKey} on {branchName} in {repoPath}. Inspect the diff, run required build, lint and tests, commit and push authorized changes, reuse a matching open PR or create a draft, and report its verified URL. Never advance workflow stages, mark reviewed, merge or clean up the worktree.`

	case "adjust":
		promptTemplate = `Adjust the existing PR for {issueKey}: {issueTitle}.
Repository: {repoPath}. Assigned branch: {branchName}.
Before modifying files, verify and record the matching task-branch PR: open, or already merged by the human. If missing, stop and use the configured earlier creation stage. Never create or replace a PR here, and never push onto a merged PR.
Fetch and reconcile the remote default branch, review the complete diff against the specification, retrieve available review feedback, fix findings and record feedback dispositions. Feedback retrieval failure blocks completion; no human comments is valid.
Run build, lint and tests on the final code; commit and push changes; update the same PR description and evidence and verify it is ready and contains the pushed final commit. Preserve work on any failure. Never merge, approve, close the ticket or clean up the worktree.`
		if settings.PromptCreatePR != "" {
			promptTemplate += "\n" + settings.PromptCreatePR
		}

	case "handoff":
		promptTemplate = `Tu es responsable de la clôture propre de la tâche pour Sectile. Le code a été revu et fusionné : il reste à documenter le handoff et à nettoyer.

Clé : {issueKey}
Titre : {issueTitle}
Description : {issueDesc}
Branche Git : {branchName}
Dossier du projet : {repoPath}

INSTRUCTIONS D'EXÉCUTION OBLIGATOIRES :
1. Vérifie que la branche '{branchName}' est bien fusionnée dans la branche principale ('git log --oneline main..{branchName}' doit être vide). Si ce n'est pas le cas, ARRÊTE-TOI et dis-le, sans rien nettoyer.
2. Rédige le compte-rendu de handoff : ce qui a été livré, ce qui a été laissé de côté, ce qu'un lecteur doit savoir pour reprendre. Mets à jour la documentation du dépôt si le changement l'exige (README, CHANGELOG, docs).
3. Nettoie l'espace de travail local : retire le worktree de la tâche s'il existe et supprime la branche locale fusionnée.
4. Termine par un compte-rendu court : ce qui a été documenté, ce qui a été nettoyé, ce qui reste à faire côté humain.`

	case "pick":
		promptTemplate = settings.PromptPick
		if promptTemplate == "" {
			promptTemplate = "Tu es le routeur d'orchestration pour Sectile. Analyse l'état de la tâche {issueKey} ({issueTitle}) et détermine la prochaine action requise dans le cycle SDLC."
		}
	}

	// Ordre de priorité : le prompt surchargé dans les réglages, puis la skill
	// installée, puis le filet codé au-dessus.
	if installedSkill != "" && !settingsPromptOverridden(settings, skillID) {
		promptTemplate = skillSlashPrompt(skillID)
	}

	if customPrompt != "" {
		promptTemplate += "\n\nInstructions supplémentaires fournies par l'utilisateur :\n" + customPrompt
	}

	if skillID == "specify" || skillID == "implement" {
		promptTemplate += "\nRead the project PR creation policy through Sectile. Specification owns creation only for specified timing; otherwise implementation owns it. After required owner checks, commit/push, discover and reuse the branch PR or create a draft only on confirmed absence, and report prUrl. Lookup failure is not absence. Preserve a reused ready PR. On PR recovery, preserve accepted work and the attained stage; do not advance to reviewed."
	}
	if skillID == "adjust" {
		promptTemplate += "\n\n" + AdjustmentContract
	}
	branchName := ""
	if task.BranchName != nil {
		branchName = *task.BranchName
	} else {
		cleanTitle := strings.ToLower(task.Title)
		cleanTitle = strings.ReplaceAll(cleanTitle, " ", "-")
		cleanTitle = strings.ReplaceAll(cleanTitle, "'", "-")
		if len(cleanTitle) > 30 {
			cleanTitle = cleanTitle[:30]
		}
		branchName = fmt.Sprintf("%s-%s", task.Key, cleanTitle)
	}

	finalPrompt := promptTemplate
	finalPrompt = strings.ReplaceAll(finalPrompt, "{issueKey}", task.Key)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{issueTitle}", task.Title)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{issueDesc}", task.Description)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{branchName}", branchName)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{repoPath}", repoDir)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{tracker}", trackerName)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{repo}", repoName)
	finalPrompt = strings.ReplaceAll(finalPrompt, "{prompt}", customPrompt)

	provider := strings.ToLower(settings.AIProvider)
	if provider == "" {
		provider = "agy"
	}

	resolvedModel := agentconfig.ResolveSkillModel(agentconfig.ModelConfig{Model: settings.AIModel, SkillModels: settings.AISkillModels}, skillID)
	engineStep := fmt.Sprintf("🤖 Moteur IA : %s", strings.ToUpper(provider))
	if resolvedModel != "" {
		engineStep += " (" + resolvedModel + ")"
	}
	steps = append(steps, engineStep)

	// The custom-template branch substitutes the task placeholders first, then
	// hands the resolved template to the shared dispatcher.
	resolvedTemplate := settings.AICommandTemplate
	if resolvedTemplate != "" {
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{issueKey}", task.Key)
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{issueTitle}", task.Title)
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{issueDesc}", task.Description)
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{branchName}", branchName)
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{repoPath}", repoDir)
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{tracker}", trackerName)
		resolvedTemplate = strings.ReplaceAll(resolvedTemplate, "{repo}", repoName)
	}

	return &AIInvocation{
		RepoDir:  repoDir,
		Provider: provider,
		Template: resolvedTemplate,
		Model:    resolvedModel,
		Prompt:   finalPrompt,
		Steps:    steps,
	}, nil
}

// escapeForDoubleQuotes makes a string safe to interpolate inside a
// double-quoted shell word. Inside double quotes sh only treats \, ", $ and `
// specially, so backslash-escaping exactly those four yields the literal text.
//
// This matters for security, not just for quoting: the AI command template is
// executed through 'sh -c', and the prompt embeds task titles and descriptions
// that come straight from Jira or GitHub. Without escaping, a ticket
// titled `"; rm -rf ~ #` would run as a shell command.
func escapeForDoubleQuotes(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch r {
		case '\\', '"', '$', '`':
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// execAgentCommand runs the configured AI CLI for an already-resolved prompt.
// cmdTemplate must have every placeholder other than {prompt} already
// substituted by the caller; it is used when it is non-empty and either the
// provider is "custom" or the template carries a {prompt} slot.
func (r *Runner) execAgentCommand(ctx context.Context, repoDir string, provider string, cmdTemplate string, model string, finalPrompt string) (string, []string, error) {
	var steps []string
	// A template owns the whole command line, so the model reaches it only through
	// its own {model} slot; injecting a flag would duplicate or contradict it.
	cmdTemplate = agentconfig.ExpandModel(cmdTemplate, model)
	modelArgs := agentconfig.ModelArgs(provider, model)

	if agentconfig.UsesCommandTemplate(provider, cmdTemplate) {
		cmdToRun := strings.ReplaceAll(cmdTemplate, "{prompt}", escapeForDoubleQuotes(finalPrompt))
		steps = append(steps, fmt.Sprintf("Exécution de la commande personnalisée : %s dans %s", cmdToRun, filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, "sh", "-c", cmdToRun)
		return out, steps, err
	}

	switch provider {
	case "agy":
		agyPath, _ := FindCliTool("agy")
		steps = append(steps, fmt.Sprintf("Exécution de : agy -p \"...\" dans %s", filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, agyPath, "-p", finalPrompt, "--dangerously-skip-permissions")
		return out, steps, err

	case "vibe":
		vibePath, _ := FindCliTool("vibe")
		steps = append(steps, fmt.Sprintf("Exécution de : vibe -p \"...\" dans %s", filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, vibePath, "-p", finalPrompt, "--auto-approve")
		return out, steps, err

	case "claude":
		claudePath, _ := FindCliTool("claude")
		steps = append(steps, fmt.Sprintf("Exécution de : claude -p \"...\" dans %s", filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, claudePath, append(modelArgs, "-p", finalPrompt)...)
		return out, steps, err

	case "gemini":
		geminiPath, _ := FindCliTool("gemini")
		steps = append(steps, fmt.Sprintf("Exécution de : gemini -p \"...\" dans %s", filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, geminiPath, append(modelArgs, "-p", finalPrompt)...)
		return out, steps, err

	case "cursor":
		cursorPath, _ := FindCliTool("cursor")
		steps = append(steps, fmt.Sprintf("Exécution de : cursor agent -p \"...\" dans %s", filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, cursorPath, append([]string{"agent"}, append(modelArgs, "-p", finalPrompt)...)...)
		return out, steps, err

	default:
		cmdStr := cmdTemplate
		if cmdStr == "" {
			cmdStr = "agy -p \"{prompt}\""
		}
		cmdToRun := strings.ReplaceAll(cmdStr, "{prompt}", escapeForDoubleQuotes(finalPrompt))
		steps = append(steps, fmt.Sprintf("Exécution de la commande personnalisée dans %s", filepath.Base(repoDir)))
		out, err := r.runCommand(ctx, repoDir, "sh", "-c", cmdToRun)
		return out, steps, err
	}
}

// RunAgentPrompt executes the configured AI CLI on a free-form prompt with no
// task context, for features that are not tied to a single work item.
func (r *Runner) RunAgentPrompt(ctx context.Context, settings *models.Settings, prompt string) (string, []string, error) {
	if settings == nil {
		return "", nil, fmt.Errorf("réglages IA indisponibles")
	}

	provider := settings.AIProvider
	if provider == "" {
		provider = "agy"
	}

	repoDir := strings.TrimSpace(settings.RepoPath)
	if repoDir == "" {
		repoDir = "."
	}
	if abs, err := filepath.Abs(repoDir); err == nil {
		repoDir = abs
	}
	if stat, err := os.Stat(repoDir); err != nil || !stat.IsDir() {
		cwd, _ := os.Getwd()
		repoDir = cwd
	}

	out, steps, err := r.execAgentCommand(ctx, repoDir, provider, settings.AICommandTemplate, settings.AIModel, prompt)
	if err != nil {
		return out, steps, fmt.Errorf("exécution de l'agent %s impossible: %w", provider, err)
	}
	return out, steps, nil
}

// GetGitDiff computes git diff for a task branch or working directory
func (r *Runner) GetGitDiff(repoDir string, branchName string, taskKey string, prURL *string) (*models.GitDiffResult, error) {
	if repoDir == "" {
		return nil, fmt.Errorf("répertoire de projet non configuré")
	}

	if _, err := os.Stat(repoDir); os.IsNotExist(err) {
		return &models.GitDiffResult{
			TaskKey:  taskKey,
			Branch:   branchName,
			RepoPath: repoDir,
			PrURL:    prURL,
			Error:    fmt.Sprintf("Le répertoire '%s' n'existe pas", repoDir),
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Verify it's a git repo
	if _, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return &models.GitDiffResult{
			TaskKey:  taskKey,
			Branch:   branchName,
			RepoPath: repoDir,
			PrURL:    prURL,
			Error:    fmt.Sprintf("Le répertoire '%s' n'est pas un dépôt Git valide", repoDir),
		}, nil
	}

	// Detect base branch (main or master)
	baseBranch := "main"
	if _, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--verify", "main"); err != nil {
		if _, errMaster := r.runCommand(ctx, repoDir, "git", "rev-parse", "--verify", "master"); errMaster == nil {
			baseBranch = "master"
		} else {
			baseBranch = "HEAD~1"
		}
	}

	// Get current active branch
	currBranch, _ := r.runCommand(ctx, repoDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	currBranch = strings.TrimSpace(currBranch)

	targetBranch := strings.TrimSpace(branchName)
	if targetBranch == "" {
		targetBranch = currBranch
	}

	// Try diffing between baseBranch and targetBranch
	var rawDiff string
	var numstat string
	var diffErr error

	// Check if targetBranch exists
	hasBranch := false
	if _, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--verify", targetBranch); err == nil {
		hasBranch = true
	}

	if hasBranch && targetBranch != baseBranch {
		// Three-dot diff against base
		rawDiff, diffErr = r.runCommand(ctx, repoDir, "git", "diff", fmt.Sprintf("%s...%s", baseBranch, targetBranch))
		numstat, _ = r.runCommand(ctx, repoDir, "git", "diff", "--numstat", fmt.Sprintf("%s...%s", baseBranch, targetBranch))
		if strings.TrimSpace(rawDiff) == "" {
			// Fallback to two-dot diff
			rawDiff, diffErr = r.runCommand(ctx, repoDir, "git", "diff", fmt.Sprintf("%s..%s", baseBranch, targetBranch))
			numstat, _ = r.runCommand(ctx, repoDir, "git", "diff", "--numstat", fmt.Sprintf("%s..%s", baseBranch, targetBranch))
		}
	}

	// If current branch is target branch and there are uncommitted working tree changes, include them
	if currBranch == targetBranch || strings.TrimSpace(rawDiff) == "" {
		workDiff, _ := r.runCommand(ctx, repoDir, "git", "diff", "HEAD")
		workNumstat, _ := r.runCommand(ctx, repoDir, "git", "diff", "--numstat", "HEAD")
		if strings.TrimSpace(workDiff) != "" {
			if rawDiff != "" {
				rawDiff = rawDiff + "\n" + workDiff
				numstat = numstat + "\n" + workNumstat
			} else {
				rawDiff = workDiff
				numstat = workNumstat
			}
		}
	}

	// If still empty and on base branch or no commits between, try diff of last commit
	if strings.TrimSpace(rawDiff) == "" {
		lastCommitDiff, _ := r.runCommand(ctx, repoDir, "git", "diff", "HEAD~1..HEAD")
		lastCommitNumstat, _ := r.runCommand(ctx, repoDir, "git", "diff", "--numstat", "HEAD~1..HEAD")
		if strings.TrimSpace(lastCommitDiff) != "" {
			rawDiff = lastCommitDiff
			numstat = lastCommitNumstat
		}
	}

	if diffErr != nil && strings.TrimSpace(rawDiff) == "" {
		return &models.GitDiffResult{
			TaskKey:    taskKey,
			Branch:     targetBranch,
			BaseBranch: baseBranch,
			RepoPath:   repoDir,
			PrURL:      prURL,
			Error:      fmt.Sprintf("Erreur lors de la récupération du diff : %v", diffErr),
		}, nil
	}

	// Parse numstat and split rawDiff into file blocks
	fileMap := make(map[string]*models.GitDiffFile)
	totalInsertions := 0
	totalDeletions := 0

	for _, line := range strings.Split(numstat, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 3 {
			adds, _ := strconv.Atoi(parts[0])
			dels, _ := strconv.Atoi(parts[1])
			path := parts[2]
			totalInsertions += adds
			totalDeletions += dels

			fileMap[path] = &models.GitDiffFile{
				Path:      path,
				Status:    "modified",
				Additions: adds,
				Deletions: dels,
			}
		}
	}

	// Parse rawDiff chunks by file
	diffChunks := strings.Split(rawDiff, "diff --git ")
	var files []models.GitDiffFile

	for _, chunk := range diffChunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		fullChunk := "diff --git " + chunk
		firstLine := strings.SplitN(chunk, "\n", 2)[0]
		parts := strings.Fields(firstLine)

		filePath := ""
		if len(parts) >= 2 {
			filePath = strings.TrimPrefix(parts[1], "b/")
		}

		status := "modified"
		if strings.Contains(chunk, "new file mode") {
			status = "added"
		} else if strings.Contains(chunk, "deleted file mode") {
			status = "deleted"
		} else if strings.Contains(chunk, "similarity index") || strings.Contains(chunk, "rename from") {
			status = "renamed"
		}

		adds := 0
		dels := 0
		if existingFile, ok := fileMap[filePath]; ok {
			adds = existingFile.Additions
			dels = existingFile.Deletions
			existingFile.Diff = fullChunk
			existingFile.Status = status
			files = append(files, *existingFile)
			delete(fileMap, filePath)
		} else {
			// Count manually
			for _, l := range strings.Split(chunk, "\n") {
				if strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++") {
					adds++
				} else if strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---") {
					dels++
				}
			}
			totalInsertions += adds
			totalDeletions += dels
			files = append(files, models.GitDiffFile{
				Path:      filePath,
				Status:    status,
				Additions: adds,
				Deletions: dels,
				Diff:      fullChunk,
			})
		}
	}

	return &models.GitDiffResult{
		TaskKey:      taskKey,
		Branch:       targetBranch,
		BaseBranch:   baseBranch,
		RepoPath:     repoDir,
		IsClean:      len(files) == 0,
		FilesChanged: len(files),
		Insertions:   totalInsertions,
		Deletions:    totalDeletions,
		Files:        files,
		RawDiff:      rawDiff,
		PrURL:        prURL,
	}, nil
}

// GetCwdGitStatus returns git status, active branch, and repo state for a directory
func (r *Runner) GetCwdGitStatus(repoDir string) (*models.GitStatusInfo, error) {
	if repoDir == "" {
		cwd, err := os.Getwd()
		if err == nil && cwd != "" {
			repoDir = cwd
		}
	}

	if repoDir == "" {
		return &models.GitStatusInfo{
			Error: "Répertoire de travail non spécifié",
		}, nil
	}

	if fi, err := os.Stat(repoDir); err != nil || !fi.IsDir() {
		return &models.GitStatusInfo{
			RepoPath:  repoDir,
			IsGitRepo: false,
			Error:     fmt.Sprintf("Le répertoire '%s' n'existe pas", repoDir),
		}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if git repo
	if _, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return &models.GitStatusInfo{
			RepoPath:  repoDir,
			IsGitRepo: false,
			Error:     "Ce dossier n'est pas un dépôt Git",
		}, nil
	}

	// Real repo top level path
	topLevel, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--show-toplevel")
	if err == nil && strings.TrimSpace(topLevel) != "" {
		repoDir = strings.TrimSpace(topLevel)
	}

	// Current active branch
	branch, _ := r.runCommand(ctx, repoDir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	branch = strings.TrimSpace(branch)
	if branch == "HEAD" || branch == "" {
		// Detached HEAD or tag
		if tag, err := r.runCommand(ctx, repoDir, "git", "describe", "--tags", "--always"); err == nil && strings.TrimSpace(tag) != "" {
			branch = strings.TrimSpace(tag)
		} else if shortHash, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--short", "HEAD"); err == nil {
			branch = strings.TrimSpace(shortHash)
		}
	}

	// Base branch detection
	baseBranch := "main"
	if _, err := r.runCommand(ctx, repoDir, "git", "rev-parse", "--verify", "main"); err != nil {
		if _, errMaster := r.runCommand(ctx, repoDir, "git", "rev-parse", "--verify", "master"); errMaster == nil {
			baseBranch = "master"
		}
	}

	// Status porcelain
	porcelainOut, _ := r.runCommand(ctx, repoDir, "git", "status", "--porcelain")
	modifiedCount := 0
	untrackedCount := 0
	for _, line := range strings.Split(porcelainOut, "\n") {
		line = strings.TrimRight(line, "\r\n")
		if len(line) < 2 {
			continue
		}
		if strings.HasPrefix(line, "??") {
			untrackedCount++
		} else {
			modifiedCount++
		}
	}

	isClean := (modifiedCount == 0 && untrackedCount == 0)

	// Remote info
	remoteName, _ := r.runCommand(ctx, repoDir, "git", "remote")
	remoteName = strings.TrimSpace(strings.Split(remoteName, "\n")[0])
	remoteURL := ""
	if remoteName != "" {
		if rURL, err := r.runCommand(ctx, repoDir, "git", "remote", "get-url", remoteName); err == nil {
			remoteURL = strings.TrimSpace(rURL)
		}
	}

	// Ahead / Behind counts against upstream if configured
	ahead := 0
	behind := 0
	if countsOut, err := r.runCommand(ctx, repoDir, "git", "rev-list", "--left-right", "--count", "HEAD...@{u}"); err == nil {
		parts := strings.Fields(countsOut)
		if len(parts) >= 2 {
			ahead, _ = strconv.Atoi(parts[0])
			behind, _ = strconv.Atoi(parts[1])
		}
	}

	// Latest commit message & short hash
	latestCommit, _ := r.runCommand(ctx, repoDir, "git", "log", "-1", "--format=%h %s (%cr)")
	latestCommit = strings.TrimSpace(latestCommit)

	return &models.GitStatusInfo{
		RepoPath:       repoDir,
		IsGitRepo:      true,
		Branch:         branch,
		BaseBranch:     baseBranch,
		IsClean:        isClean,
		ModifiedCount:  modifiedCount,
		UntrackedCount: untrackedCount,
		Ahead:          ahead,
		Behind:         behind,
		RemoteName:     remoteName,
		RemoteURL:      remoteURL,
		LatestCommit:   latestCommit,
	}, nil
}

// FindEditorBinary locates the binary or execution command for a given code editor on the host OS
func FindEditorBinary(editor string) (string, []string) {
	editor = strings.TrimSpace(editor)
	base := strings.ToLower(editor)

	// 1. Direct LookPath or standard CLI path
	if p, err := FindCliTool(editor); err == nil && p != "" {
		return p, nil
	}

	homeDir, _ := os.UserHomeDir()

	// 2. Known application paths & fallbacks on macOS
	if runtime.GOOS == "darwin" {
		switch base {
		case "code", "vscode":
			macPaths := []string{
				"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
				"/Applications/Visual Studio Code - Insiders.app/Contents/Resources/app/bin/code",
				filepath.Join(homeDir, "Applications/Visual Studio Code.app/Contents/Resources/app/bin/code"),
			}
			for _, mp := range macPaths {
				if _, err := os.Stat(mp); err == nil {
					return mp, nil
				}
			}
			return "open", []string{"-a", "Visual Studio Code"}

		case "cursor":
			macPaths := []string{
				"/Applications/Cursor.app/Contents/Resources/app/bin/cursor",
				filepath.Join(homeDir, "Applications/Cursor.app/Contents/Resources/app/bin/cursor"),
			}
			for _, mp := range macPaths {
				if _, err := os.Stat(mp); err == nil {
					return mp, nil
				}
			}
			return "open", []string{"-a", "Cursor"}

		case "zed":
			macPaths := []string{
				"/Applications/Zed.app/Contents/MacOS/cli",
				filepath.Join(homeDir, "Applications/Zed.app/Contents/MacOS/cli"),
			}
			for _, mp := range macPaths {
				if _, err := os.Stat(mp); err == nil {
					return mp, nil
				}
			}
			return "open", []string{"-a", "Zed"}

		case "subl", "sublime":
			macPaths := []string{
				"/Applications/Sublime Text.app/Contents/SharedSupport/bin/subl",
				filepath.Join(homeDir, "Applications/Sublime Text.app/Contents/SharedSupport/bin/subl"),
			}
			for _, mp := range macPaths {
				if _, err := os.Stat(mp); err == nil {
					return mp, nil
				}
			}
			return "open", []string{"-a", "Sublime Text"}

		case "idea", "intellij":
			return "open", []string{"-a", "IntelliJ IDEA"}

		case "webstorm":
			return "open", []string{"-a", "WebStorm"}
		}
	}

	return editor, nil
}

// OpenInEditor opens the specified directory or file in a code editor (defaults to 'code' for VS Code)
func (r *Runner) OpenInEditor(editorCmd string, targetPath string) error {
	if strings.TrimSpace(editorCmd) == "" {
		editorCmd = "code"
	}
	editorCmd = strings.TrimSpace(editorCmd)

	if targetPath == "" {
		targetPath = "."
	}
	targetPath = filepath.Clean(targetPath)
	if abs, err := filepath.Abs(targetPath); err == nil {
		targetPath = abs
	}

	parts := strings.Fields(editorCmd)
	if len(parts) == 0 {
		parts = []string{"code"}
	}

	bin, prefixArgs := FindEditorBinary(parts[0])
	var args []string
	if len(prefixArgs) > 0 {
		args = append(args, prefixArgs...)
		args = append(args, parts[1:]...)
		args = append(args, targetPath)
	} else {
		args = append(args, parts[1:]...)
		args = append(args, targetPath)
	}

	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "PATH="+prefixedPath())

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to open in '%s': %w", editorCmd, err)
	}
	return nil
}

// runCommandForTest runs a shell snippet and returns its raw stdout. It exists
// so the escaping tests can assert what the shell actually parses.
func (r *Runner) runCommandForTest(script string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return r.runCommand(ctx, "", "sh", "-c", script)
}

// settingsPromptOverridden says whether the user deliberately wrote their own
// prompt for a step in the settings. That choice keeps priority over the skill
// installed in the repository.
func settingsPromptOverridden(settings *models.Settings, skillID string) bool {
	if settings == nil {
		return false
	}
	switch skillID {
	case "clarify":
		return strings.TrimSpace(settings.PromptClarify) != ""
	case "specify":
		return strings.TrimSpace(settings.PromptSpecify) != ""
	case "implement":
		return strings.TrimSpace(settings.PromptImplement) != ""
	case "adjust", "review":
		return strings.TrimSpace(settings.PromptCreatePR) != ""
	}
	return false
}

// SessionCommandLine turns an invocation into a shell command line that can be
// injected into a PTY session, plus the cleanup of the temporary file it uses.
//
// The prompt goes through a file rather than the command line on purpose: it is
// several thousand characters of markdown carrying ticket text, and pushing that
// through a terminal line editor would truncate it and mangle the quoting. The
// file is read with a command substitution, so the agent still receives it as a
// single argument.
func (r *Runner) SessionCommandLine(inv *AIInvocation) (string, func(), error) {
	if inv == nil {
		return "", func() {}, fmt.Errorf("invocation vide")
	}

	f, err := os.CreateTemp("", "sectile-prompt-*.md")
	if err != nil {
		return "", func() {}, err
	}
	if _, err := f.WriteString(inv.Prompt); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", func() {}, err
	}
	_ = f.Close()
	promptFile := f.Name()
	cleanup := func() { _ = os.Remove(promptFile) }

	// $(cat 'fichier') : le chemin est un temporaire que nous fabriquons, donc
	// sans apostrophe, et le prompt n'est jamais relu par le shell.
	promptRef := fmt.Sprintf(`"$(cat '%s')"`, promptFile)

	template := agentconfig.ExpandModel(inv.Template, inv.Model)
	if agentconfig.UsesCommandTemplate(inv.Provider, template) {
		return strings.ReplaceAll(template, "{prompt}", "$(cat '"+promptFile+"')"), cleanup, nil
	}
	modelFlag := strings.Join(agentconfig.ModelArgs(inv.Provider, inv.Model), " ")
	if modelFlag != "" {
		modelFlag += " "
	}

	switch inv.Provider {
	case "agy":
		bin, _ := FindCliTool("agy")
		return fmt.Sprintf("%s -p %s --dangerously-skip-permissions", shellQuote(bin), promptRef), cleanup, nil
	case "vibe":
		bin, _ := FindCliTool("vibe")
		return fmt.Sprintf("%s -p %s --auto-approve", shellQuote(bin), promptRef), cleanup, nil
	case "claude":
		bin, _ := FindCliTool("claude")
		return fmt.Sprintf("%s %s-p %s --dangerously-skip-permissions", shellQuote(bin), modelFlag, promptRef), cleanup, nil
	case "gemini":
		bin, _ := FindCliTool("gemini")
		return fmt.Sprintf("%s %s-p %s", shellQuote(bin), modelFlag, promptRef), cleanup, nil
	case "cursor":
		bin, _ := FindCliTool("cursor")
		return fmt.Sprintf("%s agent %s-p %s", shellQuote(bin), modelFlag, promptRef), cleanup, nil
	}

	if template == "" {
		template = `agy -p "{prompt}"`
	}
	return strings.ReplaceAll(template, "{prompt}", "$(cat '"+promptFile+"')"), cleanup, nil
}

// shellQuote wraps a path in single quotes so a space in it cannot split the
// command. The binary paths come from FindCliTool, not from user input.
func shellQuote(s string) string {
	if s == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// InteractiveAgentLaunch is the command that opens the agent CLI as a live
// session, as opposed to the one-shot print mode used for headless steps.
//
// Aucun drapeau de contournement des permissions ici, contrairement au mode
// non interactif : un humain regarde la session, il peut répondre aux demandes
// de permission, et c'est précisément l'intérêt de ce mode.
func InteractiveAgentLaunch(settings *models.Settings) (string, error) {
	provider := "agy"
	model := ""
	if settings != nil && strings.TrimSpace(settings.AIProvider) != "" {
		provider = strings.ToLower(strings.TrimSpace(settings.AIProvider))
	}
	if settings != nil {
		model = strings.TrimSpace(settings.AIModel)
	}
	modelFlag := strings.Join(agentconfig.ModelArgs(provider, model), " ")
	if modelFlag != "" {
		modelFlag = " " + modelFlag
	}

	switch provider {
	case "agy", "vibe", "claude", "gemini", "codex":
		line, err := resolveAgentBinary(provider, "")
		if err != nil {
			return "", err
		}
		return line + modelFlag, nil
	case "cursor":
		line, err := resolveAgentBinary("cursor", "")
		if err != nil {
			return "", err
		}
		return line + " agent" + modelFlag, nil
	case "custom":
		// Un moteur personnalisé n'a que son modèle de commande : son premier mot
		// est le binaire, et c'est lui qu'on ouvre en interactif.
		return resolveAgentBinary(firstWord(settings.AICommandTemplate), provider)
	}
	return "", fmt.Errorf("le moteur %q n'a pas de mode interactif connu : configure un moteur agy, claude, gemini, codex, cursor ou vibe sur le projet", provider)
}

// resolveAgentBinary finds an engine binary and says where it looked when it
// fails, because "binaire introuvable" alone leaves nothing to act on.
func resolveAgentBinary(tool, provider string) (string, error) {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return "", fmt.Errorf("aucun binaire à lancer pour le moteur %q : renseigne son modèle de commande", provider)
	}
	bin, err := FindCliTool(tool)
	if err != nil || bin == "" || bin == tool {
		if _, statErr := os.Stat(bin); statErr != nil {
			home, _ := os.UserHomeDir()
			return "", fmt.Errorf(
				"moteur %q introuvable : %s absent du PATH, de %s/.local/bin, /opt/homebrew/bin, /usr/local/bin et /usr/bin",
				strings.TrimSpace(provider+" "+tool), tool, home,
			)
		}
	}
	return shellQuote(bin), nil
}

// firstWord extracts the binary from a command template such as
// `claude --dangerously-skip-permissions -p "{prompt}"`.
func firstWord(template string) string {
	fields := strings.Fields(strings.TrimSpace(template))
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], "'\"")
}

// SkillCallLine is what gets typed into the running agent: the skill's slash
// command, the ticket key, and just enough context to place the work.
//
// C'est la skill qui porte les instructions, pas ce message : le SKILL.md est
// rendu depuis l'éditeur de Taskacao, l'agent le lit en exécutant la commande.
func SkillCallLine(skillID string, task *models.Task, trackerName string) string {
	dirName := models.SkillDirNames[skillID]
	if dirName == "" {
		dirName = skillID
	}
	return SkillCallLineWithCommand("/"+dirName, task, trackerName)
}

// SkillCallLineWithCommand builds the same line for an explicit slash command,
// so a project that renamed its skills keeps its own command.
func SkillCallLineWithCommand(command string, task *models.Task, trackerName string) string {
	return SkillCallLineForProvider("", command, task, trackerName)
}

// SkillCallLineForProvider formats a skill for an interactive agent. Codex
// receives its plain skill name; other providers keep their slash invocation.
func SkillCallLineForProvider(provider, command string, task *models.Task, trackerName string) string {
	command = strings.TrimPrefix(strings.TrimSpace(command), "/")
	if !strings.EqualFold(strings.TrimSpace(provider), "codex") {
		command = "/" + command
	}
	if task == nil {
		return command
	}

	line := fmt.Sprintf("%s %s", command, task.Key)
	title := strings.TrimSpace(task.Title)
	if title != "" {
		line += fmt.Sprintf(" (%s)", collapseSpaces(title))
	}
	if trackerName != "" && trackerName != "local" {
		line += fmt.Sprintf(" suivi dans %s", trackerName)
	}
	return line
}

// collapseSpaces flattens a title to a single line: a newline in the injected
// text would be read as a validation by the agent's prompt.
func collapseSpaces(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return strings.TrimSpace(s)
}
