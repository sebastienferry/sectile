package db

import (
	"sync"
	"testing"
	"time"
)

// Two instances on one PostgreSQL database: an event one publishes reaches the
// other, never itself, and a cancellation one records stops the job the other
// runs.
func TestPostgresBusRelaysEventsAndCancellations(t *testing.T) {
	first := openPostgres(t)
	second, err := Open(Config{Driver: DriverPostgres, DSN: postgresDSN(t)})
	if err != nil {
		t.Fatalf("opening a second store: %v", err)
	}
	defer second.Close()

	var mu sync.Mutex
	var atFirst, atSecond []BusMessage
	first.OnRelayedEvent(func(m BusMessage) { mu.Lock(); atFirst = append(atFirst, m); mu.Unlock() })
	second.OnRelayedEvent(func(m BusMessage) { mu.Lock(); atSecond = append(atSecond, m); mu.Unlock() })

	stopFirst, err := first.StartEventBus()
	if err != nil {
		t.Fatal(err)
	}
	defer stopFirst()
	stopSecond, err := second.StartEventBus()
	if err != nil {
		t.Fatal(err)
	}
	defer stopSecond()

	// The listener connects in the background: publish until it hears.
	deadline := time.Now().Add(10 * time.Second)
	for {
		first.PublishEvent("task_updated", "t1", "a1", "")
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		heard := len(atSecond) > 0
		mu.Unlock()
		if heard {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the second instance never received the event")
		}
	}
	mu.Lock()
	got := atSecond[0]
	mu.Unlock()
	if got.Type != "task_updated" || got.TaskID != "t1" || got.ActivityID != "a1" || got.Origin != first.InstanceID() {
		t.Errorf("relayed message = %+v", got)
	}

	stopped := make(chan struct{})
	second.cancelMu.Lock()
	second.cancelMap["runs-on-second"] = func() { close(stopped) }
	second.cancelMu.Unlock()
	if err := first.CancelActivity("runs-on-second"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("the cancellation never reached the instance running the job")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(atFirst) != 0 {
		t.Errorf("the publishing instance received its own events: %+v", atFirst)
	}
}
