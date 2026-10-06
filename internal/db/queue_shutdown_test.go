package db

import (
	"sync"
	"testing"
	"time"
)

func TestCloseStopsQueueWorker(t *testing.T) {
	d := testDB(t)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-d.queueWorkerDone:
	default:
		t.Fatal("closing the database leaves its queue worker running")
	}
	// Closed stores cannot create blocked senders or inflate the drain count.
	d.enqueueJob(SkillJob{})
	if got := d.jobs.running.Load(); got != 0 {
		t.Fatalf("closed queue admitted work: %d", got)
	}
}

func TestConcurrentCloseWaitsForTheSameShutdown(t *testing.T) {
	d := testDB(t)
	var callers sync.WaitGroup
	for range 8 {
		callers.Go(func() {
			if err := d.Close(); err != nil {
				t.Errorf("concurrent close: %v", err)
			}
			select {
			case <-d.queueWorkerDone:
			default:
				t.Error("Close returned before the queue worker stopped")
			}
		})
	}
	callers.Wait()
}

func TestShutdownFlushesAdmittedQueueSenders(t *testing.T) {
	// Keep the consumer stopped so the second admitted send must wait in the
	// overflow goroutine. It must be delivered before shutdown closes the queue.
	d := &DB{jobQueue: make(chan SkillJob, 1)}
	d.enqueueJob(SkillJob{ActivityID: "first"})
	d.enqueueJob(SkillJob{ActivityID: "second"})
	d.queueMu.Lock()
	d.queueClosed = true
	d.queueMu.Unlock()
	finished := make(chan struct{})
	go func() { d.queueSenders.Wait(); close(d.jobQueue); close(finished) }()
	for _, want := range []string{"first", "second"} {
		select {
		case job := <-d.jobQueue:
			if job.ActivityID != want {
				t.Fatalf("received %q, want %q", job.ActivityID, want)
			}
			d.jobs.end()
		case <-time.After(time.Second):
			t.Fatal("an admitted sender was lost")
		}
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish after senders drained")
	}
	if _, open := <-d.jobQueue; open {
		t.Fatal("queue remains open")
	}
	if d.jobs.running.Load() != 0 {
		t.Fatal("queue accounting did not drain")
	}
}
