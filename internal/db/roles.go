package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Two roles. An admin owns the roster, who holds an account, what role it
// carries and whether it still opens, plus anyone's workstations and anyone's
// execution; a member does everything else, the board and its projects
// included (ADR 0018). A third role is a decision for the day someone needs
// one.
const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// LocalSubjectPrefix marks an account created by the local e-mail sign-in, the
// temporary mode a deployment runs in until an identity provider is connected.
const LocalSubjectPrefix = "local|"

// ImplicitUserID owns what runs without an HTTP request: the local agent's own
// operations and the launches started outside a browser session. Since sign-in
// became mandatory (ADR 0015) it is an ordinary account like any other, spelled
// here as well as in the handlers because the storage layer has to know which
// row may be created without a sign-in.
const ImplicitUserID = "default"

// ErrLastAdmin refuses the change that would leave the board with nobody able
// to administer it: a demotion, a block and a deletion all reach it.
var ErrLastAdmin = errors.New("the board would have no admin left")

// ErrImplicitUser refuses a deletion or a block of the implicit account. It is
// not a person: it owns everything that runs without a session, and removing it
// would orphan the local agent's own work rather than close anybody's access.
var ErrImplicitUser = errors.New("the implicit account is not a person")

// ErrAccountBlocked refuses an account an admin has closed. It is returned at
// the sign-in rather than at the first action, so the person is told once, at
// the door, instead of finding every page refusing them for no stated reason.
var ErrAccountBlocked = errors.New("this account is blocked")

// ErrInvalidEmail refuses a local sign-in that does not name an address.
var ErrInvalidEmail = errors.New("a valid e-mail address is required")

// MaxDisplayNameLength bounds the name somebody chooses for themselves. It is
// counted in runes, so an accented name is not shorter than an unaccented one.
const MaxDisplayNameLength = 80

// ErrDisplayNameTooLong and ErrDisplayNameInvalid refuse what is not a name.
// They are distinct because the person can act on each: shorten it, or remove
// what does not belong in a name. The wording shown to that person belongs to
// the handler, which is the layer that answers them.
var (
	ErrDisplayNameTooLong = errors.New("display name is longer than the ceiling")
	ErrDisplayNameInvalid = errors.New("display name carries a control character")
)

// NormalizeDisplayName is the name as it is stored: trimmed, free of control
// characters, and no longer than the ceiling. The empty string is valid and
// means "no choice": the account goes back to the name its sign-in supplies.
func NormalizeDisplayName(name string) (string, error) {
	name = strings.TrimSpace(name)
	for _, r := range name {
		// A name spanning two lines is not a name: it is something that would
		// break the row it is shown in. A line break is a control character, so
		// the one test covers both.
		if unicode.IsControl(r) {
			return "", ErrDisplayNameInvalid
		}
	}
	if utf8.RuneCountInString(name) > MaxDisplayNameLength {
		return "", ErrDisplayNameTooLong
	}
	return name, nil
}

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

// AdminCount is how many admins can actually administer the board. A blocked
// admin is not one of them: their account does not open, so counting them would
// let the last working admin be demoted or blocked behind a row that can no
// longer undo it. Zero is the bootstrap state: the next account to sign in
// takes the role.
func (d *DB) AdminCount() (int, error) {
	var count int
	err := d.conn.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ? AND blocked_at IS NULL`, RoleAdmin).Scan(&count)
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

// SetUserBlocked closes an account or opens it again. A blocked account keeps
// everything it owns: its rows, its past executions and the history that names
// it stay exactly where they are, which is the whole difference with a deletion.
// Only the sign-in stops answering.
//
// The last admin cannot be blocked, for the reason a demotion cannot: the board
// would be left with nobody able to undo it.
func (d *DB) SetUserBlocked(id string, blocked bool) (*User, error) {
	user, err := d.GetUser(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, sql.ErrNoRows
	}
	if blocked && id == ImplicitUserID {
		return nil, ErrImplicitUser
	}
	if blocked && user.Role == RoleAdmin {
		admins, err := d.AdminCount()
		if err != nil {
			return nil, err
		}
		if admins <= 1 {
			return nil, ErrLastAdmin
		}
	}
	if blocked {
		if _, err := d.conn.Exec(`UPDATE users SET blocked_at = ? WHERE id = ?`, time.Now().UTC(), id); err != nil {
			return nil, err
		}
		// The block has to reach the sessions already open, otherwise it only
		// takes effect whenever the person next signs in, which is exactly when
		// they would not.
		if err := d.RevokeUserSessions(id); err != nil {
			return nil, err
		}
	} else if _, err := d.conn.Exec(`UPDATE users SET blocked_at = NULL WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return d.GetUser(id)
}

// DeleteUser removes an account and everything that authenticates it: its
// sessions, its workstation keys and any pairing code in flight. What it does
// not remove is the work: tasks, comments and executions record a user id
// without a foreign key, so they survive their author and read as an execution
// without an owner, which is a state the board already knows how to display.
//
// The last admin cannot be deleted, and neither can the implicit user, which is
// not a person but the owner of everything that runs without a session.
func (d *DB) DeleteUser(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return sql.ErrNoRows
	}
	if id == ImplicitUserID {
		return ErrImplicitUser
	}
	user, err := d.GetUser(id)
	if err != nil {
		return err
	}
	if user == nil {
		return sql.ErrNoRows
	}
	if user.Role == RoleAdmin {
		admins, err := d.AdminCount()
		if err != nil {
			return err
		}
		if admins <= 1 {
			return ErrLastAdmin
		}
	}
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, statement := range []string{
		`DELETE FROM web_sessions WHERE user_id = ?`,
		`DELETE FROM pairing_codes WHERE user_id = ?`,
		`DELETE FROM device_credentials WHERE user_id = ?`,
		`DELETE FROM user_settings WHERE user_id = ?`,
		`DELETE FROM users WHERE id = ?`,
	} {
		if _, err := tx.Exec(statement, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetDisplayName records the name its owner chose. An empty value clears the
// choice and hands the account back to the spelling its sign-in supplies. The
// name is never an identifier, so it is not required to be unique.
func (d *DB) SetDisplayName(id, name string) (*User, error) {
	clean, err := NormalizeDisplayName(name)
	if err != nil {
		return nil, err
	}
	user, err := d.GetUser(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, sql.ErrNoRows
	}
	if _, err := d.conn.Exec(`UPDATE users SET chosen_name = ? WHERE id = ?`, clean, id); err != nil {
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
//
// The test and the write are one statement on purpose: two people signing in at
// the same instant on an empty board would otherwise both read "no admin" and
// both take the role.
func (d *DB) ensureFirstAdmin(userID string) error {
	_, err := d.conn.Exec(`UPDATE users SET role = ? WHERE id = ?
		AND NOT EXISTS (SELECT 1 FROM users WHERE role = ? AND blocked_at IS NULL)`, RoleAdmin, userID, RoleAdmin)
	return err
}

// EnsureUser makes sure a row exists for a user id that no sign-in created: the
// implicit user of a deployment without accounts, which still has to satisfy
// the foreign keys carried by API keys and pairing codes.
//
// It keys on the id, never on the subject. UpsertUser takes a provider subject,
// so handing it a user id mints a second account whose subject is the first
// one's id, an inert row that then shows up in the users view.
func (d *DB) EnsureUser(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("user id is required")
	}
	// The row is written as a member even for the implicit user, whose powers
	// come from being the implicit user and not from a stored role. Writing it
	// as an admin would spend the bootstrap: the first person to sign in would
	// then be a member, and nobody could reach the admin role again, since the
	// implicit user stops resolving as soon as an account exists.
	_, err := d.conn.Exec(`INSERT INTO users (id, subject, email, display_name, role)
		SELECT ?, ?, '', '', ? WHERE NOT EXISTS (SELECT 1 FROM users WHERE id = ?)`, id, id, RoleMember, id)
	return err
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
	if err := d.refuseBlocked(id); err != nil {
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
	// The provider authenticated them; the block is this board's own decision
	// and outranks it, claim or no claim.
	if err := d.refuseBlocked(id); err != nil {
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

// refuseBlocked stops a sign-in on a closed account, after the upsert so the
// e-mail and display name the provider supplies stay current on the row.
func (d *DB) refuseBlocked(id string) error {
	user, err := d.GetUser(id)
	if err != nil {
		return err
	}
	if user != nil && user.Blocked {
		return ErrAccountBlocked
	}
	return nil
}

func (d *DB) recordSignIn(id string) (*User, error) {
	if _, err := d.conn.Exec(`UPDATE users SET last_sign_in = ? WHERE id = ?`, time.Now().UTC(), id); err != nil {
		return nil, err
	}
	return d.GetUser(id)
}
