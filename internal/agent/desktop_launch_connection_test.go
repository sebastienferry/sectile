package agent

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTrackedConversationBindsItsAcceptedRunInEitherDispatchOrder(t *testing.T) {
	const selected = "11111111-1111-4111-8111-111111111111"
	const other = "22222222-2222-4222-8222-222222222222"
	for _, before := range []bool{true, false} {
		t.Run(map[bool]string{true: "response-first", false: "dispatch-first"}[before], func(t *testing.T) {
			d, id := conversationFixture(t)
			run := d.queue.runs[id]
			run.desktop.Conversation = false
			entry := desktopRun{ID: id, ProjectID: "project", TaskID: "ticket"}
			if before {
				d.bindDesktopLaunchConnection(id, "project", "ticket", selected)
			}
			d.startLaunchConversation(run, entry, "", "", "", nil, nil, "claude")
			if !before {
				d.bindDesktopLaunchConnection(id, "project", "ticket", selected)
			}
			got := conversationRequestForConnection(d, "GET", "/desktop/conversation?id="+id, "", "private", selected)
			if got.Code != http.StatusOK || !strings.Contains(got.Body.String(), `"readOnly":false`) {
				t.Fatalf("owner cannot control tracked conversation: %d %s", got.Code, got.Body.String())
			}
			if got := conversationRequestForConnection(d, "GET", "/desktop/conversation?id="+id, "", "private", other); got.Code != http.StatusNotFound {
				t.Fatalf("other connection read tracked conversation: %d", got.Code)
			}
			if got := conversationRequest(d, "GET", "/desktop/conversation?id="+id, "", "private"); got.Code != http.StatusOK {
				t.Fatalf("legacy client lost access: %d", got.Code)
			}
			if len(d.desktopLaunchConnections) != 0 {
				t.Fatal("accepted run binding was not consumed")
			}
		})
	}
}

func TestDesktopLaunchConnectionCannotBindAnotherRunProjectTaskOrExpiredDispatch(t *testing.T) {
	for _, kind := range []string{"run", "project", "task", "expired"} {
		t.Run(kind, func(t *testing.T) {
			d, id := conversationFixture(t)
			d.queue.runs[id].desktop.Conversation = false
			d.bindDesktopLaunchConnection(id, "project", "ticket", "connection")
			entry := desktopRun{ID: id, ProjectID: "project", TaskID: "ticket"}
			switch kind {
			case "run":
				entry.ID = "another"
			case "project":
				entry.ProjectID = "another"
			case "task":
				entry.TaskID = "another"
			case "expired":
				value := d.desktopLaunchConnections[id]
				value.at = time.Now().Add(-conversationDiscussionTTL)
				d.desktopLaunchConnections[id] = value
			}
			if got := d.takeDesktopLaunchConnectionLocked(entry); got != "" {
				t.Fatalf("mismatched dispatch inherited connection: %q", got)
			}
		})
	}
}
