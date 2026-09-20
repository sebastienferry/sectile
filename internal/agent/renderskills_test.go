package agent_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tasks/internal/agent"
)

func TestRenderSkillsTo_StdoutSingleSkill(t *testing.T) {
	var buf bytes.Buffer
	err := agent.RenderSkillsTo([]string{"-skill", "clarify", "-framework", "speckit"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "name: clarify-issue") {
		t.Errorf("expected frontmatter with clarify-issue, got: %s", out)
	}
	if !strings.Contains(out, "# Clarify Issue") {
		t.Errorf("expected title Clarify Issue, got: %s", out)
	}
}

func TestRenderSkillsTo_StdoutCommand(t *testing.T) {
	var buf bytes.Buffer
	err := agent.RenderSkillsTo([]string{"-skill", "specify", "-format", "command", "-framework", "openspec"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "argument-hint: <TICKET-KEY> [contexte]") {
		t.Errorf("expected argument-hint in command format, got: %s", out)
	}
	if !strings.Contains(out, "## Ticket\n$ARGUMENTS") {
		t.Errorf("expected $ARGUMENTS placeholder in command format, got: %s", out)
	}
}

func TestRenderSkillsTo_DiskAllSkills(t *testing.T) {
	tempDir := t.TempDir()
	outDir := filepath.Join(tempDir, "exported-skills")

	var buf bytes.Buffer
	err := agent.RenderSkillsTo([]string{"-out", outDir, "-format", "all", "-framework", "speckit"}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify clarify skill file exists
	skillFile := filepath.Join(outDir, "clarify-issue", "SKILL.md")
	content, err := os.ReadFile(skillFile)
	if err != nil {
		t.Fatalf("expected file %s to exist: %v", skillFile, err)
	}
	if !strings.Contains(string(content), "name: clarify-issue") {
		t.Errorf("expected clarify-issue frontmatter, got: %s", string(content))
	}

	// Verify clarify command file exists
	cmdFile := filepath.Join(outDir, "commands", "clarify-issue.md")
	cmdContent, err := os.ReadFile(cmdFile)
	if err != nil {
		t.Fatalf("expected file %s to exist: %v", cmdFile, err)
	}
	if !strings.Contains(string(cmdContent), "$ARGUMENTS") {
		t.Errorf("expected $ARGUMENTS in command, got: %s", string(cmdContent))
	}
}

func TestRenderSkillsTo_ValidationErrors(t *testing.T) {
	var buf bytes.Buffer

	// Invalid framework
	if err := agent.RenderSkillsTo([]string{"-framework", "unknown"}, &buf); err == nil {
		t.Error("expected error for unknown framework, got nil")
	}

	// Invalid format
	if err := agent.RenderSkillsTo([]string{"-format", "pdf"}, &buf); err == nil {
		t.Error("expected error for unknown format, got nil")
	}

	// Unknown skill
	if err := agent.RenderSkillsTo([]string{"-skill", "nonexistent"}, &buf); err == nil {
		t.Error("expected error for nonexistent skill, got nil")
	}
}
