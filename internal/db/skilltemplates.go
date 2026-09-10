package db

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"tasks/internal/models"
)

// The skills of the agentic workflow, one per step, in stage order.
//
// Before, five templates each lived their own life: the last stage had none and
// ran on a free prompt, and nothing told the agent who moves the ticket. Here a
// single table describes the steps and a single renderer produces the SKILL.md
// files, so the five read alike and carry the same contract.
//
// The files are written in English on purpose: the agents reason on them, and
// the repositories they land in are read by people who do not all speak French.
// The TaskFlow interface stays in French.
type StageSkill struct {
	ID          string // internal id, shared by the catalogue and the job queue
	Name        string
	DirName     string // skill directory, which is also the slash command
	Command     string
	FromStage   string
	ToStage     string
	Scope       string // "task" (default) or "macro"
	Interactive bool
	Description string // shown in the TaskFlow interface, in French
	Icon        string
	Color       string
	Steps       []string // the step list the interface displays

	// Body of the SKILL.md. The renderer adds the header, the stage line and the
	// contract with TaskFlow. title is the English heading of the file, kept
	// apart from Name, which the French interface displays.
	title           string
	frontmatterDesc string
	goal            string
	readFirst       string
	stepsBody       string
	guardTitle      string
	guard           string
	report          string
}

// StageSkills is the unified set: one skill per workflow step. Standalone
// pickup and the background pipeline share the same stage contracts.
var StageSkills = []StageSkill{
	{
		ID:          "clarify",
		Name:        "Clarify Issue",
		DirName:     models.SkillDirNames["clarify"],
		Command:     "/clarify-issue",
		FromStage:   "new",
		ToStage:     "clarified",
		Interactive: false,
		Description: "Résout les ambiguïtés réversibles et identifie les décisions indispensables.",
		Icon:        "HelpCircle",
		Color:       "amber",
		Steps: []string{
			"Lecture du ticket et du code concerné",
			"Détection des ambiguïtés et des dépendances",
			"Questions de cadrage et options recommandées",
			"Label 'clarified' et transition posés par TaskFlow",
		},
		title:           "Clarify Issue",
		frontmatterDesc: "Analyse a ticket against the code, surface what is genuinely undecided, and ask the few questions that unblock specification.",
		goal: `Turn a vague ticket into a decided one. You are looking for the decisions that
would be expensive to reverse later, not for a list of everything unknown.`,
		readFirst: `- The ticket: title, description, comments, parent epic if there is one.
- The code the change would touch. Name the files you actually read.
- Neighbouring features that already solve a similar problem in this codebase.`,
		stepsBody: `1. Restate the request in two sentences, including what you believe is out of scope.
2. List the ambiguities you found, worst first. An ambiguity is worth listing only
   if two readings lead to different code.
3. Name the critical dependencies: other services, other teams, migrations, data
   you do not have.
4. Resolve reversible choices using existing code and project conventions. Record
   the choice and rationale; do not ask questions merely to fill a quota.
   Ask only when an essential product decision changes acceptance criteria or an
   unavailable dependency prevents progress. In an unattended run, report the
   concrete blocker and the decision needed; do not invent settled requirements.
   A TTY alone does not make a run interactive: follow the invocation's mode.
5. Persist the settled scope and assumptions in the report for specification.`,
		guardTitle: "Do not",
		guard: `- Do not write production code at this stage, and do not start the specification.
- Do not invent an answer to your own question and move on without stating your assumption.
- Do not pad the list to reach five questions.`,
		report: `- Restated request and scope.
- Ambiguities, worst first.
- Critical dependencies.
- Numbered questions with your recommended option.
- Settled scope and assumptions.`,
	},
	{
		ID:          "specify",
		Name:        "Specify Issue",
		DirName:     models.SkillDirNames["specify"],
		Command:     "/specify-issue",
		FromStage:   "clarified",
		ToStage:     "specified",
		Description: "Rédige la spécification technique selon le cadre SDD du projet.",
		Icon:        "FileCode",
		Color:       "blue",
		Steps: []string{
			"Lecture des principes et des specs existantes",
			"Création de la branche de travail",
			"Rédaction de la spécification et de la checklist",
			"Label 'specified' et transition posés par TaskFlow",
		},
		title:           "Specify Issue",
		frontmatterDesc: "Write the executable specification of a ticket in the project's Spec-Driven Design framework, before any code.",
		goal: `Produce a specification another engineer could implement without asking you
anything. Behaviour and acceptance criteria first, implementation choices second,
and the two kept in separate files.`,
		// readFirst et stepsBody dépendent du cadre du projet : voir specifyFrameworkBody.
		guardTitle: "Do not",
		guard: `- Do not decide what the clarification left open. Mark it as open and say so.
- Do not describe implementation inside the behaviour file.
- Do not start implementing, even the easy part.`,
		report: `- The files written, with their paths.
- The work branch.
- Requirements that are still open, and what they block.`,
	},
	{
		ID:          "implement",
		Name:        "Implement Code",
		DirName:     models.SkillDirNames["implement"],
		Command:     "/code-issue",
		FromStage:   "specified",
		ToStage:     "implemented",
		Description: "Exécute le plan d'implémentation et valide par les tests.",
		Icon:        "Flame",
		Color:       "indigo",
		Steps: []string{
			"Lecture de la spécification et des tâches",
			"Implémentation sur la branche du ticket",
			"Construction, analyse statique et tests au vert",
			"Label 'implemented' et transition posés par TaskFlow",
		},
		title:           "Implement Code",
		frontmatterDesc: "Implement the ticket from its specification and prove it works with the project's own build, linters and tests.",
		goal: `Ship the change described by the specification, in code that reads like the code
already there, with the project's checks green.`,
		readFirst: `- The specification and its task checklist. It is the contract, follow its order.
- The surrounding code: naming, error handling, comment density, test style. Match it.
- How this project builds and tests. Find the real commands, do not assume them.`,
		stepsBody: `1. Reuse the assigned worktree and branch, including a shared batch branch. Never implement on the default branch.
2. Work through the checklist in small steps, each one leaving the tree buildable.
3. Add the tests that cover the new behaviour and its edge cases, not just the
   happy path. A change with no test needs a stated reason.
4. Run build, static analysis and tests. Fix until green, and quote the real output.
5. Re-read your own diff before finishing, as a reviewer would.`,
		guardTitle: "Recovery and blockers",
		guard: `- Repair routine technical issues and update design/tasks when the implementation
  needs to change while preserving acceptance criteria. Continue after documenting why.
- Establish whether a failing test predates the change. Fix failures in scope; report
  unrelated failures with baseline evidence. Never hide them or mark checks green.
- Stop only for an essential product decision, an unavailable dependency after
  bounded recovery attempts, or work that materially expands the requested scope.
- Preserve the work branch, completed checklist items and remaining next action so
  a retry can resume instead of starting over.`,
		report: `- What changed, file by file, and why.
- The real output of build, linters and tests, remaining failures included.
- What you deliberately left out, and what it would take to finish it.`,
	},
	{
		ID:          "create_pr",
		Name:        "Review & Pull Request",
		DirName:     models.SkillDirNames["create_pr"],
		Command:     "/create-pr",
		FromStage:   "implemented",
		ToStage:     "reviewed",
		Description: "Relit le diff, prépare le commit et ouvre la merge request.",
		Icon:        "ShieldCheck",
		Color:       "purple",
		Steps: []string{
			"Relecture du diff complet",
			"Commit conventionnel et poussée de la branche",
			"Ouverture de la merge request, fusion laissée à l'utilisateur",
			"Label 'reviewed' et transition posés par TaskFlow",
		},
		title:           "Review and Pull Request",
		frontmatterDesc: "Review the branch like a peer would, fix what the review finds, then open the merge request and leave the merge to the user.",
		goal: `Hand a reviewer a branch that is already worth reading: the obvious problems
found and fixed, the risky parts pointed out, the test plan written down.`,
		readFirst: `- The full diff of the branch against the default branch. All of it, not the summary.
- The specification, to check that what was asked is what was built.
- The current remote default branch: fetch the remote and identify its configured
  default branch before reviewing or publishing.`,
		stepsBody: `1. Fetch the remote (` + tick + `git fetch origin` + tick + `) and compare the work branch with the
   remote default branch (normally ` + tick + `origin/main` + tick + `; use the repository's configured default when different).
   Integrate missing base commits before the final review: prefer rebase when the branch is private, or merge when
   repository policy or shared-branch state requires it. Resolve conflicts and do not continue until the working tree is clean.
2. Review the resulting diff for correctness, side effects, security, and edge cases with no test.
3. Update documentation affected by the change. Fix what the review finds, now. A known defect belongs in the code, not in the
   description of the merge request.
4. Re-run build, static analysis and tests after integrating the default branch and on the final state.
5. Commit with a conventional message: type, scope, and why the change exists.
6. Push the branch and create or update its existing merge request: summary, test plan, and the specific
   places where you want a reviewer's eyes.
   If rebasing an already-pushed branch, use ` + tick + `git push --force-with-lease` + tick + `, never an unguarded force push.
7. If the repository has no remote, say so and stop rather than merging locally.`,
		guardTitle: "Do not",
		guard: `- Do not merge, do not approve, do not close the ticket. That is the user's call.
- Do not open a merge request on a red build. Report the failure instead.
- Do not open a merge request from a branch known to be behind the remote default branch.`,
		report: `- What the review found, and which findings you fixed.
- The merge request URL, or why there is none.
- The test plan a reviewer can replay, as a checklist.`,
	},
	{
		ID:          "handoff",
		Name:        "Handoff & clôture",
		DirName:     models.SkillDirNames["handoff"],
		Command:     "/handoff-issue",
		FromStage:   "reviewed",
		ToStage:     "finished",
		Description: "Rédige le compte-rendu de fin, vérifie la fusion et nettoie l'espace local.",
		Icon:        "CheckCircle2",
		Color:       "emerald",
		Steps: []string{
			"Vérification que la branche est fusionnée",
			"Compte-rendu de passation et vérifications de recette",
			"Nettoyage du worktree et de la branche locale",
			"Label 'finished' et transition posés par TaskFlow",
		},
		title:           "Handoff and Close",
		frontmatterDesc: "Close the ticket properly: confirm the merge, write the handover and the acceptance checklist, then clean the local workspace.",
		goal: `Leave two things behind: a handover a colleague can act on without asking you,
and a local workspace with nothing stale in it.`,
		readFirst: `- The state of the branch against the default branch.
- What the implementation and review steps reported, so the handover matches reality.`,
		stepsBody: `1. Confirm the ticket's branch is actually merged into the default branch. If it is
   not, stop, say so, and clean nothing.
2. Write the handover: what shipped, what changed for the user, what is still open.
3. Write the acceptance checklist as checkboxes, each item something a human can
   verify in the running product.
4. Confirm documentation shipped with the change. If a correction is still needed,
   record it as follow-up work; do not create uncommitted edits just before cleanup.
5. Turn any remaining follow-up into a separate ticket to create, rather than a
   paragraph nobody will read.
6. Clean up locally only after checking for uncommitted or unpushed work and other
   tickets sharing this worktree. Preserve a shared batch worktree until every ticket
   is handed off. Remove only an unused, clean worktree and its confirmed merged branch.`,
		guardTitle: "Do not",
		guard: `- Do not delete anything remote: no remote branch, no tag, no release.
- Do not clean up while the merge is unconfirmed.`,
		report: `- The handover.
- The acceptance checklist, as checkboxes.
- What was cleaned locally, and what could not be, with the reason.
- Follow-up tickets worth creating.`,
	},
	{
		ID:          "pickup",
		Name:        "Pickup & Auto-Pilot to PR",
		DirName:     models.SkillDirNames["pickup"],
		Command:     "/pickup-issue",
		FromStage:   "new",
		ToStage:     "reviewed",
		Interactive: false,
		Description: "Exécute en autonomie complète toutes les étapes d'un ticket jusqu'à la création de la Pull Request.",
		Icon:        "Sparkles",
		Color:       "purple",
		Steps: []string{
			"Cadrage des ambiguïtés (Clarify)",
			"Rédaction de la spécification technique SDD (Specify)",
			"Implémentation incrémentale et passage des tests (Code)",
			"Revue du diff, commit et ouverture de la PR/MR (Create PR)",
			"Mise à jour à chaque étape via le handler local TaskFlow",
		},
		title:           "Pickup Issue (Auto-Pilot to PR)",
		frontmatterDesc: "Pick a ticket and autonomously execute all development steps up to Pull Request creation.",
		goal: `Autonomously take a ticket from its current stage through clarification, specification,
implementation, and testing, all the way to opening a clean Pull Request, updating each stage via TaskFlow.`,
		readFirst: `- The ticket: key, title, description, parent macro, and tracker comments.
- The project's code and existing patterns.
- The project SDD framework (OpenSpec or Spec Kit).`,
		guardTitle: "Do not",
		guard: `- Do not merge into the default branch (merging is reserved for the human user).
- Do not push or open a PR if the test suite is failing.
- Follow the managed or standalone transition contract for the invocation.`,
		report: `- The created Pull Request URL.
- The work branch and files modified.
- The test results demonstrating that build, lint, and tests pass.
- Summary of settled scope and key architectural decisions.`,
	},
	{
		ID:          "rewrite_story",
		Name:        "Rewrite Story",
		DirName:     models.SkillDirNames["rewrite_story"],
		Command:     "/rewrite-story",
		FromStage:   "",
		ToStage:     "",
		Interactive: false,
		Description: "Reformate la description d'une tâche en User Story structurée GFM, avec inclusion facultative des commentaires.",
		Icon:        "Sparkles",
		Color:       "cyan",
		Steps: []string{
			"Analyse du titre, de la description et des commentaires du ticket",
			"Reformulation au format User Story + Contexte + Critères d'acceptation",
			"Aperçu et confirmation par l'utilisateur",
		},
		title:           "Rewrite Story",
		frontmatterDesc: "Reformat a story or task description into structured markdown, optionally incorporating task comments.",
		goal:            `Reformat a task's title, description, and optional comments into a clean GitHub-Flavored Markdown specification (User Story: As a..., I want..., So that... + Context + Acceptance Criteria + Notes).`,
		readFirst: `- The task: title, description, and task comments (if requested or passed as context).
- Standard GitHub-Flavored Markdown (GFM) formatting guidelines.`,
		stepsBody: `1. Inspect the task title, raw description, and comments (if provided).
2. Extract the core intent, user value, technical context, and acceptance criteria.
3. Generate a structured GFM document containing:
   - **User Story**: As a <role>, I want <feature>, So that <benefit>.
   - **Context**: Problem background and technical overview.
   - **Acceptance Criteria**: Checkbox list (- [ ]) of verifiable functional & non-functional requirements.
   - **Notes**: Extra technical details or risks mentioned in comments.
4. Output the reformatted markdown directly for preview and user confirmation.`,
		guardTitle: "Do not",
		guard: `- Do not mutate task title, status, priority, assignee, branch, or pull request.
- Do not delete or overwrite task comments.
- Do not invent artificial requirements not implied by the description or comments.`,
		report: `- The reformatted GFM description preview.
- List of comment points integrated into acceptance criteria (if any).`,
	},
	{
		ID:          "refine_macro",
		Name:        "Refine Macro",
		DirName:     models.SkillDirNames["refine_macro"],
		Command:     "/refine-macro",
		FromStage:   "macro",
		ToStage:     "macro",
		Scope:       "macro",
		Interactive: false,
		Description: "Transforme le cadrage d'une macro en un plan d'action de TODOs structuré selon le cadre SDD du projet.",
		Icon:        "ListChecks",
		Color:       "orange",
		Steps: []string{
			"Analyse du titre et du texte de cadrage de la macro",
			"Structuration du plan d'action selon le cadre SDD (SpecKit ou OpenSpec)",
			"Génération des items MacroTodo prêts à être appliqués",
		},
		title:           "Refine Macro",
		frontmatterDesc: "Refine a macro framing text into a structured action plan of todos respecting the project SDD framework.",
		goal:            `Transform high-level macro framing text into an actionable, structured todo list aligned with the active Spec-Driven Design framework (SpecKit or OpenSpec).`,
		guardTitle:      "Do not",
		guard: `- Do not overwrite existing todos without user confirmation in the UI.
- Do not generate unstructured or generic todo items.
- Do not mutate tracker issues or milestones directly without user trigger.`,
		report: `- Structured list of proposed MacroTodo items.
- Rationale behind the task breakdown.`,
	},
	{
		ID:          "pickup_issues",
		Name:        "Batch Pickup & Auto-Pilot to PR",
		DirName:     models.SkillDirNames["pickup_issues"],
		Command:     "/pickup-issues",
		FromStage:   "new",
		ToStage:     "reviewed",
		Interactive: false,
		Description: "Exécute en autonomie complète toutes les étapes d'un lot de tickets dans un unique Git worktree jusqu'à la PR.",
		Icon:        "Sparkles",
		Color:       "purple",
		Steps: []string{
			"Initialisation du Git Worktree dédié au lot",
			"Traitement séquentiel autonome de chaque ticket (Clarify, Specify, Code)",
			"Exécution des tests complets du projet",
			"Création d'une unique Pull Request combinée pour le lot",
		},
		title:           "Batch Pickup Issues (Single Worktree & Combined PR)",
		frontmatterDesc: "Batch process a list of selected board tickets sequentially in autonomy inside a single dedicated worktree, producing one combined Pull Request covered by tests and lints.",
		goal:            `Autonomously process a batch of tickets selected from the board sequentially in the exact order provided inside a single dedicated batch worktree.`,
		readFirst: `- The list of tickets in the batch.
- The project's code and existing patterns.
- The project SDD framework.`,
		guardTitle: "Do not",
		guard: `- Do not create separate branches or PRs per ticket.
- Do not merge into default branch (merging is reserved for human user).`,
		report: `- The created Pull Request URL.
- Summary of processed tickets and test results.`,
	},
}

// StageSkillByID returns the unified skill for an internal id. "review" is the
// historical alias of create_pr, still used by queued jobs.
func StageSkillByID(skillID string) (StageSkill, bool) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "review" {
		skillID = "create_pr"
	}
	if skillID == "pick_issues" || skillID == "pickup_issues" || skillID == "pickup-issues" {
		skillID = "pickup_issues"
	}
	if skillID == "pick" || skillID == "pick_issue" || skillID == "pickup_issue" || skillID == "pickup-issue" || skillID == "pick-issue" {
		skillID = "pickup"
	}
	if skillID == "rewrite" || skillID == "rewrite-story" || skillID == "rewrite_story" {
		skillID = "rewrite_story"
	}
	if skillID == "refine" || skillID == "refine-macro" || skillID == "refine_macro" {
		skillID = "refine_macro"
	}
	for _, s := range StageSkills {
		if s.ID == skillID {
			return s, true
		}
	}
	return StageSkill{}, false
}

const tick = "`"

// JSON strings are valid YAML scalars, including colons, quotes and newlines.
func skillYAMLString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// specifyFrameworkName is the display name of the specification skill: the SDD
// framework is part of what the step actually is.
func specifyFrameworkName(specFramework string) string {
	if strings.EqualFold(strings.TrimSpace(specFramework), "openspec") {
		return "Specify Issue (OpenSpec SDD)"
	} else if strings.EqualFold(strings.TrimSpace(specFramework), "speckit") {
		return "Specify Issue (Spec Kit SDD)"
	}
	return "Specify Issue (Spec-Driven Design)"
}

func refineMacroFrameworkName(specFramework string) string {
	if strings.EqualFold(strings.TrimSpace(specFramework), "openspec") {
		return "Refine Macro (OpenSpec SDD)"
	} else if strings.EqualFold(strings.TrimSpace(specFramework), "speckit") {
		return "Refine Macro (Spec Kit SDD)"
	}
	return "Refine Macro (Spec-Driven Design)"
}

func refineMacroFrameworkBody(specFramework string) (readFirst, steps string) {
	readFirst = `- The macro title and framing description.
- The active project SDD framework (SpecKit or OpenSpec).
- Existing macro todos to avoid duplicating completed work.`

	steps = `1. Read the macro title and high-level framing description.
2. Structure the action plan according to the selected SDD framework:

   **If using SpecKit SDD:**
   - Group action items into User Stories and Feature Modules.
   - Format each todo item clearly with functional scope (e.g. "[US-1] User Story description" or "[FEAT] Feature item").

   **If using OpenSpec SDD:**
   - Group action items into Capabilities and Change Proposals.
   - Format each todo item clearly with delta scope (e.g. "[CAP-1] Capability requirement" or "[CHANGE] Proposal change").

3. Output the generated checklist of actionable todos for preview before applying to the macro.`

	return readFirst, steps
}

// specifyFrameworkBody produces the specification instructions, supporting explicit
// {sdd_framework} parameterization (openspec / speckit) as well as auto-detection.
func specifyFrameworkBody(specFramework string) (readFirst, steps string) {
	readFirst = `- The clarification outcome on the ticket: the decisions are already made, apply them.
- Select the SDD framework in order: explicit {sdd_framework} or --framework=<name>,
  then the project-configured framework, then repository detection:
  - If ` + tick + `openspec/` + tick + ` exists -> use OpenSpec SDD.
  - If ` + tick + `.specify/` + tick + ` or ` + tick + `specs/` + tick + ` exists -> use Spec Kit SDD.
- Ensure the project SDD directory is initialized before writing specifications.`

	steps = `1. Reuse the assigned worktree and branch (including a shared batch branch). Only create <KEY>-<title-slug> when no work branch is assigned. Preserve existing work; never write on the default branch.
2. Select the SDD framework from {sdd_framework} argument, flag, or project detection:

   **If using OpenSpec SDD:**
   - Create change directory ` + tick + `openspec/changes/<KEY>-<title-slug>/` + tick + `
   - Write ` + tick + `proposal.md` + tick + ` (problem, value, in/out scope)
   - Write ` + tick + `design.md` + tick + ` (technical decisions, rejected alternatives)
   - Write ` + tick + `tasks.md` + tick + ` (ordered implementation checklist)
   - Write ` + tick + `specs/<capability>/spec.md` + tick + ` (requirements with Given/When/Then)
   - Validate with ` + tick + `openspec validate <change-id> --strict` + tick + `

   **If using Spec Kit SDD:**
   - Write ` + tick + `specs/<KEY>-<title-slug>/spec.md` + tick + ` (prioritised user stories, functional requirements, Given/When/Then)
   - Write ` + tick + `plan.md` + tick + ` (stack, architecture, data contracts, target files)
   - Write ` + tick + `tasks.md` + tick + ` (ordered implementation checklist with test plan)
   - Use ` + tick + `/speckit.specify` + tick + `, ` + tick + `/speckit.plan` + tick + `, ` + tick + `/speckit.tasks` + tick + ` if available.`

	if framework := strings.ToLower(strings.TrimSpace(specFramework)); framework == "openspec" || framework == "speckit" {
		readFirst = "- Project-configured SDD framework: " + framework + ". Use it unless the invocation explicitly overrides it.\n" + readFirst
	}
	return readFirst, steps
}

// renderTicketTransitionContract generates the autonomous ticket transition instructions
// for the skill based on its from/to stages in the sequence:
// new -> clarified -> specified -> implemented -> reviewed -> finished
func renderTicketTransitionContract(s StageSkill) string {
	if s.FromStage == "" || s.Scope == "macro" {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Execution and ticket state\n")
	b.WriteString("- **Managed TaskFlow run**: When the invocation supplies a result-file contract, follow it. TaskFlow validates the result and owns transitions and tracker reports. Do not also call stage/postback APIs or edit tracker labels.\n")
	b.WriteString("- **Standalone invocation**: After verifying each completed step, use the local handler below. Check its exit status and response. If it is unavailable, preserve work and report the pending transition; do not silently diverge local and tracker state.\n")
	if s.ID == "pickup" || s.ID == "pickup_issues" {
		if s.ID == "pickup_issues" {
			b.WriteString("For each ticket key in the batch, record clarified, specified and implemented after that ticket's corresponding step. Use the SAME actual batch branch for every ticket. After the combined PR is verified, record reviewed and the SAME PR URL for every implemented ticket. Never mark an unfinished ticket reviewed.\n")
		}
		b.WriteString("```bash\n")
		b.WriteString(renderStageHandlerCommand("clarified", "<settled scope and assumptions>", false, false))
		b.WriteString(renderStageHandlerCommand("specified", "<spec paths>", true, false))
		b.WriteString(renderStageHandlerCommand("implemented", "<check results>", true, false))
		b.WriteString(renderStageHandlerCommand("reviewed", "<review summary>", false, true))
		b.WriteString("```\n")
	} else {
		fmt.Fprintf(&b, "Transition %s → %s only when this step is complete.\n```bash\n", s.FromStage, s.ToStage)
		b.WriteString(renderStageHandlerCommand(s.ToStage, "<REPORT_NOTE>", s.ID == "implement" || s.ID == "specify", s.ID == "create_pr"))
		b.WriteString("```\n")
	}
	b.WriteString("This POST calls the local TaskFlow handler directly. Confirm HTTP success before continuing. If it is unavailable, preserve work and report the pending transition; do not silently diverge local and tracker state.\n")
	b.WriteString("Reuse the assigned worktree and actual branch. Never merge or delete remote objects. Keep work available for review and retry until confirmed handoff.\n")
	return b.String()
}

func renderStageHandlerCommand(stage, note string, includesBranch, includesPRURL bool) string {
	fields := []string{
		`"taskKey":"<KEY>"`,
		fmt.Sprintf(`"stage":"%s"`, stage),
		fmt.Sprintf(`"note":"%s"`, note),
	}
	if includesBranch {
		fields = append(fields, `"branch":"<ACTUAL_BRANCH>"`)
	}
	if includesPRURL {
		fields = append(fields, `"prUrl":"<PR_URL>"`)
	}
	return "curl --fail-with-body --silent --show-error -X POST http://localhost:8090/api/tasks/stage \\\n  -H 'Content-Type: application/json' \\\n  -d '{" + strings.Join(fields, ",") + "}'\n"
}

// Pickup embeds the maintained stage bodies, so batch and single-ticket runs
// cannot silently omit a validation rule added to a standalone step.
func renderPickupSteps(specFramework string, batch bool) string {
	var b strings.Builder
	b.WriteString("1. Inspect the current ticket state AND existing artifacts. Reuse assigned branches, specifications, checklist progress and PRs. Verify completed work before skipping it.\n")
	if batch {
		b.WriteString("2. Use one dedicated worktree and branch for the ordered batch. Run clarification, specification and implementation for each ticket in order. If one blocks, preserve the batch and report completed tickets and the next action; never include unfinished work as completed.\n3. Once all tickets are implemented, run review and final checks across the whole batch and create or update ONE combined PR.\n")
	} else {
		b.WriteString("2. Reuse or create a dedicated worktree and work branch. Continue through the stages below from the first incomplete stage to a verified PR.\n")
	}
	b.WriteString("Stop before merge. Stage-local boundaries apply while that stage is active; after its requirements are met, continue to the next stage without asking for routine confirmation.\n")
	for _, id := range []string{"clarify", "specify", "implement", "create_pr"} {
		step, _ := StageSkillByID(id)
		readFirst, body := step.readFirst, step.stepsBody
		if id == "specify" {
			readFirst, body = specifyFrameworkBody(specFramework)
		}
		fmt.Fprintf(&b, "\n### %s\n%s\n\n%s\n\n%s\n\nReport and persist before continuing:\n%s\n", step.title, readFirst, body, step.guard, step.report)
	}
	return b.String()
}

// RenderSkillContent builds the SKILL.md of one skill.
func RenderSkillContent(s StageSkill, specFramework string) string {
	name := s.title
	if name == "" {
		name = s.Name
	}
	readFirst := s.readFirst
	steps := s.stepsBody
	if s.ID == "specify" {
		name = specifyFrameworkName(specFramework)
		readFirst, steps = specifyFrameworkBody(specFramework)
	}
	if s.ID == "refine_macro" {
		name = refineMacroFrameworkName(specFramework)
		readFirst, steps = refineMacroFrameworkBody(specFramework)
	}

	if s.ID == "pickup" || s.ID == "pickup_issues" {
		steps = renderPickupSteps(specFramework, s.ID == "pickup_issues")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %s\ndescription: %s\n---\n", s.DirName, skillYAMLString(s.frontmatterDesc))
	fmt.Fprintf(&b, "# %s\n\n", name)
	if s.FromStage != "" && s.Scope != "macro" {
		fmt.Fprintf(&b, "Stage: %s -> %s.", s.FromStage, s.ToStage)
	}
	if s.Interactive {
		b.WriteString(" Interactive: the user answers in the terminal.")
	}
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "## Goal\n%s\n\n", s.goal)
	if readFirst != "" {
		fmt.Fprintf(&b, "## Read first\n%s\n\n", readFirst)
	}
	if steps != "" {
		fmt.Fprintf(&b, "## Steps\n%s\n\n", steps)
	}
	if s.guard != "" {
		fmt.Fprintf(&b, "## %s\n%s\n\n", s.guardTitle, s.guard)
	}
	fmt.Fprintf(&b, "## Report\n%s\n\n", s.report)
	b.WriteString(renderTicketTransitionContract(s))
	return b.String()
}

// ProjectSkillTemplates returns the unified set ready to be written, with the
// specification skill resolved for the project's SDD framework.
func ProjectSkillTemplates(specFramework string) []ProjectSkillTemplate {
	out := make([]ProjectSkillTemplate, 0, len(StageSkills))
	for _, s := range StageSkills {
		name := s.Name
		if s.ID == "specify" {
			name = specifyFrameworkName(specFramework)
		}
		if s.ID == "refine_macro" {
			name = refineMacroFrameworkName(specFramework)
		}
		out = append(out, ProjectSkillTemplate{
			ID:          s.ID,
			Name:        name,
			DirName:     s.DirName,
			Description: s.Description,
			Content:     RenderSkillContent(s, specFramework),
		})
	}
	return out
}

// SkillDirsFor returns the skill directories of one skill inside a checkout,
// one per agent CLI. The same file is written to all of them: one source,
// several readers.
func SkillDirsFor(root, dirName string) []string {
	dirs := make([]string, 0, len(models.SkillAgentDirs))
	for _, agent := range models.SkillAgentDirs {
		if agent == "" {
			// La convention agnostique, a la racine : .skills/<nom>
			dirs = append(dirs, filepath.Join(root, ".skills", dirName))
			continue
		}
		dirs = append(dirs, filepath.Join(root, agent, "skills", dirName))
	}
	return dirs
}

// SkillCommandPath is where a skill's slash command lives for Claude Code.
//
// Une skill et une commande ne sont pas la même chose : une skill sous
// .claude/skills/<nom>/SKILL.md est choisie par le modèle quand il la juge
// pertinente, alors qu'une commande sous .claude/commands/<nom>.md est
// invocable par « /<nom> ». TaskFlow appelle ses étapes par leur commande, donc
// il faut écrire les deux, sinon « claude -p "/clarify-issue PROJ-123" » se
// contente de recopier le texte.
func SkillCommandPath(root, dirName string) string {
	return filepath.Join(root, ".claude", "commands", dirName+".md")
}

// RenderSkillCommand turns a rendered SKILL.md into its slash command: same
// instructions, a command frontmatter, and the ticket passed as $ARGUMENTS.
func RenderSkillCommand(s StageSkill, specFramework string) string {
	body := RenderSkillContent(s, specFramework)

	// Le frontmatter d'une skill porte name et description ; celui d'une commande
	// porte description et argument-hint. On retire le premier pour écrire le bon.
	if strings.HasPrefix(body, "---\n") {
		if end := strings.Index(body[4:], "\n---\n"); end >= 0 {
			body = body[4+end+5:]
		}
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: %s\n", skillYAMLString(s.frontmatterDesc))
	b.WriteString("argument-hint: <TICKET-KEY> [contexte]\n")
	b.WriteString("---\n")
	b.WriteString(strings.TrimSpace(body))
	b.WriteString("\n\n## Ticket\n$ARGUMENTS\n")
	return b.String()
}

// CommandContentFor returns the slash command content of a skill id, with the
// project's specification framework resolved.
func CommandContentFor(skillID, specFramework string) (string, bool) {
	stage, ok := StageSkillByID(skillID)
	if !ok {
		return "", false
	}
	return RenderSkillCommand(stage, specFramework), true
}
