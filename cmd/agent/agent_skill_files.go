package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"tasks/internal/agentconfig"
	"tasks/internal/agentprotocol"
)

func localSkillFiles(root string, config agentconfig.Config) (map[string]agentprotocol.SkillFile, error) {
	fs, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer fs.Close()
	files := map[string]agentprotocol.SkillFile{}
	for _, skill := range config.Skills {
		file := agentprotocol.SkillFile{}
		for _, prefix := range []string{".agents/skills", ".claude/skills", ".gemini/skills", ".agy/skills", ".skills"} {
			relative := filepath.Join(prefix, skill.Directory, "SKILL.md")
			raw, err := fs.ReadFile(relative)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if len(file.Paths) == 0 {
				file.Content = string(raw)
			}
			file.Paths = append(file.Paths, filepath.Join(root, relative))
		}
		files[skill.ID] = file
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
