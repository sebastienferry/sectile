package companion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"tasks/internal/storelock"
)

var ErrUpdateBusy = errors.New("Local update requires stopped executions and preparations")

// PrepareUpdate stops an idle, explicitly identified Local companion and proves
// its transports and store ownership are released. It never opens the database,
// signals a process, or discovers an installation through the user's home.
// The desktop caller holds its installation/startup lock throughout this call.
func PrepareUpdate(ctx context.Context, root string) error {
	if !filepath.IsAbs(root) {
		return errors.New("update requires an absolute Local root")
	}
	if err := realDirectory(root); err != nil {
		return err
	}
	installation, err := readInstallation(root)
	if err != nil {
		return errors.New("Local installation unavailable")
	}
	lockInfo, err := os.Lstat(filepath.Join(root, ".runtime.lock"))
	if err != nil || !lockInfo.Mode().IsRegular() {
		return errors.New("Local lifetime lock unavailable")
	}
	var discovery struct{ URL, Token string }
	raw, err := os.ReadFile(filepath.Join(root, "agent-connection.json"))
	if err != nil || json.Unmarshal(raw, &discovery) != nil || discovery.Token == "" {
		return errors.New("Local agent discovery unavailable")
	}
	endpoint, err := url.Parse(discovery.URL)
	if err != nil || endpoint.Scheme != "http" || endpoint.Hostname() != "127.0.0.1" || endpoint.Port() == "" ||
		(endpoint.Path != "" && endpoint.Path != "/") || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return errors.New("invalid Local agent origin")
	}
	origin := fmt.Sprintf("http://127.0.0.1:%d", installation.Port)
	agentOrigin := "http://" + endpoint.Host
	client := &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request := func(method, address, token string, target any) (int, error) {
		req, err := http.NewRequestWithContext(ctx, method, address, nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(req)
		if err != nil {
			return 0, err
		}
		defer response.Body.Close()
		if response.StatusCode == http.StatusOK && target != nil {
			err = json.NewDecoder(response.Body).Decode(target)
		}
		return response.StatusCode, err
	}
	var proof struct {
		Protocol         int
		Companion, Ready bool
	}
	status, probeErr := request(http.MethodGet, origin+"/api/local-runtime", installation.Key, &proof)
	if probeErr == nil {
		if status != http.StatusOK || proof.Protocol != 1 || !proof.Companion || !proof.Ready {
			return errors.New("Local companion authentication refused")
		}
		var agent struct{ Server string }
		status, err = request(http.MethodGet, agentOrigin+"/desktop/status", discovery.Token, &agent)
		if err != nil || status != http.StatusOK || agent.Server != origin {
			return errors.New("Local agent identity mismatch")
		}
		status, err = request(http.MethodPost, agentOrigin+"/desktop/shutdown", discovery.Token, nil)
		if err != nil {
			return errors.New("Local shutdown not acknowledged")
		}
		if status == http.StatusConflict {
			return ErrUpdateBusy
		}
		if status != http.StatusNoContent {
			return errors.New("Local shutdown refused")
		}
	} else if !errors.Is(probeErr, syscall.ECONNREFUSED) {
		return errors.New("Local companion connection unconfirmed")
	}
	// A retry can arrive after shutdown was acknowledged but before the first
	// caller observed closure. Only connection refusal is an offline proof.
	closed := func(address string) bool {
		connection, err := (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "tcp", address)
		if err == nil {
			connection.Close()
			return false
		}
		return errors.Is(err, syscall.ECONNREFUSED)
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if closed(fmt.Sprintf("127.0.0.1:%d", installation.Port)) && closed(endpoint.Host) {
			ownership, err := storelock.Acquire(root)
			if err == nil {
				return ownership.Close()
			}
			if !errors.Is(err, storelock.ErrOwned) {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return errors.New("Local shutdown or store release incomplete")
		case <-ticker.C:
		}
	}
}
