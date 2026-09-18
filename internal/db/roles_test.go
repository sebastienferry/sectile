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
