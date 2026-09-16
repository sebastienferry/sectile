package agentconfig

import (
	"errors"
	"fmt"
)

// Mismatch reports a server that does not serve the agent contract this build
// speaks. It is deliberately distinct from a transport failure: the same call
// to the same server keeps producing the same answer, so no amount of
// reconnecting clears it. Only updating a build does, and the message has to
// say that rather than read like a dropped connection.
type Mismatch struct {
	// Server is the base URL that answered, without the contract route.
	Server string
	// Route is the contract route that revealed the mismatch.
	Route string
	// Status is the HTTP status that revealed it. Zero when the route answered
	// normally but carried a version this agent does not implement.
	Status int
	// Served is the schemaVersion the server sent, zero when it sent none.
	Served int
}

func (m *Mismatch) Error() string {
	if m.Status != 0 {
		return fmt.Sprintf("agent contract v%d is not served by %s: %s answered HTTP %d. The server runs a build older than this agent; update and restart sectile-server.",
			Version, m.Server, m.Route, m.Status)
	}
	return fmt.Sprintf("agent contract v%d is not served by %s: %s carried schemaVersion %d. The server and agent builds disagree; rebuild and restart both from the same revision.",
		Version, m.Server, m.Route, m.Served)
}

// IsMismatch reports whether err, or any error it wraps, is a contract
// mismatch. Callers use it to separate a build problem an operator must fix
// from a server that is merely unreachable for now.
func IsMismatch(err error) bool {
	var mismatch *Mismatch
	return errors.As(err, &mismatch)
}
