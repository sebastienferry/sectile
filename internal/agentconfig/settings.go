package agentconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"tasks/internal/models"
)

// SkillOverride is this workstation's override of one skill, under "skills" in
// settings.json. A plain string replaces the whole skill, as before #732; an
// object {"kind":"work","content":"..."} replaces only the work sections it
// states, and Sectile keeps the contracts (#732).
type SkillOverride struct {
	Kind    models.SkillOverrideKind
	Content string
}

// UnmarshalJSON reads a JSON string as a full replacement and an object as
// {"kind","content"}. The kind is not checked here, so a typo never stops the
// settings from being read: ValidateSkillOverride refuses it.
func (o *SkillOverride) UnmarshalJSON(raw []byte) error {
	var content string
	if err := json.Unmarshal(raw, &content); err == nil {
		*o = SkillOverride{Content: content}
		return nil
	}
	var object struct {
		Kind    models.SkillOverrideKind `json:"kind"`
		Content string                   `json:"content"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		return err
	}
	*o = SkillOverride{Kind: object.Kind, Content: object.Content}
	return nil
}

// dropNullSkillOverrides removes the skills raw states as null. UnmarshalJSON
// reads a null as an empty full replacement, which would blank the skill: such
// an entry is absent instead (#732).
func dropNullSkillOverrides(raw []byte, overrides map[string]SkillOverride) error {
	if len(overrides) == 0 {
		return nil
	}
	var fields struct {
		Skills map[string]json.RawMessage `json:"skills"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for id, value := range fields.Skills {
		if string(bytes.TrimSpace(value)) == "null" {
			delete(overrides, id)
		}
	}
	return nil
}

// MarshalJSON writes a full replacement back as a plain string, the shape every
// agent reads, and any other kind as an object.
func (o SkillOverride) MarshalJSON() ([]byte, error) {
	if o.Kind == models.SkillOverrideFull {
		return json.Marshal(o.Content)
	}
	return json.Marshal(struct {
		Kind    models.SkillOverrideKind `json:"kind"`
		Content string                   `json:"content"`
	}{o.Kind, o.Content})
}

// SettingsPath is shared by the standalone agent and its optional companion.
func SettingsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "sectile", "settings.json"), nil
}

// ReadSettings reads the workstation settings, and falls back to the legacy
// repository file until they are saved. A file in the layout that predates
// #305 is folded into the current one with the same meaning, and the engine
// settings of #305 are converted into the engine catalogue (#510), and the
// settings naming a retired provider are dropped (#614); the next
// WriteSettings rewrites it. The project Sandbox values are left where they
// are: only the start-up migration moves them (#744).
func ReadSettings(legacyRoot string) (Settings, error) {
	settings, _, _, err := readConverted(legacyRoot)
	return settings, err
}

// SettingsMigration is what the start-up migration changed or found, for the
// agent to log: what the drop of the retired providers removed (#614),
// whether the project Sandbox values were folded into the workstation ones
// (#730), with the entries left on their project, and the layouts that tell
// an older or a newer agent wrote the file (#744).
type SettingsMigration struct {
	RetiredDrop
	SandboxFolded   bool
	SandboxWarnings []string
	// Downgraded is the layout the file had reached before an older agent
	// rewrote it at a lower one; 0 when it was not.
	Downgraded int
	// NewerLayout is the layout of a file a newer agent wrote, which this one
	// leaves alone; 0 when it is not newer.
	NewerLayout int
	// DesktopKeysRemoved tells the file held keys only Sectile Desktop used,
	// now removed: Desktop keeps them in its own file (#746).
	DesktopKeysRemoved bool
}

// readConverted also reports whether the engine conversion or the drop of the
// retired providers changed anything, and what.
func readConverted(legacyRoot string) (Settings, bool, SettingsMigration, error) {
	settings, err := readFolded(legacyRoot)
	if err != nil {
		return settings, false, SettingsMigration{}, err
	}
	changed := convertEngines(&settings)
	report := SettingsMigration{RetiredDrop: dropRetiredProviders(&settings)}
	return settings, changed || !report.Empty(), report, nil
}

// maxLayoutKey names the highest layout ever written to the file (#744). It
// is outside ownedKeys, so an older agent, which replaces only the keys it
// owns, keeps it when it saves: a file whose layout is below it was rewritten
// by an older agent.
const maxLayoutKey = "maxLayout"

// fileLayout is the layout stamps of a settings file as written.
type fileLayout struct {
	Layout    int `json:"layout"`
	MaxLayout int `json:"maxLayout"`
}

// highest is the highest layout the file is known to have had. A file written
// before #744 has no maxLayout and is known by its layout only.
func (f fileLayout) highest() int { return max(f.Layout, f.MaxLayout) }

// downgraded reports a file an older agent rewrote after a newer one.
func (f fileLayout) downgraded() bool { return f.MaxLayout > f.Layout }

// ErrSettingsNewer refuses a save over settings a newer agent wrote: this
// agent would drop what it does not know of them (#744).
var ErrSettingsNewer = errors.New("the workstation settings were written by a newer Sectile agent")

func newerSettingsError(layout int) error {
	return fmt.Errorf("%w (layout %d, this agent writes layout %d): update this Sectile agent", ErrSettingsNewer, layout, SettingsLayout)
}

// layoutWorkstation is the layout #305 introduced, from which the file no
// longer borrows the checkout's legacy repository file.
const layoutWorkstation = 2

func readFolded(legacyRoot string) (Settings, error) {
	path, err := SettingsPath()
	if err != nil {
		return Settings{}, err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return readLegacyRepositoryFile(legacyRoot)
	}
	if err != nil {
		return Settings{}, err
	}
	var settings Settings
	if err = json.Unmarshal(raw, &settings); err != nil {
		return settings, err
	}
	if err = dropNullSkillOverrides(raw, settings.Skills); err != nil {
		return settings, err
	}
	// Legacy keys are folded whatever the layout: an agent that predates #305
	// keeps the keys it does not know when it saves, and adds its own beside
	// them. The current keys win.
	var legacy legacySettings
	if err = json.Unmarshal(raw, &legacy); err != nil {
		return settings, err
	}
	settings = overlay(legacy.fold(), settings)
	if settings.Layout >= layoutWorkstation {
		return settings, nil
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		return settings, err
	}
	checkout, err := readLegacyRepositoryFile(legacyRoot)
	if err != nil {
		return settings, err
	}
	_, hasProjects := fields["projects"]
	_, hasWorktrees := fields["worktrees"]
	_, hasParallelism := fields["parallelism"]
	for id, from := range checkout.ProjectSettings {
		p := settings.Project(id)
		if !hasProjects && p.Path == "" {
			p.Path = from.Path
		}
		if !hasWorktrees && p.UseWorktrees == nil {
			p.UseWorktrees = from.UseWorktrees
		}
		if !hasParallelism && p.Parallelism == 0 {
			p.Parallelism = from.Parallelism
		}
		settings.SetProject(id, p)
	}
	return settings, nil
}

// ownedKeys are the keys WriteSettings replaces as a whole. Any other key
// (server, deviceId, apiKey, pairedAt) is kept.
var ownedKeys = []string{"layout", "defaults", "projectSettings", "repositories", "disconnectedProjects", "mcpConnections", "skills", "seeded", "engines"}

// desktopOnlyKeys are what Sectile Desktop stored in this file before it kept
// its own (#746). Only Desktop read them; every write of the agent removes
// them, so the file holds nothing Desktop could want to write back.
var desktopOnlyKeys = []string{"appearance", "consoleView", "repo", "binary", "secret"}

// holdsDesktopOnlyKeys reports a settings file that still holds a key only
// Sectile Desktop used.
func holdsDesktopOnlyKeys(fields map[string]json.RawMessage) bool {
	for _, key := range desktopOnlyKeys {
		if _, ok := fields[key]; ok {
			return true
		}
	}
	return false
}

// WriteSettings preserves the connection fields while replacing the settings
// it owns. The legacy keys are removed and the file is written in the current
// layout. A map emptied by the caller is omitted from the JSON, and since the
// owned keys are replaced as a whole, it disappears from the file instead of
// keeping its previous content. A file a newer agent wrote is refused with
// ErrSettingsNewer and left unchanged; a file an older agent rewrote is
// copied beside it first, and the copy logged (#744).
func WriteSettings(settings Settings) error { return storeSettings(settings, true) }

// storeSettings is WriteSettings; traceDowngrade false skips the copy of a
// downgraded file, for the start-up migration that has already made it.
func storeSettings(settings Settings, traceDowngrade bool) error {
	path, err := SettingsPath()
	if err != nil {
		return err
	}
	fields := map[string]json.RawMessage{}
	var stamp fileLayout
	raw, err := os.ReadFile(path)
	if err == nil {
		if err = json.Unmarshal(raw, &fields); err != nil {
			return err
		}
		if err = json.Unmarshal(raw, &stamp); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if newest := stamp.highest(); newest > SettingsLayout {
		return newerSettingsError(newest)
	}
	if traceDowngrade && stamp.downgraded() {
		backup, err := backupSettingsFile(path, raw, stamp.Layout)
		if err != nil {
			return err
		}
		log.Printf("[Agent] Workstation settings were rewritten by an older Sectile agent (layout %d after layout %d); that file is kept as %s", stamp.Layout, stamp.MaxLayout, backup)
	}
	settings.Layout = SettingsLayout
	if len(settings.Engines.Catalogue) == 0 {
		// A file saved for the first time states the engine it runs.
		convertEngines(&settings)
	}
	settings.pruneEngines()
	raw, err = json.Marshal(settings)
	if err != nil {
		return err
	}
	updates := map[string]json.RawMessage{}
	if err = json.Unmarshal(raw, &updates); err != nil {
		return err
	}
	for _, key := range append(append(append([]string{}, legacyKeys...), ownedKeys...), desktopOnlyKeys...) {
		delete(fields, key)
	}
	for key, value := range updates {
		fields[key] = value
	}
	fields[maxLayoutKey] = json.RawMessage(strconv.Itoa(SettingsLayout))
	raw, err = json.MarshalIndent(fields, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

// settingsMu serializes the read-modify-write cycles of this process on the
// settings file. A seed triggered by a project listing must not lose a save
// the desktop makes at the same moment, nor the other way round.
var settingsMu sync.Mutex

// UpdateSettings reads the settings, lets change edit them, and writes them
// back unless change fails. The whole cycle holds settingsMu.
func UpdateSettings(legacyRoot string, change func(*Settings) error) (Settings, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	settings, err := ReadSettings(legacyRoot)
	if err != nil {
		return settings, err
	}
	if err := change(&settings); err != nil {
		return settings, err
	}
	return settings, WriteSettings(settings)
}

// LockSettings holds settingsMu for a read-modify-write cycle a caller spells
// out itself. It returns the unlock function.
func LockSettings() func() {
	settingsMu.Lock()
	return settingsMu.Unlock
}
