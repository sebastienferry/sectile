package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"tasks/internal/agenthttp"
)

// apiKeyExpiryWarning is how far ahead the agent starts telling its owner that
// the key is about to run out. Ten days covers a holiday.
const apiKeyExpiryWarning = 10 * 24 * time.Hour

// errAPIKeyExpired is the refusal the server gives a key past its expiry. It is
// matched by the reconnection loop, which then stops blaming the network.
var errAPIKeyExpired = errors.New("the server refused the API key: API key expired")

// agentIdentity is what the server says about the key it was shown.
type agentIdentity struct {
	UserID      string     `json:"userId"`
	DeviceID    string     `json:"deviceId"`
	Label       string     `json:"label"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	SharedToken bool       `json:"sharedToken"`
}

// checkIdentity asks the server who this key names before connecting, and
// warns when the key is about to expire. A server that does not serve the
// endpoint yet is not an error here: the handshake will judge the key.
func (d *agentDaemon) checkIdentity(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.link.serverURL+"/api/v1/agent/identity", nil)
	if err != nil {
		return err
	}
	resp, err := agenthttp.Client(d.link.token).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusUnauthorized {
		var detail struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &detail)
		if strings.Contains(detail.Error, "expired") {
			return errAPIKeyExpired
		}
		return fmt.Errorf("the server refused the API key: %s", detail.Error)
	}
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var identity agentIdentity
	if err := json.Unmarshal(raw, &identity); err != nil {
		return nil
	}
	if identity.SharedToken {
		log.Printf("[Agent] This agent authenticates with the deprecated shared server token. Create an API key from the web profile before the next release.")
	}
	if line := expiryNotice(identity.ExpiresAt, time.Now()); line != "" {
		log.Printf("[Agent] %s", line)
	}
	return nil
}

// expiryNotice is the warning for a key that runs out within the window, empty
// otherwise. Days are rounded up: "1 day" for anything under 24 hours left.
func expiryNotice(expiresAt *time.Time, now time.Time) string {
	if expiresAt == nil {
		return ""
	}
	remaining := expiresAt.Sub(now)
	if remaining > apiKeyExpiryWarning {
		return ""
	}
	days := int((remaining + 24*time.Hour - time.Nanosecond) / (24 * time.Hour))
	if days < 1 {
		days = 1
	}
	unit := "days"
	if days == 1 {
		unit = "day"
	}
	return fmt.Sprintf("API key expires in %d %s (%s). Renew it from the web profile to keep this workstation connected.",
		days, unit, expiresAt.Local().Format("2006-01-02"))
}
