package agentconfig

// SetupOutcome holds only Sectile-owned asset digests and explicit decisions.
// MCP credentials and provider configuration never belong in this metadata.
type SetupOutcome struct {
	Status      string            `json:"status"`
	Fingerprint string            `json:"fingerprint,omitempty"`
	ProjectID   string            `json:"projectId,omitempty"`
	Root        string            `json:"root,omitempty"`
	Framework   string            `json:"framework,omitempty"`
	Assets      map[string]string `json:"assets,omitempty"`
}
