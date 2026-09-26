package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"tasks/internal/db"
)

// A key derived from a sealing passphrase lives in the memory of the instance
// that received the passphrase. When several instances share the database,
// this side tells the other live instances when one is held or forgotten, and
// serves the keys held here to an instance that starts. The keys travel wrapped
// under the server key, and only between instances presenting the internal
// credential. What decides whether a key may be used stays in the database:
// see internal/db/unlockedkeys.go and docs/clarifications/409.md.

// internalCredentialKeysPath serves the held keys to the other instances.
const internalCredentialKeysPath = "/internal/credentials/keys"

// credentialPushTimeout bounds each peer a change is pushed to, and each peer
// asked at start. A variable so tests can shorten it.
var credentialPushTimeout = 3 * time.Second

// credentialCluster is nil on a server that shares its store with nobody, and
// every method is then a no-op.
type credentialCluster struct {
	directory MCPDirectory
	store     *db.DB
	token     string
	tokenErr  error
	client    *http.Client
	// pending lets a test wait for the pushes a change started.
	pending sync.WaitGroup
}

// keyMessage is what one instance pushes to another.
type keyMessage struct {
	Op      string        `json:"op"`
	Key     *db.SharedKey `json:"key,omitempty"`
	UserID  string        `json:"userId,omitempty"`
	Tracker string        `json:"tracker,omitempty"`
}

const (
	keyHeld      = "held"
	keyForgotten = "forgotten"
)

// setCredentialCluster makes the store share its held keys with the other
// instances. tokenErr, when set, says why no internal credential is available:
// nothing is then shared, and the internal endpoint refuses.
func (h *Handler) setCredentialCluster(directory MCPDirectory, token string, tokenErr error) {
	if directory == nil || tokenErr != nil {
		h.credentialCluster = nil
		h.db.SetUnlockedKeyRelay(nil)
		return
	}
	c := &credentialCluster{directory: directory, store: h.db, token: token, tokenErr: tokenErr, client: &http.Client{}}
	h.credentialCluster = c
	h.db.SetUnlockedKeyRelay(c)
}

// KeyHeld pushes a key held here to every other live instance. It returns at
// once: the store may call it holding its lock.
func (c *credentialCluster) KeyHeld(shared db.SharedKey) {
	c.pushAll(keyMessage{Op: keyHeld, Key: &shared})
}

// KeyForgotten tells every other live instance to forget a key.
func (c *credentialCluster) KeyForgotten(userID, tracker string) {
	c.pushAll(keyMessage{Op: keyForgotten, UserID: userID, Tracker: tracker})
}

func (c *credentialCluster) pushAll(message keyMessage) {
	c.pending.Add(1)
	go func() {
		defer c.pending.Done()
		var wg sync.WaitGroup
		for _, peer := range c.peers() {
			wg.Add(1)
			go func(peer db.InstanceLocation) {
				defer wg.Done()
				if err := c.push(peer, message); err != nil {
					// The owner's request already succeeded, and a peer that
					// missed a lock cannot use the key anyway: the generation
					// moved on. Only who and where are logged, never the key.
					log.Printf("⚠️  Clé de %s pour %s (%s) non transmise à l'instance %s : %v",
						messageUser(message), messageTracker(message), message.Op, peer.ID, err)
				}
			}(peer)
		}
		wg.Wait()
	}()
}

func messageUser(m keyMessage) string {
	if m.Key != nil {
		return m.Key.UserID
	}
	return m.UserID
}

func messageTracker(m keyMessage) string {
	if m.Key != nil {
		return m.Key.Tracker
	}
	return m.Tracker
}

func (c *credentialCluster) peers() []db.InstanceLocation {
	self := c.directory.InstanceID()
	var out []db.InstanceLocation
	for _, instance := range c.directory.LiveInstances() {
		if instance.ID != self {
			out = append(out, instance)
		}
	}
	return out
}

func (c *credentialCluster) push(peer db.InstanceLocation, message keyMessage) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	resp, err := c.call(peer, http.MethodPost, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("answered %s", resp.Status)
	}
	return nil
}

func (c *credentialCluster) call(peer db.InstanceLocation, method string, body io.Reader) (*http.Response, error) {
	if strings.TrimSpace(peer.Address) == "" {
		return nil, fmt.Errorf("no internal address advertised")
	}
	ctx, cancel := context.WithTimeout(context.Background(), credentialPushTimeout)
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(peer.Address, "/")+internalCredentialKeysPath, body)
	if err != nil {
		cancel()
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// cancelOnClose ends a request's deadline when its body is closed, so the
// deadline covers reading the answer as well.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

// PullUnlockedKeys asks every other live instance for the keys it holds and
// keeps those that open their credential as it stands. An instance calls it
// once it is registered and its internal listener serves: a key held from then
// on is pushed to it, and one held before is pulled here. A peer that does not
// answer is logged and skipped.
func (h *Handler) PullUnlockedKeys() int {
	c := h.credentialCluster
	if c == nil {
		return 0
	}
	adopted := 0
	for _, peer := range c.peers() {
		keys, err := c.heldBy(peer)
		if err != nil {
			log.Printf("⚠️  Clés descellées de l'instance %s indisponibles : %v", peer.ID, err)
			continue
		}
		for _, key := range keys {
			if err := c.store.AdoptSharedKey(key); err != nil {
				log.Printf("Clé de %s pour %s reçue de l'instance %s écartée : %v", key.UserID, key.Tracker, peer.ID, err)
				continue
			}
			adopted++
		}
	}
	return adopted
}

func (c *credentialCluster) heldBy(peer db.InstanceLocation) ([]db.SharedKey, error) {
	resp, err := c.call(peer, http.MethodGet, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("answered %s", resp.Status)
	}
	var body struct {
		Keys []db.SharedKey `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Keys, nil
}

// InternalCredentialsHandler serves the held keys to the other instances, and
// applies the keys they hold or forget. It belongs on the internal listener,
// never on the public one.
func (h *Handler) InternalCredentialsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := h.credentialCluster
		if c == nil || c.tokenErr != nil || !internalBearerMatches(r, "Authorization", c.token) {
			writeError(w, http.StatusUnauthorized, "internal call not authorized")
			return
		}
		switch r.Method {
		case http.MethodGet:
			keys, err := h.db.HeldKeys()
			if err != nil {
				writeError(w, http.StatusServiceUnavailable, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
		case http.MethodPost:
			var message keyMessage
			if err := json.NewDecoder(r.Body).Decode(&message); err != nil {
				writeError(w, http.StatusBadRequest, "invalid internal request")
				return
			}
			switch {
			case message.Op == keyHeld && message.Key != nil:
				if err := h.db.AdoptSharedKey(*message.Key); err != nil {
					// A key the credential moved past is not a failure of the
					// sender: it raced a lock or a new record, which won.
					status := http.StatusUnprocessableEntity
					if errors.Is(err, db.ErrStaleKey) {
						status = http.StatusConflict
					}
					writeError(w, status, err.Error())
					return
				}
			case message.Op == keyForgotten:
				h.db.ForgetSharedKey(message.UserID, message.Tracker)
			default:
				writeError(w, http.StatusBadRequest, "unknown key operation")
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	})
}
