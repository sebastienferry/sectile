package skills

import (
	"fmt"
	"sort"
	"strings"
)

// WorkSections are the parts of a task-scope stage skill a project or a workstation may override (#732).
// Everything else in the skill (frontmatter, task access, session title, run lifecycle, stage transition,
// exit condition and pull-request policy) is a Sectile contract and is always rendered from the catalogue.
type WorkSections struct{ Goal, ReadFirst, Steps, Guard, Report string }

// SkillOverrides are work-only overrides by skill ID.
type SkillOverrides map[string]WorkSections

// ProjectOverrides are one project's work-only overrides, for a skill copy shared by every project.
type ProjectOverrides struct {
	ProjectID string
	Skills    SkillOverrides
}

// pickupStages are the stages pickup and pickup_issues inline, in order.
var pickupStages = []string{"clarify", "specify", "implement", "adjust"}

// workFragments names the fragment of each work section, in rendering order.
var workFragments = []string{"goal", "read-first", "steps", "guard", "report"}

func isPickup(id string) bool {
	return id == "pickup" || id == "pickup_issues"
}

// Overridable says whether a skill takes a work-only override: a task-scope skill
// that runs as a stage. Macro skills and hand transitions are only replaced whole.
func Overridable(s StageSkill) bool {
	return s.Scope != "macro" && !s.HandTransition
}

// GuardHeading is the title of a skill's guard section.
func GuardHeading(s StageSkill) string {
	if s.GuardTitle != "" {
		return s.GuardTitle
	}
	return "Do not"
}

// ComposedStageIDs are the stages a skill inlines: pickup and pickup_issues carry
// the work of clarify, specify, implement and adjust, so overriding one of those
// changes them too.
func ComposedStageIDs(id string) []string {
	if s, ok := StageSkillByID(id); ok && isPickup(s.ID) {
		return append([]string(nil), pickupStages...)
	}
	return nil
}

// BuiltinWorkSections are the catalogue's work sections of a skill for one
// specification framework. Pickup's Steps are its inlined stages.
func BuiltinWorkSections(s StageSkill, specFramework string) WorkSections {
	w := WorkSections{
		Goal:      readSkillFragment(s.ID, "goal", ""),
		ReadFirst: readSkillFragment(s.ID, "read-first", specFramework),
		Steps:     readSkillFragment(s.ID, "steps", specFramework),
		Guard:     readSkillFragment(s.ID, "guard", ""),
		Report:    readSkillFragment(s.ID, "report", ""),
	}
	if isPickup(s.ID) {
		w.Steps = renderPickupSteps(specFramework, s.ID == "pickup_issues", nil)
	}
	if s.HandTransition {
		w.Steps = handTransitionSteps(s)
	}
	return w
}

// Over layers w on base: each non-blank section of w wins.
func (w WorkSections) Over(base WorkSections) WorkSections {
	pick := func(override, fallback string) string {
		if strings.TrimSpace(override) != "" {
			return override
		}
		return fallback
	}
	return WorkSections{
		Goal:      pick(w.Goal, base.Goal),
		ReadFirst: pick(w.ReadFirst, base.ReadFirst),
		Steps:     pick(w.Steps, base.Steps),
		Guard:     pick(w.Guard, base.Guard),
		Report:    pick(w.Report, base.Report),
	}
}

func (w WorkSections) section(fragment string) string {
	switch fragment {
	case "goal":
		return w.Goal
	case "read-first":
		return w.ReadFirst
	case "steps":
		return w.Steps
	case "guard":
		return w.Guard
	case "report":
		return w.Report
	}
	return ""
}

func (w *WorkSections) setSection(fragment, text string) {
	switch fragment {
	case "goal":
		w.Goal = text
	case "read-first":
		w.ReadFirst = text
	case "steps":
		w.Steps = text
	case "guard":
		w.Guard = text
	case "report":
		w.Report = text
	}
}

// sectionHeading is the "## " title of a work section.
func sectionHeading(s StageSkill, fragment string) string {
	switch fragment {
	case "goal":
		return "Goal"
	case "read-first":
		return "Read first"
	case "steps":
		return "Steps"
	case "guard":
		return GuardHeading(s)
	}
	return "Report"
}

// sectionFragment resolves a "## " title to its work section, "" when unknown.
// "Do not" is accepted for the guard of a skill that titles it otherwise.
func sectionFragment(s StageSkill, title string) string {
	for _, fragment := range workFragments {
		if strings.EqualFold(title, sectionHeading(s, fragment)) {
			return fragment
		}
	}
	if strings.EqualFold(title, "Do not") {
		return "guard"
	}
	return ""
}

// FormatWorkSections writes work sections as the markdown a work-only override
// is stored as; with the built-in sections, it is the editor's prefill.
func FormatWorkSections(s StageSkill, w WorkSections) string {
	var parts []string
	for _, fragment := range workFragments {
		if fragment == "steps" && isPickup(s.ID) {
			continue
		}
		text := strings.Trim(w.section(fragment), "\n")
		if strings.TrimSpace(text) == "" {
			continue
		}
		parts = append(parts, "## "+sectionHeading(s, fragment)+"\n"+text)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n\n") + "\n"
}

// ParseWorkSections reads a work-only override: markdown made only of the
// "## Goal", "## Read first", "## Steps", "## <guard>" and "## Report"
// sections, each at most once and none empty. A "##" heading may be indented by
// up to three spaces; headings inside code fences and "###" or deeper headings
// belong to the section they appear in, and a code fence left open is refused so
// the Sectile contracts never end up inside it. Pickup's Steps
// are its inlined stages, overridden through each stage instead.
func ParseWorkSections(s StageSkill, content string) (WorkSections, error) {
	if !Overridable(s) {
		return WorkSections{}, fmt.Errorf("%s takes no work-only override: replace the whole skill instead", s.ID)
	}
	allowed := "## Goal, ## Read first, ## Steps, ## " + GuardHeading(s) + " and ## Report"
	if isPickup(s.ID) {
		allowed = "## Goal, ## Read first, ## " + GuardHeading(s) + " and ## Report"
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.TrimSpace(line) == "---" {
			return WorkSections{}, fmt.Errorf("a work-only override has no frontmatter: Sectile writes it")
		}
		break
	}

	var w WorkSections
	seen := map[string]bool{}
	current := ""
	var body []string
	flush := func() error {
		if current == "" {
			return nil
		}
		text := strings.Trim(strings.Join(body, "\n"), "\n")
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("section ## %s is empty: remove it to keep the built-in one", sectionHeading(s, current))
		}
		w.setSection(current, text)
		return nil
	}
	fence := ""
	for _, line := range lines {
		if marker, rest := fenceMarker(line); marker != "" {
			if fence == "" {
				fence = marker
			} else if marker[0] == fence[0] && len(marker) >= len(fence) && strings.TrimSpace(rest) == "" {
				fence = ""
			}
		} else if title, ok := sectionTitle(line); ok && fence == "" {
			fragment := sectionFragment(s, title)
			if fragment == "" {
				return WorkSections{}, fmt.Errorf("unknown section %q: a work-only override uses %s", title, allowed)
			}
			if fragment == "steps" && isPickup(s.ID) {
				return WorkSections{}, fmt.Errorf("the steps of %s are its inlined stages: override clarify, specify, implement or adjust instead", s.ID)
			}
			if seen[fragment] {
				return WorkSections{}, fmt.Errorf("duplicate section ## %s", sectionHeading(s, fragment))
			}
			if err := flush(); err != nil {
				return WorkSections{}, err
			}
			seen[fragment] = true
			current, body = fragment, nil
			continue
		}
		if current == "" {
			if strings.TrimSpace(line) != "" {
				return WorkSections{}, fmt.Errorf("text before the first section: a work-only override starts with one of %s", allowed)
			}
			continue
		}
		body = append(body, line)
	}
	if fence != "" {
		return WorkSections{}, fmt.Errorf("unclosed code fence %q: close it so the Sectile contracts stay out of it", fence)
	}
	if err := flush(); err != nil {
		return WorkSections{}, err
	}
	if len(seen) == 0 {
		return WorkSections{}, fmt.Errorf("a work-only override needs at least one of %s", allowed)
	}
	return w, nil
}

// sectionTitle returns the title of a "##" heading line, indented by at most
// three spaces as in CommonMark; false when the line is no such heading.
func sectionTitle(line string) (string, bool) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || (trimmed != "##" && !strings.HasPrefix(trimmed, "## ")) {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, "##")), true
}

// fenceMarker returns the backtick or tilde run opening a code fence line, and
// what follows it; "" when the line is no fence.
func fenceMarker(line string) (string, string) {
	trimmed := strings.TrimLeft(line, " ")
	if len(line)-len(trimmed) > 3 || trimmed == "" || (trimmed[0] != '`' && trimmed[0] != '~') {
		return "", ""
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == trimmed[0] {
		n++
	}
	if n < 3 {
		return "", ""
	}
	return trimmed[:n], trimmed[n:]
}

// RenderComposedSkillContent builds the SKILL.md of one skill for one project:
// the Sectile contracts around the work sections, each taken from overrides
// when it carries one and from the catalogue otherwise. Pickup takes each
// inlined stage's work from that stage's override. Without overrides it is the
// built-in skill.
func RenderComposedSkillContent(s StageSkill, specFramework string, overrides SkillOverrides) string {
	work := BuiltinWorkSections(s, specFramework)
	if Overridable(s) {
		work = overrides[s.ID].Over(work)
		if isPickup(s.ID) {
			work.Steps = renderPickupSteps(specFramework, s.ID == "pickup_issues", overrides)
		}
	}
	return assembleSkill(s, skillDisplayName(s, specFramework), work, directTaskAccessFallback)
}

// RenderDirectComposedSkillContent builds the SKILL.md the direct setup writes,
// shared by every project of the workstation. A work section a project overrides
// becomes one subsection per such project, chosen by the projectId
// get_project_context reports, then an "Otherwise" holding the workstation's
// section or the built-in one. A section nobody overrides is rendered as the
// built-in direct skill renders it.
func RenderDirectComposedSkillContent(s StageSkill, workstation SkillOverrides, projects []ProjectOverrides) string {
	return renderDirectComposed(s, workstation, projects, directTaskAccessFallback)
}

// RenderDirectComposedSkillCommand is RenderDirectComposedSkillContent as a
// slash command, for a CLI that substitutes $ARGUMENTS.
func RenderDirectComposedSkillCommand(s StageSkill, workstation SkillOverrides, projects []ProjectOverrides) string {
	return skillCommand(s, RenderDirectComposedSkillContent(s, workstation, projects))
}

func renderDirectComposed(s StageSkill, workstation SkillOverrides, projects []ProjectOverrides, taskAccessFallback string) string {
	name := s.Title
	if name == "" {
		name = s.Name
	}
	if !Overridable(s) {
		workstation, projects = nil, nil
	}
	projects = sortedProjects(projects)
	var work WorkSections
	for _, fragment := range workFragments {
		work.setSection(fragment, directSection(s.ID, fragment, "###", workstation, projects))
	}
	if isPickup(s.ID) {
		work.Steps = renderGenericPickupSteps(s.ID == "pickup_issues", workstation, projects)
	}
	if s.HandTransition {
		work.Steps = handTransitionSteps(s)
	}
	content := assembleSkill(s, name, work, taskAccessFallback)
	if HasPullRequestPolicy(s.ID) {
		content += GenericPullRequestPolicy()
	}
	return content
}

// directSection is one work section of a direct skill copy. Its fallback is the
// workstation's section, else the built-in one with every framework variant.
// Projects that override the section get a subsection each at level, the
// fallback moving one level down under "Otherwise".
func directSection(id, fragment, level string, workstation SkillOverrides, projects []ProjectOverrides) string {
	fallback := func(level string) string {
		if text := strings.Trim(workstation[id].section(fragment), "\n"); strings.TrimSpace(text) != "" {
			return text
		}
		return genericSkillFragment(id, fragment, level)
	}
	var variants []string
	for _, project := range projects {
		if text := strings.Trim(project.Skills[id].section(fragment), "\n"); strings.TrimSpace(text) != "" {
			variants = append(variants, fmt.Sprintf("%s When get_project_context reports projectId %q\n%s", level, project.ProjectID, text))
		}
	}
	if len(variants) == 0 {
		return fallback(level)
	}
	if otherwise := strings.TrimRight(fallback(level+"#"), "\n"); strings.TrimSpace(otherwise) != "" {
		variants = append(variants, fmt.Sprintf("%s Otherwise\n%s", level, otherwise))
	}
	return "Read projectId from get_project_context and follow the subsection that matches it.\n\n" + strings.Join(variants, "\n\n")
}

func sortedProjects(projects []ProjectOverrides) []ProjectOverrides {
	out := append([]ProjectOverrides(nil), projects...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].ProjectID < out[j].ProjectID })
	return out
}

// StageLaunchContract is the Sectile contract a foreign slash command is
// launched with (#732): such a command replaces the whole skill, so the launch
// prompt carries the run lifecycle, the stage transition and its exit condition
// for the stage it stands for. A macro skill and a hand transition have none.
func StageLaunchContract(s StageSkill) string {
	if s.Scope == "macro" || s.HandTransition {
		return ""
	}
	contract := renderTicketTransitionContract(s)
	_, body, _ := strings.Cut(contract, "\n")
	return "## Sectile stage contract\n" + strings.TrimRight(body, "\n") + "\n"
}
