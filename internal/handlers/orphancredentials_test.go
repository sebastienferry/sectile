package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tasks/internal/db"
)

// memberSession signs a second account in. The first account ever created takes
// the admin role, so anyone after it is a member.
func memberSession(t *testing.T, h *Handler, email string) *http.Cookie {
	t.Helper()
	user, err := h.db.SignInLocal(email)
	if err != nil {
		t.Fatal(err)
	}
	if user.Role != db.RoleMember {
		t.Fatalf("%s should be a member, got %q", email, user.Role)
	}
	token, _, err := h.db.CreateWebSession(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: sessionCookie, Value: token}
}

func credentialsBody(t *testing.T, h *Handler, session *http.Cookie) map[string]interface{} {
	t.Helper()
	w := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(w, signedRequest(session, http.MethodGet, "/api/me/tracker-credentials", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

// The leftover is reported to everyone, because it is the answer to "my tracker
// says to configure an e-mail I did configure". Who it belongs to is not.
func TestALeftoverCredentialIsReportedToEveryoneAndDetailedToAdmins(t *testing.T) {
	h, admin := credentialHandler(t)
	member := memberSession(t, h, "bob@example.com")

	// A token stored while `default` was an acting identity, on a deployment
	// that never wrote a `default` row.
	if err := h.db.SetUserTrackerCredential(db.ImplicitUserID, "jira", "https://acme.atlassian.net", "ada@example.com", "ATATT-legacy", ""); err != nil {
		t.Fatal(err)
	}

	seenByMember := credentialsBody(t, h, member)
	if seenByMember["orphanedCount"] != float64(1) {
		t.Fatalf("a member must be told one exists: %v", seenByMember)
	}
	if _, detailed := seenByMember["orphaned"]; detailed {
		t.Fatalf("a member must not be told whose it is: %v", seenByMember)
	}
	trackers, _ := seenByMember["orphanedTrackers"].([]interface{})
	if len(trackers) != 1 || trackers[0] != "jira" {
		t.Fatalf("the tracker it occupies is what makes it actionable: %v", seenByMember)
	}

	seenByAdmin := credentialsBody(t, h, admin)
	orphaned, ok := seenByAdmin["orphaned"].([]interface{})
	if !ok || len(orphaned) != 1 {
		t.Fatalf("an admin sees the row: %v", seenByAdmin)
	}
	row, _ := orphaned[0].(map[string]interface{})
	if row["userId"] != db.ImplicitUserID || row["email"] != "ada@example.com" {
		t.Fatalf("with what it takes to decide: %v", row)
	}
	// Whoever reads it, the token never travels.
	w := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(w, signedRequest(admin, http.MethodGet, "/api/me/tracker-credentials", nil))
	if strings.Contains(w.Body.String(), "ATATT-legacy") {
		t.Fatal("the answer must not carry the token")
	}
}

func TestDiscardingALeftoverIsAnAdminsAct(t *testing.T) {
	h, admin := credentialHandler(t)
	member := memberSession(t, h, "bob@example.com")
	if err := h.db.SetUserTrackerCredential(db.ImplicitUserID, "jira", "https://acme.atlassian.net", "ada@example.com", "ATATT-legacy", ""); err != nil {
		t.Fatal(err)
	}

	target := "/api/me/tracker-credentials/orphaned?userId=" + db.ImplicitUserID + "&tracker=jira"

	refused := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(refused, signedRequest(member, http.MethodDelete, target, nil))
	if refused.Code != http.StatusForbidden {
		t.Fatalf("a member must be refused: %d %s", refused.Code, refused.Body)
	}
	if orphans, err := h.db.OrphanedTrackerCredentials(); err != nil || len(orphans) != 1 {
		t.Fatalf("and must have changed nothing: %v %v", orphans, err)
	}

	accepted := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(accepted, signedRequest(admin, http.MethodDelete, target, nil))
	if accepted.Code != http.StatusOK {
		t.Fatalf("an admin may discard it: %d %s", accepted.Code, accepted.Body)
	}
	if orphans, err := h.db.OrphanedTrackerCredentials(); err != nil || len(orphans) != 0 {
		t.Fatalf("and it is gone: %v %v", orphans, err)
	}
	// Nothing left to report, so the banner disappears on the same answer.
	var body map[string]interface{}
	if err := json.Unmarshal(accepted.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if _, present := body["orphanedCount"]; present {
		t.Fatalf("the answer still announces a leftover: %v", body)
	}
}

// The route is a cleanup for leftovers, not a way to delete a colleague's token
// by naming their id.
func TestTheDiscardRouteCannotReachALiveAccountsCredential(t *testing.T) {
	h, admin := credentialHandler(t)
	bob, err := h.db.SignInLocal("bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.db.SetUserTrackerCredential(bob.ID, "jira", "https://acme.atlassian.net", "bob@example.com", "ATATT-bob", ""); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.HandleUserTrackerCredentials(w, signedRequest(admin, http.MethodDelete, "/api/me/tracker-credentials/orphaned?userId="+bob.ID+"&tracker=jira", nil))
	if w.Code != http.StatusConflict {
		t.Fatalf("expected a refusal, got %d %s", w.Code, w.Body)
	}
	if _, _, token, err := h.db.UserTrackerCredentialsFor(bob.ID, "jira"); err != nil || token != "ATATT-bob" {
		t.Fatalf("bob keeps his token: %q %v", token, err)
	}
}
