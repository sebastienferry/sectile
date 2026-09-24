package db

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// uniqueIndexColumns names, for each unique index the code reacts to, the
// columns SQLite quotes in its error. SQLite reports the columns, never the
// index, so the index name alone cannot be matched there.
var uniqueIndexColumns = map[string]string{
	activeRunIndex:              "task_activities.task_id",
	"idx_board_views_user_name": "board_views.user_id, board_views.name_key",
}

// isUniqueViolation reports whether err is a write refused by the named unique
// index. It never matches another constraint, a primary key collision above
// all: an agent chooses its own run ids, and a repeated id is not a busy task.
func isUniqueViolation(err error, index string) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505" && pgErr.ConstraintName == index
	}
	columns, ok := uniqueIndexColumns[index]
	if !ok {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed: "+columns)
}
