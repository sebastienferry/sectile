package agentconfig

import (
	"fmt"
	"regexp"
	"strings"
)

// model accepts the identifiers the CLIs actually publish — `claude-opus-5`,
// `gpt-5-codex`, `gemini-2.5-pro`, `anthropic/claude-sonnet-5` — and nothing that
// could change the meaning of a command line. This is a security boundary, not a
// convenience check: the value is interpolated into a line run through `sh -c`
// when a command template is in play, the same reason escapeForDoubleQuotes exists.
var model = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:@/-]*$`)

// ModelConfig is one level of model configuration: a default and the skills that
// depart from it. Levels are merged most-specific-first by MergeModels.
type ModelConfig struct {
	Model       string            `json:"aiModel,omitempty"`
	SkillModels map[string]string `json:"aiSkillModels,omitempty"`
}

// ValidModel rejects an identifier that could not be placed on a command line as
// a single word. An empty identifier means "inherit" and is always valid.
func ValidModel(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !model.MatchString(value) {
		return fmt.Errorf("invalid AI model %q: use letters, digits and . _ - : @ /", value)
	}
	return nil
}

// ValidModelConfig checks a level as a whole, naming the skill when the offending
// value is a per-skill one.
func ValidModelConfig(c ModelConfig) error {
	if err := ValidModel(c.Model); err != nil {
		return err
	}
	for skill, value := range c.SkillModels {
		if err := ValidModel(value); err != nil {
			return fmt.Errorf("skill %q: %w", skill, err)
		}
	}
	return nil
}

// MergeModels folds a more specific level onto a less specific one. Precedence
// inside a level is the skill entry first, then the level's own model, which is
// why a bare model on the high level has to be materialised for every skill the
// low level names: it wins over the low level's skill entry.
func MergeModels(high, low ModelConfig) ModelConfig {
	merged := ModelConfig{Model: strings.TrimSpace(high.Model)}
	if merged.Model == "" {
		merged.Model = strings.TrimSpace(low.Model)
	}
	skills := map[string]bool{}
	for skill := range high.SkillModels {
		skills[skill] = true
	}
	for skill := range low.SkillModels {
		skills[skill] = true
	}
	for skill := range skills {
		value := strings.TrimSpace(high.SkillModels[skill])
		if value == "" {
			if strings.TrimSpace(high.Model) != "" {
				// The high level speaks for every skill it does not single out.
				continue
			}
			value = strings.TrimSpace(low.SkillModels[skill])
		}
		if value == "" {
			continue
		}
		if merged.SkillModels == nil {
			merged.SkillModels = map[string]string{}
		}
		merged.SkillModels[skill] = value
	}
	return merged
}

// Models exposes the configuration's own level, once every level above it has
// already been folded in by the server and by ApplyOverrides.
func (c Config) Models() ModelConfig {
	return ModelConfig{Model: c.AIModel, SkillModels: c.AISkillModels}
}

// ResolveSkillModel returns the model a given skill runs against within one
// already-merged level, or "" when nothing is configured, which means the
// provider keeps its own default.
func ResolveSkillModel(c ModelConfig, skillID string) string {
	skillID = strings.TrimSpace(skillID)
	if skillID != "" {
		if value := strings.TrimSpace(c.SkillModels[skillID]); value != "" {
			return value
		}
	}
	return strings.TrimSpace(c.Model)
}

// ResolveModel is ResolveSkillModel over the execution contract.
func ResolveModel(c Config, skillID string) string {
	return ResolveSkillModel(c.Models(), skillID)
}

// ModelFlag reports how a provider takes a model on its command line. A provider
// that accepts none is not an error: its runs simply carry no model.
func ModelFlag(provider string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "claude", "codex", "gemini", "cursor":
		return "--model", true
	}
	return "", false
}

// ModelArgs is the argument pair to splice into a provider invocation, empty when
// the provider takes no model or none is configured.
func ModelArgs(provider, resolved string) []string {
	flag, ok := ModelFlag(provider)
	if !ok || strings.TrimSpace(resolved) == "" {
		return nil
	}
	return []string{flag, strings.TrimSpace(resolved)}
}
