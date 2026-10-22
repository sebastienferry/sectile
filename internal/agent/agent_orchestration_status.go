package agent

import "strings"

// A failed run can retain its provider's sign-in diagnostic across agent
// restarts. Check the bounded local trace, including restored runs, without
// passing console contents to the renderer's discovery response.
func runNeedsAuthentication(run desktopRun, trace *runTrace) bool {
	if run.Status != "failed" || (run.Provider != "claude" && run.Provider != "codex") {
		return false
	}
	lines, _ := trace.snapshot()
	for _, line := range lines {
		line = strings.ToLower(stripTraceStyle(line))
		for _, diagnostic := range []string{"failed to authenticate:", "oauth session expired", "authentication token has expired", "not logged in", "please run /login", "please run `codex login`"} {
			if strings.Contains(line, diagnostic) {
				return true
			}
		}
	}
	return false
}
