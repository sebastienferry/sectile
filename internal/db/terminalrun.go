package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"tasks/internal/models"
	"tasks/internal/runner"
)

// Exécution d'un pas du workflow dans la session PTY de la tâche.
//
// Un pas autonome tournait dans des tubes anonymes : invisible pendant qu'il
// travaillait, coupé à cinq minutes, et le journal disait « annulée par
// l'utilisateur » même quand personne n'avait rien annulé. Ici il tourne dans la
// session de la tâche, donc il apparaît dans la liste des sessions, s'ouvre d'un
// clic, et l'utilisateur peut répondre à l'agent sans rien relancer.
//
// La session n'est pas ouverte automatiquement : c'est la seule différence avec
// un pas interactif.

// TerminalSessionRunner is the slice of the terminal manager the worker needs.
// Declared here rather than imported so the database keeps no dependency on the
// terminal package; cmd/server wires the adapter.
type TerminalSessionRunner interface {
	RunCommandInSession(
		ctx context.Context,
		sessionID string,
		cwd string,
		envVars map[string]string,
		commandLine string,
		idleBudget time.Duration,
	) (output string, exitCode int, idleStopped bool, err error)

	// EnsureAgentReady starts the agent CLI in the session if none runs there
	// yet, and waits until it is ready to receive a line.
	EnsureAgentReady(
		ctx context.Context,
		sessionID string,
		cwd string,
		envVars map[string]string,
		launchLine string,
	) (launched bool, err error)

	// InjectLine types one line into the session.
	InjectLine(sessionID string, line string) error

	// AgentLaunched says whether an agent CLI already runs in the session.
	AgentLaunched(sessionID string) bool

	// ForgetAgent clears that flag, so a relaunch really starts an agent.
	ForgetAgent(sessionID string)

	// RunInAgentSession types a line into the agent running in the session and
	// waits for its turn to end, detected by silence.
	RunInAgentSession(
		ctx context.Context,
		sessionID string,
		line string,
		turnQuiet time.Duration,
	) (output string, exitCode int, idleStopped bool, err error)
}

// agentTurnQuietDefault is how long an agent must stay silent for its turn to be
// considered finished. An agent at work prints constantly, so silence is the
// signal; a duration ceiling would cut a long turn in half.
const agentTurnQuietDefault = 45 * time.Second

// AgentTurnQuiet reads the threshold, overridable by TASKFLOW_AGENT_TURN_QUIET
// (a Go duration such as "90s"), because the right value depends on the engine.
func AgentTurnQuiet() time.Duration {
	raw := strings.TrimSpace(os.Getenv("TASKFLOW_AGENT_TURN_QUIET"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("TASKACAO_AGENT_TURN_QUIET"))
	}
	if raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return agentTurnQuietDefault
}

// ErrTerminalBusy says the session was not at a prompt, so the command was not
// run and the caller must fall back instead of assuming it did.
var ErrTerminalBusy = errors.New("session de terminal occupée")

// SkillIdleBudget is how long a step may print nothing before being stopped.
// A duration ceiling would kill a long but talkative implementation; silence is
// the signal that actually means something went wrong.
const SkillIdleBudget = 10 * time.Minute

// SetTerminalRunner registers the terminal manager. Without it the worker keeps
// the headless path.
func (d *DB) SetTerminalRunner(t TerminalSessionRunner) {
	d.termRunner = t
}

// TaskSessionID is the PTY session of a task, shared by the interactive step and
// by the autonomous runs, so one console carries the whole chain of a ticket.
func TaskSessionID(taskID string) string {
	return "task-" + taskID
}

// runSkillInSession runs one workflow step in the task's session. It returns
// ErrTerminalBusy when the shell was busy, and the caller falls back to the
// headless path.
func (d *DB) runSkillInSession(
	ctx context.Context,
	settings *models.Settings,
	skillID string,
	task *models.Task,
	prompt string,
	executionDir string,
) (string, []string, error) {
	if d.termRunner == nil {
		return "", nil, ErrTerminalBusy
	}

	sessionID := TaskSessionID(task.ID)

	// Un agent déjà ouvert dans la session de la tâche : on lui parle plutôt que
	// de lancer un second moteur à côté. C'est la même console, la même
	// conversation, et l'appel de skill y est simplement tapé.
	if d.termRunner.AgentLaunched(sessionID) {
		return d.runSkillThroughAgent(ctx, settings, skillID, task, sessionID, prompt)
	}

	inv, err := d.runner.PrepareAI(settings, skillID, task, prompt)
	if err != nil {
		return "", nil, err
	}
	commandLine, cleanup, err := d.runner.SessionCommandLine(inv)
	if err != nil {
		return "", inv.Steps, err
	}
	defer cleanup()

	// La session peut préexister avec un autre répertoire courant : elle a pu
	// être ouverte avant que le worktree du ticket n'existe. On se replace donc
	// explicitement, sinon l'agent travaillerait dans le mauvais checkout.
	if strings.TrimSpace(executionDir) != "" {
		commandLine = fmt.Sprintf("cd %s && %s", shellQuoteDir(executionDir), commandLine)
	}

	steps := append([]string{}, inv.Steps...)
	steps = append(steps, fmt.Sprintf("🖥 Exécution dans la session de terminal %s, ouvrable pendant le run", sessionID))

	output, exitCode, idleStopped, runErr := d.termRunner.RunCommandInSession(
		ctx, sessionID, executionDir, nil, commandLine, SkillIdleBudget,
	)
	if runErr != nil {
		if errors.Is(runErr, ErrTerminalBusy) {
			return "", steps, ErrTerminalBusy
		}
		return output, steps, runErr
	}

	if idleStopped {
		steps = append(steps, fmt.Sprintf("⏱ Arrêté après %s sans aucune sortie", SkillIdleBudget))
		return output, steps, fmt.Errorf("aucune sortie pendant %s : exécution arrêtée", SkillIdleBudget)
	}
	if exitCode != 0 {
		steps = append(steps, fmt.Sprintf("⚠️ Le moteur IA est sorti en code %d", exitCode))
		return output, steps, fmt.Errorf("le moteur IA est sorti en code %d", exitCode)
	}

	steps = append(steps, "✅ Réponse générée par le modèle IA avec succès")
	return output, steps, nil
}

// runSkill picks the session path and falls back to the headless one, so a busy
// shell or a missing terminal never blocks a step.
func (d *DB) runSkill(
	ctx context.Context,
	settings *models.Settings,
	skillID string,
	task *models.Task,
	prompt string,
	executionDir string,
) (string, []string, error) {
	output, steps, err := d.runSkillInSession(ctx, settings, skillID, task, prompt, executionDir)
	if err == nil {
		return output, steps, nil
	}
	if !errors.Is(err, ErrTerminalBusy) {
		return output, steps, err
	}

	log.Printf("[skill] %s: session occupée ou indisponible, repli sur l'exécution directe", task.Key)
	out, hlSteps, hlErr := d.runner.RunAI(settings, skillID, task, prompt)
	merged := append([]string{"🖥 Session de terminal indisponible : exécution directe, sans console à ouvrir"}, hlSteps...)
	if strings.TrimSpace(out) == "" && hlErr == nil {
		merged = append(merged, "⚠️ Le moteur IA n'a rien écrit")
	}
	return out, merged, hlErr
}

// shellQuoteDir quotes a directory we produced ourselves, so a space in a path
// cannot split the command.
func shellQuoteDir(dir string) string {
	return "'" + strings.ReplaceAll(dir, "'", `'\''`) + "'"
}

// TTYSkillLaunch describes what was done to start a skill in a live session.
type TTYSkillLaunch struct {
	SessionID     string `json:"sessionId"`
	AgentLaunched bool   `json:"agentLaunched"`
	AgentRunning  bool   `json:"agentRunning"`
	Call          string `json:"call"`
	Cwd           string `json:"cwd"`
	Provider      string `json:"provider"`
	LaunchCommand string `json:"launchCommand"`
}

// ttyContext resolves what a task needs to work in its own session: the engine
// of its project, the directory, and the session id.
func (d *DB) ttyContext(taskID, skillID string) (*models.Task, *models.Settings, string, string, error) {
	if d.termRunner == nil {
		return nil, nil, "", "", fmt.Errorf("gestionnaire de terminal indisponible")
	}
	task, err := d.GetTaskByID(taskID)
	if err != nil || task == nil {
		return nil, nil, "", "", fmt.Errorf("tâche %s non trouvée", taskID)
	}
	settings, err := d.GetSettings()
	if err != nil || settings == nil {
		return nil, nil, "", "", fmt.Errorf("réglages illisibles")
	}
	// Le moteur du projet gagne sur le moteur global : sans ça, un projet
	// configuré pour Claude tentait de démarrer « agy », absent de la machine.
	d.applyProjectSettings(settings, task, skillID)

	cwd := d.ResolveTaskRepoPath(task)
	if task.WorktreePath != nil && strings.TrimSpace(*task.WorktreePath) != "" {
		cwd = *task.WorktreePath
	}
	if strings.TrimSpace(cwd) == "" {
		cwd = settings.RepoPath
	}
	return task, settings, cwd, TaskSessionID(task.ID), nil
}

// StartAgentInTTY opens the task's session and starts the agent configured on
// its project, and nothing else.
//
// Le lancement de la skill est un geste séparé : enchaîner les deux tout seul
// suppose de devinier quand l'invite de l'agent est prête, ce qui n'est pas
// fiable d'un moteur à l'autre. Deux boutons, deux gestes, rien à deviner.
func (d *DB) StartAgentInTTY(taskID string, force bool) (*TTYSkillLaunch, error) {
	task, settings, cwd, sessionID, err := d.ttyContext(taskID, "")
	if err != nil {
		return nil, err
	}

	launchLine, err := runner.InteractiveAgentLaunch(settings)
	if err != nil {
		return nil, err
	}
	if force {
		d.termRunner.ForgetAgent(sessionID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	launched, err := d.termRunner.EnsureAgentReady(ctx, sessionID, cwd, nil, launchLine)
	if err != nil {
		return nil, err
	}

	log.Printf("[tty] %s: agent %s dans %s (démarré: %v)", task.Key, settings.AIProvider, sessionID, launched)
	return &TTYSkillLaunch{
		SessionID:     sessionID,
		AgentLaunched: launched,
		AgentRunning:  d.termRunner.AgentLaunched(sessionID),
		Cwd:           cwd,
		Provider:      settings.AIProvider,
		LaunchCommand: launchLine,
	}, nil
}

// InjectSkillInTTY types the skill call into the agent running in the task's
// session. It refuses when no agent runs there: the call would land in the
// shell, which answers "no such file or directory: /clarify-issue".
func (d *DB) InjectSkillInTTY(taskID, skillID string) (*TTYSkillLaunch, error) {
	if _, ok := StageSkillByID(skillID); !ok {
		return nil, fmt.Errorf("skill %q inconnue", skillID)
	}
	task, settings, cwd, sessionID, err := d.ttyContext(taskID, skillID)
	if err != nil {
		return nil, err
	}
	if !d.termRunner.AgentLaunched(sessionID) {
		return nil, fmt.Errorf("aucun agent démarré dans la session de %s : démarre-le d'abord", task.Key)
	}

	trackerName := task.Source
	if trackerName == "" {
		trackerName = settings.IssueTracker
	}
	call := runner.SkillCallLineWithCommand(d.ProjectSkillCommand(task, skillID), task, strings.ToLower(trackerName))
	if err := d.termRunner.InjectLine(sessionID, call); err != nil {
		return nil, err
	}

	log.Printf("[tty] %s: %s injectée dans %s", task.Key, skillID, sessionID)
	return &TTYSkillLaunch{
		SessionID:    sessionID,
		AgentRunning: true,
		Call:         call,
		Cwd:          cwd,
		Provider:     settings.AIProvider,
	}, nil
}

// runSkillThroughAgent types the skill call into the agent already running in the
// task's session. The end of the turn is detected by silence: an agent prompt
// gives no exit code.
func (d *DB) runSkillThroughAgent(
	ctx context.Context,
	settings *models.Settings,
	skillID string,
	task *models.Task,
	sessionID string,
	prompt string,
) (string, []string, error) {
	trackerName := task.Source
	if trackerName == "" && settings != nil {
		trackerName = settings.IssueTracker
	}
	call := runner.SkillCallLineWithCommand(d.ProjectSkillCommand(task, skillID), task, strings.ToLower(trackerName))
	if strings.TrimSpace(prompt) != "" {
		// One terminal line, including the managed result contract and user context.
		call += " " + strings.NewReplacer("\r", " ", "\n", " ").Replace(prompt)
	}

	steps := []string{
		fmt.Sprintf("🖥 Agent déjà ouvert dans %s : l'appel y est tapé", sessionID),
		fmt.Sprintf("⌨️ %s", call),
	}

	output, _, idleStopped, err := d.termRunner.RunInAgentSession(ctx, sessionID, call, AgentTurnQuiet())
	if err != nil {
		return output, steps, err
	}
	if idleStopped {
		steps = append(steps, "⏱ L'agent n'a rien répondu : appel probablement perdu")
		return output, steps, fmt.Errorf("aucune réponse de l'agent après %s", AgentTurnQuiet())
	}

	steps = append(steps, fmt.Sprintf("✅ Tour terminé, détecté par %s de silence", AgentTurnQuiet()))
	return output, steps, nil
}
