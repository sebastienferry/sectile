package db

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agentprotocol"
	"tasks/internal/models"
	"tasks/internal/skills"
)

// packStub stands in for the workstation: it answers the two marketplace
// actions from a fixed pack and counts what the server asked for, which is how
// the tests prove a preview reaches no further than a read.
type packStub struct {
	bodies map[string]string
	calls  []string
	fail   error
}

func (s *packStub) install(t *testing.T, d *DB) {
	t.Helper()
	d.SetAgentOperations(func(ctx context.Context, op agentprotocol.Operation) (json.RawMessage, error) {
		s.calls = append(s.calls, op.Action)
		if s.fail != nil && strings.HasPrefix(op.Action, "marketplace_") {
			return nil, s.fail
		}
		switch op.Action {
		case "marketplace_catalog":
			return json.Marshal(models.MarketplaceCatalog{
				Name:    "acme",
				Owner:   "Acme Platform",
				Commit:  "1111111111111111111111111111111111111111",
				Plugins: []models.MarketplacePlugin{{Name: "acme-flow", Source: "./plugins/workflow", Version: "2.1.0", Skills: []string{"clarify-issue"}}},
			})
		case "marketplace_pack":
			return json.Marshal(models.SkillPack{
				Marketplace: "acme",
				Plugin:      "acme-flow",
				Version:     "2.1.0",
				Commit:      "1111111111111111111111111111111111111111",
				Bodies:      s.bodies,
				Ignored:     []string{"docs-writer"},
			})
		}
		// sync_config and the editor's skill_files probe answer empty, the way
		// a workstation without the checkout does.
		return json.RawMessage(`{}`), nil
	})
}

func (s *packStub) count(action string) int {
	n := 0
	for _, call := range s.calls {
		if call == action {
			n++
		}
	}
	return n
}

func packTestDB(t *testing.T, bodies map[string]string) (*DB, *models.Project, *packStub) {
	t.Helper()
	d, err := NewDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	no := false
	project, err := d.CreateProject(models.CreateProjectRequest{Name: "Packs", RepoPath: "/not-mounted", IssueTracker: "local", UseWorktrees: &no})
	if err != nil {
		t.Fatal(err)
	}
	stub := &packStub{bodies: bodies}
	stub.install(t, d)
	if _, err := d.AddSkillMarketplace(models.SkillMarketplace{Name: "acme", Kind: models.MarketplaceKindPath, Locator: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	return d, project, stub
}

const clarifyPack = "---\nname: clarify-issue\ndescription: acme\n---\n\n# Acme clarify\n\nAsk the three Acme questions.\n"

func skillContent(t *testing.T, d *DB, projectID, skillID string) string {
	t.Helper()
	for _, entry := range d.EffectiveProjectSkills(projectID, "") {
		if entry.ID == skillID {
			return entry.Content
		}
	}
	t.Fatalf("no skill %q", skillID)
	return ""
}

func editorEntry(t *testing.T, d *DB, projectID, skillID string) models.SkillEditorEntry {
	t.Helper()
	entries, err := d.ListProjectSkillEditor(projectID)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.ID == skillID {
			return entry
		}
	}
	t.Fatalf("no editor entry %q", skillID)
	return models.SkillEditorEntry{}
}

// Registering resolves the source once: a locator that does not answer is
// refused with its reason, and nothing is stored.
func TestAddSkillMarketplaceRefusesASourceThatDoesNotResolve(t *testing.T) {
	d, _, stub := packTestDB(t, map[string]string{"clarify-issue": clarifyPack})
	stub.fail = context.DeadlineExceeded

	if _, err := d.AddSkillMarketplace(models.SkillMarketplace{Name: "broken", Kind: models.MarketplaceKindPath, Locator: "/nowhere"}); err == nil {
		t.Fatal("an unresolvable marketplace was registered")
	}
	if _, err := d.SkillMarketplace("broken"); err == nil {
		t.Fatal("a refused registration was stored anyway")
	}

	stub.fail = nil
	for _, bad := range []models.SkillMarketplace{
		{Name: "", Kind: models.MarketplaceKindPath, Locator: "/x"},
		{Name: "team/pack", Kind: models.MarketplaceKindPath, Locator: "/x"},
		{Name: "ok", Kind: "ftp", Locator: "/x"},
		{Name: "ok", Kind: models.MarketplaceKindPath, Locator: ""},
		{Name: "ok", Kind: models.MarketplaceKindGit, Locator: "--upload-pack=touch /tmp/x"},
	} {
		if _, err := d.AddSkillMarketplace(bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}

// Previewing is a read: it writes nothing on the project, neither a skill row
// nor a pin, and it does not reinstall anything.
func TestPreviewSkillPackWritesNothing(t *testing.T) {
	d, project, stub := packTestDB(t, map[string]string{"clarify-issue": clarifyPack})

	before := skillContent(t, d, project.ID, "clarify")
	preview, err := d.PreviewSkillPack(project.ID, "acme", "acme-flow", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Entries) != 1 || preview.Entries[0].SkillID != "clarify" || !preview.Entries[0].Changed {
		t.Fatalf("preview entries: %+v", preview.Entries)
	}
	if !strings.Contains(preview.Entries[0].Proposed, "Ask the three Acme questions") {
		t.Fatal("the proposed content does not carry the pack body")
	}
	if !strings.Contains(preview.Entries[0].Proposed, "transition_stage") {
		t.Fatal("the proposed content dropped a generated contract")
	}
	if len(preview.Missing) != len(skills.StageSkills)-1 {
		t.Fatalf("missing: %v", preview.Missing)
	}

	if got := skillContent(t, d, project.ID, "clarify"); got != before {
		t.Fatal("the preview changed what the project runs")
	}
	if pin, _ := d.SkillPackPin(project.ID); pin != nil {
		t.Fatalf("the preview left a pin behind: %+v", pin)
	}
	if stub.count("sync_config") != 0 {
		t.Fatal("the preview reinstalled the files")
	}
}

// The pinned revision comes from the request body and reaches git on the
// workstation: anything but a commit id is refused before the agent is asked.
func TestSkillPackRefusesACommitThatIsNotAnId(t *testing.T) {
	d, project, stub := packTestDB(t, map[string]string{"clarify-issue": clarifyPack})

	for _, commit := range []string{"--output=/tmp/x", "main", "abc12", "zzzzzzzz", strings.Repeat("a", 41)} {
		if _, err := d.PreviewSkillPack(project.ID, "acme", "acme-flow", commit); err == nil {
			t.Fatalf("preview accepted commit %q", commit)
		}
		if _, err := d.ApplySkillPack(project.ID, "acme", "acme-flow", commit); err == nil {
			t.Fatalf("apply accepted commit %q", commit)
		}
	}
	if stub.count("marketplace_pack") != 0 {
		t.Fatal("an invalid commit reached the agent")
	}
	if _, err := d.PreviewSkillPack(project.ID, "acme", "acme-flow", "1111111"); err != nil {
		t.Fatalf("an abbreviated commit id was refused: %v", err)
	}
}

// Precedence: built-in, then the pack, then the project's own edit — and the
// edit always wins.
func TestSkillPackPrecedenceAndReset(t *testing.T) {
	d, project, _ := packTestDB(t, map[string]string{"clarify-issue": clarifyPack})

	if _, err := d.ApplySkillPack(project.ID, "acme", "acme-flow", ""); err != nil {
		t.Fatal(err)
	}
	if got := skillContent(t, d, project.ID, "clarify"); !strings.Contains(got, "Ask the three Acme questions") {
		t.Fatal("the applied pack body is not what the project runs")
	}

	entry := editorEntry(t, d, project.ID, "clarify")
	if entry.IsCustom {
		t.Fatal("a pack body nobody edited is not a project customization")
	}
	if entry.Origin != models.SkillOriginMarketplace || !strings.HasPrefix(entry.PackOrigin, "acme/acme-flow@2.1.0+") {
		t.Fatalf("origin: %q %q", entry.Origin, entry.PackOrigin)
	}
	if !strings.Contains(entry.DefaultContent, "Ask the three Acme questions") {
		t.Fatal("the editor default is not the resolved baseline")
	}

	if _, err := d.SaveProjectSkillContent(project.ID, "clarify", "---\nname: clarify-issue\ndescription: mine\n---\n\nMy own clarify."); err != nil {
		t.Fatal(err)
	}
	if got := skillContent(t, d, project.ID, "clarify"); !strings.Contains(got, "My own clarify.") {
		t.Fatal("the pack overwrote the project's own edit")
	}
	if !editorEntry(t, d, project.ID, "clarify").IsCustom {
		t.Fatal("an edit over a pack is not reported as custom")
	}

	// Resetting lands on the pack, not on the catalogue.
	if _, err := d.ResetProjectSkillContent(project.ID, "clarify"); err != nil {
		t.Fatal(err)
	}
	if got := skillContent(t, d, project.ID, "clarify"); !strings.Contains(got, "Ask the three Acme questions") {
		t.Fatal("the reset went past the pack to the built-in body")
	}

	// Unpinning goes back to the catalogue and keeps the other edits.
	if _, err := d.SaveProjectSkillContent(project.ID, "handoff", "---\nname: handoff-issue\ndescription: mine\n---\n\nMy own handoff."); err != nil {
		t.Fatal(err)
	}
	if err := d.UnpinSkillPack(project.ID); err != nil {
		t.Fatal(err)
	}
	if got := skillContent(t, d, project.ID, "clarify"); strings.Contains(got, "Ask the three Acme questions") {
		t.Fatal("unpinning kept the pack body")
	}
	if got := skillContent(t, d, project.ID, "handoff"); !strings.Contains(got, "My own handoff.") {
		t.Fatal("unpinning destroyed an edit that was not the pack's")
	}
	if pin, _ := d.SkillPackPin(project.ID); pin != nil {
		t.Fatal("the pin survived unpinning")
	}
}

// A newer pack that no longer supplies a skill returns it to the built-in body
// rather than leaving an orphaned one behind.
func TestApplyingAPackThatDropsASkillReturnsItToTheCatalogue(t *testing.T) {
	d, project, stub := packTestDB(t, map[string]string{"clarify-issue": clarifyPack, "handoff-issue": "---\nname: handoff-issue\ndescription: acme\n---\n\nAcme handoff prose.\n"})

	if _, err := d.ApplySkillPack(project.ID, "acme", "acme-flow", ""); err != nil {
		t.Fatal(err)
	}
	if got := skillContent(t, d, project.ID, "handoff"); !strings.Contains(got, "Acme handoff prose.") {
		t.Fatal("the first pack did not supply handoff")
	}

	stub.bodies = map[string]string{"clarify-issue": clarifyPack}
	pin, err := d.ApplySkillPack(project.ID, "acme", "acme-flow", "")
	if err != nil {
		t.Fatal(err)
	}
	if got := skillContent(t, d, project.ID, "handoff"); strings.Contains(got, "Acme handoff prose.") {
		t.Fatal("a body the new pack does not supply stayed applied")
	}
	if len(pin.Skills) != 1 || pin.Skills[0] != "clarify-issue" {
		t.Fatalf("pin skills: %v", pin.Skills)
	}
}

// Applying records the coordinates a re-run needs, and the whole set is
// reinstalled the way saving an edited skill reinstalls it.
func TestApplySkillPackPinsTheRevisionAndReinstalls(t *testing.T) {
	d, project, stub := packTestDB(t, map[string]string{"clarify-issue": clarifyPack})

	pin, err := d.ApplySkillPack(project.ID, "acme", "acme-flow", "")
	if err != nil {
		t.Fatal(err)
	}
	if pin.Marketplace != "acme" || pin.Plugin != "acme-flow" || pin.Version != "2.1.0" {
		t.Fatalf("pin: %+v", pin)
	}
	if len(pin.Commit) != 40 || pin.Orphaned {
		t.Fatalf("pin revision: %+v", pin)
	}
	if stub.count("sync_config") == 0 {
		t.Fatal("applying did not reinstall the files")
	}

	// Removing the marketplace leaves the project running what it applied, and
	// says the coordinates no longer resolve.
	pinned, err := d.RemoveSkillMarketplace("acme")
	if err != nil {
		t.Fatal(err)
	}
	if len(pinned) != 1 || pinned[0] != project.ID {
		t.Fatalf("removal did not report the pinning project: %v", pinned)
	}
	if got := skillContent(t, d, project.ID, "clarify"); !strings.Contains(got, "Ask the three Acme questions") {
		t.Fatal("removing a marketplace changed what a project runs")
	}
	after, _ := d.SkillPackPin(project.ID)
	if after == nil || !after.Orphaned {
		t.Fatalf("an unregistered marketplace must orphan the pin: %+v", after)
	}
}

// An agent that cannot be reached is an error the caller sees, not a silent
// no-op and not a half-applied pack.
func TestSkillPackOperationsSurfaceAnOfflineAgent(t *testing.T) {
	d, project, stub := packTestDB(t, map[string]string{"clarify-issue": clarifyPack})
	before := skillContent(t, d, project.ID, "clarify")
	stub.fail = context.DeadlineExceeded

	if _, err := d.PreviewSkillPack(project.ID, "acme", "acme-flow", ""); err == nil {
		t.Fatal("an offline preview reported success")
	}
	if _, err := d.ApplySkillPack(project.ID, "acme", "acme-flow", ""); err == nil {
		t.Fatal("an offline apply reported success")
	}
	if got := skillContent(t, d, project.ID, "clarify"); got != before {
		t.Fatal("a failed apply changed what the project runs")
	}
	if pin, _ := d.SkillPackPin(project.ID); pin != nil {
		t.Fatal("a failed apply left a pin")
	}
}

// The batch skills compose from the resolved bodies, so a pack that updates a
// stage updates the batch that runs it.
func TestAppliedPackReachesTheBatchSkills(t *testing.T) {
	d, project, _ := packTestDB(t, map[string]string{"clarify-issue": clarifyPack})
	if _, err := d.ApplySkillPack(project.ID, "acme", "acme-flow", ""); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"pickup", "pickup_issues"} {
		if got := skillContent(t, d, project.ID, id); !strings.Contains(got, "Ask the three Acme questions") {
			t.Fatalf("%s still embeds the catalogue clarify", id)
		}
	}
}
