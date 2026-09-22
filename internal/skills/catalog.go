// Package skills holds the static catalogue of workflow skills and the
// rendering of their SKILL.md and slash-command files.
//
// It is deliberately free of any storage dependency. The agent renders skills
// from this catalogue, and ADR 0006 keeps the agent runtime clear of the
// database layer and of the SQLite driver; cmd/server/runtime_boundary_test.go
// enforces that. Anything that reads or writes persisted state belongs in
// internal/db, which consumes this package rather than hosting it.
package skills

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"tasks/internal/models"
)

//go:embed fragments/*
var embeddedSkillsFS embed.FS

// The skills of the agentic workflow, one per step, in stage order.
//
// All prose lives in markdown fragments under internal/skills/fragments/ embedded with
// go:embed. StageSkill holds strictly the metadata needed by the catalogue,
// router, UI, and template assembly.
type StageSkill struct {
	ID          string // internal id, shared by the catalogue and the job queue
	Name        string
	DirName     string // skill directory, which is also the slash command
	Command     string
	FromStage   string
	ToStage     string
	Scope       string // "task" (default) or "macro"
	Mode        string
	Description string // shown in the Sectile interface, in French
	Icon        string
	Color       string
	Steps       []string // the step list the interface displays

	Title           string
	FrontmatterDesc string
	GuardTitle      string
}

// StageSkills is the unified set: one skill per workflow step. Standalone
// pickup and the background pipeline share the same stage contracts.
var StageSkills = []StageSkill{
	{
		ID:          "clarify",
		Name:        "Clarify",
		DirName:     models.SkillDirNames["clarify"],
		Command:     "/clarify-issue",
		FromStage:   "new",
		ToStage:     "clarified",
		Description: "Résout les ambiguïtés réversibles et mène des rounds d'échange jusqu'à confirmation du cadrage.",
		Icon:        "HelpCircle",
		Color:       "amber",
		Steps: []string{
			"Lecture du ticket et du code concerné",
			"Itération en rounds (Round 1, Round N) consignés dans docs/clarifications/<n>.md",
			"Questions bloquantes posées à l'interlocuteur ou en commentaire de ticket",
			"Arrêt sans transition tant que des choix produit restent ouverts",
			"Label 'clarified' et transition posés uniquement après confirmation",
		},
		Title:           "Clarify Issue",
		FrontmatterDesc: "Analyse a ticket against the code, iterate in clarification rounds, and resolve open questions until the owner confirms satisfaction.",
		GuardTitle:      "Do not",
	},
	{
		ID:          "specify",
		Name:        "Specify",
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
			"Label 'specified' et transition posés par Sectile",
		},
		Title:           "Specify Issue",
		FrontmatterDesc: "Write the executable specification of a ticket in the project's Spec-Driven Design framework, before any code.",
		GuardTitle:      "Do not",
	},
	{
		ID:          "implement",
		Name:        "Implement",
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
			"Label 'implemented' et transition posés par Sectile",
		},
		Title:           "Implement Code",
		FrontmatterDesc: "Implement the ticket from its specification and prove it works with the project's own build, linters and tests.",
		GuardTitle:      "Recovery and blockers",
	},
	{
		ID:          "adjust",
		Name:        "Adjust",
		DirName:     models.SkillDirNames["adjust"],
		Command:     "/adjust-issue",
		FromStage:   "implemented",
		ToStage:     "reviewed",
		Description: "Review the complete branch, fix findings and update the existing pull request.",
		Icon:        "ShieldCheck",
		Color:       "purple",
		Steps: []string{
			"Relecture du diff complet",
			"Commit conventionnel et poussée de la branche",
			"Update the existing pull request and verify readiness; human merge",
			"Label 'reviewed' et transition posés par Sectile",
		},
		Title:           "Adjust Existing Pull Request",
		FrontmatterDesc: "Review the branch like a peer would, fix what the review finds, then update the existing merge request and leave the merge to the user.",
		GuardTitle:      "Do not",
	},
	{
		ID:          "handoff",
		Name:        "Handoff",
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
			"Label 'finished' et transition posés par Sectile",
		},
		Title:           "Handoff and Close",
		FrontmatterDesc: "Close the ticket properly: confirm the merge, write the handover and the acceptance checklist, then clean the local workspace.",
		GuardTitle:      "Do not",
	},
	{
		ID:              "create_pr",
		Name:            "Create PR",
		DirName:         models.SkillDirNames["create_pr"],
		Command:         "/create-pr",
		Description:     "Create or reuse a pull request independently of the workflow stages.",
		Icon:            "GitPullRequest",
		Color:           "purple",
		Steps:           []string{"Inspect the branch and existing pull requests", "Run the required checks", "Create or update the pull request without changing the workflow stage"},
		Title:           "Create PR",
		FrontmatterDesc: "Create or reuse a pull request for the current task branch without advancing its workflow stage.",
		GuardTitle:      "Do not",
	},
	{
		ID:          "pickup",
		Name:        "Pickup & Auto-Pilot to PR",
		DirName:     models.SkillDirNames["pickup"],
		Command:     "/pickup-issue",
		FromStage:   "new",
		ToStage:     "reviewed",
		Description: "Exécute en autonomie complète toutes les étapes d'un ticket jusqu'à la création de la Pull Request.",
		Icon:        "Sparkles",
		Color:       "purple",
		Steps: []string{
			"Cadrage des ambiguïtés (Clarify)",
			"Rédaction de la spécification technique SDD (Specify)",
			"Implémentation incrémentale et passage des tests (Code)",
			"Adjust the complete diff and existing PR after earlier-stage creation",
			"Mise à jour à chaque étape via le handler local Sectile",
		},
		Title:           "Pickup Issue (Auto-Pilot to PR)",
		FrontmatterDesc: "Pick a ticket and autonomously execute all development steps up to Pull Request creation.",
		GuardTitle:      "Do not",
	},
	{
		ID:          "rewrite_story",
		Name:        "Rewrite Story",
		DirName:     models.SkillDirNames["rewrite_story"],
		Command:     "/rewrite-story",
		FromStage:   "",
		ToStage:     "",
		Description: "Reformate la description d'une tâche en User Story structurée GFM, avec inclusion facultative des commentaires.",
		Icon:        "Sparkles",
		Color:       "cyan",
		Steps: []string{
			"Analyse du titre, de la description et des commentaires du ticket",
			"Reformulation au format User Story + Contexte + Critères d'acceptation",
			"Aperçu et confirmation par l'utilisateur",
		},
		Title:           "Rewrite Story",
		FrontmatterDesc: "Reformat a story or task description into structured markdown, optionally incorporating task comments.",
		GuardTitle:      "Do not",
	},
	{
		ID:          "refine_macro",
		Name:        "Refine Macro",
		DirName:     models.SkillDirNames["refine_macro"],
		Command:     "/refine-macro",
		FromStage:   "macro",
		ToStage:     "macro",
		Scope:       "macro",
		Mode:        models.SkillModeInteractive,
		Description: "Clarifie de manière interactive le cadrage d'une macro et le décompose en TODOs structurés et cartes Sectile.",
		Icon:        "ListChecks",
		Color:       "orange",
		Steps: []string{
			"Analyse du titre et du texte de cadrage de la macro",
			"Évaluation de la complétude du cadrage et questions de clarification si nécessaire",
			"Structuration du plan d'action selon le cadre SDD (SpecKit ou OpenSpec)",
			"Génération des items MacroTodo et découpage des tickets Sectile prêts à être créés",
		},
		Title:           "Refine Macro",
		FrontmatterDesc: "Interactively clarify macro framing text with the user and break it down into structured todos and Sectile tickets.",
		GuardTitle:      "Do not",
	},
	{
		ID:          "pickup_issues",
		Name:        "Batch Pickup & Auto-Pilot to PR",
		DirName:     models.SkillDirNames["pickup_issues"],
		Command:     "/pickup-issues",
		FromStage:   "new",
		ToStage:     "reviewed",
		Description: "Exécute en autonomie complète toutes les étapes d'un lot de tickets dans un unique Git worktree jusqu'à la PR.",
		Icon:        "Sparkles",
		Color:       "purple",
		Steps: []string{
			"Initialisation du Git Worktree dédié au lot",
			"Traitement séquentiel autonome de chaque ticket (Clarify, Specify, Code)",
			"Exécution des tests complets du projet",
			"Création d'une unique Pull Request combinée pour le lot",
		},
		Title:           "Batch Pickup Issues (Single Worktree & Combined PR)",
		FrontmatterDesc: "Batch process a list of selected board tickets sequentially in autonomy inside a single dedicated worktree, producing one combined Pull Request covered by tests and lints.",
		GuardTitle:      "Do not",
	},
}

// StageSkillByID resolves canonical and legacy workflow identities.
func StageSkillByID(skillID string) (StageSkill, bool) {
	skillID = models.NormalizeSkillID(skillID)
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

func readSkillFragment(skillID, name, framework string) string {
	framework = strings.ToLower(strings.TrimSpace(framework))
	if framework != "" {
		path := filepath.Join("fragments", skillID, name+"."+framework+".md")
		if data, err := embeddedSkillsFS.ReadFile(path); err == nil {
			return string(data)
		}
	}
	path := filepath.Join("fragments", skillID, name+".md")
	data, err := embeddedSkillsFS.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func readContractFragment(name string) string {
	data, err := embeddedSkillsFS.ReadFile(filepath.Join("fragments", "contracts", name+".md"))
	if err != nil {
		return ""
	}
	return string(data)
}

func executeContractTemplate(tmplStr string, data any) string {
	t, err := template.New("contract").Parse(tmplStr)
	if err != nil {
		return ""
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return ""
	}
	return buf.String()
}

// YAMLString encodes a value as a YAML scalar: JSON strings are valid YAML
// scalars, including colons, quotes and newlines.
func YAMLString(value string) string {
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

// renderTaskAccessContract keeps task access consistent across skills and commands.
func renderTaskAccessContract() string {
	return strings.TrimRight(readContractFragment("task-access"), "\n") + "\n\n"
}

// renderSessionTitleContract names the agent session after the work item it runs
// on, so a board of parallel sessions stays readable.
func renderSessionTitleContract(s StageSkill) string {
	item := "ticket"
	if s.Scope == "macro" {
		item = "macro"
	}
	tmpl := readContractFragment("session-title")
	res := executeContractTemplate(tmpl, map[string]any{
		"Item": item,
		"ID":   s.ID,
	})
	return strings.TrimRight(res, "\n") + "\n\n"
}

// renderTicketTransitionContract generates the autonomous ticket transition instructions
// for the skill based on its from/to stages in the sequence:
// new -> clarified -> specified -> implemented -> reviewed -> finished
func renderTicketTransitionContract(s StageSkill) string {
	if s.Scope == "macro" {
		return ""
	}
	tmpl := readContractFragment("transition")
	res := executeContractTemplate(tmpl, map[string]any{
		"FromStage": s.FromStage,
		"ToStage":   s.ToStage,
		"ID":        s.ID,
	})
	return strings.TrimRight(res, "\n") + "\n"
}

// StripFrontmatter removes a leading `---` block. Sectile generates the only
// frontmatter a rendered file carries, so foreign prose spliced into it — a
// legacy customization, a marketplace pack body — leaves its own behind.
func StripFrontmatter(content string) string {
	if strings.HasPrefix(content, "---\n") {
		if end := strings.Index(content[4:], "\n---\n"); end >= 0 {
			return content[4+end+5:]
		}
	}
	return content
}

// Pickup embeds the maintained stage bodies, so batch and single-ticket runs
// cannot silently omit a validation rule added to a standalone step.
//
// baselines carries, per stage id, the body that stage resolved to when it is
// not the built-in one — a marketplace pack, today. A batch run must not lag
// behind a stage a pack updated, so the composed section is that body rather
// than the catalogue prose.
func renderPickupSteps(specFramework string, batch bool, baselines map[string]string) string {
	var b strings.Builder
	tmpl := readContractFragment("pickup-header")
	header := executeContractTemplate(tmpl, map[string]any{
		"Batch": batch,
	})
	b.WriteString(strings.TrimRight(header, "\n") + "\n")

	for _, id := range []string{"clarify", "specify", "implement", "adjust"} {
		step, _ := StageSkillByID(id)
		title := step.Title
		if resolved := strings.TrimSpace(StripFrontmatter(baselines[id])); resolved != "" {
			fmt.Fprintf(&b, "\n### %s\n%s\n", title, resolved)
			continue
		}
		readFirst := readSkillFragment(id, "read-first", specFramework)
		steps := readSkillFragment(id, "steps", specFramework)
		guard := readSkillFragment(id, "guard", "")
		report := readSkillFragment(id, "report", "")
		fmt.Fprintf(&b, "\n### %s\n%s\n\n%s\n\n%s\n\nReport and persist before continuing:\n%s\n", title, readFirst, steps, guard, report)
	}
	return b.String()
}

// RenderSkillContent builds the SKILL.md of one skill from the built-in
// catalogue.
func RenderSkillContent(s StageSkill, specFramework string) string {
	return renderSkillContent(s, specFramework, nil)
}

// RenderSkillWithBody renders a skill whose prose comes from a marketplace
// pack: Sectile's header and contracts, the pack body in between.
//
// Nothing a pack ships can remove a generated contract. The frontmatter, the
// title, the stage line, the task-access and session-title contracts and the
// ticket transition are Sectile's, and the pack body sits between them under
// `## Project instructions` — the shape adjustmentCustomContent already
// established for a reconciled adjustment, so there is one way of putting
// foreign prose inside a Sectile contract and not two.
//
// baselines is what the batch skills compose from; it is read for pickup and
// pickup_issues only, whose embedded stage sections have to follow the pack
// rather than the catalogue.
func RenderSkillWithBody(s StageSkill, specFramework, body string, baselines map[string]string) string {
	generated := renderSkillContent(s, specFramework, baselines)
	head, _, found := strings.Cut(generated, "## Goal\n")
	if !found {
		head = generated
	}

	var b strings.Builder
	b.WriteString(head)
	b.WriteString("## Project instructions\n\n")
	b.WriteString(strings.TrimSpace(StripFrontmatter(body)))
	b.WriteString("\n\n")
	// A batch skill keeps its composed stages: the pack replaces the prose of
	// the step, not the steps the batch has to walk through.
	if s.ID == "pickup" || s.ID == "pickup_issues" {
		fmt.Fprintf(&b, "## Steps\n%s\n\n", renderPickupSteps(specFramework, s.ID == "pickup_issues", baselines))
	}
	if contract := renderTicketTransitionContract(s); contract != "" {
		b.WriteString(contract)
	}
	return b.String()
}

func renderSkillContent(s StageSkill, specFramework string, baselines map[string]string) string {
	name := s.Title
	if name == "" {
		name = s.Name
	}
	readFirst := readSkillFragment(s.ID, "read-first", specFramework)
	steps := readSkillFragment(s.ID, "steps", specFramework)
	if s.ID == "specify" {
		name = specifyFrameworkName(specFramework)
	}
	if s.ID == "refine_macro" {
		name = refineMacroFrameworkName(specFramework)
	}

	if s.ID == "pickup" || s.ID == "pickup_issues" {
		steps = renderPickupSteps(specFramework, s.ID == "pickup_issues", baselines)
	}

	goal := readSkillFragment(s.ID, "goal", "")
	guard := readSkillFragment(s.ID, "guard", "")
	report := readSkillFragment(s.ID, "report", "")

	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %s\ndescription: %s\n---\n", s.DirName, YAMLString(s.FrontmatterDesc))
	fmt.Fprintf(&b, "# %s\n\n", name)
	if s.FromStage != "" && s.Scope != "macro" {
		fmt.Fprintf(&b, "Stage: %s -> %s.", s.FromStage, s.ToStage)
		if s.Mode == models.SkillModeInteractive {
			b.WriteString(" Interactive: the user answers in the terminal.")
		}
	} else if s.Mode == models.SkillModeInteractive {
		b.WriteString("Interactive: the user answers in the terminal.")
	}
	b.WriteString("\n\n")
	b.WriteString(renderTaskAccessContract())
	b.WriteString(renderSessionTitleContract(s))
	fmt.Fprintf(&b, "## Goal\n%s\n\n", goal)
	if readFirst != "" {
		fmt.Fprintf(&b, "## Read first\n%s\n\n", readFirst)
	}
	if steps != "" {
		fmt.Fprintf(&b, "## Steps\n%s\n\n", steps)
	}
	if guard != "" {
		guardTitle := s.GuardTitle
		if guardTitle == "" {
			guardTitle = "Do not"
		}
		fmt.Fprintf(&b, "## %s\n%s\n\n", guardTitle, guard)
	}
	if contract := renderTicketTransitionContract(s); contract != "" {
		fmt.Fprintf(&b, "## Report\n%s\n\n", report)
		b.WriteString(contract)
	} else {
		fmt.Fprintf(&b, "## Report\n%s\n", report)
	}
	return b.String()
}

// ProjectSkillTemplate is one skill ready to be provisioned into a checkout:
// the catalogue metadata plus the content rendered for a given framework.
type ProjectSkillTemplate struct {
	ID          string
	Name        string
	DirName     string
	Description string
	Content     string
}

// ProjectSkillTemplates returns the unified set ready to be written, with the
// specification skill resolved for the project's SDD framework.
func ProjectSkillTemplates(specFramework string) []ProjectSkillTemplate {
	return ProjectSkillTemplatesOver(specFramework, nil)
}

// ProjectSkillTemplatesOver renders the whole set over a baseline map: the
// body a marketplace pack supplies for a skill replaces the catalogue prose of
// that skill, and the batch skills compose from the same map.
func ProjectSkillTemplatesOver(specFramework string, baselines map[string]string) []ProjectSkillTemplate {
	out := make([]ProjectSkillTemplate, 0, len(StageSkills))
	for _, s := range StageSkills {
		name := s.Name
		if s.ID == "refine_macro" {
			name = refineMacroFrameworkName(specFramework)
		}
		content := renderSkillContent(s, specFramework, baselines)
		if body, ok := baselines[s.ID]; ok && strings.TrimSpace(body) != "" {
			content = RenderSkillWithBody(s, specFramework, body, baselines)
		}
		out = append(out, ProjectSkillTemplate{
			ID:          s.ID,
			Name:        name,
			DirName:     s.DirName,
			Description: s.Description,
			Content:     content,
		})
	}
	return out
}

// StageSkillByDirName resolves the skill a marketplace pack entry names. The
// format carries directory names and nothing else, which is exactly what makes
// the mapping unambiguous: one directory, one workflow step.
func StageSkillByDirName(dirName string) (StageSkill, bool) {
	dirName = strings.TrimSpace(dirName)
	for _, s := range StageSkills {
		if s.DirName == dirName {
			return s, true
		}
	}
	return StageSkill{}, false
}

// SkillDirsFor returns the skill directories of one skill inside a checkout,
// one per agent CLI. The same file is written to all of them: one source,
// several readers.
func SkillDirsFor(root, dirName string) []string {
	dirs := make([]string, 0, len(models.SkillAgentDirs))
	for _, agent := range models.SkillAgentDirs {
		if agent == "" {
			dirs = append(dirs, filepath.Join(root, ".skills", dirName))
			continue
		}
		dirs = append(dirs, filepath.Join(root, agent, "skills", dirName))
	}
	return dirs
}

// SkillCommandPath is where a skill's slash command lives for Claude Code.
func SkillCommandPath(root, dirName string) string {
	return filepath.Join(root, ".claude", "commands", dirName+".md")
}

// RenderSkillCommand turns a rendered SKILL.md into its slash command: same
// instructions, a command frontmatter, and the ticket passed as $ARGUMENTS.
func RenderSkillCommand(s StageSkill, specFramework string) string {
	body := RenderSkillContent(s, specFramework)

	if strings.HasPrefix(body, "---\n") {
		if end := strings.Index(body[4:], "\n---\n"); end >= 0 {
			body = body[4+end+5:]
		}
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "description: %s\n", YAMLString(s.FrontmatterDesc))
	hint := "<TICKET-KEY> [contexte]"
	if s.Scope == "macro" {
		hint = "<MACRO-KEY> [contexte]"
	}
	fmt.Fprintf(&b, "argument-hint: %s\n", hint)
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
