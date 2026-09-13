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

// Overrides stays on the workstation and is never uploaded to the server.
type Overrides struct {
	Commands          map[string]string `json:"commands,omitempty"`
	Parallelism       map[string]int    `json:"parallelism,omitempty"`
	Worktrees         map[string]bool   `json:"worktrees,omitempty"`
	Projects          map[string]string `json:"projects"`
	AIProvider        string            `json:"aiProvider"`
	AICommandTemplate string            `json:"aiCommandTemplate"`
	Terminal          string            `json:"terminal"`
	Skills            map[string]string `json:"skills"`
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

// Scaffold installs the fresh server-owned skill set. Changed local copies are
// backed up before replacement. Unrelated personal skill paths are never touched.
func Scaffold(root string, config Config) ([]string, error) {
	if config.SchemaVersion != Version {
		return nil, fmt.Errorf("unsupported configuration version %d", config.SchemaVersion)
	}
	files, err := skillFiles(config.Skills)
	if err != nil {
		return nil, err
	}
	for _, skill := range config.Skills {
		if skill.ID == "adjust" && !hasCreatePR(config.Skills) {
			forward := "---\nname: create-pr\ndescription: Compatibility alias for adjust-issue.\n---\nInvoke adjust-issue with the same arguments. Require the existing task-branch PR and the full adjustment quality gate. Never create a PR.\n"
			for _, prefix := range []string{".agents/skills/", ".claude/skills/", ".gemini/skills/", ".agy/skills/", ".skills/"} {
				files[prefix+"create-pr/SKILL.md"] = forward
			}
			files[".claude/commands/create-pr.md"] = forward + "\n$ARGUMENTS\n"
		}
	}
	fs, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer fs.Close()
	manifest := map[string]string{}
	if raw, err := fs.ReadFile(".taskflow/agent-manifest.json"); err == nil {
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	for p := range manifest {
		if !managedSkillPath(p) {
			return nil, fmt.Errorf("invalid managed skill path %q", p)
		}
	}
	if err := fs.MkdirAll(".taskflow", 0755); err != nil {
		return nil, err
	}
	atomicWrite := func(path string, raw []byte) error {
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
	digest := func(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
	backups := []string{}
	backup := func(path string, raw []byte) error {
		dst := filepath.Join(".taskflow/skill-backups", digest(raw), path)
		if err := atomicWrite(dst, raw); err != nil {
			return err
		}
		backups = append(backups, dst)
		return nil
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	next := map[string]string{}
	for _, p := range paths {
		content := files[p]
		raw, err := fs.ReadFile(p)
		if err != nil && !os.IsNotExist(err) {
			return backups, err
		}
		if err == nil && string(raw) != content {
			if manifest[p] != digest(raw) {
				if !hasCreatePR(config.Skills) && (strings.Contains(p, "/create-pr/") || strings.HasSuffix(p, "/create-pr.md")) {
					backups = append(backups, "Divergent legacy command preserved: "+p)
					next[p] = manifest[p]
					continue
				}
				if err := backup(p, raw); err != nil {
					return backups, err
				}
			}
		}
		if err != nil || string(raw) != content {
			if err := atomicWrite(p, []byte(content)); err != nil {
				return backups, err
			}
		}
		next[p] = digest([]byte(content))
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
			return backups, err
		}
		if digest(raw) != oldDigest {
			continue
		}
		if err := backup(p, raw); err != nil {
			return backups, err
		}
		if err := fs.Remove(p); err != nil {
			return backups, err
		}
	}
	raw, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return backups, err
	}
	if err := atomicWrite(".taskflow/remote-config.json", raw); err != nil {
		return backups, err
	}
	raw, err = json.MarshalIndent(next, "", "  ")
	if err != nil {
		return backups, err
	}
	return backups, atomicWrite(".taskflow/agent-manifest.json", raw)
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
	if n > 3 {
		return 3
	}
	return n
}
