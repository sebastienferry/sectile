package db

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"tasks/internal/models"
)

func openRolesDB(t *testing.T) *DB {
	t.Helper()
	database, err := NewDB(filepath.Join(t.TempDir(), "roles.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

// The first account is the only way an admin can ever exist on a fresh server;
// every later one is a member until an admin says otherwise.
func TestFirstLocalAccountIsAdminAndLaterOnesAreMembers(t *testing.T) {
	d := openRolesDB(t)
	if d.HasLocalAccounts() {
		t.Fatal("a fresh database reports local accounts")
	}
	alice, err := d.SignInLocal("  Alice@Example.com ")
	if err != nil {
		t.Fatal(err)
	}
	if alice.Role != RoleAdmin || alice.Email != "alice@example.com" || alice.Subject != LocalSubjectPrefix+"alice@example.com" {
		t.Fatalf("first account = %+v", alice)
	}
	if alice.LastSignIn == nil {
		t.Fatal("sign-in not recorded")
	}
	if !d.HasLocalAccounts() {
		t.Fatal("local account not counted")
	}
	bob, err := d.SignInLocal("bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if bob.Role != RoleMember {
		t.Fatalf("second account role = %q, want member", bob.Role)
	}
	// Same address, other casing: same account, same role.
	again, err := d.SignInLocal("ALICE@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != alice.ID || again.Role != RoleAdmin {
		t.Fatalf("re-sign-in resolved to %+v, want %s as admin", again, alice.ID)
	}
	if _, err := d.SignInLocal("not an address"); !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("invalid address accepted: %v", err)
	}
}

// Two people signing in at the same instant on an empty board must not both
// take the admin role: the test and the write are one statement.
func TestFirstAdminIsGrantedOnlyOnce(t *testing.T) {
	d := openRolesDB(t)
	alice, err := d.SignInLocal("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := d.SignInLocal("bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	// Re-running the bootstrap rule on a board that already has one must be a
	// no-op, whoever asks.
	if err := d.ensureFirstAdmin(bob.ID); err != nil {
		t.Fatal(err)
	}
	admins, err := d.AdminCount()
	if err != nil || admins != 1 {
		t.Fatalf("AdminCount = %d, %v; want exactly one admin", admins, err)
	}
	if role := d.UserRole(alice.ID); role != RoleAdmin {
		t.Fatalf("the first account lost the role: %q", role)
	}
}

// EnsureUser creates the implicit user's row and is idempotent; it must never
// create a second row for an id that already exists.
func TestEnsureUserKeysOnTheIdNotTheSubject(t *testing.T) {
	d := openRolesDB(t)
	if err := d.EnsureUser(ImplicitUserID); err != nil {
		t.Fatal(err)
	}
	implicit, err := d.GetUser(ImplicitUserID)
	if err != nil || implicit == nil {
		t.Fatalf("implicit user = %+v, %v; want a row", implicit, err)
	}
	// The row must not hold the admin role: the implicit user's powers come
	// from its identity, and spending the bootstrap here would leave the first
	// real account a member with no admin anyone can reach.
	if implicit.Role != RoleMember {
		t.Fatalf("the implicit row holds %q and stole the bootstrap", implicit.Role)
	}
	// The implicit row is not a local account, so it does not end the mode.
	if d.HasLocalAccounts() {
		t.Fatal("the implicit row counted as a local account")
	}
	alice, _ := d.SignInLocal("alice@example.com")
	if alice.Role != RoleAdmin {
		t.Fatalf("the first real account is %q; a workstation key created before sign-in must not cost the board its admin", alice.Role)
	}
	for i := 0; i < 3; i++ {
		if err := d.EnsureUser(alice.ID); err != nil {
			t.Fatal(err)
		}
		if err := d.EnsureUser(ImplicitUserID); err != nil {
			t.Fatal(err)
		}
	}
	users, _ := d.ListUsers()
	if len(users) != 2 {
		t.Fatalf("EnsureUser created rows: %d users, want 2", len(users))
	}
	if again, _ := d.GetUser(alice.ID); again == nil || again.Email != "alice@example.com" || again.Role != RoleAdmin {
		t.Fatalf("EnsureUser overwrote an existing account: %+v", again)
	}
	if err := d.EnsureUser("  "); err == nil {
		t.Fatal("an empty id was accepted")
	}
}

func TestLastAdminCannotBeDemoted(t *testing.T) {
	d := openRolesDB(t)
	alice, _ := d.SignInLocal("alice@example.com")
	bob, _ := d.SignInLocal("bob@example.com")
	if _, err := d.SetUserRole(alice.ID, RoleMember); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demoting the last admin: %v", err)
	}
	if _, err := d.SetUserRole(bob.ID, RoleAdmin); err != nil {
		t.Fatal(err)
	}
	demoted, err := d.SetUserRole(alice.ID, RoleMember)
	if err != nil || demoted.Role != RoleMember {
		t.Fatalf("demotion with another admin present: %+v, %v", demoted, err)
	}
	if _, err := d.SetUserRole(bob.ID, "owner"); err == nil {
		t.Fatal("unknown role accepted")
	}
	users, err := d.ListUsers()
	if err != nil || len(users) != 2 || users[0].ID != bob.ID {
		t.Fatalf("ListUsers = %+v, %v; want bob (admin) first", users, err)
	}
}

// The provider's claim is the authority when it speaks: it overrides a manual
// promotion at the next sign-in. Without a claim the stored role stands and the
// bootstrap rule applies.
func TestProviderClaimOverridesStoredRole(t *testing.T) {
	d := openRolesDB(t)
	carol, err := d.SignInProvider("https://idp|1", "carol@example.com", "Carol", RoleMember, true)
	if err != nil {
		t.Fatal(err)
	}
	if carol.Role != RoleMember {
		t.Fatalf("claim said member, stored %q", carol.Role)
	}
	if _, err := d.SetUserRole(carol.ID, RoleAdmin); err != nil {
		t.Fatal(err)
	}
	carol, _ = d.SignInProvider("https://idp|1", "carol@example.com", "Carol", RoleMember, true)
	if carol.Role != RoleMember {
		t.Fatalf("manual promotion survived a claimed sign-in: %q", carol.Role)
	}
	// No claim configured: the first-admin rule applies to the next person.
	dave, err := d.SignInProvider("https://idp|2", "dave@example.com", "Dave", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if dave.Role != RoleAdmin {
		t.Fatalf("first admin rule without a claim gave %q", dave.Role)
	}
}

// Rows written before ownership existed keep an empty owner; new ones carry it
// on every read path, with the owner's name resolved from the users table.
func TestActivityOwnerRoundTrips(t *testing.T) {
	d := openRolesDB(t)
	owner, _ := d.SignInLocal("alice@example.com")
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Owned run", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	run, err := d.StartRemoteRunBy(owner.ID, task.ID, "implement", "")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := d.StartRemoteRun(task.ID, "clarify", "")
	if err != nil {
		t.Fatal(err)
	}
	byID, _ := d.GetActivityByID(run.ID)
	if byID.UserID != owner.ID || byID.UserName != "alice@example.com" {
		t.Fatalf("GetActivityByID owner = %q / %q", byID.UserID, byID.UserName)
	}
	legacyByID, _ := d.GetActivityByID(legacy.ID)
	if legacyByID.UserID != "" || legacyByID.UserName != "" {
		t.Fatalf("ownerless run reads %q / %q", legacyByID.UserID, legacyByID.UserName)
	}
	perTask, _ := d.GetTaskActivities(task.ID)
	all, _ := d.GetActivities("", "", "", task.ID, "", 10)
	for name, list := range map[string][]models.TaskActivity{"GetTaskActivities": perTask, "GetActivities": all} {
		found := false
		for _, a := range list {
			if a.ID == run.ID {
				found = true
				if a.UserID != owner.ID || a.UserName != "alice@example.com" {
					t.Fatalf("%s owner = %q / %q", name, a.UserID, a.UserName)
				}
			}
		}
		if !found {
			t.Fatalf("%s does not list the run", name)
		}
	}
	// An agent reporting a run it owns stamps the owner on a record that has none.
	now := time.Now()
	adopted, err := d.SyncRemoteRunStatusFor(owner.ID, legacy.ID, task.ID, "default", task.Key, "clarify", "running", "Recovered", &now)
	if err != nil {
		t.Fatal(err)
	}
	if adopted.UserID != owner.ID {
		t.Fatalf("legacy run did not adopt its agent's user: %q", adopted.UserID)
	}
	inserted, err := d.SyncRemoteRunStatusFor(owner.ID, "run-new", task.ID, "default", task.Key, "clarify", "queued", "", nil)
	if err != nil || inserted.UserID != owner.ID {
		t.Fatalf("inserted run owner = %q, %v", inserted.UserID, err)
	}
}

// Closing a run is reporting its outcome, which is the run's own business: a
// colleague doing it hands the workflow back and leaves the real process
// running on the owner's machine.
func TestFinishRunRefusesAColleaguesExecution(t *testing.T) {
	d := openRolesDB(t)
	owner, _ := d.SignInLocal("alice@example.com")
	other, _ := d.SignInLocal("bob@example.com")
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Owned", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	run, err := d.StartRemoteRunBy(owner.ID, task.ID, "implement", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.FinishRemoteRunAs(Actor{ID: other.ID}, false, task.ID, run.ID, "completed", "not mine"); !errors.Is(err, ErrRunNotYours) {
		t.Fatalf("a member closed another user's run: %v", err)
	}
	if still, _ := d.GetActivityByID(run.ID); still.Status != "running" {
		t.Fatalf("the refused close changed the run to %q", still.Status)
	}
	// The owner closes their own run, and an admin closes anyone's.
	if _, err := d.FinishRemoteRunAs(Actor{ID: owner.ID}, false, task.ID, run.ID, "completed", "done"); err != nil {
		t.Fatalf("the owner could not close their run: %v", err)
	}
	second, _ := d.StartRemoteRunBy(owner.ID, task.ID, "review", "")
	if _, err := d.FinishRemoteRunAs(Actor{ID: other.ID}, true, task.ID, second.ID, "canceled", "admin"); err != nil {
		t.Fatalf("an admin could not close a run: %v", err)
	}
	// A run written before ownership existed belongs to nobody and stays
	// closable, which is what keeps an upgrade from stranding live runs.
	legacy, _ := d.StartRemoteRun(task.ID, "clarify", "")
	if _, err := d.FinishRemoteRunAs(Actor{ID: other.ID}, false, task.ID, legacy.ID, "completed", "legacy"); err != nil {
		t.Fatalf("an ownerless run was refused: %v", err)
	}
}

func TestTransitionAndCommentRecordTheActor(t *testing.T) {
	d := openRolesDB(t)
	alice, _ := d.SignInLocal("alice@example.com")
	task, err := d.CreateTask(models.CreateTaskRequest{ProjectID: "default", Title: "Attributed", Status: models.StatusToClarify})
	if err != nil {
		t.Fatal(err)
	}
	_, act, err := d.TransitionTaskStageBy(alice.ID, task.ID, "clarified", "done", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if act.UserID != alice.ID {
		t.Fatalf("transition activity owner = %q", act.UserID)
	}
	comments, err := d.PostTaskCommentBy(Actor{ID: alice.ID, Name: alice.Name()}, task.ID, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].UserID != alice.ID || comments[0].Author != "alice@example.com" {
		t.Fatalf("comment = %+v", comments)
	}
}
