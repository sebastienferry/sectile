package agent

import "testing"

func TestIsolatedCompanionDoesNotRefreshMCPRegistrations(t *testing.T) {
	t.Setenv("SECTILE_DISABLE_MCP_REFRESH", "1")
	// No daemon configuration is needed: isolation must return before reading
	// provider settings or discovering existing global registrations.
	var daemon *agentDaemon
	if err := daemon.refreshMCPConnections(); err != nil {
		t.Fatal(err)
	}
}
