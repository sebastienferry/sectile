package db

import (
	"strconv"
	"strings"
)

// rebindNumbered rewrites the shared "?" placeholders as PostgreSQL's $1, $2...
//
// It has to skip anything inside a string literal: several queries carry a
// literal default such as '[]' or '{}', and one carrying a question mark would
// otherwise shift every placeholder after it by one, producing a query that
// still runs and binds the wrong arguments. Single quotes, double-quoted
// identifiers and the doubled-quote escape SQL uses inside both are handled;
// dollar-quoted bodies are not, because no query here uses one.
func rebindNumbered(query string) string {
	var out strings.Builder
	out.Grow(len(query) + 8)
	n := 0
	for i := 0; i < len(query); i++ {
		c := query[i]
		switch c {
		case '\'', '"':
			// Copy the literal or quoted identifier whole. Inside it, the quote
			// character doubled is an escaped quote, not the end.
			quote := c
			out.WriteByte(c)
			i++
			for i < len(query) {
				if query[i] == quote {
					if i+1 < len(query) && query[i+1] == quote {
						out.WriteByte(query[i])
						out.WriteByte(query[i+1])
						i += 2
						continue
					}
					out.WriteByte(query[i])
					break
				}
				out.WriteByte(query[i])
				i++
			}
		case '?':
			n++
			out.WriteByte('$')
			out.WriteString(strconv.Itoa(n))
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// ddlTypes are the schema's type names that PostgreSQL spells differently.
var ddlTypes = strings.NewReplacer(
	" DATETIME", " TIMESTAMPTZ",
	" BLOB", " BYTEA",
)

// rewriteDDLTypes adapts the type names of a schema statement.
//
// It only touches statements that begin with CREATE or ALTER. That guard is
// what makes a blunt textual replacement safe: no DML statement starts with
// either word, so the replacer can never reach a value, a column name or a
// string literal in a query. Within DDL, "DATETIME" and "BLOB" preceded by a
// space are always the type of the column just named.
func rewriteDDLTypes(stmt string) string {
	head := strings.ToUpper(strings.TrimLeft(stmt, " \t\n\r"))
	if !strings.HasPrefix(head, "CREATE") && !strings.HasPrefix(head, "ALTER") {
		return stmt
	}
	return ddlTypes.Replace(stmt)
}
