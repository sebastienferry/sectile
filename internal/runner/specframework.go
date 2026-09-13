package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tasks/internal/models"
)

// -------------------------------------------------------------
// SPEC-DRIVEN DESIGN TOOLCHAIN INSTALLERS (Spec Kit / OpenSpec)
// -------------------------------------------------------------
//
// TaskFlow supports two Spec-Driven Design frameworks:
//
//   - speckit  : GitHub Spec Kit, a Python CLI named 'specify' distributed from
//                git+https://github.com/github/spec-kit.git and normally run
//                through uv / uvx. It scaffolds a .specify/ directory plus the
//                /specify, /plan and /tasks slash commands for the coding agent.
//
//   - openspec : OpenSpec, a Node CLI distributed on npm as
//                @fission-ai/openspec. It scaffolds an openspec/ directory
//                holding project.md, the AGENTS.md conventions and the
//                changes/ + specs/ folders.
//
// Both installers are network operations that can take a while, so they run
// with a generous timeout and report every executed command back to the UI.

const specFrameworkInstallTimeout = 6 * time.Minute

// SpecFrameworkLabel returns the human-facing name of a framework id.
func SpecFrameworkLabel(framework string) string {
	switch NormalizeSpecFramework(framework) {
	case "openspec":
		return "OpenSpec"
	default:
		return "GitHub Spec Kit"
	}
}

// NormalizeSpecFramework maps user input (and the legacy "openfeature" value
// that used to be stored in the database) onto a supported framework id.
func NormalizeSpecFramework(framework string) string { return models.NormalizeSpecFramework(framework) }

// specKitIntegration maps a TaskFlow AI provider onto the value accepted by
// `specify init --integration`. Spec Kit's own non-interactive default is
// copilot, so that is the fallback for providers it does not know (including
// the "custom" shell-template provider).
func specKitIntegration(aiAgent string) string {
	switch strings.ToLower(strings.TrimSpace(aiAgent)) {
	case "agy", "antigravity":
		return "antigravity"
	case "claude":
		return "claude"
	case "vibe":
		return "vibe"
	case "gemini":
		return "gemini"
	case "cursor":
		return "cursor"
	case "codex":
		return "codex"
	case "copilot", "github-copilot":
		return "copilot"
	default:
		return "copilot"
	}
}

// openSpecTools maps a TaskFlow AI provider onto the value accepted by
// `openspec init --tools`. This flag is what makes the initializer
// non-interactive, so it is always passed.
func openSpecTools(aiAgent string) string {
	switch strings.ToLower(strings.TrimSpace(aiAgent)) {
	case "agy", "antigravity":
		return "antigravity"
	case "claude":
		return "claude"
	case "vibe":
		return "vibe"
	case "gemini":
		return "gemini"
	case "cursor":
		return "cursor"
	case "codex":
		return "codex"
	case "copilot", "github-copilot":
		return "github-copilot"
	default:
		return "claude"
	}
}

// specFrameworkMarkers lists the paths whose presence means the framework has
// already been initialized in this working directory.
func specFrameworkMarkers(framework string, repoPath string) []string {
	switch NormalizeSpecFramework(framework) {
	case "openspec":
		// OpenSpec >= 1.4 writes openspec/config.yaml; older releases wrote
		// openspec/project.md. Either one means the project is initialized.
		return []string{
			filepath.Join(repoPath, "openspec"),
			filepath.Join(repoPath, "openspec", "config.yaml"),
			filepath.Join(repoPath, "openspec", "project.md"),
		}
	default:
		return []string{
			filepath.Join(repoPath, ".specify"),
			filepath.Join(repoPath, ".specify", "memory", "constitution.md"),
		}
	}
}

func anyPathExists(paths []string) (bool, []string) {
	var found []string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			found = append(found, p)
		}
	}
	return len(found) > 0, found
}

// GetSpecFrameworkStatus reports whether the framework CLI is reachable on the
// host and whether the project working directory has already been initialized.
func (r *Runner) GetSpecFrameworkStatus(framework string, repoPath string) *models.SpecFrameworkStatus {
	framework = NormalizeSpecFramework(framework)
	repoPath = strings.TrimSpace(repoPath)
	if repoPath == "" {
		repoPath = "."
	}

	status := &models.SpecFrameworkStatus{
		Framework:      framework,
		FrameworkLabel: SpecFrameworkLabel(framework),
		RepoPath:       repoPath,
	}

	if framework == "openspec" {
		status.CliCommand = "openspec"
		status.InstallHint = "npm install -g @fission-ai/openspec@latest"
		if p, err := FindCliTool("openspec"); err == nil && p != "" {
			status.CliAvailable = true
			status.CliCommand = p
		} else if p, err := FindCliTool("npx"); err == nil && p != "" {
			// npx can run the package without a global install.
			status.CliAvailable = true
			status.CliCommand = "npx -y @fission-ai/openspec@latest"
		}
	} else {
		status.CliCommand = "specify"
		status.InstallHint = "uv tool install specify-cli --from git+https://github.com/github/spec-kit.git"
		if p, err := FindCliTool("specify"); err == nil && p != "" {
			status.CliAvailable = true
			status.CliCommand = p
		} else if p, err := FindCliTool("uvx"); err == nil && p != "" {
			status.CliAvailable = true
			status.CliCommand = "uvx --from git+https://github.com/github/spec-kit.git specify"
		}
	}

	initialized, found := anyPathExists(specFrameworkMarkers(framework, repoPath))
	status.Initialized = initialized
	status.MarkerPaths = found

	return status
}

// runInstallStep executes one command and records it as a reportable step.
func (r *Runner) runInstallStep(ctx context.Context, dir string, label string, name string, args ...string) models.SpecFrameworkStep {
	step := models.SpecFrameworkStep{
		Label:   label,
		Command: strings.TrimSpace(name + " " + strings.Join(args, " ")),
	}

	out, err := r.runCommand(ctx, dir, name, args...)
	step.Output = truncateOutput(out, 4000)
	if err != nil {
		step.Error = err.Error()
		return step
	}
	step.Success = true
	return step
}

func truncateOutput(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n… (sortie tronquée)"
}

// InstallSpecFramework bootstraps the requested Spec-Driven Design toolchain in
// the project working directory. It installs the CLI when it is missing, then
// runs the framework initializer. Every attempted command is reported so the
// user can see exactly what ran and copy it into a terminal if needed.
func (r *Runner) InstallSpecFramework(req models.SpecFrameworkInstallRequest) *models.SpecFrameworkInstallResult {
	framework := NormalizeSpecFramework(req.Framework)
	repoPath := strings.TrimSpace(req.RepoPath)
	if repoPath == "" {
		repoPath = "."
	}

	res := &models.SpecFrameworkInstallResult{
		Framework:      framework,
		FrameworkLabel: SpecFrameworkLabel(framework),
		RepoPath:       repoPath,
		Steps:          []models.SpecFrameworkStep{},
	}

	if err := os.MkdirAll(repoPath, 0755); err != nil {
		res.Error = fmt.Sprintf("impossible de créer le répertoire de travail %s: %v", repoPath, err)
		res.Message = res.Error
		return res
	}

	alreadyInit, markers := anyPathExists(specFrameworkMarkers(framework, repoPath))
	res.AlreadyInit = alreadyInit
	res.MarkerPaths = markers

	if alreadyInit && !req.Force {
		res.Installed = true
		res.Message = fmt.Sprintf("%s est déjà initialisé dans %s (%s). Relancez avec l'option de forçage pour réinitialiser.",
			res.FrameworkLabel, repoPath, strings.Join(markers, ", "))
		return res
	}

	ctx, cancel := context.WithTimeout(context.Background(), specFrameworkInstallTimeout)
	defer cancel()

	if framework == "openspec" {
		r.installOpenSpec(ctx, repoPath, req, res)
	} else {
		r.installSpecKit(ctx, repoPath, req, res)
	}

	initialized, found := anyPathExists(specFrameworkMarkers(framework, repoPath))
	res.Installed = initialized
	res.MarkerPaths = found

	if initialized {
		res.Message = fmt.Sprintf("%s initialisé dans %s", res.FrameworkLabel, repoPath)
		res.Error = ""
	} else if res.Error == "" {
		res.Error = fmt.Sprintf("l'initialisation de %s n'a produit aucun des marqueurs attendus (%s)",
			res.FrameworkLabel, strings.Join(specFrameworkMarkers(framework, repoPath), ", "))
		res.Message = res.Error
	} else {
		res.Message = res.Error
	}

	return res
}

// installSpecKit installs the GitHub Spec Kit 'specify' CLI and runs
// 'specify init --here'. uv is required; uvx is preferred because it runs the
// CLI straight from the git repository without polluting the user's tools.
func (r *Runner) installSpecKit(ctx context.Context, repoPath string, req models.SpecFrameworkInstallRequest, res *models.SpecFrameworkInstallResult) {
	const specKitSource = "git+https://github.com/github/spec-kit.git"
	integration := specKitIntegration(req.AIAgent)

	specifyPath, _ := FindCliTool("specify")
	uvxPath, _ := FindCliTool("uvx")
	uvPath, _ := FindCliTool("uv")

	if specifyPath == "" && uvxPath == "" && uvPath == "" {
		res.Error = "aucun de 'specify', 'uvx' ou 'uv' n'est disponible dans le PATH. " +
			"Installez uv d'abord (curl -LsSf https://astral.sh/uv/install.sh | sh), puis relancez."
		res.Steps = append(res.Steps, models.SpecFrameworkStep{
			Label:   "Détection de la chaîne d'outils uv",
			Command: "command -v specify uvx uv",
			Skipped: true,
			Error:   res.Error,
		})
		return
	}

	// Install the CLI as a uv tool when only 'uv' is present, so later runs are
	// fast and 'specify' lands in the PATH.
	if specifyPath == "" && uvxPath == "" && uvPath != "" {
		step := r.runInstallStep(ctx, repoPath, "Installation de la CLI Spec Kit via uv",
			uvPath, "tool", "install", "specify-cli", "--from", specKitSource)
		res.Steps = append(res.Steps, step)
		if step.Success {
			if p, err := FindCliTool("specify"); err == nil {
				specifyPath = p
			}
		}
	}

	// Current Spec Kit selects the coding agent with --integration; releases
	// before that used --ai. Try the current flag first, then the legacy one,
	// then no agent flag at all (the CLI defaults to Copilot when it cannot
	// prompt). --here scaffolds into the existing directory and --force skips
	// the "directory not empty" confirmation, both required to run unattended.
	argSets := [][]string{
		{"init", "--here", "--force", "--integration", integration},
		{"init", "--here", "--force", "--ai", integration},
		{"init", "--here", "--force"},
	}

	type attempt struct {
		label string
		name  string
		args  []string
	}
	var attempts []attempt

	if specifyPath != "" {
		for i, set := range argSets {
			label := "Initialisation de Spec Kit (specify init --here)"
			if i > 0 {
				label = fmt.Sprintf("Initialisation de Spec Kit (variante d'options %d)", i+1)
			}
			attempts = append(attempts, attempt{label, specifyPath, set})
		}
	}
	if uvxPath != "" {
		uvxBase := []string{"--from", specKitSource, "specify"}
		for i, set := range argSets {
			label := "Initialisation de Spec Kit via uvx"
			if i > 0 {
				label = fmt.Sprintf("Initialisation de Spec Kit via uvx (variante d'options %d)", i+1)
			}
			attempts = append(attempts, attempt{label, uvxPath, append(append([]string{}, uvxBase...), set...)})
		}
	}

	for _, a := range attempts {
		step := r.runInstallStep(ctx, repoPath, a.label, a.name, a.args...)
		res.Steps = append(res.Steps, step)
		if step.Success {
			res.Version = r.probeVersion(ctx, repoPath, specifyPath, uvxPath, specKitSource)
			return
		}
		if ctx.Err() != nil {
			res.Error = "délai dépassé pendant l'installation de Spec Kit"
			return
		}
	}

	res.Error = "toutes les tentatives d'initialisation de Spec Kit ont échoué (voir le détail des commandes)"
}

// probeVersion best-effort reads the installed Spec Kit version.
func (r *Runner) probeVersion(ctx context.Context, repoPath string, specifyPath string, uvxPath string, source string) string {
	if specifyPath != "" {
		if out, err := r.runCommand(ctx, repoPath, specifyPath, "--version"); err == nil {
			return strings.TrimSpace(out)
		}
	}
	if uvxPath != "" {
		if out, err := r.runCommand(ctx, repoPath, uvxPath, "--from", source, "specify", "--version"); err == nil {
			return strings.TrimSpace(out)
		}
	}
	return ""
}

// installOpenSpec installs the OpenSpec CLI from npm and runs 'openspec init'
// in the working directory.
func (r *Runner) installOpenSpec(ctx context.Context, repoPath string, req models.SpecFrameworkInstallRequest, res *models.SpecFrameworkInstallResult) {
	const openSpecPkg = "@fission-ai/openspec@latest"

	openspecPath, _ := FindCliTool("openspec")
	npxPath, _ := FindCliTool("npx")
	npmPath, _ := FindCliTool("npm")

	if openspecPath == "" && npxPath == "" && npmPath == "" {
		res.Error = "aucun de 'openspec', 'npx' ou 'npm' n'est disponible dans le PATH. " +
			"Installez Node.js (>= 20) puis relancez."
		res.Steps = append(res.Steps, models.SpecFrameworkStep{
			Label:   "Détection de la chaîne d'outils Node",
			Command: "command -v openspec npx npm",
			Skipped: true,
			Error:   res.Error,
		})
		return
	}

	// Prefer a global install so the CLI is reusable, but only when npx is
	// unavailable: npx already runs the package without touching global state.
	if openspecPath == "" && npxPath == "" && npmPath != "" {
		step := r.runInstallStep(ctx, repoPath, "Installation globale de la CLI OpenSpec via npm",
			npmPath, "install", "-g", openSpecPkg)
		res.Steps = append(res.Steps, step)
		if step.Success {
			if p, err := FindCliTool("openspec"); err == nil {
				openspecPath = p
			}
		}
	}

	// --tools is what makes `openspec init` non-interactive: without it the CLI
	// prompts for the AI tools to configure. --force auto-cleans legacy files
	// instead of asking. A bare `openspec init` is deliberately NOT attempted:
	// it would sit on a prompt until the install timeout expires.
	tools := openSpecTools(req.AIAgent)
	argSets := [][]string{
		{"init", "--tools", tools, "--force"},
		{"init", "--tools", tools},
	}

	type attempt struct {
		label string
		name  string
		args  []string
	}
	var attempts []attempt

	if openspecPath != "" {
		for i, set := range argSets {
			label := "Initialisation d'OpenSpec (openspec init --tools)"
			if i > 0 {
				label = fmt.Sprintf("Initialisation d'OpenSpec (variante d'options %d)", i+1)
			}
			attempts = append(attempts, attempt{label, openspecPath, set})
		}
	}
	if npxPath != "" {
		for i, set := range argSets {
			label := "Initialisation d'OpenSpec via npx"
			if i > 0 {
				label = fmt.Sprintf("Initialisation d'OpenSpec via npx (variante d'options %d)", i+1)
			}
			attempts = append(attempts, attempt{label, npxPath, append([]string{"-y", openSpecPkg}, set...)})
		}
	}

	for _, a := range attempts {
		step := r.runInstallStep(ctx, repoPath, a.label, a.name, a.args...)
		res.Steps = append(res.Steps, step)
		if step.Success {
			if openspecPath != "" {
				if out, err := r.runCommand(ctx, repoPath, openspecPath, "--version"); err == nil {
					res.Version = strings.TrimSpace(out)
				}
			}
			return
		}
		if ctx.Err() != nil {
			res.Error = "délai dépassé pendant l'installation d'OpenSpec"
			return
		}
	}

	res.Error = "toutes les tentatives d'initialisation d'OpenSpec ont échoué (voir le détail des commandes)"
}
