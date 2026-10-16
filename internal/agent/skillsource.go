package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"tasks/internal/agentconfig"
	"tasks/internal/models"
	"tasks/internal/skills"
)

// What a dispatch runs for a workflow skill (#267). The agent never installs
// anything at dispatch: it uses what it finds, in this order, without comparing
// versions: the project's custom skill, then the installed source the
// workstation prefers, then the other one.
const (
	// skillKindCustom is the project's edited skill, handed to the CLI through
	// a file private to the run.
	skillKindCustom = "custom"
	// skillKindCommand is a command the project section names explicitly,
	// run as written without looking for a source.
	skillKindCommand = "command"
	// skillKindDirect is the copy the direct setup wrote into the CLI's
	// user-level skill folder, run as /<dir>.
	skillKindDirect = "direct"
	// skillKindPlugin is the skill of the Claude plugin, run as /sectile:<dir>.
	skillKindPlugin = "plugin"
)

// skillChoice is the resolved skill of one dispatch.
type skillChoice struct {
	Kind string
	// Command is the slash command without its leading "/", empty for a
	// custom skill.
	Command string
	// File is the run-private SKILL.md of a custom skill.
	File string
	// SkillID and Directory name the skill, in messages and in the passive
	// signal.
	SkillID   string
	Directory string
}

// errSkillNotInstalled is a dispatch that found nothing to run. It fails the
// launch before any CLI starts, and nothing is written to repair it.
type errSkillNotInstalled struct {
	Directory string
	Provider  string
}

func (e errSkillNotInstalled) Error() string {
	if e.Provider == "claude" {
		return fmt.Sprintf("Skill « %s » introuvable pour %s : aucune copie directe, aucun plugin Sectile installé et activé. "+
			"Installez le plugin `sectile` dans Claude, lancez `sectile-agent init --provider %s`, "+
			"ou utilisez « Initialize » dans les réglages du projet de l'application de bureau.", e.Directory, e.Provider, e.Provider)
	}
	return fmt.Sprintf("Skill « %s » introuvable pour %s : aucune copie directe. "+
		"Lancez `sectile-agent init --provider %s`, "+
		"ou utilisez « Initialize » dans les réglages du projet de l'application de bureau.", e.Directory, e.Provider, e.Provider)
}

// chooseSkill picks what a dispatch runs for one skill. It only reads: the
// custom skill's file is written by the caller once the choice is made.
// workDirs are the folders the run works in, which decide whether a plugin
// installed for one project folder applies.
func chooseSkill(defaults agentconfig.Defaults, config agentconfig.Config, skill agentconfig.Skill, workDirs ...string) (skillChoice, error) {
	choice := skillChoice{SkillID: skill.ID, Directory: skill.Directory}
	if skill.Custom && defaults.CustomSkillsWinOrDefault() {
		choice.Kind = skillKindCustom
		return choice, nil
	}
	command := strings.TrimPrefix(strings.TrimSpace(skill.Command), "/")
	if command == "" {
		command = skill.Directory
	}
	if skill.CommandOverridden {
		choice.Kind, choice.Command = skillKindCommand, command
		return choice, nil
	}
	provider := agentconfig.EffectiveProvider(config.AIProvider, config.AICommandTemplate)
	loc, err := agentconfig.ResolveLocations(provider)
	if err != nil || !loc.InstallsSkills() {
		// A CLI without a skill folder has nothing to probe: the command is
		// sent as it always was.
		choice.Kind, choice.Command = skillKindDirect, command
		return choice, nil
	}
	sources := []string{agentconfig.SkillSourceDirect, agentconfig.SkillSourcePlugin}
	if defaults.InstalledSkillSourceOrDefault() == agentconfig.SkillSourcePlugin {
		sources = []string{agentconfig.SkillSourcePlugin, agentconfig.SkillSourceDirect}
	}
	// A workstation set up before a skill was renamed has only its former
	// directory, which still holds the full skill: it runs under that name
	// until the next setup installs the new one (#608).
	directories := []string{skill.Directory}
	if legacy := models.LegacySkillDirs[skill.Directory]; legacy != "" {
		directories = append(directories, legacy)
	}
	for _, directory := range directories {
		directCommand := command
		if directory != skill.Directory {
			directCommand = directory
		}
		for _, source := range sources {
			switch source {
			case agentconfig.SkillSourceDirect:
				if fileExists(filepath.Join(loc.Home, filepath.FromSlash(loc.SkillDir), directory, "SKILL.md")) {
					choice.Kind, choice.Command = skillKindDirect, directCommand
					return choice, nil
				}
			case agentconfig.SkillSourcePlugin:
				if provider == "claude" && claudePluginSkill(loc.Home, directory, workDirs...) {
					choice.Kind, choice.Command = skillKindPlugin, skills.PluginName+":"+directory
					return choice, nil
				}
			}
		}
	}
	return choice, errSkillNotInstalled{Directory: skill.Directory, Provider: provider}
}

// claudeInstalledPlugins is ~/.claude/plugins/installed_plugins.json, version 2.
type claudeInstalledPlugins struct {
	Plugins map[string][]struct {
		Scope       string `json:"scope"`
		ProjectPath string `json:"projectPath"`
		InstallPath string `json:"installPath"`
	} `json:"plugins"`
}

// claudePluginSkill says whether the Sectile plugin is installed and enabled
// for these folders and ships the skill. Claude's own files are read, never
// written: the plugin is the user's to install.
func claudePluginSkill(home, directory string, workDirs ...string) bool {
	var installed claudeInstalledPlugins
	if !readClaudeJSON(filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), &installed) {
		return false
	}
	enabled := claudeEnabledPlugins(home, workDirs...)
	for key, entries := range installed.Plugins {
		if !strings.HasPrefix(key, skills.PluginName+"@") {
			continue
		}
		if on, stated := enabled[key]; stated && !on {
			continue
		}
		for _, entry := range entries {
			if entry.Scope != "user" && !underAny(entry.ProjectPath, workDirs) {
				continue
			}
			if entry.InstallPath != "" && fileExists(filepath.Join(entry.InstallPath, "skills", directory, "SKILL.md")) {
				return true
			}
		}
	}
	return false
}

// claudeEnabledPlugins merges enabledPlugins as Claude resolves it for a
// session started in the run's folder: the user settings, then the project's
// .claude/settings.json, then its .claude/settings.local.json, each stated key
// overriding the one before. A missing file states nothing, and a plugin no
// file mentions is enabled, as an absent key is.
func claudeEnabledPlugins(home string, workDirs ...string) map[string]bool {
	files := []string{filepath.Join(home, ".claude", "settings.json")}
	if len(workDirs) > 0 && strings.TrimSpace(workDirs[0]) != "" {
		project := filepath.Join(workDirs[0], ".claude")
		files = append(files, filepath.Join(project, "settings.json"), filepath.Join(project, "settings.local.json"))
	}
	enabled := map[string]bool{}
	for _, file := range files {
		var settings struct {
			EnabledPlugins map[string]bool `json:"enabledPlugins"`
		}
		if !readClaudeJSON(file, &settings) {
			continue
		}
		for key, on := range settings.EnabledPlugins {
			enabled[key] = on
		}
	}
	return enabled
}

// claudeFileWarnings keeps an unreadable Claude file from being logged at
// every dispatch.
var claudeFileWarnings sync.Map

// readClaudeJSON reads one of Claude's files. A missing file is silent; one
// that cannot be read or parsed reads as absent and is logged once.
func readClaudeJSON(path string, into any) bool {
	raw, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(raw, into)
	} else if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		if _, logged := claudeFileWarnings.LoadOrStore(path, true); !logged {
			log.Printf("[Agent] Ignoring %s: %v", path, err)
		}
		return false
	}
	return true
}

// underAny says whether one of dirs is root or inside it.
func underAny(root string, dirs []string) bool {
	root = strings.TrimSpace(root)
	if root == "" {
		return false
	}
	root = filepath.Clean(root)
	for _, dir := range dirs {
		if dir = strings.TrimSpace(dir); dir == "" {
			continue
		}
		rel, err := filepath.Rel(root, filepath.Clean(dir))
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// runIDShape is what may name a run directory: a run ID never carries a path
// separator or a dot-dot, whoever sent it.
var runIDShape = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

// runSkillsMaxAge is how long a run directory a crash left behind survives.
const runSkillsMaxAge = 7 * 24 * time.Hour

// runSkillsRoot is where custom skills are handed to their run, beside the
// workstation settings: a folder Sectile owns, outside every checkout.
func runSkillsRoot() (string, error) {
	path, err := agentconfig.SettingsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "runs"), nil
}

// writeRunSkill writes a custom skill where only its run reads it, and returns
// the file. The ticket travels in the prompt, so the file holds the skill
// alone.
func writeRunSkill(runID string, skill agentconfig.Skill) (string, error) {
	if !runIDShape.MatchString(runID) {
		return "", fmt.Errorf("run ID %q cannot name a run directory", runID)
	}
	root, err := runSkillsRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, runID, skill.Directory)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	file := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(file, []byte(skill.Content), 0o600); err != nil {
		return "", err
	}
	return file, nil
}

// removeRunSkills drops what writeRunSkill wrote for a run.
func removeRunSkills(runID string) {
	if !runIDShape.MatchString(runID) {
		return
	}
	root, err := runSkillsRoot()
	if err != nil {
		return
	}
	if err := os.RemoveAll(filepath.Join(root, runID)); err != nil {
		log.Printf("[Agent] Could not remove the skills of run %s: %v", runID, err)
	}
}

// sweepRunSkills removes the run directories older than maxAge, which a crash
// can leave behind since removal happens when the run ends.
func sweepRunSkills(maxAge time.Duration) {
	root, err := runSkillsRoot()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !entry.IsDir() || !runIDShape.MatchString(entry.Name()) || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(root, entry.Name()))
	}
}

// customSkillPrompt is what the CLI receives for a custom skill: the file to
// follow, then the dispatch's own prompt.
func customSkillPrompt(taskKey, file, prompt string) string {
	out := "Sectile task: " + taskKey + "\n\nFollow the skill in " + file + " for this task. It replaces any installed skill of the same name."
	if strings.TrimSpace(prompt) != "" {
		out += "\n\n" + prompt
	}
	return out
}

// dispatchedSkill finds the configured skill a launch runs, nil for a launch
// that runs none (a discussion, a terminal, free instructions) or that names
// its command in its prompt.
func dispatchedSkill(config agentconfig.Config, skillID, action, prompt string) *agentconfig.Skill {
	skillID = models.NormalizeSkillID(skillID)
	action = models.NormalizeSkillID(action)
	if skillID == "discuss" || skillID == "custom" || action == "open_terminal" || strings.HasPrefix(strings.TrimSpace(prompt), "/") {
		return nil
	}
	for i, skill := range config.Skills {
		if skillID == skill.ID || skillID == skill.Directory || action == skill.ID {
			if skill.RequiresReconciliation {
				return nil
			}
			return &config.Skills[i]
		}
	}
	return nil
}

// prepareSkill resolves what a launch runs and, for a custom skill, writes its
// run-private file, removed once done is closed. It returns nil when the
// launch runs no configured skill, which dispatchCommand then handles as it
// always did. The use of a custom skill is recorded by recordCustomSkillUse,
// once the CLI is launched.
func (d *agentDaemon) prepareSkill(config agentconfig.Config, skillID, action, prompt, runID string, done <-chan struct{}, workDirs ...string) (*skillChoice, error) {
	skill := dispatchedSkill(config, skillID, action, prompt)
	if skill == nil {
		return nil, nil
	}
	settings, err := agentconfig.ReadSettings(d.localSettingsRoot())
	if err != nil {
		return nil, err
	}
	choice, err := chooseSkill(settings.Defaults, config, *skill, workDirs...)
	if err != nil {
		return nil, err
	}
	if choice.Kind == skillKindCustom {
		// A launch without a run still needs a folder of its own, so that two
		// of them never share a file.
		if strings.TrimSpace(runID) == "" {
			runID = "local-" + uuid.NewString()
		}
		// A work-only override is composed here, from the embedded fragments,
		// so the run reads this workstation's sections too (#732).
		s := *skill
		s.Content = runSkillContent(config, s)
		if choice.File, err = writeRunSkill(runID, s); err != nil {
			return nil, err
		}
		go func() {
			<-done
			removeRunSkills(runID)
		}()
	}
	return &choice, nil
}

// customSkillUse is one project skill that ran as its custom version on this
// workstation, as the desktop lists it.
type customSkillUse struct {
	ProjectID   string    `json:"projectId"`
	ProjectName string    `json:"projectName"`
	SkillID     string    `json:"skillId"`
	Directory   string    `json:"directory"`
	LastRun     time.Time `json:"lastRun"`
}

// customSkillLog keeps, per project and skill, when a custom skill last ran
// since the agent started. It lives in memory only: it says what ran, which is
// run state, not a setting.
type customSkillLog struct {
	mu   sync.Mutex
	uses map[string]customSkillUse
}

// recordCustomSkillUse records that a launch runs its project's custom skill:
// on the desktop's passive signal and, for a run, on its activity. The caller
// calls it once the CLI is launched, so a launch that failed on its command
// line, its wrapper or its terminal is not counted as a use.
func (d *agentDaemon) recordCustomSkillUse(config agentconfig.Config, choice *skillChoice, runID string) {
	if choice == nil || choice.Kind != skillKindCustom {
		return
	}
	d.noteCustomSkillUse(config, choice.SkillID, choice.Directory)
	go d.postCustomSkillUse(runID, choice.Directory)
}

func (d *agentDaemon) noteCustomSkillUse(config agentconfig.Config, skillID, directory string) {
	d.customSkills.mu.Lock()
	defer d.customSkills.mu.Unlock()
	if d.customSkills.uses == nil {
		d.customSkills.uses = map[string]customSkillUse{}
	}
	d.customSkills.uses[config.ProjectID+"\x00"+skillID] = customSkillUse{
		ProjectID: config.ProjectID, ProjectName: config.ProjectName, SkillID: skillID, Directory: directory, LastRun: time.Now().UTC(),
	}
}

// customSkillsUsed lists the custom skills that ran, the latest first.
func (d *agentDaemon) customSkillsUsed() []customSkillUse {
	d.customSkills.mu.Lock()
	defer d.customSkills.mu.Unlock()
	out := make([]customSkillUse, 0, len(d.customSkills.uses))
	for _, use := range d.customSkills.uses {
		out = append(out, use)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].LastRun.Equal(out[j].LastRun) {
			return out[i].LastRun.After(out[j].LastRun)
		}
		return out[i].ProjectName+out[i].SkillID < out[j].ProjectName+out[j].SkillID
	})
	return out
}

// postCustomSkillUse tells the server the run was launched with its project's
// custom skill, which the run's activity then says. Like the engine report it
// is best effort: the run goes on whether or not the server records it.
func (d *agentDaemon) postCustomSkillUse(runID, directory string) {
	if strings.TrimSpace(runID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.postAPI(ctx, "/api/activities/"+url.PathEscape(runID)+"/custom-skill", map[string]string{"directory": directory}, nil); err != nil {
		log.Printf("[Agent] Could not record the custom skill of run %s: %v", runID, err)
	}
}
