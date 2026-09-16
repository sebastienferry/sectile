package agentconfig

import "strings"

// UsesCommandTemplate reports whether a launch runs the configured command template.
// Named providers ignore a template without {prompt} — legacy rows hold a bare CLI
// name there — and fall back to their built-in command; custom providers have nothing else.
func UsesCommandTemplate(provider, template string) bool {
	return template != "" && (provider == "custom" || strings.Contains(template, "{prompt}"))
}

// EffectiveCommandTemplate returns the template a launch would run, or "" for the provider default.
func EffectiveCommandTemplate(provider, template string) string {
	if UsesCommandTemplate(provider, template) {
		return template
	}
	return ""
}

// ExpandModel fills the optional {model} slot a command template may carry. An
// unconfigured model yields the empty string rather than a literal placeholder,
// so a template written with the slot still runs when no model is set.
func ExpandModel(template, model string) string {
	if !strings.Contains(template, "{model}") {
		return template
	}
	return strings.ReplaceAll(template, "{model}", strings.TrimSpace(model))
}
