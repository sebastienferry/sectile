package db

import (
	"errors"
	"strings"
	"time"
)

// A Jira consent (#654) is a round trip through Atlassian's screen. Its state
// travels in the browser, so only its hash is stored, with the web session and
// the person that started it: a callback is accepted only for the same
// session and the same person, once, within jiraOAuthFlowTTL, on whichever
// instance it lands.
//
// GitHub and GitLab consents (#804) are the same round trip and share the
// table, which keeps its Jira name. Each flow carries the tracker that started
// it, and only that tracker's callback consumes it: a state issued for one
// provider is worthless on another's callback.

// jiraOAuthFlowTTL bounds a consent left open.
const jiraOAuthFlowTTL = 10 * time.Minute

// ErrJiraOAuthFlow is a callback that matches no pending consent of this
// session and person: unknown, used, expired, started by somebody else, or
// for another tracker. The causes are deliberately not told apart.
var ErrJiraOAuthFlow = errors.New("unknown, used or expired Jira connection attempt")

// StartJiraOAuthFlow records one pending Jira consent and answers its state.
func (d *DB) StartJiraOAuthFlow(userID, sessionToken string) (string, error) {
	return d.StartOAuthFlow("jira", userID, sessionToken)
}

// ConsumeJiraOAuthFlow accepts a Jira callback's state for this session and
// person.
func (d *DB) ConsumeJiraOAuthFlow(state, userID, sessionToken string) error {
	return d.ConsumeOAuthFlow("jira", state, userID, sessionToken)
}

// StartOAuthFlow records one pending consent for tracker and answers its
// state.
func (d *DB) StartOAuthFlow(tracker, userID, sessionToken string) (string, error) {
	tracker = strings.ToLower(strings.TrimSpace(tracker))
	userID, sessionToken = strings.TrimSpace(userID), strings.TrimSpace(sessionToken)
	if tracker == "" || userID == "" || sessionToken == "" {
		return "", ErrJiraOAuthFlow
	}
	state, err := newSecret()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	// The attempts nobody finished go with each new one, so the table holds
	// a few rows at most.
	_, _ = d.conn.Exec(`DELETE FROM jira_oauth_flows WHERE expires_at < ? OR consumed_at IS NOT NULL`, now)
	if _, err := d.conn.Exec(`INSERT INTO jira_oauth_flows (state_hash, user_id, session_hash, created_at, expires_at, tracker) VALUES (?, ?, ?, ?, ?, ?)`,
		hashSecret(state), userID, hashSecret(sessionToken), now, now.Add(jiraOAuthFlowTTL), tracker); err != nil {
		return "", err
	}
	return state, nil
}

// ConsumeOAuthFlow accepts a callback's state for this tracker, session and
// person. It is one statement, so of two callbacks carrying the same state, on
// one instance or two, exactly one wins.
func (d *DB) ConsumeOAuthFlow(tracker, state, userID, sessionToken string) error {
	tracker = strings.ToLower(strings.TrimSpace(tracker))
	state, userID, sessionToken = strings.TrimSpace(state), strings.TrimSpace(userID), strings.TrimSpace(sessionToken)
	if tracker == "" || state == "" || userID == "" || sessionToken == "" {
		return ErrJiraOAuthFlow
	}
	now := time.Now().UTC()
	result, err := d.conn.Exec(`UPDATE jira_oauth_flows SET consumed_at = ?
		WHERE state_hash = ? AND tracker = ? AND user_id = ? AND session_hash = ? AND consumed_at IS NULL AND expires_at > ?`,
		now, hashSecret(state), tracker, userID, hashSecret(sessionToken), now)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return ErrJiraOAuthFlow
	}
	return nil
}
