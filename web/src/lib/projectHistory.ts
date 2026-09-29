/**
 * The projects opened in this browser, most recent first (#582). It feeds the
 * picker's "Recent" section and orders the projects that are not favorites. It
 * is a per-browser convenience, never sent to the server: a lost history only
 * empties "Recent".
 */

/** One opening: which project, and when (ISO 8601). */
export interface ProjectOpening {
  id: string
  openedAt: string
}

export const PROJECT_HISTORY_KEY = 'sectile_recent_project_ids'

/**
 * Enough to order the projects a search lists beyond the first few, not only
 * the three recents: favorites are skipped when "Recent" is built.
 */
export const PROJECT_HISTORY_LIMIT = 50

const isValidDate = (value: unknown): value is string =>
  typeof value === 'string' && !Number.isNaN(new Date(value).getTime())

/**
 * Reads the stored value. Anything that is not an array of openings yields an
 * empty history, and a malformed entry is dropped rather than failing the rest.
 */
export function parseProjectHistory(raw: string | null): ProjectOpening[] {
  if (!raw) return []
  let value: unknown
  try {
    value = JSON.parse(raw)
  } catch {
    return []
  }
  if (!Array.isArray(value)) return []
  const seen = new Set<string>()
  const history: ProjectOpening[] = []
  for (const entry of value) {
    if (!entry || typeof entry !== 'object') continue
    const { id, openedAt } = entry as Record<string, unknown>
    if (typeof id !== 'string' || !id || seen.has(id) || !isValidDate(openedAt)) continue
    seen.add(id)
    history.push({ id, openedAt })
  }
  return history.slice(0, PROJECT_HISTORY_LIMIT)
}

/** Moves `id` to the front, opened at `now`, and keeps the history bounded. */
export function recordProjectOpening(history: ProjectOpening[], id: string, now: Date): ProjectOpening[] {
  return [{ id, openedAt: now.toISOString() }, ...history.filter(entry => entry.id !== id)]
    .slice(0, PROJECT_HISTORY_LIMIT)
}

/** The stored history; empty when storage is unavailable. */
export function readProjectHistory(): ProjectOpening[] {
  try {
    return parseProjectHistory(localStorage.getItem(PROJECT_HISTORY_KEY))
  } catch {
    return []
  }
}

/**
 * Stores the history. A storage that refuses the write leaves the caller's
 * in-memory state as the only copy, which lasts until the page is reloaded.
 */
export function writeProjectHistory(history: ProjectOpening[]): void {
  try {
    localStorage.setItem(PROJECT_HISTORY_KEY, JSON.stringify(history))
  } catch {
    // Private mode or blocked site data: the session keeps its own copy.
  }
}
