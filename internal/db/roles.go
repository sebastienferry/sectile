package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Two roles. An admin manages users, projects, global settings, tracker
// credentials, anyone's workstations and anyone's execution; a member does
// everything else. A third role is a decision for the day someone needs one.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// LocalSubjectPrefix marks an account created by the local e-mail sign-in, the
// temporary mode a deployment runs in until an identity provider is connected.
const LocalSubjectPrefix = "local|"

// ErrLastAdmin refuses the change that would leave the board with nobody able
// to administer it.
var ErrLastAdmin = errors.New("the board would have no admin left")

// ErrInvalidEmail refuses a local sign-in that does not name an address.
var ErrInvalidEmail = errors.New("a valid e-mail address is required")

// Actor is who performs an action, as the storage layer needs it: the id to
// record and the name to show where a row is displayed without a join.
type Actor struct {
	ID   string
	Name string
}

// NormalizeRole folds any stored or requested value onto the two roles. An
// unknown value is a member: the safe reading of a bad row is the lesser role.
func NormalizeRole(role string) string {
	if strings.EqualFold(strings.TrimSpace(role), RoleAdmin) {
		return RoleAdmin
	}
	return RoleMember
}

// ValidRole reports whether the value names one of the two roles exactly.
func ValidRole(role string) bool {
	role = strings.TrimSpace(role)
	return role == RoleAdmin || role == RoleMember
}

// NormalizeLocalEmail is the identity of a local account: the address, trimmed
// and lower-cased, so the same person typing it differently signs into the same
// account rather than creating a second one.
func NormalizeLocalEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.Index(email, "@")
	if at < 1 || at == len(email)-1 || strings.ContainsAny(email, " \t\r\n") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

// AdminCount is how many admins exist. Zero is the bootstrap state: the next
// account to sign in takes the role.
func (d *DB) AdminCount() (int, error) {
	var count int
	err := d.conn.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ?`, RoleAdmin).Scan(&count)
	return count, err
}

// HasLocalAccounts reports whether the local e-mail sign-in has created at
// least one account. It is what ends the implicit user: once someone has an
// account, an anonymous request is a stranger.
func (d *DB) HasLocalAccounts() bool {
	var count int
	if err := d.conn.QueryRow(`SELECT COUNT(*) FROM users WHERE subject LIKE ?`, LocalSubjectPrefix+"%").Scan(&count); err != nil {
		return false
	}
	return count > 0
}

// UserRole is the stored role of a user, or an empty string for an unknown id.
func (d *DB) UserRole(id string) string {
	user, err := d.GetUser(id)
	if err != nil || user == nil {
		return ""
	}
	return user.Role
}

// ListUsers returns every account, admins first, then by name.
func (d *DB) ListUsers() ([]User, error) {
	rows, err := d.conn.Query(`SELECT ` + userColumns + ` FROM users ORDER BY CASE role WHEN 'admin' THEN 0 ELSE 1 END, LOWER(display_name), LOWER(email), id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	users := []User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *user)
	}
	return users, rows.Err()
}

// SetUserRole is the admin's change of another user's role, or of their own.
// The last admin cannot be demoted: the board would have nobody left to undo
// it.
func (d *DB) SetUserRole(id, role string) (*User, error) {
	if !ValidRole(role) {
		return nil, fmt.Errorf("role must be %s or %s", RoleAdmin, RoleMember)
	}
	user, err := d.GetUser(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, sql.ErrNoRows
	}
	if user.Role == RoleAdmin && role == RoleMember {
		admins, err := d.AdminCount()
		if err != nil {
			return nil, err
		}
		if admins <= 1 {
			return nil, ErrLastAdmin
		}
	}
	if err := d.setUserRole(id, role); err != nil {
		return nil, err
	}
	return d.GetUser(id)
}

// setUserRole writes the role without the last-admin check, for the provider's
// claim, which is the authority when it speaks, and for the bootstrap rule.
func (d *DB) setUserRole(id, role string) error {
	_, err := d.conn.Exec(`UPDATE users SET role = ? WHERE id = ?`, NormalizeRole(role), id)
	return err
}

// ensureFirstAdmin gives the user the admin role when no admin exists yet.
// Without it the roles could never be assigned: every account arrives through
// sign-in as an equal.
func (d *DB) ensureFirstAdmin(userID string) error {
	admins, err := d.AdminCount()
	if err != nil {
		return err
	}
	if admins > 0 {
		return nil
	}
	return d.setUserRole(userID, RoleAdmin)
}

// SignInLocal is the local e-mail sign-in: an unknown address creates the
// account, a known one signs into it, and the first account ever created is
// admin. It identifies without authenticating, which is the mode's declared
// limit, so it must only be offered while no identity provider is configured.
func (d *DB) SignInLocal(email string) (*User, error) {
	email, err := NormalizeLocalEmail(email)
	if err != nil {
		return nil, err
	}
	id, err := d.UpsertUser(LocalSubjectPrefix+email, email, email)
	if err != nil {
		return nil, err
	}
	if err := d.ensureFirstAdmin(id); err != nil {
		return nil, err
	}
	return d.recordSignIn(id)
}

// SignInProvider records a sign-in through the identity provider. When the
// provider supplied a role it is written as is, an admin's manual change
// included: the claim is the authority. Otherwise the stored role stands and
// only the bootstrap rule applies.
func (d *DB) SignInProvider(subject, email, displayName, role string, roleFromClaim bool) (*User, error) {
	id, err := d.UpsertUser(subject, email, displayName)
	if err != nil {
		return nil, err
	}
	if roleFromClaim {
		if err := d.setUserRole(id, role); err != nil {
			return nil, err
		}
	} else if err := d.ensureFirstAdmin(id); err != nil {
		return nil, err
	}
	return d.recordSignIn(id)
}

func (d *DB) recordSignIn(id string) (*User, error) {
	if _, err := d.conn.Exec(`UPDATE users SET last_sign_in = ? WHERE id = ?`, time.Now().UTC(), id); err != nil {
		return nil, err
	}
	return d.GetUser(id)
}
