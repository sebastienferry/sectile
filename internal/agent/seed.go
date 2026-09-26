package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"tasks/internal/agentconfig"
	"tasks/internal/agenthttp"
)

// fetchSeed reads the execution values the server stored before #305. It does
// not go through readAPI: a server that predates the route answers 404, which
// is not a contract disagreement worth reporting to the desktop, only a seed
// to retry at the next connection.
func (d *agentDaemon) fetchSeed(ctx context.Context, projectID string) (agentconfig.Seed, error) {
	var seed agentconfig.Seed
	path := "/api/v1/agent/execution-seed"
	if projectID != "" {
		path += "?projectId=" + url.QueryEscape(projectID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.link.serverURL+path, nil)
	if err != nil {
		return seed, err
	}
	resp, err := agenthttp.Client(d.link.token).Do(req)
	if err != nil {
		return seed, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return seed, fmt.Errorf("execution seed: HTTP %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&seed); err != nil {
		return seed, err
	}
	if seed.SchemaVersion != agentconfig.Version {
		return seed, fmt.Errorf("execution seed: unsupported version %d", seed.SchemaVersion)
	}
	return seed, nil
}

// seedSettings copies the server's former execution values into the local
// file, once for the workstation defaults and once per project (US6). With an
// empty config it seeds the defaults only. A failure writes and marks nothing,
// so the next connection or resolution retries.
func (d *agentDaemon) seedSettings(ctx context.Context, config agentconfig.Config) {
	if d.link.serverURL == "" {
		return
	}
	root := d.localSettingsRoot()
	settings, err := agentconfig.ReadSettings(root)
	if err != nil {
		return
	}
	id := config.ProjectID
	project := id != "" && !settings.HasSeededProject(id) && !settings.DisconnectedProjects[id]
	if settings.HasSeededDefaults() && !project {
		return
	}
	query := ""
	if project {
		query = id
	}
	seed, err := d.fetchSeed(ctx, query)
	if err != nil {
		log.Printf("[Agent] Execution settings not seeded yet: %v", err)
		return
	}
	changed := false
	_, err = agentconfig.UpdateSettings(root, func(s *agentconfig.Settings) error {
		if !s.HasSeededDefaults() && seed.Defaults != nil {
			agentconfig.ApplyWorkstationSeed(s, *seed.Defaults, d.link.serverURL)
			changed = true
		}
		if project && !s.HasSeededProject(id) && seed.Project != nil && seed.Project.ProjectID == id {
			agentconfig.ApplyProjectSeed(s, config, *seed.Project, time.Now().UTC().Format(time.RFC3339))
			changed = true
		}
		if !changed {
			return errNothingToSeed
		}
		return nil
	})
	if err == errNothingToSeed {
		return
	}
	if err != nil {
		log.Printf("[Agent] Could not save the seeded execution settings: %v", err)
		return
	}
	log.Printf("[Agent] Execution settings seeded from %s", d.link.serverURL)
	d.reportCapabilitiesLater()
}

var errNothingToSeed = fmt.Errorf("nothing to seed")
