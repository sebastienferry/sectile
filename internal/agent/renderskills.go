package agent

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"tasks/internal/db"
)

// RenderSkills handles CLI execution for rendering embedded skills to stdout or disk.
func RenderSkills(args []string) error {
	return RenderSkillsTo(args, os.Stdout)
}

// RenderSkillsTo handles CLI execution, directing standard output to the provided writer.
func RenderSkillsTo(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("render-skills", flag.ContinueOnError)
	frameworkFlag := fs.String("framework", "speckit", "Specification framework: speckit or openspec")
	skillFlag := fs.String("skill", "", "Filter to a specific skill by ID or directory name (e.g. clarify, specify, adjust)")
	formatFlag := fs.String("format", "skill", "Output format: skill (SKILL.md), command (slash command .md), or all")
	outDirFlag := fs.String("out", "", "Directory to write rendered files into (prints to stdout if omitted for a single skill)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	framework := strings.ToLower(strings.TrimSpace(*frameworkFlag))
	if framework != "speckit" && framework != "openspec" {
		return fmt.Errorf("invalid framework %q: must be 'speckit' or 'openspec'", framework)
	}

	format := strings.ToLower(strings.TrimSpace(*formatFlag))
	if format != "skill" && format != "command" && format != "all" {
		return fmt.Errorf("invalid format %q: must be 'skill', 'command', or 'all'", format)
	}

	skillFilter := strings.TrimSpace(*skillFlag)
	var targetSkills []db.StageSkill
	if skillFilter != "" {
		skill, ok := db.StageSkillByID(skillFilter)
		if !ok {
			// Check if matched by DirName directly
			for _, s := range db.StageSkills {
				if strings.EqualFold(s.DirName, skillFilter) {
					skill = s
					ok = true
					break
				}
			}
		}
		if !ok {
			return fmt.Errorf("unknown skill %q: run without -skill to see available skills", skillFilter)
		}
		targetSkills = append(targetSkills, skill)
	} else {
		targetSkills = db.StageSkills
	}

	outDir := strings.TrimSpace(*outDirFlag)

	// If a single skill was targeted and no output directory was specified, print to out (stdout)
	if len(targetSkills) == 1 && outDir == "" {
		skill := targetSkills[0]
		switch format {
		case "skill":
			fmt.Fprint(out, db.RenderSkillContent(skill, framework))
		case "command":
			fmt.Fprint(out, db.RenderSkillCommand(skill, framework))
		case "all":
			fmt.Fprintln(out, "=== SKILL.md ===")
			fmt.Fprint(out, db.RenderSkillContent(skill, framework))
			fmt.Fprintln(out, "\n=== COMMAND.md ===")
			fmt.Fprint(out, db.RenderSkillCommand(skill, framework))
		}
		return nil
	}

	// Default output directory if writing multiple skills without -out
	if outDir == "" {
		outDir = ".skills"
	}

	writtenCount := 0
	for _, skill := range targetSkills {
		dirName := skill.DirName
		if dirName == "" {
			dirName = skill.ID
		}

		if format == "skill" || format == "all" {
			skillPath := filepath.Join(outDir, dirName, "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(skillPath), 0755); err != nil {
				return fmt.Errorf("create skill dir for %s: %w", dirName, err)
			}
			content := db.RenderSkillContent(skill, framework)
			if err := os.WriteFile(skillPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("write skill file %s: %w", skillPath, err)
			}
			writtenCount++
		}

		if format == "command" || format == "all" {
			cmdPath := filepath.Join(outDir, "commands", dirName+".md")
			if err := os.MkdirAll(filepath.Dir(cmdPath), 0755); err != nil {
				return fmt.Errorf("create commands dir: %w", err)
			}
			content := db.RenderSkillCommand(skill, framework)
			if err := os.WriteFile(cmdPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("write command file %s: %w", cmdPath, err)
			}
			writtenCount++
		}
	}

	fmt.Fprintf(out, "Rendered %d file(s) (%s framework, format: %s) into %s\n",
		writtenCount, framework, format, outDir)
	return nil
}
