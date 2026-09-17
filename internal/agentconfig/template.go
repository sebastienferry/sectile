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

// ExpandModel fills the optional {model} slot a command template may carry. A
// configured model is substituted in place; an unconfigured one takes its slot
// away together with the option the slot belongs to. Erasing the marker alone
// left the flag dangling, and `--model  -p "…"` makes the CLI read -p as the
// model name, so the prompt degrades to a positional argument and the run
// either fails or executes truncated.
func ExpandModel(template, model string) string {
	if !strings.Contains(template, "{model}") {
		return template
	}
	if resolved := strings.TrimSpace(model); resolved != "" {
		return strings.ReplaceAll(template, "{model}", resolved)
	}
	return dropModelSlots(template)
}

// dropModelSlots removes every {model} occurrence and, when the marker is the
// value of a neighbouring option, that option too. A preceding token counts as
// an option only when it starts with "-": anything else is a word the template
// author wrote on purpose, and guessing there would silently break templates
// that use the slot positionally. A slot written as --flag={model} is a single
// token that already carries its own option, so it leaves on its own.
func dropModelSlots(template string) string {
	for {
		at := strings.Index(template, "{model}")
		if at < 0 {
			return strings.TrimSpace(template)
		}
		start, end := tokenAround(template, at)
		if strings.Contains(template[start:end], "{prompt}") {
			// A template that glues the two markers into one token still has to
			// keep the prompt the command line exists to carry: take the marker
			// alone and leave the rest of the token alone.
			template = cutRun(template, at, at+len("{model}"))
			continue
		}
		if !strings.HasPrefix(template[start:end], "-") {
			if begin, stop, ok := precedingToken(template, start); ok && template[begin] == '-' &&
				!strings.Contains(template[begin:stop], "{prompt}") {
				start = begin
			}
		}
		template = cutRun(template, start, end)
	}
}

// tokenAround returns the bounds of the blank-delimited token covering at. It
// is what carries the quotes of a "{model}" or '{model}' slot along with the
// marker.
func tokenAround(s string, at int) (int, int) {
	start, end := at, at
	for start > 0 && !isBlank(s[start-1]) {
		start--
	}
	for end < len(s) && !isBlank(s[end]) {
		end++
	}
	return start, end
}

// precedingToken returns the bounds of the token before start, if there is one.
func precedingToken(s string, start int) (int, int, bool) {
	end := start
	for end > 0 && isBlank(s[end-1]) {
		end--
	}
	if end == 0 {
		return 0, 0, false
	}
	begin := end
	for begin > 0 && !isBlank(s[begin-1]) {
		begin--
	}
	return begin, end, true
}

// cutRun removes s[start:end] along with the blanks a removal would otherwise
// leave behind, so the words around the hole keep exactly one separator. Only
// the blanks touching the cut are taken; the rest of the template, {prompt}
// included, is left byte-identical.
func cutRun(s string, start, end int) string {
	for start > 0 && isBlank(s[start-1]) {
		start--
	}
	if start == 0 {
		for end < len(s) && isBlank(s[end]) {
			end++
		}
	}
	return s[:start] + s[end:]
}

func isBlank(c byte) bool { return c == ' ' || c == '\t' }
