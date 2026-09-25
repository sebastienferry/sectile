package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/agenthttp"
	"tasks/internal/models"
)

// Capability is what this workstation will run for one project: the engine a
// web launch announces before the run, and the models it may pick (US5).
type Capability struct {
	ProjectID   string            `json:"projectId"`
	Provider    string            `json:"provider"`
	Model       string            `json:"model"`
	SkillModels map[string]string `json:"skillModels"`
	Models      []string          `json:"models"`
	ModelSlot   bool              `json:"modelSlot"`
	Headless    bool              `json:"headless"`
}

// CapabilityReport is the body of PUT /api/v1/agent/capabilities.
type CapabilityReport struct {
	SchemaVersion int          `json:"schemaVersion"`
	Projects      []Capability `json:"projects"`
}

// capabilityOf describes a resolved configuration. A model is only reported
// when it reaches the command line: a template without a {model} slot, or a
// provider without a model flag, runs the CLI's own default.
func capabilityOf(c agentconfig.Config, defaults agentconfig.Defaults) Capability {
	template := c.AICommandTemplate
	capability := Capability{
		ProjectID:   c.ProjectID,
		Provider:    c.AIProvider,
		Model:       agentconfig.EffectiveModel(c.AIProvider, template, c.AIModel),
		SkillModels: map[string]string{},
		Models:      agentconfig.ProviderModels(defaults, c.AIProvider),
		ModelSlot:   agentconfig.EffectiveModel(c.AIProvider, template, "model") != "",
		Headless:    models.SupportsAutonomousRun(c.AIProvider, c.AICommandTemplate, c.AICommandTemplateAutonomous),
	}
	for skill, model := range c.AISkillModels {
		if effective := agentconfig.EffectiveModel(c.AIProvider, template, model); effective != "" {
			capability.SkillModels[skill] = effective
		}
	}
	if !capability.ModelSlot {
		capability.Models = []string{}
	}
	if capability.Models == nil {
		capability.Models = []string{}
	}
	return capability
}

// capabilityReporter serializes the reports: a save made while one is being
// sent asks for one more, never for a second concurrent one.
type capabilityReporter struct {
	mu      sync.Mutex
	running bool
	again   bool
}

// reportCapabilitiesLater sends the report in the background, as postRunEngine
// does: a failure is logged and the next connection or save reports again.
func (d *agentDaemon) reportCapabilitiesLater() {
	if d.link.serverURL == "" {
		return
	}
	r := &d.capabilities
	r.mu.Lock()
	if r.running {
		r.again = true
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()
	go func() {
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := d.reportCapabilities(ctx); err != nil {
				log.Printf("[Agent] Capability report not sent: %v", err)
			}
			cancel()
			r.mu.Lock()
			if !r.again {
				r.running = false
				r.mu.Unlock()
				return
			}
			r.again = false
			r.mu.Unlock()
		}
	}()
}

// reportCapabilities tells the server, for every project this workstation
// serves, the engine it will run.
func (d *agentDaemon) reportCapabilities(ctx context.Context) error {
	projects, err := d.discoverProjects(ctx)
	if err != nil {
		return err
	}
	report := CapabilityReport{SchemaVersion: agentconfig.Version, Projects: []Capability{}}
	for _, p := range projects.Projects {
		stub := agentconfig.Config{ProjectID: p.ID, GitRemoteURL: p.GitRemoteURL}
		_, settings, err := d.localProjectRoot(ctx, stub)
		if err != nil {
			continue
		}
		report.Projects = append(report.Projects, capabilityOf(agentconfig.Resolve(stub, settings), settings.Defaults))
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, d.link.serverURL+"/api/v1/agent/capabilities", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := agenthttp.Client(d.link.token).Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return &httpStatusError{status: resp.StatusCode}
	}
	return nil
}

type httpStatusError struct{ status int }

func (e *httpStatusError) Error() string { return "HTTP " + http.StatusText(e.status) }
