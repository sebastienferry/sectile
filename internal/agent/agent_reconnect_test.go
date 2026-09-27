package agent

import (
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestConnectionAttemptsResetAfterAnEstablishedSession(t *testing.T) {
	attempt := 0
	for i := 0; i < 8; i++ {
		attempt = nextConnectionAttempt(attempt, false)
	}
	if attempt != 8 || connectionBackoff(attempt) != time.Minute {
		t.Fatalf("consecutive failures: attempt=%d backoff=%s", attempt, connectionBackoff(attempt))
	}
	attempt = nextConnectionAttempt(attempt, true)
	if attempt != 1 || connectionBackoff(attempt) != 2*time.Second {
		t.Fatalf("established session inherited failure history: attempt=%d backoff=%s", attempt, connectionBackoff(attempt))
	}
	attempt = nextConnectionAttempt(attempt, false)
	if attempt != 2 || connectionBackoff(attempt) != 4*time.Second {
		t.Fatalf("consecutive failed dial did not increase delay: attempt=%d backoff=%s", attempt, connectionBackoff(attempt))
	}
}

func TestReceivedServerCloseExcludesSyntheticAbnormalClosure(t *testing.T) {
	abnormal := fmt.Errorf("read error: %w", &websocket.CloseError{Code: websocket.CloseAbnormalClosure, Text: "unexpected EOF"})
	if got := receivedServerClose(abnormal); got != nil {
		t.Fatalf("synthetic closure reported as a server close: %+v", got)
	}
	explicit := fmt.Errorf("read error: %w", &websocket.CloseError{Code: 4002, Text: "Agent silent for 45s"})
	if got := receivedServerClose(explicit); got == nil || got.Code != 4002 || got.Text != "Agent silent for 45s" {
		t.Fatalf("explicit close reason lost: %+v", got)
	}
}
