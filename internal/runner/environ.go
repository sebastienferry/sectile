package runner

import (
	"os"
	"strings"
)

// serverOnlySecrets are the variables the server may hold and no process it
// starts has any use for. The key encrypting personal tracker credentials is
// the one that matters: on a personal deployment the server, the agent and the
// user's own embedded terminal share one environment, so a skill prompt — which
// can come from a ticket body somebody else wrote — could print it, and with
// the database on the same host that yields every unsealed token in clear.
//
// The name is written out rather than imported: internal/runner is agent-side
// and the agent has no business linking the server's crypto. environ_test.go
// pins it against secrets.KeyEnvVar so the two cannot drift.
var serverOnlySecrets = []string{"SECTILE_SECRET_KEY"}

// SanitizedEnviron is os.Environ() without what a child process must not see.
func SanitizedEnviron() []string {
	env := os.Environ()
	out := env[:0:0]
	for _, entry := range env {
		name := entry
		if i := strings.IndexByte(entry, '='); i >= 0 {
			name = entry[:i]
		}
		if hidden(name) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func hidden(name string) bool {
	for _, secret := range serverOnlySecrets {
		if name == secret {
			return true
		}
	}
	return false
}
