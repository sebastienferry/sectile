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

// jiraOAuthFlowTTL bounds a consent left open.
const jiraOAuthFlowTTL = 10 * time.Minute

// ErrJiraOAuthFlow is a callback that matches no pending consent of this
// session and person: unknown, used, expired, or started by somebody else.
// The causes are deliberately not told apart.
var ErrJiraOAuthFlow = errors.New("unknown, used or expired Jira connection attempt")

// StartJiraOAuthFlow records one pending consent and answers its state.
func (d *DB) StartJiraOAuthFlow(userID, sessionToken string) (string, error) {
	userID, sessionToken = strings.TrimSpace(userID), strings.TrimSpace(sessionToken)
	if userID == "" || sessionToken == "" {
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
	if _, err := d.conn.Exec(`INSERT INTO jira_oauth_flows (state_hash, user_id, session_hash, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		hashSecret(state), userID, hashSecret(sessionToken), now, now.Add(jiraOAuthFlowTTL)); err != nil {
		return "", err
	}
	return state, nil
}

// ConsumeJiraOAuthFlow accepts a callback's state for this session and person.
// It is one statement, so of two callbacks carrying the same state, on one
// instance or two, exactly one wins.
func (d *DB) ConsumeJiraOAuthFlow(state, userID, sessionToken string) error {
	state, userID, sessionToken = strings.TrimSpace(state), strings.TrimSpace(userID), strings.TrimSpace(sessionToken)
	if state == "" || userID == "" || sessionToken == "" {
		return ErrJiraOAuthFlow
	}
	now := time.Now().UTC()
	result, err := d.conn.Exec(`UPDATE jira_oauth_flows SET consumed_at = ?
		WHERE state_hash = ? AND user_id = ? AND session_hash = ? AND consumed_at IS NULL AND expires_at > ?`,
		now, hashSecret(state), userID, hashSecret(sessionToken), now)
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return ErrJiraOAuthFlow
	}
	return nil
}
