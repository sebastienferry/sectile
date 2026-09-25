/**
 * Saved board views (#387).
 *
 * A view is a named, personal selection of projects and labels laid over the
 * all-projects board. The server applies it (`viewId` on the task list and its
 * facets); what the interface decides is only where the board filters are
 * remembered, how the address carries the open view, and what a ticket created
 * from a view starts with.
 */

import type { BoardView } from '../types'

/** The address parameter that names the open view. */
export const VIEW_PARAM = 'view'

/**
 * The key the board filters are remembered under, on this browser. A view has
 * its own, so leaving it for a project and coming back restores each side's
 * filters without carrying one over to the other.
 */
export const filterScopeKey = (projectId: string | null | undefined, viewId: string | null | undefined): string =>
  viewId ? `view_${viewId}` : (projectId || 'all')

/** The view the address names, or null. */
export const readViewParam = (search: string): string | null => {
  const value = new URLSearchParams(search).get(VIEW_PARAM)
  return value && value.trim() ? value.trim() : null
}

/**
 * The address with the open view written in, or removed when `viewId` is null.
 * Every other parameter, a deep-linked `task` for one, is left as it was.
 */
export const withViewParam = (href: string, viewId: string | null): string => {
  const url = new URL(href)
  if (viewId) url.searchParams.set(VIEW_PARAM, viewId)
  else url.searchParams.delete(VIEW_PARAM)
  return url.pathname + url.search + url.hash
}

/**
 * Labels a ticket created from the view starts with. Only a single-label view
 * is unambiguous: its label is what makes the ticket appear in it. With
 * several labels, picking one would be a guess, and with none there is nothing
 * to add.
 */
export const initialLabelsForView = (view: BoardView | null | undefined): string[] =>
  view && view.labels.length === 1 ? [view.labels[0]] : []

/**
 * The fold that decides whether two labels are the same one. A-Z only, because
 * that is all a view's selection folds: PostgreSQL's LOWER follows the
 * cluster's collation and SQLite's leaves accents alone, so the query compares
 * `Équipe` and `équipe` as two distinct labels on every engine
 * (docs/adrs/0025). `toLowerCase` here would drop one of them from the form
 * while the board still selects on both.
 */
export const foldViewLabel = (label: string): string => label.replace(/[A-Z]/g, c => c.toLowerCase())

/**
 * Trimmed labels without empties, one entry per spelling that differs only by
 * ASCII case, the first one kept. The server normalizes the same way; doing it
 * here keeps the chips of the form in line with what will be saved.
 */
export const normalizeViewLabels = (labels: string[]): string[] => {
  const seen = new Set<string>()
  const out: string[] = []
  for (const raw of labels) {
    const label = raw.trim()
    const key = foldViewLabel(label)
    if (!label || seen.has(key)) continue
    seen.add(key)
    out.push(label)
  }
  return out
}

/**
 * Why the form cannot be saved yet, or null. The server stays the authority,
 * this only spares a round trip for what the form already knows.
 */
export const boardViewFormError = (
  name: string,
  projectIds: string[],
  others: Pick<BoardView, 'id' | 'name'>[],
  editingId: string | null,
  repository = '',
): 'name' | 'duplicate' | 'projects' | 'repository' | null => {
  const key = name.trim().toLowerCase()
  if (!key) return 'name'
  if (others.some(v => v.id !== editingId && v.name.trim().toLowerCase() === key)) return 'duplicate'
  if (projectIds.length === 0) return 'projects'
  if (!isViewRepository(repository)) return 'repository'
  return null
}

/**
 * A view's repository is optional; when given, it is a Git remote naming a
 * host and a path, in URL or scp-like form (#429). The server decides; this
 * only spares a round trip.
 */
export const isViewRepository = (value: string): boolean => {
  const repository = value.trim()
  if (!repository) return true
  if (repository.length > 500 || /\s/.test(repository)) return false
  return /^[a-z][a-z0-9+.-]*:\/\/[^/]+\/[^/].*$/i.test(repository) || /^([^@/:]+@)?[^:/]+:[^/].*$/.test(repository)
}
