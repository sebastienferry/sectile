package agentconfig

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"
)

// resolveLegacyEngine is what a level resolved before #510: the project
// section over the workstation defaults, a provider change dropping the
// inherited templates, the most specific model statement winning. Templates
// are kept as stated, so a provider-default command stays implicit.
func resolveLegacyEngine(defaults, project Execution) Engine {
	provider := strings.TrimSpace(defaults.AIProvider)
	if provider == "" {
		provider = DefaultProvider
	}
	command, autonomous := defaults.AICommandTemplate, defaults.AICommandTemplateAutonomous
	if own := strings.TrimSpace(project.AIProvider); own != "" {
		if own != provider {
			command, autonomous = "", ""
		}
		provider = own
	}
	if strings.TrimSpace(project.AICommandTemplate) != "" {
		command = project.AICommandTemplate
	}
	if strings.TrimSpace(project.AICommandTemplateAutonomous) != "" {
		autonomous = project.AICommandTemplateAutonomous
	}
	merged := MergeModels(
		ModelConfig{Model: project.AIModel, SkillModels: project.AISkillModels},
		ModelConfig{Model: defaults.AIModel, SkillModels: defaults.AISkillModels},
	)
	return normalizeProfile(Engine{
		Provider: provider, Command: command, CommandAutonomous: autonomous,
		Model: merged.Model, SkillModels: merged.SkillModels,
	})
}

// normalizeProfile trims a profile so two equal ones compare equal.
func normalizeProfile(e Engine) Engine {
	e.Provider = strings.ToLower(strings.TrimSpace(e.Provider))
	if e.Provider == "" {
		e.Provider = DefaultProvider
	}
	e.Model = strings.TrimSpace(e.Model)
	e.Command, e.CommandAutonomous = strings.TrimSpace(e.Command), strings.TrimSpace(e.CommandAutonomous)
	var skills map[string]string
	for skill, model := range e.SkillModels {
		if model = strings.TrimSpace(model); model != "" {
			skills = setEntry(skills, skill, model)
		}
	}
	e.SkillModels = skills
	return e
}

// sameProfile compares what two engines run, ignoring identity and name.
func sameProfile(a, b Engine) bool {
	a, b = normalizeProfile(a), normalizeProfile(b)
	a.ID, a.Name, b.ID, b.Name = "", "", "", ""
	return reflect.DeepEqual(a, b)
}

// derivedEngineID is the identity of a converted engine. The conversion runs
// in memory on every read until it is persisted, so the identity must be the
// same from one read to the next, or a task switched in between would dangle.
func derivedEngineID(profile Engine) string {
	profile = normalizeProfile(profile)
	profile.ID, profile.Name = "", ""
	raw, _ := json.Marshal(profile) // map keys are sorted: the encoding is canonical
	return fmt.Sprintf("e-%x", sha256.Sum256(raw))[:14]
}

// findOrCreate returns the catalogue entry running profile, appending one
// when none does. Identical profiles collapse into one entry.
func (s *Settings) findOrCreate(profile Engine) string {
	for _, engine := range s.Engines.Catalogue {
		if sameProfile(engine, profile) {
			return engine.ID
		}
	}
	profile = normalizeProfile(profile)
	base := derivedEngineID(profile)
	profile.ID = base
	for n := 2; ; n++ {
		if _, taken := s.Engine(profile.ID); !taken {
			break
		}
		profile.ID = fmt.Sprintf("%s-%d", base, n)
	}
	profile.Name = s.engineName(profile)
	s.Engines.Catalogue = append(s.Engines.Catalogue, profile)
	return profile.ID
}

// engineName names a converted engine after its provider and model, made
// unique in the catalogue. The owner renames it at will.
func (s Settings) engineName(e Engine) string {
	name := providerNames[e.Provider]
	if name == "" {
		name = e.Provider
	}
	if e.Model != "" {
		name += " - " + e.Model
	}
	if len(name) > MaxEngineName {
		name = name[:MaxEngineName]
	}
	taken := map[string]bool{}
	for _, engine := range s.Engines.Catalogue {
		taken[strings.ToLower(strings.TrimSpace(engine.Name))] = true
	}
	candidate := name
	for n := 2; taken[strings.ToLower(candidate)]; n++ {
		candidate = fmt.Sprintf("%s (%d)", name, n)
	}
	return candidate
}

// convertEngines turns the engine settings of #305 into catalogue entries, in
// memory, keeping what every project resolves. It runs on every read, whatever
// the layout, since an agent that predates #510 may write those fields again;
// on a converted file it changes nothing. It reports whether it changed
// anything.
func convertEngines(s *Settings) bool {
	changed := false
	defaults := s.Defaults.Execution
	if defaults.statesEngine() {
		// A catalogue default the owner chose outranks a stale field an older
		// agent wrote: the workstation profile only joins the catalogue.
		id := s.findOrCreate(resolveLegacyEngine(defaults, Execution{}))
		if !s.hasDefaultEngine() {
			s.Engines.Default = id
		}
		changed = true
	} else if !s.hasDefaultEngine() {
		s.Engines.Default = s.addImplicitEngine()
		changed = true
	}
	ids := make([]string, 0, len(s.ProjectSettings))
	for id := range s.ProjectSettings {
		ids = append(ids, id)
	}
	sort.Strings(ids) // entries are appended in a stable order
	for _, id := range ids {
		p := s.ProjectSettings[id]
		if !p.Execution.statesEngine() {
			continue
		}
		engine := s.findOrCreate(resolveLegacyEngine(defaults, p.Execution))
		if _, picked := s.Engine(s.Engines.Projects[id]); !picked {
			s.SetProjectEngine(id, engine)
		}
		p.Execution.clearEngine()
		s.SetProject(id, p)
		changed = true
	}
	if defaults.statesEngine() {
		s.Defaults.Execution.clearEngine()
		changed = true
	}
	if w := s.Seeded.DefaultValues; w != nil && w.Execution.statesEngine() {
		w.Execution.clearEngine()
		changed = true
	}
	return changed
}

// implicitEngineID is the identity of the engine a workstation stating none
// runs. It differs from the one a workstation stating the same provider gets,
// so the server seed can still tell the two apart.
var implicitEngineID = fmt.Sprintf("e-%x", sha256.Sum256([]byte("implicit:"+DefaultProvider)))[:14]

// addImplicitEngine gives a catalogue without a default the engine of a
// workstation that states none, reusing an entry running the same profile.
func (s *Settings) addImplicitEngine() string {
	implicit := implicitEngine()
	for _, engine := range s.Engines.Catalogue {
		if sameProfile(engine, implicit) {
			return engine.ID
		}
	}
	implicit.ID, implicit.Name = implicitEngineID, s.engineName(implicit)
	s.Engines.Catalogue = append(s.Engines.Catalogue, implicit)
	return implicit.ID
}

func (s Settings) hasDefaultEngine() bool {
	_, ok := s.Engine(s.Engines.Default)
	return ok
}

// MigrateSettings persists the conversion of the #305 engine settings, once,
// at agent start. The previous file is copied beside it first. A file that
// does not exist yet, or needs no conversion, is left alone.
func MigrateSettings(legacyRoot string) (bool, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	path, err := SettingsPath()
	if err != nil {
		return false, err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var before struct {
		Layout int `json:"layout"`
	}
	if err = json.Unmarshal(raw, &before); err != nil {
		return false, err
	}
	settings, changed, err := readConverted(legacyRoot)
	if err != nil || (!changed && before.Layout >= SettingsLayout) {
		return false, err
	}
	backup := fmt.Sprintf("%s.bak-layout%d", path, before.Layout)
	if _, err := os.Stat(backup); err == nil {
		backup += "-" + time.Now().UTC().Format("20060102T150405Z")
	}
	if err = os.WriteFile(backup, raw, 0600); err != nil {
		return false, err
	}
	return true, WriteSettings(settings)
}
