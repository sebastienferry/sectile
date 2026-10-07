package agent

import (
	"strings"
	"tasks/internal/testhome"
	"testing"
)

func TestDesktopCodexReviewerPersistsAndSurvivesOtherSettings(t *testing.T) {
	d, _ := disconnectFixture(t)
	if got := d.workstationCodexReviewer(); got != "user" {
		t.Fatal(got)
	}
	for _, body := range []string{`{"approvalsReviewer":"never"}`, `{"approvalsReviewer":""}`, `not json`} {
		if rec := disconnectRequest(d, "PUT", "/desktop/codex-settings", body); rec.Code != 400 {
			t.Fatalf("invalid settings accepted: %d", rec.Code)
		}
	}
	if rec := disconnectRequest(d, "PUT", "/desktop/codex-settings", `{"approvalsReviewer":"auto_review"}`); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	if rec := disconnectRequest(d, "PUT", "/desktop/workstation", `{"editorCommand":"vim"}`); rec.Code != 204 {
		t.Fatal(rec.Body.String())
	}
	if got := d.workstationCodexReviewer(); got != "auto_review" {
		t.Fatalf("reviewer erased: %s", got)
	}
	if rec := disconnectRequest(d, "GET", "/desktop/codex-settings", ""); !strings.Contains(rec.Body.String(), `"auto_review"`) {
		t.Fatal(rec.Body.String())
	}
	if rec := disconnectRequest(d, "PUT", "/desktop/codex-settings", `{"approvalsReviewer":"user"}`); rec.Code != 200 || d.workstationCodexReviewer() != "user" {
		t.Fatal(rec.Body.String())
	}
}

func TestCodexReviewerAppliedToNextTurn(t *testing.T) {
	testhome.Temp(t)
	d, id, log := codexConversationFixture(t)
	if rec := disconnectRequest(d, "PUT", "/desktop/codex-settings", `{"approvalsReviewer":"auto_review"}`); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	codexMessage(t, d, id, `{"message":"hello"}`)
	waitCodex(t, d, id, func(c *providerConversation) bool { return !c.busy })
	if data := codexLog(t, log); !strings.Contains(data, `"approvalsReviewer":"auto_review"`) {
		t.Fatalf("native request did not carry reviewer: %s", data)
	}
}
