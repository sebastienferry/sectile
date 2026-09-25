package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"tasks/internal/secrets"
)

// A key derived from a sealing passphrase lives in the memory of the server
// instance that received the passphrase. When several instances share the
// database behind a load balancer, the owner's next tracker call may reach
// another one, which would read the credential as locked. So an instance tells
// the others when it comes to hold a key or forgets one, and a starting
// instance asks the others for the keys they hold (#409).
//
// Nothing here is authoritative. What decides whether a held key may be used
// is the credential's unlock generation, in the database: a lock or a new
// record moves it on, and a copy held from before stops opening anything, on
// an instance that missed the message as much as on one that heard it. The
// messages only let an instance use a key sooner, or forget one sooner.
//
// A key crosses between instances wrapped under the server key and bound to
// its owner, and is written nowhere.

// SharedKey is a held key as it crosses between instances: wrapped, with the
// unlock generation it belongs to.
type SharedKey struct {
	UserID     string `json:"userId"`
	Tracker    string `json:"tracker"`
	Generation int64  `json:"generation"`
	Wrapped    []byte `json:"wrapped"`
}

// UnlockedKeyRelay hears of the keys this instance comes to hold or forgets.
// Its methods are called with the store's lock possibly held, so they must
// return at once and never call back into the store.
type UnlockedKeyRelay interface {
	KeyHeld(SharedKey)
	KeyForgotten(userID, tracker string)
}

// ErrStaleKey means a key from another instance belongs to an unlock the
// credential has moved past since: a lock or a new record.
var ErrStaleKey = errors.New("this key belongs to an earlier unlock of the credential")

// SetUnlockedKeyRelay makes the store tell relay of every key it comes to hold
// or forgets. nil stops it.
func (d *DB) SetUnlockedKeyRelay(relay UnlockedKeyRelay) {
	if relay == nil {
		d.keyRelay.Store(nil)
		return
	}
	d.keyRelay.Store(&relay)
}

func (d *DB) relayHeld(userID, tracker string, key secrets.Key, generation int64) {
	relay := d.keyRelay.Load()
	if relay == nil {
		return
	}
	shared, err := d.shareKey(userID, tracker, key, generation)
	if err != nil {
		log.Printf("⚠️  Clé de %s pour %s non partagée avec les autres instances : %v", userID, tracker, err)
		return
	}
	(*relay).KeyHeld(shared)
}

func (d *DB) relayForgotten(userID, tracker string) {
	if relay := d.keyRelay.Load(); relay != nil {
		(*relay).KeyForgotten(userID, tracker)
	}
}

func (d *DB) shareKey(userID, tracker string, key secrets.Key, generation int64) (SharedKey, error) {
	if d.serverKeyErr != nil {
		return SharedKey{}, fmt.Errorf("no server key to wrap it under: %w", d.serverKeyErr)
	}
	wrapped, err := secrets.WrapKey(d.serverKey, secrets.Binding{UserID: userID, Tracker: tracker}, key)
	if err != nil {
		return SharedKey{}, err
	}
	return SharedKey{UserID: userID, Tracker: tracker, Generation: generation, Wrapped: wrapped}, nil
}

// HeldKeys returns every key this instance holds, wrapped, for an instance
// that starts and asks for them.
func (d *DB) HeldKeys() ([]SharedKey, error) {
	if d.serverKeyErr != nil {
		return nil, fmt.Errorf("no server key to wrap the held keys under: %w", d.serverKeyErr)
	}
	d.unlocked.mu.RLock()
	held := make(map[string]heldKey, len(d.unlocked.keys))
	for name, key := range d.unlocked.keys {
		held[name] = key
	}
	d.unlocked.mu.RUnlock()

	out := []SharedKey{}
	for name, key := range held {
		userID, tracker, ok := strings.Cut(name, "\x00")
		if !ok {
			continue
		}
		shared, err := d.shareKey(userID, tracker, key.key, key.generation)
		if err != nil {
			return nil, err
		}
		out = append(out, shared)
	}
	return out, nil
}

// AdoptSharedKey keeps a key another instance holds, once it has proved to
// open the credential as it stands: sealed, at the same unlock generation, and
// with this very key. Anything else is refused and nothing is kept, so a stale
// or forged key never becomes usable here.
func (d *DB) AdoptSharedKey(shared SharedKey) error {
	userID := strings.TrimSpace(shared.UserID)
	tracker := strings.ToLower(strings.TrimSpace(shared.Tracker))
	if userID == "" || tracker == "" {
		return fmt.Errorf("a shared key must name its owner and its tracker")
	}
	if d.serverKeyErr != nil {
		return fmt.Errorf("no server key to unwrap it with: %w", d.serverKeyErr)
	}
	binding := secrets.Binding{UserID: userID, Tracker: tracker}
	key, err := secrets.UnwrapKey(d.serverKey, binding, shared.Wrapped)
	if err != nil {
		return err
	}

	d.mu.RLock()
	var record []byte
	var sealed int
	var generation int64
	err = d.conn.QueryRow(`SELECT record, sealed, unlock_generation FROM user_tracker_credentials WHERE user_id = ? AND tracker = ?`, userID, tracker).Scan(&record, &sealed, &generation)
	d.mu.RUnlock()
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNoUserCredential
	case err != nil:
		return err
	case sealed == 0:
		return ErrNotSealed
	case generation != shared.Generation:
		return ErrStaleKey
	}
	if _, err := secrets.Open(key, binding, record); err != nil {
		return secrets.ErrWrongKey
	}
	d.unlocked.set(unlockKey(userID, tracker), heldKey{key: key, generation: generation})
	return nil
}

// ForgetSharedKey drops a key another instance forgot. The database is not
// touched: the instance that forgot it already moved the generation on when
// the forgetting was a lock.
func (d *DB) ForgetSharedKey(userID, tracker string) {
	d.unlocked.clear(unlockKey(strings.TrimSpace(userID), tracker))
}
