package runner

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// externalTerminalScript renders values as literal shell data. In particular,
// displaying the initial command must not execute its substitutions a second time.
func externalTerminalScript(targetPath, initialCommand string, env map[string]string) (string, error) {
	var b strings.Builder
	b.WriteString("#!/bin/bash\n# Sectile external terminal session\nrm -- \"$0\"\n")
	if customPath := GetDynamicCustomPath(); customPath != "" {
		fmt.Fprintf(&b, "export PATH=%s:\"$PATH\"\n", shellQuote(customPath))
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(key) {
			return "", fmt.Errorf("invalid terminal environment key %q", key)
		}
		fmt.Fprintf(&b, "export %s=%s\n", key, shellQuote(env[key]))
	}
	fmt.Fprintf(&b, "cd %s || exit 1\n", shellQuote(targetPath))
	fmt.Fprintf(&b, "printf '%%s\\n' %s\n", shellQuote("Sectile external terminal — "+targetPath))
	if initialCommand = strings.TrimSpace(initialCommand); initialCommand != "" {
		fmt.Fprintf(&b, "printf '%%s\\n' %s\n%s\n", shellQuote("Running: "+initialCommand), initialCommand)
	}
	b.WriteString("exec \"${SHELL:-/bin/zsh}\" -l\n")
	return b.String(), nil
}
