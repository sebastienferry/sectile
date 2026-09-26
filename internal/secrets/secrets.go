// Package secrets keeps credentials Sectile has to replay, such as a Jira API
// token, unreadable in a copy of the database.
//
// A credential Sectile *verifies* is stored as a hash and never read back; that
// is what internal/db does for device tokens. A credential Sectile *presents*
// to a third party cannot be hashed: the plaintext has to come back to build
// the Authorization header. The only remaining protection is encryption, and
// the whole value of it is where the key lives.
//
// Two keys exist here, for two threats.
//
// The server key lives outside the database, in an environment variable or a
// 0600 file next to it. It makes a stolen copy of the database useless on its
// own. It does not protect against someone who is root on the server: that
// person reads the key too, and the server must be able to decrypt to work at
// all. Saying otherwise would be theatre.
//
// A sealing passphrase, chosen by one person and never stored, derives a key
// that only exists while that person supplies it. A copy of the database and of
// the key file stay useless without it. The cost is exact: nothing can decrypt
// that credential while its owner is away, so no background job can use it.
//
// Both are bound to their owner. The additional authenticated data carries the
// user id and the tracker name, so moving a row from one user to another in the
// database makes it fail to open rather than handing the thief a working
// credential.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrSealed is returned when a credential needs a passphrase nobody has
// supplied yet. It is a state, not a failure: the caller says "unlock it"
// rather than "it is broken".
var ErrSealed = errors.New("credential is sealed: its owner must unlock it")

// ErrWrongKey covers both a wrong passphrase and a record that does not belong
// where it was found. The two are deliberately indistinguishable from outside:
// an attacker learns nothing from the message.
var ErrWrongKey = errors.New("credential cannot be opened with this key")

const (
	// KeyEnvVar names the server key, so a deployment can hold it in its own
	// secret manager instead of a file.
	KeyEnvVar = "SECTILE_SECRET_KEY"
	// keyFileName is where the key is generated when the environment carries
	// none. It sits beside the database on purpose: an operator who backs up
	// one without the other gets a useless copy, which is the point.
	keyFileName = "secret.key"
	// argonTime, argonMemory and argonThreads are the interactive parameters
	// recommended for Argon2id. Unlocking is a human action, so a tenth of a
	// second is affordable and a dictionary attack is not.
	argonTime    = 3
	argonMemory  = 64 * 1024
	argonThreads = 4
	keyLength    = 32
	saltLength   = 16
)

// Key is a 32-byte AEAD key. It never leaves the process in clear: a key derived
// from a passphrase that another server instance needs travels wrapped, see
// WrapKey.
type Key [keyLength]byte

// ServerKey loads the server key: the environment first, then the file beside
// the database, which is created on first use. dir is the directory holding the
// database.
func ServerKey(dir string) (Key, error) {
	var key Key
	if raw := strings.TrimSpace(os.Getenv(KeyEnvVar)); raw != "" {
		decoded, err := decodeKey(raw)
		if err != nil {
			return key, fmt.Errorf("%s is not a usable key: %w", KeyEnvVar, err)
		}
		return decoded, nil
	}
	if strings.TrimSpace(dir) == "" {
		return key, fmt.Errorf("no directory to hold the secret key, and %s is not set", KeyEnvVar)
	}
	path := filepath.Join(dir, keyFileName)
	if raw, err := os.ReadFile(path); err == nil {
		decoded, err := decodeKey(strings.TrimSpace(string(raw)))
		if err != nil {
			return key, fmt.Errorf("%s is not a usable key: %w", path, err)
		}
		return decoded, nil
	} else if !os.IsNotExist(err) {
		return key, err
	}
	if _, err := rand.Read(key[:]); err != nil {
		return key, err
	}
	// 0600 and written whole: a key readable by the rest of the machine would
	// protect nothing.
	if err := os.WriteFile(path, []byte(hex.EncodeToString(key[:])+"\n"), 0o600); err != nil {
		return key, fmt.Errorf("secret key not written to %s: %w", path, err)
	}
	return key, nil
}

func decodeKey(raw string) (Key, error) {
	var key Key
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		if decoded, err = base64.StdEncoding.DecodeString(raw); err != nil {
			return key, fmt.Errorf("expected hexadecimal or base64")
		}
	}
	if len(decoded) != keyLength {
		return key, fmt.Errorf("expected %d bytes, got %d", keyLength, len(decoded))
	}
	copy(key[:], decoded)
	return key, nil
}

// NewSalt draws the salt a sealing passphrase is stretched with. It is stored
// beside the ciphertext; a salt is not a secret, it only stops one precomputed
// table from covering every user at once.
func NewSalt() ([]byte, error) {
	salt := make([]byte, saltLength)
	_, err := rand.Read(salt)
	return salt, err
}

// DeriveKey stretches a sealing passphrase into a key. The passphrase itself is
// never stored, in any form: a hash of it would let an attacker confirm a guess
// offline, and we have nothing to gain from being able to check it.
//
// Surrounding spaces are dropped on both sides of the exchange. A phrase typed
// once with a trailing space and once without would otherwise derive two
// different keys, and the person would be told their phrase is wrong while
// looking at what they believe they typed.
func DeriveKey(passphrase string, salt []byte) Key {
	var key Key
	copy(key[:], argon2.IDKey([]byte(strings.TrimSpace(passphrase)), salt, argonTime, argonMemory, argonThreads, keyLength))
	return key
}

// Binding is what a record belongs to. It travels as additional authenticated
// data rather than as plaintext: it is not hidden, it is pinned. A ciphertext
// copied into another user's row no longer opens.
//
// Server marks the credential Sectile itself uses for a tracker, which belongs
// to no user. It serialises under a prefix of its own, so a server record never
// opens as somebody's personal one, nor the other way round.
type Binding struct {
	UserID  string
	Tracker string
	Server  bool
}

// ServerBinding is the binding of the server credential of one tracker.
func ServerBinding(tracker string) Binding {
	return Binding{Tracker: tracker, Server: true}
}

// The parts are length-prefixed rather than merely joined: concatenation alone
// lets one user id spell the boundary of the next field, so two different
// bindings could serialise the same and a record could open under the wrong
// owner. No identity in use today can do that; the prefix costs nothing and
// means the guarantee does not depend on how identities are shaped tomorrow.
func (b Binding) bytes() []byte {
	user := strings.TrimSpace(b.UserID)
	name := strings.ToLower(strings.TrimSpace(b.Tracker))
	if b.Server {
		return fmt.Appendf(nil, "sectile:v1:server:tracker:%d:%s", len(name), name)
	}
	return fmt.Appendf(nil, "sectile:v1:user:%d:%s:tracker:%d:%s", len(user), user, len(name), name)
}

// Seal encrypts a credential for one owner. The nonce is random and prepended,
// so two identical tokens never produce the same record.
func Seal(key Key, binding Binding, plaintext string) ([]byte, error) {
	if (binding.UserID == "" && !binding.Server) || binding.Tracker == "" {
		return nil, fmt.Errorf("a credential must name its owner and its tracker")
	}
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, []byte(plaintext), binding.bytes()), nil
}

// Open decrypts a credential. A wrong key, a tampered record and a record that
// belongs to somebody else all answer ErrWrongKey.
func Open(key Key, binding Binding, record []byte) (string, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return "", err
	}
	if len(record) < aead.NonceSize() {
		return "", ErrWrongKey
	}
	nonce, ciphertext := record[:aead.NonceSize()], record[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, binding.bytes())
	if err != nil {
		return "", ErrWrongKey
	}
	return string(plaintext), nil
}

// A key derived from a passphrase lives in memory only. When several server
// instances share one database, the ones that did not receive the passphrase
// need the key too, and it crosses the network between them. WrapKey seals it
// under the server key every instance holds, bound to its owner, so the network
// only ever carries ciphertext. Its associated data has a prefix of its own: a
// wrapped key never opens as a credential record, nor a record as a key.

// WrapKey seals a derived key under a wrapping key, for one owner.
func WrapKey(wrapping Key, owner Binding, key Key) ([]byte, error) {
	if owner.UserID == "" || owner.Tracker == "" {
		return nil, fmt.Errorf("a wrapped key must name its owner and its tracker")
	}
	aead, err := newAEAD(wrapping)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, key[:], wrappedKeyData(owner)), nil
}

// UnwrapKey opens what WrapKey sealed. A wrong wrapping key, another owner and
// tampered bytes all answer ErrWrongKey.
func UnwrapKey(wrapping Key, owner Binding, wrapped []byte) (Key, error) {
	var key Key
	aead, err := newAEAD(wrapping)
	if err != nil {
		return key, err
	}
	if len(wrapped) < aead.NonceSize() {
		return key, ErrWrongKey
	}
	nonce, ciphertext := wrapped[:aead.NonceSize()], wrapped[aead.NonceSize():]
	plaintext, err := aead.Open(nil, nonce, ciphertext, wrappedKeyData(owner))
	if err != nil || len(plaintext) != keyLength {
		return key, ErrWrongKey
	}
	copy(key[:], plaintext)
	return key, nil
}

func wrappedKeyData(owner Binding) []byte {
	return append([]byte("sectile:v1:unlocked-key:"), owner.bytes()...)
}

func newAEAD(key Key) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Fingerprint identifies a key without revealing it, so a log or a test can say
// "not the same key" without printing one.
func Fingerprint(key Key) string {
	sum := sha256.Sum256(key[:])
	return hex.EncodeToString(sum[:4])
}
