package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"tasks/internal/agentconfig"
	"tasks/internal/models"
)

// A project that drops its specification artefacts (#487) keeps them out of Git
// through a block of the checkout's info/exclude file, shared by every worktree
// of the checkout and never part of the repository. The block belongs to one
// project, so two projects sharing a checkout each manage their own, and the
// lines around it are the user's and are never touched.

// safeArtefactKey is what a task key must look like, once its leading # is
// stripped, to be written into an ignore pattern: no glob character, no
// separator, no whitespace, nothing that could escape its line.
var safeArtefactKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func specExcludeMarkers(projectID string) (string, string) {
	return "# >>> sectile: dropped specification artefacts, project " + projectID + " >>>",
		"# <<< sectile: dropped specification artefacts, project " + projectID + " <<<"
}

// artefactPatterns returns the anchored ignore patterns of a task's
// specification artefacts under both frameworks, plus their lower-case forms
// when the key has upper-case letters, since some tools lower-case change
// identifiers. A key that cannot be written safely yields none.
func artefactPatterns(key string) []string {
	key = strings.TrimPrefix(strings.TrimSpace(key), "#")
	if !safeArtefactKey.MatchString(key) {
		return nil
	}
	forms := []string{key}
	if lower := strings.ToLower(key); lower != key {
		forms = append(forms, lower)
	}
	var patterns []string
	for _, k := range forms {
		patterns = append(patterns, "/docs/clarifications/"+k+".md", "/openspec/changes/"+k+"-*/", "/specs/"+k+"-*/")
	}
	sort.Strings(patterns)
	return patterns
}

// excludeFilePath is the info/exclude file of the repository at repo, in its
// common directory, so a linked worktree and its main checkout share it.
func excludeFilePath(ctx context.Context, repo string) (string, error) {
	common, err := gitLocal(ctx, repo, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(repo, common)
	}
	return filepath.Join(common, "info", "exclude"), nil
}

// specExcludeBlock locates a project's block among lines, each carrying its
// own line ending. end is the index of the closing marker. An opening marker
// with no closing line is a block of that marker alone: the lines after it
// cannot be told from the user's, so they are kept.
func specExcludeBlock(lines []string, projectID string) (start, end int, found bool) {
	open, closing := specExcludeMarkers(projectID)
	start = -1
	for i, line := range lines {
		text := strings.TrimRight(line, "\r\n")
		if start < 0 {
			if text == open {
				start = i
			}
			continue
		}
		if text == closing {
			return start, i, true
		}
	}
	if start < 0 {
		return 0, 0, false
	}
	return start, start, true
}

func validProjectMarker(projectID string) error {
	if strings.TrimSpace(projectID) == "" || strings.ContainsAny(projectID, "\r\n") {
		return fmt.Errorf("invalid project identifier for the exclude block")
	}
	return nil
}

// ensureSpecExclusions makes the project's block in the checkout's exclude
// file carry the rules of the task's artefacts, keeping the rules it already
// has. It reports whether the task has rules at all: a key that cannot be
// written safely gets none, and the caller keeps the artefacts instead.
func ensureSpecExclusions(ctx context.Context, checkout, projectID, key string) (bool, error) {
	patterns := artefactPatterns(key)
	if len(patterns) == 0 {
		return false, nil
	}
	if err := validProjectMarker(projectID); err != nil {
		return false, err
	}
	path, err := excludeFilePath(ctx, checkout)
	if err != nil {
		return false, err
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	lines := strings.SplitAfter(string(raw), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	wanted := map[string]bool{}
	for _, pattern := range patterns {
		wanted[pattern] = true
	}
	start, end, found := specExcludeBlock(lines, projectID)
	if found && end > start {
		for _, line := range lines[start+1 : end] {
			if text := strings.TrimSpace(line); text != "" && !strings.HasPrefix(text, "#") {
				wanted[text] = true
			}
		}
	}
	rules := make([]string, 0, len(wanted))
	for pattern := range wanted {
		rules = append(rules, pattern)
	}
	sort.Strings(rules)
	open, closing := specExcludeMarkers(projectID)
	block := open + "\n" + strings.Join(rules, "\n") + "\n" + closing + "\n"

	var content string
	if found {
		content = strings.Join(lines[:start], "") + block + strings.Join(lines[end+1:], "")
	} else {
		content = string(raw)
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += block
	}
	if content == string(raw) {
		return true, nil
	}
	return true, writeExcludeFile(path, content)
}

// removeSpecExclusions removes the project's block, markers included, and
// leaves every other line of the exclude file as it was.
func removeSpecExclusions(ctx context.Context, checkout, projectID string) error {
	if err := validProjectMarker(projectID); err != nil {
		return err
	}
	path, err := excludeFilePath(ctx, checkout)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	lines := strings.SplitAfter(string(raw), "\n")
	start, end, found := specExcludeBlock(lines, projectID)
	if !found {
		return nil
	}
	return writeExcludeFile(path, strings.Join(lines[:start], "")+strings.Join(lines[end+1:], ""))
}

// writeExcludeFile replaces the exclude file atomically, through a temporary
// file in the same directory, so a Git command running meanwhile never reads
// half a file. The file keeps its mode.
func writeExcludeFile(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	file, err := os.CreateTemp(dir, ".exclude-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		return err
	}
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// applySpecArtifacts brings the checkout's exclude block in line with the
// effective setting before a task's session starts (#487): the task's rules
// when the project drops its artefacts, no block at all when it keeps them. A
// failure stops the launch, since a session that should drop must not start
// committing. A key that gets no rule proceeds as keep, and config says so.
func applySpecArtifacts(ctx context.Context, config *agentconfig.Config, checkout, key string) error {
	if !config.DropsSpecArtifacts() {
		return removeSpecExclusions(ctx, checkout, config.ProjectID)
	}
	covered, err := ensureSpecExclusions(ctx, checkout, config.ProjectID, key)
	if err != nil {
		return fmt.Errorf("excluding the specification artefacts of %s: %w", key, err)
	}
	if !covered {
		log.Printf("[Agent] Task key %q cannot be written as an ignore pattern; its specification artefacts are kept", key)
		config.SpecArtifacts = models.SpecArtifactsKeep
	}
	return nil
}

// clearSpecExclusions removes the project's block from every checkout this
// workstation maps for it: the project root and each mapped repository. It is
// what saving the desktop settings with an effective keep does, so the rules
// go at once rather than at the next launch. Failures are logged: the
// settings are saved already, and the next launch retries.
func clearSpecExclusions(ctx context.Context, config agentconfig.Config, overrides agentconfig.Overrides, projectRoot string) {
	code := codeIdentity(config)
	seen := map[string]bool{}
	checkouts := []string{projectRoot}
	for _, repository := range projectRepositories(config) {
		if root, ok := repositoryRoot(overrides, projectRoot, code, repository.Identity); ok {
			checkouts = append(checkouts, root)
		}
	}
	for _, checkout := range checkouts {
		if checkout == "" || seen[filepath.Clean(checkout)] {
			continue
		}
		seen[filepath.Clean(checkout)] = true
		if err := removeSpecExclusions(ctx, checkout, config.ProjectID); err != nil {
			log.Printf("[Agent] Could not remove the specification artefacts exclusions of project %s from %s: %v", config.ProjectID, checkout, err)
		}
	}
}

// specArtifactsNotice is the prompt line telling a stage that writes or reads
// the specification artefacts that this workstation drops them (#487). The
// skill decides from Git; the line is for whoever reads the session.
func specArtifactsNotice(config agentconfig.Config, skillID string) string {
	if !config.DropsSpecArtifacts() {
		return ""
	}
	switch skillID {
	case "clarify", "specify", "implement", "adjust", "pickup", "pickup_issues":
		return "\nSpecification artefacts are dropped on this workstation: write them in the worktree, never commit or push them (they are excluded through .git/info/exclude)."
	}
	return ""
}

// specArtifactsMode answers the server's spec_artifacts question (#487): the
// effective value on this workstation for the task, keep for a key that gets
// no rule, as its launches do. Without a task, the project's effective value.
func specArtifactsMode(config agentconfig.Config, key string) map[string]string {
	if config.DropsSpecArtifacts() && (key == "" || artefactPatterns(key) != nil) {
		return map[string]string{"mode": models.SpecArtifactsDrop}
	}
	return map[string]string{"mode": models.SpecArtifactsKeep}
}
