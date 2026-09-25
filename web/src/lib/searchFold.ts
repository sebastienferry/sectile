/**
 * Free-text search in the browser, folded the way the server folds it on
 * PostgreSQL: case and diacritics do not matter, so `equipe`, `Equipe` and
 * `ÉQUIPE` all find `Équipe` (#447). A view that filters in memory must agree
 * with the board, which asks the server.
 */

/** Folds text for free-text search: lower case, diacritics stripped. */
export const foldForSearch = (text: string | null | undefined): string =>
  (text || '').normalize('NFD').replace(/\p{M}/gu, '').toLowerCase()

/**
 * True when `query`, folded and trimmed, occurs in any of `fields`, folded. A
 * blank query matches everything; a missing field matches nothing.
 */
export const matchesSearch = (query: string, ...fields: Array<string | null | undefined>): boolean => {
  const q = foldForSearch(query.trim())
  if (!q) return true
  return fields.some(field => !!field && foldForSearch(field).includes(q))
}
