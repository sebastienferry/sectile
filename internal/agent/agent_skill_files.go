package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
)

// localSkillFiles reads the managed skills back from the location they were
// installed in for the selected provider. A provider without a skill convention
// reports empty entries rather than an error: nothing is installed, by design.
func localSkillFiles(config agentconfig.Config) (map[string]agentprotocol.SkillFile, error) {
	files := map[string]agentprotocol.SkillFile{}
	loc, err := agentconfig.ResolveLocations(agentconfig.EffectiveProvider(config.AIProvider, config.AICommandTemplate))
	if err != nil {
		return nil, err
	}
	for _, skill := range config.Skills {
		files[skill.ID] = agentprotocol.SkillFile{}
	}
	if !loc.InstallsSkills() {
		return files, nil
	}
	fs, err := os.OpenRoot(loc.Home)
	if err != nil {
		return nil, err
	}
	defer fs.Close()
	for _, skill := range config.Skills {
		relative := filepath.Join(loc.SkillDir, skill.Directory, "SKILL.md")
		raw, err := fs.ReadFile(relative)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		files[skill.ID] = agentprotocol.SkillFile{Content: string(raw), Paths: []string{filepath.Join(loc.Home, relative)}}
	}
	return files, nil
}

func localWorktreePaths(ctx context.Context, root string) ([]string, error) {
	output, err := gitLocal(ctx, root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	paths := []string{root}
	seen := map[string]bool{filepath.Clean(root): true}
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		path := filepath.Clean(strings.TrimPrefix(line, "worktree "))
		if !seen[path] {
			paths = append(paths, path)
			seen[path] = true
		}
	}
	return paths, nil
}
