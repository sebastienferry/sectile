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

// ModelPlaceholder is the optional slot a command template may carry for the
// resolved model.
const ModelPlaceholder = "{model}"

// ExpandModel fills the optional {model} slot a command template may carry. With
// no model configured the slot is removed rather than emptied: a flag left with
// nothing behind it does not disappear, it consumes the next word, so
// `--model {model} -p ...` would hand the CLI "-p" as a model name.
func ExpandModel(template, model string) string {
	if !strings.Contains(template, ModelPlaceholder) {
		return template
	}
	if strings.TrimSpace(model) == "" {
		return DropModelSlot(template)
	}
	return strings.ReplaceAll(template, ModelPlaceholder, strings.TrimSpace(model))
}

// DropModelSlot removes every {model} slot from a template that has no model to
// put in it, along with the option the slot is the value of.
//
// The slot is removed with the whole word that carries it, so `'{model}'` and
// `--model={model}` go too, and the word before it is removed as well when it
// looks like an option: that word exists only to introduce the value, and
// keeping it would break the command line instead of shortening it. A template
// that puts a bare {model} after something that is not an option keeps that
// something, since nothing says the two belong together.
func DropModelSlot(template string) string {
	for {
		at := strings.Index(template, ModelPlaceholder)
		if at < 0 {
			return template
		}
		start := strings.LastIndexAny(template[:at], " \t") + 1
		end := at + len(ModelPlaceholder)
		for end < len(template) && template[end] != ' ' && template[end] != '\t' {
			end++
		}
		cut := start
		if before := strings.TrimRight(template[:start], " \t"); before != "" {
			if option := strings.LastIndexAny(before, " \t") + 1; strings.HasPrefix(before[option:], "-") {
				cut = option
			}
		}
		head := strings.TrimRight(template[:cut], " \t")
		tail := strings.TrimLeft(template[end:], " \t")
		switch {
		case head == "":
			template = tail
		case tail == "":
			template = head
		default:
			template = head + " " + tail
		}
	}
}
