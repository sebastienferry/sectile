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

	"tasks/internal/models"
)

// Overrides stays on the workstation and is never uploaded to the server.
type Overrides struct {
	DisconnectedProjects map[string]bool   `json:"disconnectedProjects,omitempty"`
	Commands             map[string]string `json:"commands,omitempty"`
	Parallelism          map[string]int    `json:"parallelism,omitempty"`
	Worktrees            map[string]bool   `json:"worktrees,omitempty"`
	Projects             map[string]string `json:"projects"`
	AIProvider           string            `json:"aiProvider"`
	AICommandTemplate    string            `json:"aiCommandTemplate"`
	Terminal             string            `json:"terminal"`
	Skills               map[string]string `json:"skills"`
}

func ReadOverrides(root string) (Overrides, error) {
	var result Overrides
	raw, err := os.ReadFile(filepath.Join(root, ".taskflow", "agent.json"))
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(raw, &result)
	// Disconnection is workstation-owned, never a repository override.
	result.DisconnectedProjects = nil
	return result, err
}

func ApplyOverrides(c Config, overrides Overrides) Config {
	if value, ok := overrides.Worktrees[c.ProjectID]; ok {
		c.UseWorktrees = value
	}
	serverCommand := c.AICommandTemplate
	c.Skills = append([]Skill{}, c.Skills...)
	if overrides.AIProvider != "" {
		if overrides.AIProvider != c.AIProvider && overrides.AICommandTemplate == "" {
			c.AICommandTemplate = ""
		}
		c.AIProvider = overrides.AIProvider
	}
	if overrides.AICommandTemplate != "" {
		c.AICommandTemplate = overrides.AICommandTemplate
	}
	if overrides.Terminal != "" {
		c.ExternalTerminalCommand = overrides.Terminal
	}
	for i := range c.Skills {
		id := c.Skills[i].ID
		if id == "adjust" {
			for _, legacy := range []string{"review"} {
				if strings.TrimSpace(overrides.Skills[id]) == "" && strings.TrimSpace(overrides.Skills[legacy]) != "" {
					c.Skills[i].RequiresReconciliation = true
				}
			}
		}
		if content, ok := overrides.Skills[id]; ok {
			if id == "adjust" {
				content += "\n" + c.Skills[i].Content
			}
			c.Skills[i].Content = content
			c.Skills[i].CommandContent = content + "\n\n## Ticket\n$ARGUMENTS\n"
		}
	}
	if command, ok := overrides.Commands[c.ProjectID]; ok {
		c.AICommandTemplate = command
		if command == "" {
			c.AICommandTemplate = serverCommand
		}
	}
	return c
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
	install, err := refresh(fs, work, files, manifest, &backups, !hasCreatePR(config.Skills))
	if err != nil {
		return backups, err
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
	return managedRetiredPath(p)
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

// ExecutionLimit is workstation-owned and serializes shared checkout execution.
func ExecutionLimit(projectID string, useWorktrees bool, overrides Overrides, serverDefault ...int) int {
	if !useWorktrees {
		return 1
	}
	n, overridden := overrides.Parallelism[projectID]
	if !overridden && len(serverDefault) > 0 {
		n = serverDefault[0]
	}
	if n < 1 {
		return 1
	}
	if n > models.MaxParallelism {
		return models.MaxParallelism
	}
	return n
}
