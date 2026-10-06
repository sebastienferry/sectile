package db

import (
	"errors"
	"sync"
	"testing"
)

func TestPaddedRoleCannotDemoteLastAdmin(t *testing.T) {
	d := testDB(t)
	admin, err := d.SignInLocal("admin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetUserRole(admin.ID, " member "); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("padded role bypassed last-admin protection: %v", err)
	}
}

func TestBlockedAdminCanBeRemovedWithoutAffectingLastActiveAdmin(t *testing.T) {
	for _, action := range []string{"demote", "delete"} {
		t.Run(action, func(t *testing.T) {
			d := testDB(t)
			active, err := d.SignInLocal("active@example.com")
			if err != nil {
				t.Fatal(err)
			}
			blocked, err := d.SignInLocal("blocked@example.com")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.SetUserRole(blocked.ID, RoleAdmin); err != nil {
				t.Fatal(err)
			}
			if _, err := d.SetUserBlocked(blocked.ID, true); err != nil {
				t.Fatal(err)
			}
			if action == "demote" {
				_, err = d.SetUserRole(blocked.ID, RoleMember)
			} else {
				err = d.DeleteUser(blocked.ID)
			}
			if err != nil {
				t.Fatalf("removing inactive admin was refused: %v", err)
			}
			if d.UserRole(active.ID) != RoleAdmin {
				t.Fatal("active admin was changed")
			}
		})
	}
}

func TestConcurrentAdminChangesLeaveOneActiveAdmin(t *testing.T) {
	for _, action := range []string{"demote", "block", "delete"} {
		t.Run(action, func(t *testing.T) {
			d := testDB(t)
			other, err := NewDB(d.cfg.Path)
			if err != nil {
				t.Fatal(err)
			}
			defer other.Close()
			for attempt := range 20 {
				first, err := d.SignInLocal("first@example.com")
				if err != nil {
					t.Fatal(err)
				}
				second, err := d.SignInLocal("second@example.com")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := d.SetUserRole(first.ID, RoleAdmin); err != nil {
					t.Fatal(err)
				}
				if _, err := d.SetUserRole(second.ID, RoleAdmin); err != nil {
					t.Fatal(err)
				}
				start := make(chan struct{})
				results := make(chan error, 2)
				var callers sync.WaitGroup
				change := func(store *DB, id string) {
					defer callers.Done()
					<-start
					var err error
					switch action {
					case "demote":
						_, err = store.SetUserRole(id, RoleMember)
					case "block":
						_, err = store.SetUserBlocked(id, true)
					case "delete":
						err = store.DeleteUser(id)
					}
					results <- err
				}
				callers.Add(2)
				go change(d, first.ID)
				go change(other, second.ID)
				close(start)
				callers.Wait()
				one, two := <-results, <-results
				if !((one == nil && errors.Is(two, ErrLastAdmin)) || (two == nil && errors.Is(one, ErrLastAdmin))) {
					t.Fatalf("attempt %d: want one success and one last-admin refusal, got %v / %v", attempt, one, two)
				}
				if count, err := d.AdminCount(); err != nil || count != 1 {
					t.Fatalf("active admins: %d, %v", count, err)
				}
				if action == "block" {
					if _, err := d.SetUserBlocked(first.ID, false); err != nil {
						t.Fatal(err)
					}
					if _, err := d.SetUserBlocked(second.ID, false); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
