package db

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

// Several server instances may share one PostgreSQL database, and each one
// only knows what went through it: a browser streaming from one instance would
// never see a change made through another, and a cancellation would only stop
// a job that happens to run where it was asked. The bus relays both over
// PostgreSQL LISTEN/NOTIFY. A message only names things, never carries them:
// the receiver reads the task and the activity back from the database, which
// keeps every message far below the 8000-byte limit NOTIFY puts on a payload.
//
// SQLite serves one process, so there is nobody to relay to and the bus does
// nothing there.

// busChannel is the one channel every instance listens on.
const busChannel = "sectile_events"

const (
	busKindEvent  = "event"
	busKindCancel = "cancel"
)

// busErrorMax bounds the error text a relayed event carries.
const busErrorMax = 1000

// The listener's reconnection delay, doubled on each failure up to the maximum.
var (
	busRetryMin = time.Second
	busRetryMax = 30 * time.Second
)

// BusMessage is what one instance tells the others.
type BusMessage struct {
	// Origin is the instance that published it; that instance ignores it.
	Origin string `json:"o"`
	// Kind is "event" for a live update, "cancel" for a cancellation.
	Kind string `json:"k"`
	// Type is the event type browsers receive, for an event.
	Type       string `json:"t,omitempty"`
	TaskID     string `json:"task,omitempty"`
	ActivityID string `json:"act,omitempty"`
	Error      string `json:"err,omitempty"`
}

// PublishEvent tells the other instances about a live update this one just
// delivered to its own browsers.
func (d *DB) PublishEvent(eventType, taskID, activityID, errText string) {
	d.publish(BusMessage{Kind: busKindEvent, Type: eventType, TaskID: taskID, ActivityID: activityID, Error: truncateRunes(errText, busErrorMax)})
}

// OnRelayedEvent registers what to do with an event another instance published.
func (d *DB) OnRelayedEvent(fn func(BusMessage)) {
	d.relayedMu.Lock()
	defer d.relayedMu.Unlock()
	d.relayed = append(d.relayed, fn)
}

// publish sends a message to the other instances. It never fails its caller: a
// relay that cannot be sent costs a stale board elsewhere, which the next
// refresh repairs, and must not turn the change itself into an error.
func (d *DB) publish(msg BusMessage) {
	if d.dialect.ServesOneProcess() {
		return
	}
	msg.Origin = d.instanceID
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	if _, err := d.conn.Exec(`SELECT pg_notify(?, ?)`, busChannel, string(payload)); err != nil {
		log.Printf("⚠️  Relais d'un événement aux autres instances : %v", err)
	}
}

// handleBusMessage acts on one message received from the channel.
func (d *DB) handleBusMessage(payload string) {
	var msg BusMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil || msg.Origin == "" || msg.Origin == d.instanceID {
		return
	}
	switch msg.Kind {
	case busKindCancel:
		if msg.ActivityID != "" {
			d.cancelLocal(msg.ActivityID)
		}
	case busKindEvent:
		d.relayedMu.RLock()
		listeners := append([]func(BusMessage){}, d.relayed...)
		d.relayedMu.RUnlock()
		for _, fn := range listeners {
			fn(msg)
		}
	}
}

// StartEventBus listens for what the other instances publish, until stop is
// called. Only a serving process calls it; publishing does not need it.
func (d *DB) StartEventBus() (stop func(), err error) {
	if d.dialect.ServesOneProcess() {
		return func() {}, nil
	}
	pg, ok := d.dialect.(postgresDialect)
	if !ok {
		return nil, errors.New("event bus: no listener for this engine")
	}
	config, err := pg.connConfig(d.cfg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		retry := busRetryMin
		for ctx.Err() == nil {
			connected, err := d.listen(ctx, config)
			if ctx.Err() != nil {
				return
			}
			// A connection that held resets the delay: a drop after days of
			// listening is not the tenth failure in a row.
			if connected {
				retry = busRetryMin
			}
			log.Printf("⚠️  Écoute des autres instances interrompue (%v) ; reprise dans %s", err, retry)
			select {
			case <-ctx.Done():
				return
			case <-time.After(retry):
			}
			retry = min(retry*2, busRetryMax)
		}
	}()
	return func() {
		cancel()
		<-finished
	}, nil
}

// listen holds one listening connection until it fails or ctx ends, and
// reports whether it got as far as listening.
func (d *DB) listen(ctx context.Context, config *pgx.ConnConfig) (bool, error) {
	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		return false, err
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "LISTEN "+busChannel); err != nil {
		return false, err
	}
	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return true, err
		}
		d.handleBusMessage(notification.Payload)
	}
}

// truncateRunes cuts s to at most max characters without splitting one.
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max])
}
