package db

import (
	"strings"
	"testing"
)

// A project's label is matched by lowerASCII on PostgreSQL as on SQLite:
// whole token, A-Z folded, an accented letter left as it is, whatever the
// server's collation (#741).
func TestPostgresMembershipFoldsCaseLikeSqlite(t *testing.T) {
	d := openPostgres(t)
	delivery := spaceProject(t, d, "Delivery", "Delivery-Admin")
	accented := spaceProject(t, d, "Equipe", "Équipe")
	importTickets(t, d, delivery.DefaultTrackerID, map[string][]string{
		"GODE-1": {"delivery-admin"},
		"GODE-2": {"DELIVERY-ADMIN"},
		"GODE-3": {"delivery-admin-v2"},
		"GODE-4": {"Équipe"},
		"GODE-5": {"équipe"},
	})

	if got := listedKeys(t, d, delivery.ID); strings.Join(got, ",") != "GODE-1,GODE-2" {
		t.Fatalf("Delivery-Admin lists %v", got)
	}
	if got := listedKeys(t, d, accented.ID); strings.Join(got, ",") != "GODE-4" {
		t.Fatalf("Équipe lists %v", got)
	}
}
