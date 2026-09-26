package agentconfig

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MCPConnection records an explicit workstation connection preference.
type MCPConnection struct {
	Transport string `json:"transport"`
	Target    string `json:"target"`
}

// Settings is the workstation's configuration, read from
// ~/.config/sectile/settings.json and never uploaded to the server. Since #305
// it is the only source of the execution settings: the workstation defaults,
// one section per project, and what the server no longer holds.
type Settings struct {
	Layout          int                        `json:"layout,omitempty"`
	Defaults        Defaults                   `json:"defaults"`
	ProjectSettings map[string]ProjectSettings `json:"projectSettings,omitempty"`
	// Repositories maps a repository, by identity, to the local folder that
	// holds its checkout (#456). Keyed by repository rather than by project,
	// one checkout serves every project that works in it.
	Repositories         map[string]string        `json:"repositories,omitempty"`
	DisconnectedProjects map[string]bool          `json:"disconnectedProjects,omitempty"`
	MCPConnections       map[string]MCPConnection `json:"mcpConnections,omitempty"`
	// Skills overrides a skill's content, by skill ID.
	Skills map[string]string `json:"skills,omitempty"`
	Seeded Seeded            `json:"seeded"`
}

// legacySettings is the layout that predates #305: global scalars mixed with
// per-project maps. It is only read, and folded into Settings.
type legacySettings struct {
	Commands                    map[string]string `json:"commands,omitempty"`
	CommandsAutonomous          map[string]string `json:"commandsAutonomous,omitempty"`
	Parallelism                 map[string]int    `json:"parallelism,omitempty"`
	Worktrees                   map[string]bool   `json:"worktrees,omitempty"`
	SpecArtifacts               map[string]string `json:"specArtifacts,omitempty"`
	Projects                    map[string]string `json:"projects"`
	SpecRepos                   map[string]string `json:"specRepos,omitempty"`
	AIProviders                 map[string]string `json:"aiProviders,omitempty"`
	AIModels                    map[string]string `json:"aiModels,omitempty"`
	AIProvider                  string            `json:"aiProvider"`
	AICommandTemplate           string            `json:"aiCommandTemplate"`
	AICommandTemplateAutonomous string            `json:"aiCommandTemplateAutonomous,omitempty"`
	AIModel                     string            `json:"aiModel,omitempty"`
	AISkillModels               map[string]string `json:"aiSkillModels,omitempty"`
	Terminal                    string            `json:"terminal"`
	Terminals                   map[string]string `json:"terminals,omitempty"`
}

// legacyKeys are the keys of legacySettings, removed from the file when it is
// rewritten in the current layout.
var legacyKeys = []string{"specArtifacts", "projects", "worktrees", "parallelism", "commands", "commandsAutonomous", "specRepos", "aiProviders", "aiModels", "aiProvider", "aiCommandTemplate", "aiCommandTemplateAutonomous", "aiModel", "aiSkillModels", "terminal", "terminals"}

// fold maps the legacy keys onto the current layout, with the same meaning.
func (l legacySettings) fold() Settings {
	var s Settings
	s.Defaults.AIProvider = l.AIProvider
	s.Defaults.AICommandTemplate = l.AICommandTemplate
	s.Defaults.AICommandTemplateAutonomous = l.AICommandTemplateAutonomous
	s.Defaults.AIModel = l.AIModel
	s.Defaults.AISkillModels = l.AISkillModels
	s.Defaults.Terminal = l.Terminal
	edit := func(id string, change func(*ProjectSettings)) {
		p := s.Project(id)
		change(&p)
		s.SetProject(id, p)
	}
	for id, path := range l.Projects {
		edit(id, func(p *ProjectSettings) { p.Path = path })
	}
	for id, path := range l.SpecRepos {
		edit(id, func(p *ProjectSettings) { p.SpecPath = path })
	}
	for id, value := range l.Worktrees {
		value := value
		edit(id, func(p *ProjectSettings) { p.UseWorktrees = &value })
	}
	for id, value := range l.Parallelism {
		edit(id, func(p *ProjectSettings) { p.Parallelism = value })
	}
	for id, value := range l.Terminals {
		edit(id, func(p *ProjectSettings) { p.Terminal = value })
	}
	for id, value := range l.AIProviders {
		edit(id, func(p *ProjectSettings) { p.AIProvider = value })
	}
	for id, value := range l.AIModels {
		edit(id, func(p *ProjectSettings) { p.AIModel = value })
	}
	for id, value := range l.Commands {
		edit(id, func(p *ProjectSettings) { p.AICommandTemplate = value })
	}
	for id, value := range l.CommandsAutonomous {
		edit(id, func(p *ProjectSettings) { p.AICommandTemplateAutonomous = value })
	}
	for id, value := range l.SpecArtifacts {
		edit(id, func(p *ProjectSettings) { p.SpecArtifacts = value })
	}
	return s
}

// overlay lays top over base: a value top states wins, a value it leaves empty
// is taken from base. Maps merge key by key.
func overlay(base, top Settings) Settings {
	out := top
	out.Defaults = Defaults{
		Execution:        overlayExecution(base.Defaults.Execution, top.Defaults.Execution),
		AIProviderModels: top.Defaults.AIProviderModels,
		EditorCommand:    firstSet(top.Defaults.EditorCommand, base.Defaults.EditorCommand),
	}
	if out.Defaults.AIProviderModels == nil {
		out.Defaults.AIProviderModels = base.Defaults.AIProviderModels
	}
	out.ProjectSettings = nil
	for id, p := range base.ProjectSettings {
		out.SetProject(id, p)
	}
	for id, p := range top.ProjectSettings {
		b := out.Project(id)
		out.SetProject(id, ProjectSettings{
			Path:          firstSet(p.Path, b.Path),
			SpecPath:      firstSet(p.SpecPath, b.SpecPath),
			Execution:     overlayExecution(b.Execution, p.Execution),
			SkillCommands: mergeStrings(b.SkillCommands, p.SkillCommands),
			SpecArtifacts: firstSet(p.SpecArtifacts, b.SpecArtifacts),
		})
	}
	out.Repositories = mergeStrings(base.Repositories, top.Repositories)
	out.Skills = mergeStrings(base.Skills, top.Skills)
	return out
}

func overlayExecution(base, top Execution) Execution {
	out := Execution{
		AIProvider:                  firstSet(top.AIProvider, base.AIProvider),
		AICommandTemplate:           firstSet(top.AICommandTemplate, base.AICommandTemplate),
		AICommandTemplateAutonomous: firstSet(top.AICommandTemplateAutonomous, base.AICommandTemplateAutonomous),
		AIModel:                     firstSet(top.AIModel, base.AIModel),
		AISkillModels:               mergeStrings(base.AISkillModels, top.AISkillModels),
		Terminal:                    firstSet(top.Terminal, base.Terminal),
		UseWorktrees:                top.UseWorktrees,
		Parallelism:                 top.Parallelism,
		SetupProviders:              top.SetupProviders,
	}
	if out.UseWorktrees == nil {
		out.UseWorktrees = base.UseWorktrees
	}
	if out.Parallelism == 0 {
		out.Parallelism = base.Parallelism
	}
	if out.SetupProviders == nil {
		out.SetupProviders = base.SetupProviders
	}
	return out
}

func firstSet(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func mergeStrings(base, top map[string]string) map[string]string {
	if len(base) == 0 && len(top) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(top))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range top {
		if strings.TrimSpace(value) != "" || out[key] == "" {
			out[key] = value
		}
	}
	return out
}

// readLegacyRepositoryFile reads a checkout's .taskflow/agent.json, the path
// that preceded ~/.config/sectile/settings.json. It is read only, as a
// fallback, and never written.
func readLegacyRepositoryFile(root string) (Settings, error) {
	raw, err := os.ReadFile(filepath.Join(root, ".taskflow", "agent.json"))
	if os.IsNotExist(err) {
		return Settings{}, nil
	}
	if err != nil {
		return Settings{}, err
	}
	var legacy legacySettings
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return Settings{}, err
	}
	var current struct {
		Skills       map[string]string `json:"skills"`
		Repositories map[string]string `json:"repositories"`
	}
	if err := json.Unmarshal(raw, &current); err != nil {
		return Settings{}, err
	}
	s := legacy.fold()
	// Disconnection and MCP connections are workstation-owned, never a
	// repository override.
	s.Skills, s.Repositories = current.Skills, current.Repositories
	return s, nil
}

// WithRepositoryFile lays the settings over a checkout's legacy
// .taskflow/agent.json, which fills what they leave unset.
func WithRepositoryFile(s Settings, root string) (Settings, error) {
	legacy, err := readLegacyRepositoryFile(root)
	if err != nil {
		return s, err
	}
	return overlay(legacy, s), nil
}

func hasCreatePR(skills []Skill) bool {
	for _, skill := range skills {
		if skill.ID == "create_pr" {
			return true
		}
	}
	return false
}

// ManifestPath locates the record of what is installed for the current user.
// It is global because the installation itself is: a provider switch retires the
// previous provider's files through the same record.
func ManifestPath() (string, error) {
	settings, err := SettingsPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(settings), "agent-manifest.json"), nil
}

// Scaffold installs the fresh server-owned skill set into the user configuration
// of every agent the project sets up. Changed local copies are backed up before
// replacement, and unrelated personal skill paths are never touched. The checkout
// receives no managed file; it only holds the backups and the copies being retired
// from the layout that preceded this release.
func Scaffold(checkout string, config Config) ([]string, error) {
	return scaffold(checkout, config, false)
}

// ScaffoldProvider refreshes one explicitly selected provider without retiring
// managed skills installed for other providers on this workstation.
func ScaffoldProvider(checkout string, config Config, provider string) ([]string, error) {
	config.AIProvider = provider
	config.SetupProviders = []string{provider}
	return scaffold(checkout, config, true)
}

func scaffold(checkout string, config Config, preserveOtherProviders bool) ([]string, error) {
	if config.SchemaVersion != Version {
		return nil, fmt.Errorf("unsupported configuration version %d", config.SchemaVersion)
	}
	providers, err := SetupProviders(config)
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, provider := range providers {
		loc, err := ResolveLocations(provider)
		if err != nil {
			return nil, err
		}
		installed, err := skillFiles(config.Skills, loc)
		if err != nil {
			return nil, err
		}
		for path, content := range installed {
			files[path] = content
		}
		if loc.InstallsSkills() && !hasCreatePR(config.Skills) {
			for _, skill := range config.Skills {
				if skill.ID != "adjust" {
					continue
				}
				forward := "---\nname: create-pr\ndescription: Compatibility alias for adjust-issue.\n---\nInvoke adjust-issue with the same arguments. Require the existing task-branch PR and the full adjustment quality gate. Never create a PR.\n"
				if loc.SubstitutesArguments {
					forward += "\n$ARGUMENTS\n"
				}
				files[filepath.Join(loc.SkillDir, "create-pr/SKILL.md")] = forward
			}
		}
	}
	work, err := os.OpenRoot(checkout)
	if err != nil {
		return nil, err
	}
	defer work.Close()
	backups, err := retireCheckout(work, config)
	if err != nil {
		return backups, err
	}
	fs, err := os.OpenRoot(home)
	if err != nil {
		return backups, err
	}
	defer fs.Close()
	manifestPath, err := ManifestPath()
	if err != nil {
		return backups, err
	}
	manifestPath, err = filepath.Rel(home, manifestPath)
	if err != nil {
		return backups, err
	}
	manifest := map[string]string{}
	if raw, err := fs.ReadFile(manifestPath); err == nil {
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return backups, err
		}
	} else if !os.IsNotExist(err) {
		return backups, err
	}
	for p := range manifest {
		if !anyManagedPath(p) {
			return backups, fmt.Errorf("invalid managed skill path %q", p)
		}
	}
	untouched := map[string]string{}
	if preserveOtherProviders {
		loc, err := ResolveLocations(config.AIProvider)
		if err != nil {
			return backups, err
		}
		for path, hash := range manifest {
			if !loc.InstallsSkills() || !managedPath(path, loc) {
				untouched[path] = hash
				delete(manifest, path)
			}
		}
	}
	install, err := refresh(fs, work, files, manifest, &backups, !hasCreatePR(config.Skills))
	if err != nil {
		return backups, err
	}
	// Earlier releases registered a Claude Code hook in ~/.claude/settings.json
	// (#174); it was withdrawn (#260). The refresh above retires the script
	// through the manifest, and this drops the registrations that pointed at
	// it, for whichever provider is being set up: a Claude Code session used by
	// hand would otherwise fail on every turn. A file that cannot be parsed is
	// left alone and reported; the rest of the setup is still valid without it.
	report, err := retireClaudeHooks(fs)
	if err != nil {
		return backups, err
	}
	if report != "" {
		backups = append(backups, report)
	}
	// The directory only ever held Sectile's scripts; Remove refuses a
	// directory that still holds anything, which is the guard wanted here.
	_ = fs.Remove(claudeHookDir)
	for path, hash := range untouched {
		install[path] = hash
	}
	raw, err := json.MarshalIndent(install, "", "  ")
	if err != nil {
		return backups, err
	}
	if err := atomicWrite(fs, manifestPath, raw); err != nil {
		return backups, err
	}
	raw, err = json.MarshalIndent(config, "", "  ")
	if err != nil {
		return backups, err
	}
	return backups, atomicWrite(work, ".taskflow/remote-config.json", raw)
}

// anyManagedPath keeps a manifest written for another provider, or by an earlier
// release, readable, so the files it records can be retired instead of rejected
// as foreign.
func anyManagedPath(p string) bool {
	for _, provider := range SkillProviders {
		loc, err := ResolveLocations(provider)
		if err == nil && managedPath(p, loc) {
			return true
		}
	}
	return managedRetiredPath(p) || managedHookPath(p)
}

func digest(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }

func atomicWrite(fs *os.Root, path string, raw []byte) error {
	if err := fs.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp-" + uuid.NewString()
	defer fs.Remove(tmp)
	if err := fs.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return fs.Rename(tmp, path)
}

// refresh installs the managed set, retires what left it, and reports every local
// edit it preserved. Backups stay in the checkout, where the previous release
// already put them.
func refresh(fs, work *os.Root, files, manifest map[string]string, backups *[]string, legacyCreatePR bool) (map[string]string, error) {
	backup := func(path string, raw []byte) error {
		dst := filepath.Join(".taskflow/skill-backups", digest(raw), path)
		if err := atomicWrite(work, dst, raw); err != nil {
			return err
		}
		*backups = append(*backups, dst)
		return nil
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	install := map[string]string{}
	for _, p := range paths {
		content := files[p]
		raw, err := fs.ReadFile(p)
		if err != nil && !os.IsNotExist(err) {
			return install, err
		}
		if err == nil && string(raw) != content && manifest[p] != digest(raw) {
			// A create-pr alias the user edited stays theirs: it is reported on every
			// refresh instead of being replaced by the compatibility forward.
			if legacyCreatePR && (strings.Contains(p, "/create-pr/") || strings.HasSuffix(p, "/create-pr.md")) {
				*backups = append(*backups, "Divergent legacy command preserved: "+p)
				install[p] = manifest[p]
				continue
			}
			if err := backup(p, raw); err != nil {
				return install, err
			}
		}
		if err != nil || string(raw) != content {
			if err := atomicWrite(fs, p, []byte(content)); err != nil {
				return install, err
			}
		}
		install[p] = digest([]byte(content))
	}
	// Removed/renamed skills are retired only when their managed content is unchanged.
	// Modified copies are personal after retirement and are left intact.
	for p, oldDigest := range manifest {
		if _, exists := files[p]; exists {
			continue
		}
		raw, err := fs.ReadFile(p)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return install, err
		}
		if digest(raw) != oldDigest {
			continue
		}
		if err := backup(p, raw); err != nil {
			return install, err
		}
		if err := fs.Remove(p); err != nil {
			return install, err
		}
	}
	return install, nil
}

// MaxParallelism bounds the concurrent executions a workstation may run for one
// project. Parallelism is workstation-owned: the server neither stores nor
// supplies it, so every surface that accepts or clamps a value reads this.
const MaxParallelism = 10

// ExecutionLimit is workstation-owned and serializes shared checkout execution.
// The project section speaks over the workstation defaults; without either a
// project runs a single execution at a time.
func ExecutionLimit(projectID string, useWorktrees bool, settings Settings) int {
	if !useWorktrees {
		return 1
	}
	n := settings.ProjectSettings[projectID].Parallelism
	if n == 0 {
		n = settings.Defaults.Parallelism
	}
	if n < 1 {
		return 1
	}
	if n > MaxParallelism {
		return MaxParallelism
	}
	return n
}
